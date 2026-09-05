package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"

	"github.com/z-chenhao/ad-agent/internal/ads"
	"github.com/z-chenhao/ad-agent/internal/app"
	ar "github.com/z-chenhao/ad-agent/internal/runtime"
	"github.com/z-chenhao/ad-agent/internal/store"
)

type verificationRuntime struct{}

func (verificationRuntime) Run(ctx context.Context, request ar.Request, hooks ar.Hooks) (ar.Result, error) {
	if request.MaxRounds != 0 || !strings.Contains(request.System, "# Advertiser workspace") {
		return ar.Result{}, errors.New("compiled_prompt_contract_failed")
	}
	listed := hooks.Execute(ctx, ar.Call{ID: "verify-list", Name: "list_campaigns", Arguments: json.RawMessage(`{}`), Round: 1})
	if !listed.OK {
		return ar.Result{}, errors.New(listed.Error)
	}
	var campaigns []ads.Entity
	if json.Unmarshal(listed.Data, &campaigns) != nil || len(campaigns) < 3 {
		return ar.Result{}, errors.New("sandbox_hierarchy_failed")
	}
	hooks.Emit(ar.Event{Type: "tool.delta", ID: "verify-card", Name: "present_entities"})
	arguments, _ := json.Marshal(struct {
		IDs        []string `json:"ids"`
		Annotation string   `json:"annotation"`
	}{[]string{campaigns[0].ID}, "Deterministic harness verification"})
	presented := hooks.Execute(ctx, ar.Call{ID: "verify-card", Name: "present_entities", Arguments: arguments, Round: 2})
	if !presented.OK {
		return ar.Result{}, errors.New(presented.Error)
	}
	hooks.Emit(ar.Event{Type: "text.delta", Text: "Verification complete."})
	return ar.Result{Stop: "stop", Text: "Verification complete."}, nil
}

type verificationReport struct {
	OK          bool     `json:"ok"`
	Checks      []string `json:"checks"`
	EventCount  int      `json:"event_count"`
	ToolCalls   int      `json:"tool_calls"`
	Cards       int      `json:"cards"`
	FinalStatus string   `json:"final_status"`
}

func verifyHarness(ctx context.Context, out io.Writer) error {
	dir, err := os.MkdirTemp("", "ad-agent-verify-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	if err = os.Chmod(dir, 0700); err != nil {
		return err
	}
	a, err := app.OpenSandboxRuntime(dir, "cli-verification", verificationRuntime{})
	if err != nil {
		return err
	}
	defer a.Store.Close()
	a.Host.AutomaticMemoryCapture = false
	events := make([]store.Event, 0, 16)
	result, err := a.Host.Run(ctx, "verification", "Inspect the campaign hierarchy and present one verified object.", func(event store.Event) {
		events = append(events, event)
	})
	if err != nil {
		return err
	}
	if result.Status != "completed" || result.Text != "Verification complete." || len(result.Cards) != 1 || len(result.Cards[0].Entities) != 1 {
		return errors.New("agent_result_contract_failed")
	}
	wanted := []string{"turn.started", "progress.updated", "tool.started", "tool.finished", "ui.partial", "ui.upsert", "text.delta", "turn.completed"}
	positions := make(map[string]int, len(wanted))
	toolStarts := map[string]bool{}
	toolFinishes := map[string]bool{}
	partialID, finalCardID := "", ""
	partialPosition, finalCardPosition := -1, -1
	for index, event := range events {
		if event.Seq != int64(index+1) {
			return errors.New("event_sequence_failed")
		}
		if _, exists := positions[event.Type]; !exists {
			positions[event.Type] = index
		}
		if event.Type == "tool.started" {
			var value struct {
				ID string `json:"id"`
			}
			_ = json.Unmarshal(event.Data, &value)
			if value.ID == "" || toolStarts[value.ID] {
				return errors.New("tool_event_idempotency_failed")
			}
			toolStarts[value.ID] = true
		}
		if event.Type == "tool.finished" {
			var value struct {
				ID string `json:"id"`
			}
			_ = json.Unmarshal(event.Data, &value)
			if value.ID == "" || toolFinishes[value.ID] {
				return errors.New("tool_finish_idempotency_failed")
			}
			toolFinishes[value.ID] = true
		}
		if event.Type == "ui.partial" || event.Type == "ui.upsert" {
			var value struct {
				ID string `json:"id"`
			}
			_ = json.Unmarshal(event.Data, &value)
			if event.Type == "ui.partial" {
				partialID = value.ID
				partialPosition = index
			} else if value.ID == result.Cards[0].ID {
				finalCardID = value.ID
				finalCardPosition = index
			}
		}
	}
	for _, eventType := range wanted {
		if _, ok := positions[eventType]; !ok {
			return errors.New("missing_public_event_" + eventType)
		}
	}
	for id := range toolStarts {
		if !toolFinishes[id] {
			return errors.New("unpaired_tool_lifecycle")
		}
	}
	if len(toolStarts) != len(toolFinishes) {
		return errors.New("orphan_tool_finish")
	}
	if partialID == "" || partialID != finalCardID || partialPosition >= finalCardPosition || events[len(events)-1].Type != "turn.completed" {
		return errors.New("presentation_replacement_failed")
	}
	report := verificationReport{
		OK: true,
		Checks: []string{
			"compiled prompt boundary",
			"sandbox hierarchy grounding",
			"monotonic public event sequence",
			"paired tool lifecycle",
			"stable partial-to-final card identity",
			"completed response contract",
		},
		EventCount: len(events), ToolCalls: len(toolStarts), Cards: len(result.Cards), FinalStatus: result.Status,
	}
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}
