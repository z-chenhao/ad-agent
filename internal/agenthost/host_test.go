package agenthost

import (
	"context"
	"encoding/json"
	"github.com/shopspring/decimal"
	"github.com/z-chenhao/ad-agent/internal/ads"
	"github.com/z-chenhao/ad-agent/internal/fixture"
	ar "github.com/z-chenhao/ad-agent/internal/runtime"
	"github.com/z-chenhao/ad-agent/internal/store"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeRuntime func(context.Context, ar.Request, ar.Hooks) (ar.Result, error)

func (f fakeRuntime) Run(c context.Context, r ar.Request, h ar.Hooks) (ar.Result, error) {
	return f(c, r, h)
}
func testHost(t *testing.T, r ar.Runtime) (*Host, *fixture.Backend) {
	t.Helper()
	dir := t.TempDir()
	os.Chmod(dir, 0700)
	s, e := store.Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close() })
	b, e := fixture.New()
	if e != nil {
		t.Fatal(e)
	}
	h, e := New(b, r, s, Changes{Backend: b, Writer: b, Store: s, Policy: ads.FixturePolicy()})
	if e != nil {
		t.Fatal(e)
	}
	h.AutomaticMemoryCapture = false
	return h, b
}
func call(name, args string) ar.Call {
	return ar.Call{ID: store.ID("call"), Name: name, Arguments: json.RawMessage(args), Round: 1}
}
func TestHostIsolationAndEvents(t *testing.T) {
	model := fakeRuntime(func(ctx context.Context, r ar.Request, h ar.Hooks) (ar.Result, error) {
		for _, tool := range r.Tools {
			if tool.Name == "apply_change" || tool.Name == "bash" {
				t.Fatal("unsafe tool")
			}
		}
		for _, c := range []ar.Call{call("apply_change", `{}`), call("get_entity", `{"level":"campaign","id":"campaign_example_1","advertiser_id":"other"}`), call("stage_status_change", `{"level":"campaign","id":"campaign_example_1","status":"ENABLE","reason":"test"}`)} {
			if h.Execute(ctx, c).OK {
				t.Fatal("gate bypass", c.Name)
			}
		}
		if !h.Execute(ctx, call("get_entity", `{"level":"campaign","id":"campaign_example_1"}`)).OK {
			t.Fatal("read failed")
		}
		if !h.Execute(ctx, call("stage_budget_change", `{"level":"campaign","id":"campaign_example_1","budget":"55","currency":"USD","reason":"operator request"}`)).OK {
			t.Fatal("stage failed")
		}
		h.Emit(ar.Event{Type: "text.delta", Text: "Draft awaiting approval"})
		return ar.Result{Stop: "stop", Text: "Draft awaiting approval"}, nil
	})
	h, b := testHost(t, model)
	result, err := h.Run(context.Background(), "test", "Change the budget to 55", nil)
	if err != nil {
		t.Fatal(err)
	}
	entity, _ := b.Get(context.Background(), ads.Campaign, "campaign_example_1")
	if entity.Budget.String() != "50" {
		t.Fatal("model changed backend")
	}
	changes, _ := h.Store.Changes(context.Background(), "test")
	if len(changes) != 1 || changes[0].State != ads.Staged {
		t.Fatal("missing draft")
	}
	events, e := h.Store.Events(context.Background(), result.TurnID, 0)
	if e != nil {
		t.Fatal(e)
	}
	for i, event := range events {
		if event.Seq != int64(i+1) {
			t.Fatal("nonmonotonic sequence")
		}
	}
	if events[len(events)-1].Type != "turn.completed" {
		t.Fatal("no terminal")
	}
}
func TestConcurrentApprovalAndDrift(t *testing.T) {
	h, b := testHost(t, nil)
	ctx := context.Background()
	a, _ := b.Account(ctx)
	before, _ := b.Get(ctx, ads.Campaign, "campaign_example_1")
	after := before
	budget := decimal.NewFromInt(55)
	after.Budget = &budget
	session := store.Session{ID: "s", Source: a.Source, Provenance: map[string]store.Seen{before.ID: {Entity: before, At: time.Now()}}}
	c, e := h.Changes.Stage(ctx, session, before, after, ads.BudgetChange, "test")
	if e != nil {
		t.Fatal(e)
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	success := 0
	for i := 0; i < 12; i++ {
		wg.Go(func() {
			v, e := h.Changes.Apply(ctx, "s", c.ID, "operator")
			if e == nil && v.State == ads.Applied {
				mu.Lock()
				success++
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	if success != 1 {
		t.Fatalf("execution count %d", success)
	}
	record, _ := h.Store.Change(ctx, c.ID)
	if record.AttemptID == "" || record.ApprovedBy != "operator" {
		t.Fatal("missing approval audit")
	}
	before, _ = b.Get(ctx, ads.Campaign, before.ID)
	session.Provenance[before.ID] = store.Seen{Entity: before, At: time.Now()}
	after = before
	after.Status = "ENABLE"
	c, e = h.Changes.Stage(ctx, session, before, after, ads.StatusChange, "test")
	if e != nil {
		t.Fatal(e)
	}
	v := decimal.NewFromInt(56)
	b.Write(ctx, ads.WriteRequest{Target: before, Kind: "budget", Budget: &v})
	c, e = h.Changes.Apply(ctx, "s", c.ID, "operator")
	if e != nil || c.State != ads.Expired {
		t.Fatal("drift not rejected", e, c.State)
	}
}

type unknownWriter struct{}

func (unknownWriter) Write(context.Context, ads.WriteRequest) ads.WriteOutcome {
	return ads.WriteOutcome{State: "unknown", Message: "response lost"}
}
func TestUnknownWriteNeverRetries(t *testing.T) {
	h, b := testHost(t, nil)
	h.Changes.Writer = unknownWriter{}
	ctx := context.Background()
	a, _ := b.Account(ctx)
	before, _ := b.Get(ctx, ads.Campaign, "campaign_example_1")
	after := before
	after.Status = "ENABLE"
	s := store.Session{ID: "s", Source: a.Source, Provenance: map[string]store.Seen{before.ID: {Entity: before, At: time.Now()}}}
	c, e := h.Changes.Stage(ctx, s, before, after, ads.StatusChange, "test")
	if e != nil {
		t.Fatal(e)
	}
	c, e = h.Changes.Apply(ctx, "s", c.ID, "operator")
	if e != nil || c.State != ads.Indeterminate {
		t.Fatal("unknown is not indeterminate", e)
	}
	if _, e = h.Changes.Apply(ctx, "s", c.ID, "operator"); e == nil {
		t.Fatal("unknown write retried")
	}
}
func TestAnalysisDoesNotGrantProvenance(t *testing.T) {
	var parent bool
	model := fakeRuntime(func(ctx context.Context, r ar.Request, h ar.Hooks) (ar.Result, error) {
		if !parent {
			parent = true
			result := h.Execute(ctx, call("get_performance_report", `{"level":"campaign","start_date":"2022-07-11","end_date":"2022-07-17"}`))
			var report ads.Report
			json.Unmarshal(result.Data, &report)
			args, _ := json.Marshal(struct {
				Question string   `json:"question"`
				Refs     []string `json:"dataset_refs"`
			}{"rank", []string{report.ID}})
			result = h.Execute(ctx, ar.Call{ID: "analysis", Name: "run_analysis", Arguments: args})
			if !result.OK {
				t.Fatal(result.Error)
			}
			if h.Execute(ctx, call("stage_status_change", `{"level":"campaign","id":"campaign_example_1","status":"ENABLE","reason":"analysis suggested"}`)).OK {
				t.Fatal("child granted provenance")
			}
		} else {
			for _, tool := range r.Tools {
				if tool.Name == "get_entity" || tool.Name == "stage_budget_change" || tool.Name == "run_analysis" {
					t.Fatal("child authority leak")
				}
			}
			ref := r.Prompt[len(r.Prompt)-39:] // report_ plus 32 hex bytes, supplied only as a server handle.
			args, _ := json.Marshal(struct {
				Ref       string `json:"dataset_ref"`
				Operation string `json:"operation"`
			}{ref, "rank"})
			result := h.Execute(ctx, ar.Call{ID: "calculate", Name: "analysis_calculate", Arguments: args})
			if !result.OK {
				t.Fatal(result.Error)
			}
			var calc ads.Calculation
			json.Unmarshal(result.Data, &calc)
			payload := `{"summary":"fixture","findings":[{"evidence_id":"` + calc.ID + `","entity_id":"campaign_example_1","observation":"lower ROAS"}],"counter_evidence":["control stable"],"limitations":["synthetic"],"method":"weighted ratio"}`
			if !h.Execute(ctx, call("submit_analysis", payload)).OK {
				t.Fatal("submit failed")
			}
		}
		return ar.Result{Stop: "stop", Text: "done"}, nil
	})
	h, _ := testHost(t, model)
	if _, e := h.Run(context.Background(), "test", "diagnose", nil); e != nil {
		t.Fatal(e)
	}
}

func TestExplicitMemoryLifecycleAndInjection(t *testing.T) {
	runs := 0
	var savedID string
	model := fakeRuntime(func(ctx context.Context, r ar.Request, h ar.Hooks) (ar.Result, error) {
		runs++
		if runs == 1 {
			result := h.Execute(ctx, call("save_memory", `{"kind":"constraint","text":"Limit each budget change to 10%"}`))
			if !result.OK {
				t.Fatal(result.Error)
			}
			var memory store.Memory
			if err := json.Unmarshal(result.Data, &memory); err != nil {
				t.Fatal(err)
			}
			savedID = memory.ID
			if result = h.Execute(ctx, call("recall_memory", `{}`)); !result.OK || !strings.Contains(string(result.Data), savedID) {
				t.Fatal("saved memory not recalled", result.Error)
			}
		} else {
			if !strings.Contains(r.Prompt, "Limit each budget change to 10%") || !strings.Contains(r.Prompt, "<saved_facts>") {
				t.Fatal("memory not fenced into the next turn")
			}
			result := h.Execute(ctx, call("delete_memory", `{"memory_id":"`+savedID+`"}`))
			if !result.OK {
				t.Fatal(result.Error)
			}
		}
		return ar.Result{Stop: "stop", Text: "done"}, nil
	})
	h, _ := testHost(t, model)
	if _, err := h.Run(context.Background(), "memory-test", "Remember this constraint", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := h.Run(context.Background(), "memory-test", "Delete that constraint", nil); err != nil {
		t.Fatal(err)
	}
	memories, err := h.Store.Memories(context.Background(), ads.Source{Backend: "fixture", Environment: "fixture", AccountID: "advertiser_example_1"}, 50)
	if err != nil || len(memories) != 0 {
		t.Fatalf("memory still present: %#v %v", memories, err)
	}
}

func TestMemoryRejectsCredentialAndObjectFacts(t *testing.T) {
	for _, text := range []string{
		"App Secret is secret-value",
		"remember access_token abc",
		"advertiser_id is 123",
		"account number 1234567890123456789",
		"sk-" + "abcdefghijklmnopqrstuvwxyz",
	} {
		if !unsafeMemoryText(text) {
			t.Fatalf("unsafe memory accepted: %q", text)
		}
	}
	for _, text := range []string{
		"Limit each budget change to 10%",
		"Prioritize conversion-cost optimization",
		"Review last week every Monday",
	} {
		if unsafeMemoryText(text) {
			t.Fatalf("safe memory rejected: %q", text)
		}
	}
}

func TestHarnessForcesPerformanceGroundingAndDeduplicatesPartialCards(t *testing.T) {
	model := fakeRuntime(func(ctx context.Context, request ar.Request, hooks ar.Hooks) (ar.Result, error) {
		if !strings.Contains(request.Prompt, "<host_grounding>") || !strings.Contains(request.Prompt, `"tool":"get_performance_report"`) || !strings.Contains(request.Prompt, "report_") {
			t.Fatalf("required performance grounding missing from prompt: %s", request.Prompt)
		}
		hooks.Emit(ar.Event{Type: "tool.delta", ID: "presentation-1", Name: "present_metrics"})
		hooks.Emit(ar.Event{Type: "tool.delta", ID: "presentation-1", Name: "present_metrics"})
		return ar.Result{Stop: "stop", Text: "grounded", Checkpoint: `{"id":"checkpoint"}`}, nil
	})
	host, _ := testHost(t, model)
	result, err := host.Run(context.Background(), "grounding-test", "Compare campaign performance for the latest seven days", nil)
	if err != nil {
		t.Fatal(err)
	}
	events, err := host.Store.Events(context.Background(), result.TurnID, 0)
	if err != nil {
		t.Fatal(err)
	}
	partials := 0
	forcedReads := 0
	for _, event := range events {
		if event.Type == "ui.partial" {
			partials++
		}
		if event.Type == "tool.started" && strings.Contains(string(event.Data), "get_performance_report") {
			forcedReads++
		}
	}
	if partials != 1 {
		t.Fatalf("partial presentation events = %d, want 1", partials)
	}
	if forcedReads != 1 {
		t.Fatalf("forced grounding reads = %d, want 1", forcedReads)
	}
}

func TestHarnessFollowsThroughOnRequestedChange(t *testing.T) {
	runs := 0
	model := fakeRuntime(func(ctx context.Context, request ar.Request, hooks ar.Hooks) (ar.Result, error) {
		runs++
		if runs == 1 {
			return ar.Result{Stop: "stop", Text: "I recommend changing it.", Checkpoint: `{"id":"first"}`}, nil
		}
		if !strings.Contains(request.Checkpoint, "first") {
			t.Fatal("follow-through did not resume the first runtime checkpoint")
		}
		if request.Prompt != stagingFollowThroughReminder {
			t.Fatalf("unexpected follow-through prompt: %s", request.Prompt)
		}
		if result := hooks.Execute(ctx, call("get_entity", `{"level":"campaign","id":"campaign_example_1"}`)); !result.OK {
			t.Fatal(result.Error)
		}
		result := hooks.Execute(ctx, call("stage_budget_change", `{"level":"campaign","id":"campaign_example_1","budget":"55","currency":"USD","reason":"operator requested 55 USD"}`))
		if !result.OK {
			t.Fatal(result.Error)
		}
		return ar.Result{Stop: "stop", Text: "Draft staged.", Checkpoint: `{"id":"second"}`}, nil
	})
	host, _ := testHost(t, model)
	result, err := host.Run(context.Background(), "follow-through", "Change campaign_example_1 budget to 55 USD", nil)
	if err != nil {
		t.Fatal(err)
	}
	if runs != 2 || !strings.Contains(result.Text, "Draft staged") {
		t.Fatalf("follow-through result = %#v, runs = %d", result, runs)
	}
	changes, err := host.Store.Changes(context.Background(), "follow-through")
	if err != nil || len(changes) != 1 || changes[0].State != ads.Staged {
		t.Fatalf("staged changes = %#v, err = %v", changes, err)
	}
}

func TestHarnessEnrichesDigestAndClosesOnlyTerminalPresentation(t *testing.T) {
	model := fakeRuntime(func(ctx context.Context, request ar.Request, hooks ar.Hooks) (ar.Result, error) {
		if result := hooks.Execute(ctx, call("get_entity", `{"level":"campaign","id":"campaign_example_1"}`)); !result.OK {
			t.Fatal(result.Error)
		}
		digest := hooks.Execute(ctx, call("present_digest", `{"title":"Today","items":[{"kind":"delivery","headline":"Inspect delivery","ref_id":"campaign_example_1","action":"Review campaign"}]}`))
		if !digest.OK || hooks.CloseAfter(call("present_digest", `{}`), digest) {
			t.Fatalf("digest must render but keep the turn open: %#v", digest)
		}
		bad := hooks.Execute(ctx, call("present_digest", `{"title":"Bad","items":[{"kind":"warning","headline":"Ungrounded","ref_id":"campaign_unknown"}]}`))
		if bad.OK || bad.Error != "digest_reference_not_grounded" {
			t.Fatalf("ungrounded digest accepted: %#v", bad)
		}
		suggestions := hooks.Execute(ctx, call("present_suggestions", `{"suggestions":["Review campaign"]}`))
		if !suggestions.OK || !hooks.CloseAfter(call("present_suggestions", `{}`), suggestions) {
			t.Fatalf("suggestions must close the presentation sequence: %#v", suggestions)
		}
		return ar.Result{Stop: "stop", Text: "done"}, nil
	})
	host, _ := testHost(t, model)
	result, err := host.Run(context.Background(), "digest-test", "Give me a briefing", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Cards) != 2 || result.Cards[0].Digest == nil || result.Cards[0].Digest.Items[0].Entity == nil {
		t.Fatalf("digest was not server-enriched: %#v", result.Cards)
	}
}

func TestAutomaticMemoryExtractionUsesIsolatedFilteredTool(t *testing.T) {
	extractions := 0
	model := fakeRuntime(func(ctx context.Context, request ar.Request, hooks ar.Hooks) (ar.Result, error) {
		if request.System != memoryExtractionSystem {
			return ar.Result{Stop: "stop", Text: "Understood."}, nil
		}
		extractions++
		if len(request.Tools) != 1 || request.Tools[0].Name != "record_memory_fact" || strings.Contains(request.Prompt, "campaign_example") {
			t.Fatalf("memory authority or transcript leak: %#v", request)
		}
		text := "Keep each budget change at or below 10%"
		if extractions == 2 {
			text = "Keep each budget change at or below 8%"
		}
		arguments, _ := json.Marshal(map[string]string{"key": "budget_guardrail", "kind": "constraint", "text": text})
		result := hooks.Execute(ctx, ar.Call{ID: "memory-call", Name: "record_memory_fact", Arguments: arguments, Round: 1})
		if !result.OK {
			t.Fatal(result.Error)
		}
		return ar.Result{Stop: "stop", Text: ""}, nil
	})
	host, _ := testHost(t, model)
	host.AutomaticMemoryCapture = true
	if !host.PublicHarness().AutomaticMemoryCapture {
		t.Fatal("automatic memory capability not projected")
	}
	for _, message := range []string{
		"I want every budget adjustment kept at or below 10%.",
		"Update that standing limit to 8%.",
	} {
		if _, err := host.Run(context.Background(), "auto-memory", message, nil); err != nil {
			t.Fatal(err)
		}
	}
	account, _ := host.Backend.Account(context.Background())
	memories, err := host.Store.Memories(context.Background(), account.Source, 50)
	if err != nil || len(memories) != 1 || memories[0].Key != "budget_guardrail" || !strings.Contains(memories[0].Text, "8%") {
		t.Fatalf("extracted memories = %#v, err = %v", memories, err)
	}
}
