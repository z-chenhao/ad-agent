package httpapi

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/z-chenhao/ad-agent/internal/store"
)

func TestSettingsKeepConversationButNeverRebindItsAdSource(t *testing.T) {
	s, ts, client, key := setup(t)
	ctx := context.Background()
	_, raw := request(t, client, "POST", ts.URL+"/api/v1/login", ts.URL, "", map[string]string{"key": key})
	var auth struct {
		CSRF string `json:"csrf"`
	}
	json.Unmarshal(raw, &auth)
	account, _ := s.App.Backend.Account(ctx)
	saved := store.Session{ID: "keep-conversation", Source: account.Source, Runtime: "pi", Model: s.settings.Model, Checkpoint: "private-pi", Messages: []store.Message{{Role: "user", Text: "Keep campaign budgets unchanged", TurnID: "original", Status: "completed"}}}
	if err := s.App.Store.SaveSession(ctx, saved); err != nil {
		t.Fatal(err)
	}
	value := s.settings
	value.Runtime = "custom"
	value.Model.Model = "gpt-5.4-mini"
	input := map[string]any{"settings": value, "session_id": saved.ID}
	if err := s.App.Store.Lease(ctx, saved.ID, "other-process", time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	code, _ := request(t, client, "POST", ts.URL+"/api/v1/settings", ts.URL, auth.CSRF, input)
	s.App.Store.Release(saved.ID, "other-process")
	if code != 409 {
		t.Fatal("settings changed during a CLI turn", code)
	}
	code, raw = request(t, client, "POST", ts.URL+"/api/v1/settings", ts.URL, auth.CSRF, input)
	var response struct {
		SessionID string `json:"session_id"`
	}
	json.Unmarshal(raw, &response)
	if code != 200 || response.SessionID != saved.ID {
		t.Fatal("runtime/model change replaced business session", code, string(raw))
	}
	stored, _ := s.App.Store.Session(ctx, saved.ID, account.Source)
	if stored.Checkpoint != "private-pi" || len(stored.Messages) != 1 {
		t.Fatal("settings rewrote history before execution")
	}
	// The next turn adopts the selection under its session lease; no provider is called here.
	value.Backend.Environment = "isolated-settings-test"
	code, raw = request(t, client, "POST", ts.URL+"/api/v1/settings", ts.URL, auth.CSRF, map[string]any{"settings": value, "session_id": saved.ID})
	json.Unmarshal(raw, &response)
	if code != 200 || response.SessionID == saved.ID {
		t.Fatal("ad source change reused old authority", code, string(raw))
	}
	stored, err := s.App.Store.Session(ctx, saved.ID, account.Source)
	if err != nil || len(stored.Messages) != 1 {
		t.Fatal("old source history was deleted", err)
	}
	code, _ = request(t, client, "POST", ts.URL+"/api/v1/settings", ts.URL, auth.CSRF, map[string]any{"settings": value, "session_id": saved.ID})
	if code == 200 {
		t.Fatal("foreign source conversation accepted")
	}
}

func TestManagerSettingsKeepConversation(t *testing.T) {
	s, ts, client, key := setupManager(t)
	ctx := context.Background()
	_, raw := request(t, client, "POST", ts.URL+"/api/v1/login", ts.URL, "", map[string]string{"key": key})
	var auth struct {
		CSRF string `json:"csrf"`
	}
	json.Unmarshal(raw, &auth)
	session := store.Session{ID: "manager-kept", Source: s.Manager.Scope.Source(), Runtime: "custom", Model: s.settings.Model, Messages: []store.Message{{Role: "user", Text: "Review accounts separately", TurnID: "manager-original", Status: "completed"}}}
	if err := s.Manager.Store.SaveSession(ctx, session); err != nil {
		t.Fatal(err)
	}
	value := s.settings
	value.Model.Model = "gpt-5.4-mini"
	code, raw := request(t, client, "POST", ts.URL+"/api/v1/settings", ts.URL, auth.CSRF, map[string]any{"settings": value, "session_id": session.ID})
	var response struct {
		SessionID string `json:"session_id"`
	}
	json.Unmarshal(raw, &response)
	if code != 200 || response.SessionID != session.ID {
		t.Fatal(code, string(raw))
	}
	stored, err := s.Manager.Store.Session(ctx, session.ID, session.Source)
	if err != nil || len(stored.Messages) != 1 {
		t.Fatal("manager history lost", err)
	}
}
