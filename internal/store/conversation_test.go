package store

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	ar "github.com/z-chenhao/ad-agent/internal/runtime"
)

func TestExecutionSelectionInvalidatesOnlyIncompatibleCheckpoint(t *testing.T) {
	model := ar.DefaultModelSelection()
	session := Session{ID: "same", Runtime: "pi", Model: model, Checkpoint: "private-pi", Messages: []Message{{Text: "retained", Status: "completed"}}}
	if changed, err := session.SelectExecution("pi", ar.ModelSelection{}, model); err != nil || changed || session.Checkpoint != "private-pi" {
		t.Fatal(changed, err, session)
	}
	if _, err := session.SelectExecution("claude", model, model); err == nil || session.Runtime != "pi" || session.Checkpoint != "private-pi" {
		t.Fatal("incompatible model mutated session")
	}
	for _, runtime := range []string{"builtin", "codex", "pi"} {
		if changed, err := session.SelectExecution(runtime, model, model); err != nil || !changed || session.Checkpoint != "" || session.ID != "same" || len(session.Messages) != 1 {
			t.Fatal(changed, err, session)
		}
		session.Checkpoint = "private-" + runtime
	}
	other := model
	other.Model = "gpt-5.4-mini"
	if changed, err := session.SelectExecution("pi", other, model); err != nil || !changed || session.Checkpoint != "" {
		t.Fatal(changed, err, session)
	}
	if _, err := session.SelectExecution("builtin", ar.ModelSelection{}, model); err != nil || session.Model != other {
		t.Fatal("runtime-only switch silently replaced the selected model", err)
	}
	direct := ar.ModelSelection{Provider: "openai", Model: "operator-model", Reasoning: "medium", AuthMode: ar.APIKeyAuth, API: ar.OpenAIResponses, BaseURL: "https://api.openai.com/v1", APIKeyEnv: "TEST_MODEL_KEY", ContextWindow: 32000, MaxOutputTokens: 4096}
	if _, err := session.SelectExecution("builtin", direct, model); err != nil {
		t.Fatal(err)
	}
	session.Checkpoint = "private-http"
	direct.BaseURL = "https://other.example/v1"
	if changed, err := session.SelectExecution("builtin", direct, model); err != nil || !changed || session.Checkpoint != "" {
		t.Fatal("provider destination reused private context", err)
	}
	for _, status := range []string{"running", "failed", "cancelled", "budget_exhausted"} {
		session.Messages[0].Status, session.Checkpoint = status, "pre-interruption"
		if _, err := session.SelectExecution("builtin", direct, model); err != nil || session.Checkpoint != "" {
			t.Fatal("unsettled persisted turn reused stale checkpoint", status, err)
		}
	}
}

func TestExecutionContractInvalidatesChangedInstructionsAndTools(t *testing.T) {
	session := Session{ID: "retained", Checkpoint: "unbound-native-state"}
	tools := []ar.Tool{{Name: "read", Description: "Current evidence", Parameters: json.RawMessage(`{"type":"object"}`)}}
	session.BindExecutionContract("system", tools)
	if session.Checkpoint != "" {
		t.Fatal("unbound checkpoint reused")
	}
	session.Checkpoint = "native"
	session.BindExecutionContract("system", tools)
	if session.Checkpoint != "native" {
		t.Fatal("unchanged contract lost checkpoint")
	}
	session.BindExecutionContract("updated system", tools)
	if session.Checkpoint != "" {
		t.Fatal("updated system reused checkpoint")
	}
	session.Checkpoint = "native"
	tools[0].Description = "Updated schema guidance"
	session.BindExecutionContract("updated system", tools)
	if session.Checkpoint != "" || session.ID != "retained" {
		t.Fatal("tool update lost conversation or reused checkpoint")
	}
	public, _ := json.Marshal(session)
	if strings.Contains(string(public), session.ExecutionContract) {
		t.Fatal("private contract fingerprint exposed")
	}
}

func TestConversationPagingPreservesPublicRecordsAndBounds(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	session := Session{ID: "conversation", Checkpoint: "PRIVATE_CHECKPOINT"}
	for i := 0; i < 8; i++ {
		id := fmt.Sprintf("turn_%d", i)
		session.Messages = append(session.Messages, Message{Role: "user", Text: fmt.Sprintf("request %d", i), TurnID: id, Status: "completed"}, Message{Role: "assistant", Text: "historical answer", TurnID: id, Status: "completed"})
		if i == 7 {
			session.Messages[len(session.Messages)-1].Status = "failed"
		}
		for j, event := range []Event{
			{Type: "context.bound", Data: json.RawMessage(`{"start_date":"2026-09-01","entity_id":"campaign_saved"}`)},
			{Type: "ui.upsert", Data: json.RawMessage(`{"id":"card_saved","type":"digest","digest":{"title":"Historical finding"}}`)},
			{Type: "tool.finished", Data: json.RawMessage(`{"name":"get_entity","ok":false,"error":"not_found"}`)},
			{Type: "private.transcript", Data: json.RawMessage(`{"reasoning":"PRIVATE_TRANSCRIPT"}`)},
		} {
			event.TurnID, event.Seq = id, int64(j+1)
			if err := s.AddEvent(ctx, event); err != nil {
				t.Fatal(err)
			}
		}
	}
	page, err := s.Conversation(ctx, session, "")
	if err != nil || len(page.Turns) != 6 || page.Turns[0].ID != "turn_2" || page.NextBeforeTurnID != "turn_2" {
		t.Fatal(page, err)
	}
	raw, _ := json.Marshal(page)
	for _, expected := range []string{"campaign_saved", "Historical finding", "not_found", `"status":"failed"`} {
		if !strings.Contains(string(raw), expected) {
			t.Fatal("lost public context", expected)
		}
	}
	if strings.Contains(string(raw), "PRIVATE_") {
		t.Fatal("private state escaped")
	}
	older, err := s.Conversation(ctx, session, page.NextBeforeTurnID)
	if err != nil || len(older.Turns) != 2 || older.Turns[0].ID != "turn_0" || older.NextBeforeTurnID != "" {
		t.Fatal(older, err)
	}
	if _, err := s.Conversation(ctx, session, "foreign-turn"); err == nil {
		t.Fatal("cross-session cursor accepted")
	}
	session.Messages = []Message{{Role: "user", Text: strings.Repeat("<", 20000), TurnID: "huge"}, {Role: "assistant", Text: strings.Repeat("\u5b57", 20000), TurnID: "huge", Status: "cancelled"}}
	page, err = s.Conversation(ctx, session, "")
	raw, _ = json.Marshal(page)
	if err != nil || len(page.Turns) != 1 || !page.Turns[0].Truncated || len(raw) > ConversationLimit || !json.Valid(raw) {
		t.Fatal("invalid bounded context", len(raw), err)
	}
}
