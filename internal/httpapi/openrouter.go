package httpapi

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"github.com/z-chenhao/ad-agent/internal/store"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func (s *Server) startOpenRouter(w http.ResponseWriter, r *http.Request, auth loginSession) {
	if !s.lockSettings(r.Context()) {
		writeError(w, 409, "workspace_busy_retry_when_idle")
		return
	}
	defer s.appMu.Unlock()
	var random [32]byte
	if _, err := rand.Read(random[:]); err != nil {
		writeError(w, 500, "oauth_random_failed")
		return
	}
	verifier := base64.RawURLEncoding.EncodeToString(random[:])
	hash := sha256.Sum256([]byte(verifier))
	state := store.ID("oauth")
	for id, value := range s.oauthAttempts {
		if time.Now().After(value.Until) {
			delete(s.oauthAttempts, id)
		}
	}
	if len(s.oauthAttempts) >= 8 {
		writeError(w, 429, "oauth_attempt_limit")
		return
	}
	s.oauthAttempts[state] = oauthAttempt{Verifier: verifier, Owner: auth.CSRF, Until: time.Now().Add(10 * time.Minute)}
	callback := s.Origin + "/model-auth/callback?state=" + url.QueryEscape(state)
	query := url.Values{"callback_url": {callback}, "code_challenge": {base64.RawURLEncoding.EncodeToString(hash[:])}, "code_challenge_method": {"S256"}}
	writeJSON(w, 200, struct {
		URL string `json:"url"`
	}{"https://openrouter.ai/auth?" + query.Encode()})
}

func (s *Server) completeOpenRouter(w http.ResponseWriter, r *http.Request, auth loginSession) {
	var input struct {
		Code  string `json:"code"`
		State string `json:"state"`
	}
	if !readJSON(w, r, &input) {
		return
	}
	s.appMu.Lock()
	attempt, ok := s.oauthAttempts[input.State]
	if !ok || attempt.Owner != auth.CSRF || !attempt.Until.After(time.Now()) || input.Code == "" || len(input.Code) > 4096 {
		s.appMu.Unlock()
		writeError(w, 400, "oauth_state_invalid")
		return
	}
	delete(s.oauthAttempts, input.State)
	s.appMu.Unlock()
	body, _ := json.Marshal(struct {
		Code     string `json:"code"`
		Verifier string `json:"code_verifier"`
		Method   string `json:"code_challenge_method"`
	}{input.Code, attempt.Verifier, "S256"})
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, "https://openrouter.ai/api/v1/auth/keys", bytes.NewReader(body))
	if err != nil {
		writeError(w, 500, "oauth_exchange_failed")
		return
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(req)
	if err != nil {
		writeError(w, 502, "oauth_exchange_failed")
		return
	}
	defer response.Body.Close()
	var result struct {
		Key string `json:"key"`
	}
	if response.StatusCode != 200 || json.NewDecoder(io.LimitReader(response.Body, 16000)).Decode(&result) != nil || result.Key == "" || len(result.Key) > 8192 || strings.ContainsAny(result.Key, "\r\n\x00") {
		writeError(w, 502, "oauth_exchange_failed")
		return
	}
	s.appMu.Lock()
	s.openRouterKey = result.Key
	s.appMu.Unlock()
	writeJSON(w, 200, struct {
		Ready bool `json:"ready"`
	}{true})
}
