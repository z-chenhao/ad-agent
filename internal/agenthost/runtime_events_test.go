package agenthost

import (
	"context"
	"errors"
	"strings"
	"testing"

	ar "github.com/z-chenhao/ad-agent/internal/runtime"
	"github.com/z-chenhao/ad-agent/internal/store"
)

func TestRuntimeFailureHasSafeCodeAndRetainsPublicCommentary(t *testing.T) {
	h, _ := testHost(t, fakeRuntime(func(_ context.Context, _ ar.Request, hooks ar.Hooks) (ar.Result, error) {
		hooks.Emit(ar.Event{Type: "text.delta", ID: "message-1", Text: "I will inspect the account."})
		return ar.Result{}, errors.New("Builtin model bridge failed: provider_history_rejected private-secret")
	}))
	var events []store.Event
	out, err := h.Run(context.Background(), "runtime-error", "Explain the account", func(event store.Event) { events = append(events, event) })
	if err == nil || out.Status != "failed" || out.ErrorCode != "provider_history_rejected" {
		t.Fatalf("out=%+v err=%v", out, err)
	}
	textSeen := false
	for _, event := range events {
		if strings.Contains(string(event.Data), "private-secret") {
			t.Fatal("provider details escaped")
		}
		if event.Type == "text.delta" && strings.Contains(string(event.Data), "1:message-1") {
			textSeen = true
		}
	}
	if !textSeen {
		t.Fatal("public message identity lost")
	}
}
