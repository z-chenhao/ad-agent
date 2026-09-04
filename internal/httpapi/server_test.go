package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/z-chenhao/ad-agent/internal/app"
	ar "github.com/z-chenhao/ad-agent/internal/runtime"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type testRuntime struct{}

func (testRuntime) Run(ctx context.Context, r ar.Request, h ar.Hooks) (ar.Result, error) {
	h.Emit(ar.Event{Type: "text.delta", Text: "fixture account read"})
	result := h.Execute(ctx, ar.Call{ID: "get", Name: "get_advertiser_context", Arguments: json.RawMessage(`{}`), Round: 1})
	if !result.OK {
		return ar.Result{}, context.Canceled
	}
	return ar.Result{Text: "fixture account read", Stop: "stop"}, nil
}
func setup(t *testing.T) (*Server, *httptest.Server, *http.Client, string) {
	t.Helper()
	dir := t.TempDir()
	os.Chmod(dir, 0700)
	a, e := app.Open(t.TempDir(), dir)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { a.Store.Close() })
	a.Host.Runtime = testRuntime{}
	a.Host.AutomaticMemoryCapture = false
	ts := httptest.NewUnstartedServer(nil)
	origin := "http://" + ts.Listener.Addr().String()
	s, e := New(a, origin, t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	ts.Config.Handler = s.Handler()
	ts.Start()
	t.Cleanup(ts.Close)
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	b, e := os.ReadFile(filepath.Join(dir, "operator-key"))
	if e != nil {
		t.Fatal(e)
	}
	return s, ts, client, string(b)
}
func request(t *testing.T, c *http.Client, method, url, origin, csrf string, body any) (int, []byte) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reader = bytes.NewReader(b)
	}
	req, e := http.NewRequest(method, url, reader)
	if e != nil {
		t.Fatal(e)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	if csrf != "" {
		req.Header.Set("X-CSRF-Token", csrf)
	}
	resp, e := c.Do(req)
	if e != nil {
		t.Fatal(e)
	}
	defer resp.Body.Close()
	b, e := io.ReadAll(resp.Body)
	if e != nil {
		t.Fatal(e)
	}
	return resp.StatusCode, b
}
func TestAuthOriginCSRFAndPublicSurface(t *testing.T) {
	_, ts, c, key := setup(t)
	code, _ := request(t, c, "GET", ts.URL+"/api/v1/advertisers/current", "", "", nil)
	if code != 401 {
		t.Fatal("unauthenticated access", code)
	}
	code, _ = request(t, c, "POST", ts.URL+"/api/v1/login", "https://evil.example", "", map[string]string{"key": key})
	if code != 403 {
		t.Fatal("cross-origin login", code)
	}
	code, b := request(t, c, "POST", ts.URL+"/api/v1/login", ts.URL, "", map[string]string{"key": key})
	if code != 200 {
		t.Fatal("login failed", code)
	}
	var auth struct {
		CSRF string `json:"csrf"`
	}
	json.Unmarshal(b, &auth)
	code, b = request(t, c, "GET", ts.URL+"/api/v1/config", "", "", nil)
	if code != 200 || !strings.Contains(string(b), `"runtime":"pi"`) {
		t.Fatal("runtime config missing", code, string(b))
	}
	code, b = request(t, c, "GET", ts.URL+"/api/v1/advertisers/current", "", "", nil)
	if code != 200 || strings.Contains(string(b), key) {
		t.Fatal("account or secret leak", code)
	}
	code, _ = request(t, c, "POST", ts.URL+"/api/v1/agent/turn", ts.URL, "", map[string]string{"session_id": "web", "message": "hi"})
	if code != 403 {
		t.Fatal("missing CSRF accepted", code)
	}
	code, _ = request(t, c, "POST", ts.URL+"/api/v1/agent/turn", ts.URL, auth.CSRF, map[string]string{"session_id": "web", "message": "hi", "operator": "admin"})
	if code != 400 {
		t.Fatal("identity override", code)
	}
	code, b = request(t, c, "POST", ts.URL+"/api/v1/agent/turn", ts.URL, auth.CSRF, map[string]string{"session_id": "web", "message": "read fixture account"})
	if code != 200 || !strings.Contains(string(b), "turn.completed") || !strings.Contains(string(b), "text.delta") {
		t.Fatal("SSE missing lifecycle", code, string(b))
	}
	code, b = request(t, c, "GET", ts.URL+"/api/v1/session?session_id=web", "", "", nil)
	if code != 200 || strings.Contains(string(b), "checkpoint") || strings.Contains(string(b), key) {
		t.Fatal("private checkpoint exposed")
	}
	code, _ = request(t, c, "GET", ts.URL+"/AGENT.md", "", "", nil)
	if code != 404 {
		t.Fatal("source file served")
	}
	req, _ := http.NewRequest("GET", ts.URL+"/api/v1/health/live", nil)
	req.Host = "evil.example"
	resp, e := c.Do(req)
	if e != nil {
		t.Fatal(e)
	}
	resp.Body.Close()
	if resp.StatusCode != 403 {
		t.Fatal("DNS rebinding host accepted")
	}
}
func TestTurnReplayIsSessionScoped(t *testing.T) {
	s, ts, c, key := setup(t)
	_, b := request(t, c, "POST", ts.URL+"/api/v1/login", ts.URL, "", map[string]string{"key": key})
	var auth struct {
		CSRF string `json:"csrf"`
	}
	json.Unmarshal(b, &auth)
	request(t, c, "POST", ts.URL+"/api/v1/agent/turn", ts.URL, auth.CSRF, map[string]string{"session_id": "one", "message": "hi"})
	account, _ := s.App.Backend.Account(context.Background())
	session, _ := s.App.Store.Session(context.Background(), "one", account.Source)
	turn := session.Messages[0].TurnID
	code, b := request(t, c, "GET", ts.URL+"/api/v1/turns/"+turn+"/events?session_id=one", "", "", nil)
	if code != 200 || !strings.Contains(string(b), "turn.completed") {
		t.Fatal("replay failed")
	}
	code, _ = request(t, c, "GET", ts.URL+"/api/v1/turns/"+turn+"/events?session_id=two", "", "", nil)
	if code != 404 {
		t.Fatal("cross-session replay")
	}
}

func TestOperatorKeyRejectsSymlinkAndLooseMode(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte(strings.Repeat("x", 64)), 0600); err != nil {
		t.Fatal(err)
	}
	linked := filepath.Join(dir, "linked")
	if err := os.Symlink(target, linked); err != nil {
		t.Fatal(err)
	}
	if _, err := loadOrCreateOperatorKey(linked); err == nil {
		t.Fatal("operator key symlink accepted")
	}
	loose := filepath.Join(dir, "loose")
	if err := os.WriteFile(loose, []byte(strings.Repeat("x", 64)), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadOrCreateOperatorKey(loose); err == nil {
		t.Fatal("loose operator key accepted")
	}
}

func TestStaticPathCannotEscapeWebRoot(t *testing.T) {
	_, ts, client, _ := setup(t)
	for _, candidate := range []string{
		"/AGENT.md",
		"/assets/../../AGENT.md",
		"/assets/%2e%2e/%2e%2e/AGENT.md",
		"/not-an-asset.txt",
	} {
		resp, err := client.Get(ts.URL + candidate)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			t.Fatalf("static path escaped: %s", candidate)
		}
	}
}
