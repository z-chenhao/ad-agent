package agenthost

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/z-chenhao/ad-agent/internal/ads"
	ar "github.com/z-chenhao/ad-agent/internal/runtime"
	"github.com/z-chenhao/ad-agent/internal/store"
)

type namedRuntime struct {
	fakeRuntime
	name string
}

func (r namedRuntime) RuntimeName() string { return r.name }

func TestRuntimeAndModelHandoffKeepsBusinessConversationAcrossRestart(t *testing.T) {
	ctx := context.Background()
	first := namedRuntime{name: "pi", fakeRuntime: func(ctx context.Context, req ar.Request, hooks ar.Hooks) (ar.Result, error) {
		if req.Checkpoint != "" {
			t.Fatal("unexpected initial checkpoint")
		}
		if result := hooks.Execute(ctx, call("get_entity", `{"level":"campaign","id":"campaign_prospect_us"}`)); !result.OK {
			t.Fatal(result.Error)
		}
		if result := hooks.Execute(ctx, call("stage_budget_change", `{"level":"campaign","id":"campaign_prospect_us","budget":"860","currency":"USD","reason":"operator request"}`)); !result.OK {
			t.Fatal(result.Error)
		}
		return ar.Result{Stop: "stop", Text: "A budget proposal is awaiting approval, not applied.", Checkpoint: "PRIVATE_PI"}, nil
	}}
	host, backend := testHost(t, first)
	before, _ := backend.Get(ctx, ads.Campaign, "campaign_prospect_us")
	view := ViewContext{Page: "campaigns", EntityID: "campaign_prospect_us", EntityLevel: "campaign", StartDate: "2026-08-28", EndDate: "2026-09-03"}
	if _, err := host.RunWithModelAndView(ctx, "continuous", "Prepare my campaign budget proposal", ar.ModelSelection{}, view, nil); err != nil {
		t.Fatal(err)
	}
	dir := host.Store.Dir
	if err := host.Store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	changes := host.Changes
	changes.Store = reopened
	pass := 0
	runner := namedRuntime{name: "builtin", fakeRuntime: func(ctx context.Context, req ar.Request, hooks ar.Hooks) (ar.Result, error) {
		pass++
		if pass == 1 || pass == 2 || pass == 4 || pass == 5 {
			if req.Checkpoint != "" || !strings.Contains(req.Prompt, "conversation_history") || !strings.Contains(req.Prompt, "budget proposal") || strings.Contains(req.Prompt, "PRIVATE_") {
				t.Fatal("handoff lost conversation or crossed native state", pass)
			}
		} else if req.Checkpoint != "PRIVATE_BUILTIN" || strings.Contains(req.Prompt, "conversation_history") {
			t.Fatal("unchanged execution did not reuse its checkpoint")
		}
		if pass == 1 {
			if !strings.Contains(req.Prompt, "2026-08-28") || !strings.Contains(req.Prompt, "campaign_prospect_us") {
				t.Fatal("lost saved scope and preview")
			}
			history := hooks.Execute(ctx, call("read_conversation", `{}`))
			if !history.OK || !strings.Contains(string(history.Data), "awaiting approval") || strings.Contains(string(history.Data), "Continue the investigation") {
				t.Fatal("history retrieval includes current turn or lost prior answer")
			}
			if bad := hooks.Execute(ctx, call("read_conversation", `{"before_turn_id":"another-session-turn"}`)); bad.OK {
				t.Fatal("foreign history accepted")
			}
			pending := hooks.Execute(ctx, call("get_pending_changes", `{}`))
			var drafts []ads.Change
			json.Unmarshal(pending.Data, &drafts)
			if !pending.OK || len(drafts) != 1 || drafts[0].State != ads.Staged {
				t.Fatal("draft lost or implicitly applied")
			}
		}
		if pass == 4 {
			return ar.Result{Checkpoint: "PRIVATE_UNSETTLED"}, errors.New("provider_failed")
		}
		if pass == 5 && !strings.Contains(req.Prompt, `\"status\":\"failed\"`) && !strings.Contains(req.Prompt, `"status":"failed"`) {
			t.Fatal("failure status lost")
		}
		return ar.Result{Stop: "stop", Text: "The proposal remains unapproved.", Checkpoint: "PRIVATE_BUILTIN"}, nil
	}}
	next, err := New(backend, runner, reopened, changes)
	if err != nil {
		t.Fatal(err)
	}
	next.AutomaticMemoryCapture = false
	if _, err = next.Run(ctx, "continuous", "Continue the investigation", nil); err != nil {
		t.Fatal(err)
	}
	model := ar.DefaultModelSelection()
	model.Model = "gpt-5.4-mini"
	if _, err = next.RunWithModel(ctx, "continuous", "Review the same proposal", model, nil); err != nil {
		t.Fatal(err)
	}
	if _, err = next.Run(ctx, "continuous", "Continue", nil); err != nil {
		t.Fatal(err)
	}
	runner.name = "pi"
	next.Runtime = runner
	if _, err = next.Run(ctx, "continuous", "Continue", nil); err == nil {
		t.Fatal("provider failure hidden")
	}
	if _, err = next.Run(ctx, "continuous", "Resume after failure", nil); err != nil {
		t.Fatal(err)
	}
	after, _ := backend.Get(ctx, ads.Campaign, "campaign_prospect_us")
	if !before.Budget.Equal(*after.Budget) {
		t.Fatal("switch executed a write")
	}
	account, _ := backend.Account(ctx)
	saved, err := reopened.Session(ctx, "continuous", account.Source)
	if err != nil || len(saved.Messages) != 12 || len(saved.Provenance) == 0 {
		t.Fatal("business history or provenance lost", err)
	}
}
