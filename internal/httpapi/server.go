// Package httpapi exposes the authenticated loopback application, never the public OAuth callback.
package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/z-chenhao/ad-agent/internal/ads"
	"github.com/z-chenhao/ad-agent/internal/agenthost"
	"github.com/z-chenhao/ad-agent/internal/app"
	"github.com/z-chenhao/ad-agent/internal/store"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type loginSession struct {
	CSRF  string
	Until time.Time
}
type Server struct {
	App       *app.App
	Origin    string
	DevOrigin string
	WebDir    string
	keyHash   [32]byte
	mu        sync.Mutex
	sessions  map[string]loginSession
	attempts  int
	window    time.Time
}

// New keeps the operator key in a permission-restricted local file, never a URL or stdout.
func New(a *app.App, origin, webDir string) (*Server, error) {
	u, err := url.Parse(origin)
	if err != nil || u.Scheme != "http" || !loopback(u.Hostname()) || u.Path != "" {
		return nil, errors.New("application origin must be loopback HTTP")
	}
	path := filepath.Join(a.Store.Dir, "operator-key")
	b, err := loadOrCreateOperatorKey(path)
	if err != nil {
		return nil, err
	}
	if len(b) < 32 {
		return nil, errors.New("invalid operator key")
	}
	return &Server{App: a, Origin: origin, WebDir: webDir, keyHash: sha256.Sum256(b), sessions: map[string]loginSession{}}, nil
}

