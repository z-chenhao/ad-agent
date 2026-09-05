package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiagnosticsCorrelateSSEWithoutCredentialsOrBodies(t *testing.T) {
	s, ts, client, key := setup(t)
	code, body := request(t, client, "POST", ts.URL+"/api/v1/login", ts.URL, "", map[string]string{"key": key})
	if code != 200 {
		t.Fatalf("login %d", code)
	}
	var auth struct {
		CSRF string `json:"csrf"`
	}
	if err := json.Unmarshal(body, &auth); err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest("POST", ts.URL+"/api/v1/agent/turn?secret=sentinel-query", strings.NewReader(`{"session_id":"trace-test","message":"sentinel-prompt"}`))
	req.Header.Set("Origin", ts.URL)
	req.Header.Set("X-CSRF-Token", auth.CSRF)
	req.Header.Set("Content-Type", "application/json")
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(res.Body)
	res.Body.Close()
	if err != nil || res.StatusCode != 200 || !strings.Contains(string(data), "turn.completed") {
		t.Fatalf("SSE status=%d body=%s err=%v", res.StatusCode, data, err)
	}
	id := res.Header.Get("X-Request-ID")
	if id == "" {
		t.Fatal("missing request ID")
	}
	for _, name := range []string{"server", "agent-trace"} {
		path := filepath.Join(s.App.Store.Dir, "logs", name+".jsonl")
		log, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(log), id) {
			t.Fatalf("missing correlation in %s", name)
		}
		if name == "agent-trace" {
			for _, line := range strings.Split(strings.TrimSpace(string(log)), "\n") {
				var event struct {
					RequestID string `json:"request_id"`
					Type      string `json:"type"`
				}
				if err := json.Unmarshal([]byte(line), &event); err != nil || event.RequestID != id {
					t.Fatalf("lost correlation for %s: %v", event.Type, err)
				}
			}
		}
		for _, secret := range []string{key, auth.CSRF, "sentinel-query", "sentinel-prompt"} {
			if strings.Contains(string(log), secret) {
				t.Fatalf("sensitive data in %s", name)
			}
		}
		info, _ := os.Stat(path)
		if info.Mode().Perm() != 0600 {
			t.Fatal("diagnostic file is not private")
		}
	}
}
