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
	h.Emit(ar.Event{Type: "text.delta", Text: "sandbox account read"})
	result := h.Execute(ctx, ar.Call{ID: "get", Name: "get_advertiser_context", Arguments: json.RawMessage(`{}`), Round: 1})
	if !result.OK {
		return ar.Result{}, context.Canceled
	}
	return ar.Result{Text: "sandbox account read", Stop: "stop"}, nil
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

func setupManager(t *testing.T) (*Server, *httptest.Server, *http.Client, string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	a, err := app.OpenManagerSandboxRuntime(dir, "web_manager", testRuntime{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.Store.Close() })
	ts := httptest.NewUnstartedServer(nil)
	origin := "http://" + ts.Listener.Addr().String()
	server, err := NewManager(a, origin, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ts.Config.Handler = server.Handler()
	ts.Start()
	t.Cleanup(ts.Close)
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	key, err := os.ReadFile(filepath.Join(dir, "operator-key"))
	if err != nil {
		t.Fatal(err)
	}
	return server, ts, client, string(key)
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
	if code != 200 || !strings.Contains(string(b), `"runtime":"pi"`) || !strings.Contains(string(b), `"gpt-5.6-luna"`) || !strings.Contains(string(b), `"writes":true`) || !strings.Contains(string(b), `"sandbox":true`) {
		t.Fatal("runtime config missing", code, string(b))
	}
	code, b = request(t, c, "GET", ts.URL+"/api/v1/ads/ad_prospect_creator/detail", "", "", nil)
	if code != 200 || !strings.Contains(string(b), `"name":"Vintage shelf styling · room tour"`) || !strings.Contains(string(b), `Collected books, warm wood, and objects`) {
		t.Fatal("sandbox ad detail missing", code, string(b))
	}
	code, b = request(t, c, "GET", ts.URL+"/api/v1/sandbox", "", "", nil)
	if code != 200 || !strings.Contains(string(b), `"granularity":"hour"`) || !strings.Contains(string(b), `"seed_end":"2026-09-03"`) {
		t.Fatal("sandbox clock missing", code, string(b))
	}
	code, _ = request(t, c, "POST", ts.URL+"/api/v1/sandbox/advance", ts.URL, "", map[string]int{"hours": 1})
	if code != 403 {
		t.Fatal("sandbox advance accepted without CSRF", code)
	}
	code, b = request(t, c, "POST", ts.URL+"/api/v1/sandbox/advance", ts.URL, auth.CSRF, map[string]int{"hours": 1})
	if code != 200 || !strings.Contains(string(b), `"advanced_by_hours":1`) || !strings.Contains(string(b), `"facts_created":12`) {
		t.Fatal("sandbox did not advance", code, string(b))
	}
	for _, hours := range []int{0, 745} {
		code, _ = request(t, c, "POST", ts.URL+"/api/v1/sandbox/advance", ts.URL, auth.CSRF, map[string]int{"hours": hours})
		if code != 400 {
			t.Fatalf("sandbox accepted invalid advance %d: %d", hours, code)
		}
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
	code, _ = request(t, c, "POST", ts.URL+"/api/v1/agent/turn", ts.URL, auth.CSRF, map[string]any{
		"session_id": "web", "message": "hi", "model": map[string]string{"provider": "deepseek", "model": "chat", "reasoning": "medium"},
	})
	if code != 400 {
		t.Fatal("unsupported model accepted", code)
	}
	code, _ = request(t, c, "POST", ts.URL+"/api/v1/agent/turn", ts.URL, auth.CSRF, map[string]any{
		"session_id": "web", "message": "hi", "view_context": map[string]string{"page": "admin"},
	})
	if code != http.StatusBadRequest {
		t.Fatal("invalid view context accepted", code)
	}
	code, b = request(t, c, "POST", ts.URL+"/api/v1/agent/turn", ts.URL, auth.CSRF, map[string]string{"session_id": "web", "message": "read sandbox account"})
	if code != 200 || !strings.Contains(string(b), "turn.completed") || !strings.Contains(string(b), "text.delta") {
		t.Fatal("SSE missing lifecycle", code, string(b))
	}
	code, b = request(t, c, "GET", ts.URL+"/api/v1/session?session_id=web", "", "", nil)
	if code != 200 || strings.Contains(string(b), "checkpoint") || strings.Contains(string(b), key) || !strings.Contains(string(b), `"model":"gpt-5.6-luna"`) {
		t.Fatal("private checkpoint exposed")
	}
	code, _ = request(t, c, "GET", ts.URL+"/prompts/ad-agent-system.md", "", "", nil)
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

func TestManagerSurfaceIsScopedAndAuthenticated(t *testing.T) {
	_, ts, client, key := setupManager(t)
	code, _ := request(t, client, "GET", ts.URL+"/api/v1/manager", "", "", nil)
	if code != http.StatusUnauthorized {
		t.Fatal("unauthenticated manager access", code)
	}
	code, body := request(t, client, "POST", ts.URL+"/api/v1/login", ts.URL, "", map[string]string{"key": key})
	if code != http.StatusOK {
		t.Fatal("manager login failed", code, string(body))
	}
	var auth struct {
		CSRF string `json:"csrf"`
	}
	if err := json.Unmarshal(body, &auth); err != nil {
		t.Fatal(err)
	}
	code, body = request(t, client, "GET", ts.URL+"/api/v1/config", "", "", nil)
	if code != http.StatusOK || !strings.Contains(string(body), `"mode":"manager"`) || strings.Contains(string(body), "agency") {
		t.Fatal("manager config missing or mislabeled", code, string(body))
	}
	code, body = request(t, client, "GET", ts.URL+"/api/v1/manager", "", "", nil)
	if code != http.StatusOK || strings.Count(string(body), `"account_id"`) < 3 || !strings.Contains(string(body), "Northstar Apps") {
		t.Fatal("manager accounts missing", code, string(body))
	}
	var scope struct {
		Accounts []struct {
			ID string `json:"id"`
		} `json:"accounts"`
	}
	if err := json.Unmarshal(body, &scope); err != nil || len(scope.Accounts) == 0 {
		t.Fatalf("manager scope decode failed: %#v err=%v", scope, err)
	}
	accountID := scope.Accounts[0].ID
	code, body = request(t, client, "GET", ts.URL+"/api/v1/manager/accounts/"+accountID+"/entities/campaign", "", "", nil)
	if code != http.StatusOK || !strings.Contains(string(body), `"level":"campaign"`) {
		t.Fatal("manager campaign drill-down missing", code, string(body))
	}
	code, body = request(t, client, "GET", ts.URL+"/api/v1/manager/accounts/"+accountID+"/report?level=campaign&start_date=2026-08-28&end_date=2026-09-03", "", "", nil)
	if code != http.StatusOK || !strings.Contains(string(body), `"calculation"`) {
		t.Fatal("manager account report missing", code, string(body))
	}
	code, _ = request(t, client, "GET", ts.URL+"/api/v1/manager/accounts/outside/entities/campaign", "", "", nil)
	if code != http.StatusBadRequest {
		t.Fatal("outside manager account accepted", code)
	}
	code, body = request(t, client, "GET", ts.URL+"/api/v1/manager/report?start_date=2026-08-28&end_date=2026-09-03", "", "", nil)
	if code != http.StatusOK || !strings.Contains(string(body), "no cross-currency total") {
		t.Fatal("manager report semantics missing", code, string(body))
	}
	code, _ = request(t, client, "POST", ts.URL+"/api/v1/changes/not-real/apply", ts.URL, auth.CSRF, map[string]string{"session_id": "web", "operator": "attacker"})
	if code != http.StatusBadRequest {
		t.Fatal("manager identity override accepted", code)
	}
	code, _ = request(t, client, "GET", ts.URL+"/api/v1/advertisers/current", "", "", nil)
	if code != http.StatusNotFound {
		t.Fatal("advertiser route exposed in manager mode", code)
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
		"/prompts/ad-agent-system.md",
		"/assets/../../prompts/ad-agent-system.md",
		"/assets/%2e%2e/%2e%2e/prompts/ad-agent-system.md",
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