func loadOrCreateOperatorKey(path string) ([]byte, error) {
	for attempt := 0; attempt < 2; attempt++ {
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			var random [32]byte
			if _, err = rand.Read(random[:]); err != nil {
				return nil, errors.New("generate operator key")
			}
			key := []byte("operator_" + hex.EncodeToString(random[:]))
			f, createErr := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
			if os.IsExist(createErr) {
				continue
			}
			if createErr != nil {
				return nil, createErr
			}
			_, writeErr := f.Write(key)
			if writeErr == nil {
				writeErr = f.Sync()
			}
			closeErr := f.Close()
			if writeErr != nil {
				_ = os.Remove(path)
				return nil, errors.New("write operator key")
			}
			if closeErr != nil {
				return nil, errors.New("close operator key")
			}
			return key, nil
		}
		if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
			return nil, errors.New("operator key must be a regular file with mode 0600")
		}
		f, openErr := os.Open(path)
		if openErr != nil {
			return nil, openErr
		}
		opened, statErr := f.Stat()
		if statErr != nil || !opened.Mode().IsRegular() || opened.Mode().Perm() != 0600 {
			f.Close()
			return nil, errors.New("operator key changed during open")
		}
		key, readErr := io.ReadAll(io.LimitReader(f, 257))
		closeErr := f.Close()
		if readErr != nil || closeErr != nil {
			return nil, errors.New("read operator key")
		}
		if len(key) < 32 || len(key) > 256 {
			return nil, errors.New("invalid operator key")
		}
		return key, nil
	}
	return nil, errors.New("operator key creation raced")
}
func loopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/login", s.login)
	mux.HandleFunc("GET /api/v1/health/live", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, struct {
			Status string `json:"status"`
		}{"ok"})
	})
	mux.HandleFunc("GET /api/v1/auth", s.authorize(func(w http.ResponseWriter, r *http.Request, auth loginSession) {
		writeJSON(w, 200, struct {
			CSRF string `json:"csrf"`
		}{auth.CSRF})
	}))
	mux.HandleFunc("GET /api/v1/config", s.authorize(func(w http.ResponseWriter, r *http.Request, _ loginSession) {
		writeJSON(w, 200, struct {
			Runtime string                  `json:"runtime"`
			Harness agenthost.PublicHarness `json:"harness"`
		}{s.App.Runtime, s.App.Host.PublicHarness()})
	}))
	mux.HandleFunc("POST /api/v1/logout", s.authorize(func(w http.ResponseWriter, r *http.Request, _ loginSession) {
		c, _ := r.Cookie("ad_session")
		s.mu.Lock()
		delete(s.sessions, c.Value)
		s.mu.Unlock()
		http.SetCookie(w, &http.Cookie{Name: "ad_session", Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteStrictMode})
		writeJSON(w, 200, struct {
			OK bool `json:"ok"`
		}{true})
	}))
	mux.HandleFunc("GET /api/v1/advertisers/current", s.authorize(func(w http.ResponseWriter, r *http.Request, _ loginSession) {
		v, e := s.App.Backend.Account(r.Context())
		respond(w, v, e)
	}))
	mux.HandleFunc("GET /api/v1/entities/{level}", s.authorize(func(w http.ResponseWriter, r *http.Request, _ loginSession) {
		v, e := s.App.Backend.List(r.Context(), ads.EntityQuery{Level: ads.Level(r.PathValue("level")), ParentID: r.URL.Query().Get("parent_id")})
		respond(w, v, e)
	}))
	mux.HandleFunc("GET /api/v1/report", s.authorize(func(w http.ResponseWriter, r *http.Request, _ loginSession) {
		q := r.URL.Query()
		v, e := s.App.Backend.Report(r.Context(), ads.ReportQuery{Level: ads.Level(q.Get("level")), Start: q.Get("start_date"), End: q.Get("end_date"), EntityID: q.Get("entity_id")})
		if e != nil {
			respond(w, nil, e)
			return
		}
		c, calcErr := ads.Analyze(v)
		var calculation *ads.Calculation
		if calcErr == nil {
			calculation = &c
		}
		writeJSON(w, 200, struct {
			Report      ads.Report       `json:"report"`
			Calculation *ads.Calculation `json:"calculation"`
		}{v, calculation})
	}))
	mux.HandleFunc("GET /api/v1/session", s.authorize(func(w http.ResponseWriter, r *http.Request, _ loginSession) {
		session, e := s.session(r)
		respond(w, session, e)
	}))
	mux.HandleFunc("GET /api/v1/turns/{id}/events", s.authorize(s.events))
	mux.HandleFunc("GET /api/v1/changes", s.authorize(func(w http.ResponseWriter, r *http.Request, _ loginSession) {
		session, e := s.session(r)
		if e != nil {
			respond(w, nil, e)
			return
		}
		v, e := s.App.Store.Changes(r.Context(), session.ID)
		respond(w, v, e)
	}))
	mux.HandleFunc("GET /api/v1/memories", s.authorize(func(w http.ResponseWriter, r *http.Request, _ loginSession) {
		account, err := s.App.Backend.Account(r.Context())
		if err != nil {
			respond(w, nil, err)
			return
		}
		memories, err := s.App.Store.Memories(r.Context(), account.Source, 50)
		respond(w, memories, err)
	}))
	mux.HandleFunc("POST /api/v1/changes/{id}/{action}", s.authorize(s.change))
	mux.HandleFunc("POST /api/v1/agent/turn", s.authorize(s.turn))
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) { writeError(w, 404, "not_found") })
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" && r.Method != "HEAD" {
			writeError(w, 405, "method_not_allowed")
			return
		}
		s.static(w, r)
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		u, _ := url.Parse(s.Origin)
		if r.Host != u.Host {
			writeError(w, 403, "invalid_host")
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" {
			origin := r.Header.Get("Origin")
			if origin != s.Origin && (s.DevOrigin == "" || origin != s.DevOrigin) {
				writeError(w, 403, "invalid_origin")
				return
			}
		}
		mux.ServeHTTP(w, r)
	})
}
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	if time.Since(s.window) > time.Minute {
		s.window = time.Now()
		s.attempts = 0
	}
	s.attempts++
	blocked := s.attempts > 10
	s.mu.Unlock()
	if blocked {
		writeError(w, 429, "login_rate_limited")
		return
	}
	var p struct {
		Key string `json:"key"`
	}
	if !readJSON(w, r, &p) {
		return
	}
	hash := sha256.Sum256([]byte(strings.TrimSpace(p.Key)))
	if subtle.ConstantTimeCompare(hash[:], s.keyHash[:]) != 1 {
		writeError(w, 401, "invalid_operator_key")
		return
	}
	token := store.ID("session")
	auth := loginSession{CSRF: store.ID("csrf"), Until: time.Now().Add(12 * time.Hour)}
	s.mu.Lock()
	for id, old := range s.sessions {
		if time.Now().After(old.Until) {
			delete(s.sessions, id)
		}
	}
	if len(s.sessions) >= 32 {
		s.mu.Unlock()
		writeError(w, 429, "session_limit")
		return
	}
	s.sessions[token] = auth
	s.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: "ad_session", Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: 43200})
	writeJSON(w, 200, struct {
		CSRF string `json:"csrf"`
	}{auth.CSRF})
}
func (s *Server) authorize(next func(http.ResponseWriter, *http.Request, loginSession)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, e := r.Cookie("ad_session")
		if e != nil {
			writeError(w, 401, "login_required")
			return
		}
		s.mu.Lock()
		auth, ok := s.sessions[c.Value]
		s.mu.Unlock()
		if !ok || time.Now().After(auth.Until) {
			writeError(w, 401, "login_required")
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" && subtle.ConstantTimeCompare([]byte(r.Header.Get("X-CSRF-Token")), []byte(auth.CSRF)) != 1 {
			writeError(w, 403, "csrf_failed")
			return
		}
		next(w, r, auth)
	}
}
func (s *Server) session(r *http.Request) (store.Session, error) {
	id := r.URL.Query().Get("session_id")
	if id == "" {
		id = "web"
	}
	if !validSession(id) {
		return store.Session{}, errors.New("invalid_session")
	}
	a, e := s.App.Backend.Account(r.Context())
	if e != nil {
		return store.Session{}, e
	}
	return s.App.Store.Session(r.Context(), id, a.Source)
}
func validSession(s string) bool {
	if len(s) < 1 || len(s) > 100 {
		return false
	}
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-') {
			return false
		}
	}
	return true
}
func (s *Server) turn(w http.ResponseWriter, r *http.Request, _ loginSession) {
	var p struct {
		Message   string `json:"message"`
		SessionID string `json:"session_id"`
	}
	if !readJSON(w, r, &p) {
		return
	}
	if !validSession(p.SessionID) || len(p.Message) == 0 || len(p.Message) > 16000 {
		writeError(w, 400, "invalid_turn")
		return
	}
	f, ok := w.(http.Flusher)
	if !ok {
		writeError(w, 500, "stream_unavailable")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("X-Accel-Buffering", "no")
	f.Flush()
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	_, err := s.App.Host.Run(ctx, p.SessionID, p.Message, func(e store.Event) {
		b, _ := json.Marshal(e)
		if _, writeErr := fmt.Fprintf(w, "id: %d\nevent: message\ndata: %s\n\n", e.Seq, b); writeErr != nil {
			cancel()
			return
		}
		f.Flush()
	})
	if err != nil && ctx.Err() == nil {
		fmt.Fprint(w, "event: error\ndata: {\"error\":\"turn_failed\"}\n\n")
		f.Flush()
	}
}
func (s *Server) events(w http.ResponseWriter, r *http.Request, _ loginSession) {
	session, e := s.session(r)
	if e != nil {
		respond(w, nil, e)
		return
	}
	id := r.PathValue("id")
	allowed := false
	for _, m := range session.Messages {
		if m.TurnID == id {
			allowed = true
			break
		}
	}
	if !allowed {
		writeError(w, 404, "turn_not_found")
		return
	}
	v, e := s.App.Store.Events(r.Context(), id, 0)
	respond(w, v, e)
}
func (s *Server) change(w http.ResponseWriter, r *http.Request, _ loginSession) {
	var p struct {
		SessionID string `json:"session_id"`
	}
	if !readJSON(w, r, &p) {
		return
	}
	if !validSession(p.SessionID) {
		writeError(w, 400, "invalid_session")
		return
	}
	var c ads.Change
	var err error
	switch r.PathValue("action") {
	case "apply":
		c, err = s.App.Changes.Apply(r.Context(), p.SessionID, r.PathValue("id"), "local-web-operator")
	case "discard":
		c, err = s.App.Changes.Discard(r.Context(), p.SessionID, r.PathValue("id"))
	case "reconcile":
		c, err = s.App.Changes.Reconcile(r.Context(), p.SessionID, r.PathValue("id"))
	default:
		writeError(w, 404, "not_found")
		return
	}
	respond(w, c, err)
}
func (s *Server) static(w http.ResponseWriter, r *http.Request) {
	clean := path.Clean("/" + strings.TrimPrefix(r.URL.Path, "/"))
	if clean == "/" {
		clean = "/index.html"
	}
	if clean != "/index.html" && !strings.HasPrefix(clean, "/assets/") && !strings.HasPrefix(clean, "/favicon") {
		http.NotFound(w, r)
		return
	}
	rel := filepath.FromSlash(strings.TrimPrefix(clean, "/"))
	file := filepath.Join(s.WebDir, rel)
	within, err := filepath.Rel(s.WebDir, file)
	if err != nil || within == ".." || strings.HasPrefix(within, ".."+string(filepath.Separator)) {
		http.NotFound(w, r)
		return
	}
	info, err := os.Stat(file)
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, file)
}
func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if r.Header.Get("Content-Type") != "application/json" {
		writeError(w, 415, "json_required")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 20000)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if d.Decode(v) != nil {
		writeError(w, 400, "invalid_json")
		return false
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		writeError(w, 400, "invalid_json")
		return false
	}
	return true
}
func respond(w http.ResponseWriter, v any, err error) {
	if err != nil {
		writeError(w, 400, "request_rejected")
		return
	}
	writeJSON(w, 200, v)
}
func writeError(w http.ResponseWriter, code int, reason string) {
	writeJSON(w, code, struct {
		Error string `json:"error"`
	}{reason})
}
func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
