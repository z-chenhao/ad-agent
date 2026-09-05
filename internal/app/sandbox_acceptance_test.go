package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/z-chenhao/ad-agent/internal/ads"
	ar "github.com/z-chenhao/ad-agent/internal/runtime"
)

type sandboxTestRuntime struct{}

func (sandboxTestRuntime) Run(context.Context, ar.Request, ar.Hooks) (ar.Result, error) {
	return ar.Result{Stop: "stop"}, nil
}

type sandboxDraftRuntime struct{}

func (sandboxDraftRuntime) Run(ctx context.Context, _ ar.Request, hooks ar.Hooks) (ar.Result, error) {
	read := hooks.Execute(ctx, ar.Call{ID: "read", Name: "get_entity", Arguments: json.RawMessage(`{"level":"campaign","id":"campaign_prospect_us"}`), Round: 1})
	if !read.OK {
		return ar.Result{}, errors.New(read.Error)
	}
	staged := hooks.Execute(ctx, ar.Call{ID: "stage", Name: "stage_budget_change", Arguments: json.RawMessage(`{"level":"campaign","id":"campaign_prospect_us","budget":"860","currency":"USD","reason":"sandbox agent acceptance"}`), Round: 2})
	if !staged.OK {
		return ar.Result{}, errors.New(staged.Error)
	}
	return ar.Result{Stop: "stop", Text: "Draft staged for operator approval."}, nil
}

type sandboxCreateRuntime struct{}

func (sandboxCreateRuntime) Run(ctx context.Context, _ ar.Request, hooks ar.Hooks) (ar.Result, error) {
	staged := hooks.Execute(ctx, ar.Call{ID: "create", Name: "stage_entity_create", Arguments: json.RawMessage(`{"level":"campaign","name":"Agent launch","status":"DISABLE","budget":"80","budget_mode":"BUDGET_MODE_TOTAL","objective":"TRAFFIC","reason":"sandbox lifecycle acceptance"}`), Round: 1})
	if !staged.OK {
		return ar.Result{}, errors.New(staged.Error)
	}
	return ar.Result{Stop: "stop", Text: "Creation draft staged."}, nil
}

type managerDraftRuntime struct{}

func (managerDraftRuntime) Run(ctx context.Context, _ ar.Request, hooks ar.Hooks) (ar.Result, error) {
	read := hooks.Execute(ctx, ar.Call{ID: "read", Name: "get_account_entity", Arguments: json.RawMessage(`{"advertiser_id":"sandbox_adv_north","level":"campaign","id":"campaign_prospect_us"}`), Round: 1})
	if !read.OK {
		return ar.Result{}, errors.New(read.Error)
	}
	staged := hooks.Execute(ctx, ar.Call{ID: "stage", Name: "stage_account_budget_change", Arguments: json.RawMessage(`{"advertiser_id":"sandbox_adv_north","level":"campaign","id":"campaign_prospect_us","budget":"860","currency":"USD","reason":"manager persistence acceptance"}`), Round: 2})
	if !staged.OK {
		return ar.Result{}, errors.New(staged.Error)
	}
	return ar.Result{Stop: "stop", Text: "One advertiser-scoped draft staged."}, nil
}

func openSandboxEnvironment(t *testing.T, dir, environment string, runtime ar.Runtime) *App {
	t.Helper()
	application, err := OpenSandboxRuntime(dir, environment, runtime)
	if err != nil {
		t.Fatal(err)
	}
	return application
}

func TestSandboxEnvironmentCreatesAndPersistsHierarchy(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	first := openSandboxEnvironment(t, dir, "experiment-a", sandboxTestRuntime{})
	creator, ok := first.Backend.(ads.Creator)
	if !ok {
		t.Fatal("sandbox does not expose its lifecycle seam")
	}
	common, ok := first.Backend.(ads.CommonAdsReader)
	if !ok {
		t.Fatal("persistent sandbox does not expose common advertising reads")
	}
	forms, err := common.ListLeadForms(context.Background())
	if err != nil || len(forms) == 0 || len(forms[0].FieldNames) == 0 {
		t.Fatalf("lead form metadata=%#v err=%v", forms, err)
	}
	budget := decimal.NewFromInt(75)
	campaign, err := creator.Create(context.Background(), ads.CreateRequest{Level: ads.Campaign, Name: "Launch", Status: "DISABLE", Budget: &budget, BudgetMode: "BUDGET_MODE_TOTAL", Objective: "TRAFFIC"})
	if err != nil {
		t.Fatal(err)
	}
	group, err := creator.Create(context.Background(), ads.CreateRequest{Level: ads.AdGroup, ParentID: campaign.ID, Name: "Prospecting", Status: "DISABLE", Budget: &budget, BudgetMode: "BUDGET_MODE_DAY"})
	if err != nil {
		t.Fatal(err)
	}
	ad, err := creator.Create(context.Background(), ads.CreateRequest{Level: ads.Ad, ParentID: group.ID, Name: "Creative A", Status: "DISABLE"})
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Store.Close(); err != nil {
		t.Fatal(err)
	}
	second := openSandboxEnvironment(t, dir, "experiment-a", sandboxTestRuntime{})
	defer second.Store.Close()
	got, err := second.Backend.Get(context.Background(), ads.Ad, ad.ID)
	if err != nil || got.ParentID != group.ID {
		t.Fatalf("created ad=%#v err=%v", got, err)
	}
	adsInGroup, err := second.Backend.List(context.Background(), ads.EntityQuery{Level: ads.Ad, ParentID: group.ID})
	if err != nil || len(adsInGroup) != 1 || adsInGroup[0].ID != ad.ID {
		t.Fatalf("ads=%#v err=%v", adsInGroup, err)
	}
}

func TestSandboxEnvironmentsAreIsolated(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	a := openSandboxEnvironment(t, dir, "a", sandboxTestRuntime{})
	defer a.Store.Close()
	created, err := a.Backend.(ads.Creator).Create(context.Background(), ads.CreateRequest{Level: ads.Campaign, Name: "Only A", Status: "DISABLE"})
	if err != nil {
		t.Fatal(err)
	}
	b := openSandboxEnvironment(t, dir, "b", sandboxTestRuntime{})
	defer b.Store.Close()
	if _, err := b.Backend.Get(context.Background(), ads.Campaign, created.ID); !errors.Is(err, ads.ErrNotFound) {
		t.Fatalf("environment b saw environment a entity: %v", err)
	}
	accountA, _ := a.Backend.Account(context.Background())
	accountB, _ := b.Backend.Account(context.Background())
	if accountA.Source.Environment != "a" || accountB.Source.Environment != "b" {
		t.Fatalf("sources=%#v %#v", accountA.Source, accountB.Source)
	}
}

func TestSandboxClockAndHourlyFactsPersistAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	first := openSandboxEnvironment(t, dir, "clock-persist", sandboxTestRuntime{})
	if _, err := first.Sandbox.Advance(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	result, err := first.Sandbox.Advance(context.Background(), 24)
	if err != nil || result.State.GeneratedHours != 25 || result.State.FactCount != 300 {
		t.Fatalf("advance=%#v err=%v", result, err)
	}
	if err = first.Store.Close(); err != nil {
		t.Fatal(err)
	}
	second := openSandboxEnvironment(t, dir, "clock-persist", sandboxTestRuntime{})
	defer second.Store.Close()
	state, err := second.Sandbox.SimulationState(context.Background())
	if err != nil || state.CurrentTime != result.State.CurrentTime || state.FactCount != result.State.FactCount {
		t.Fatalf("restored=%#v err=%v", state, err)
	}
	report, err := second.Backend.Report(context.Background(), ads.ReportQuery{Level: ads.Advertiser, Start: "2026-09-04", End: "2026-09-04"})
	if err != nil || !report.Complete || len(report.Rows) != 1 {
		t.Fatalf("restored report=%#v err=%v", report, err)
	}
}

func TestSandboxDeveloperTracePersistsWithoutEnteringReportContract(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	application := openSandboxEnvironment(t, dir, "debug-trace", sandboxTestRuntime{})
	result, traces, err := application.Sandbox.AdvanceDebug(context.Background(), 1)
	if err != nil || result.FactsCreated != 12 || len(traces) != 12 {
		t.Fatalf("debug advance=%#v traces=%d err=%v", result, len(traces), err)
	}
	for _, trace := range traces {
		if trace.Opportunities == 0 || trace.Hour.IsZero() || trace.AdID == "" {
			t.Fatalf("incomplete causal trace: %#v", trace)
		}
	}
	report, err := application.Backend.Report(context.Background(), ads.ReportQuery{Level: ads.Advertiser, Start: "2026-09-04", End: "2026-09-04"})
	if err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(report)
	if string(payload) == "" || containsJSONField(payload, "debug_trace") || containsJSONField(payload, "true_metrics") {
		t.Fatalf("hidden simulator state leaked into report: %s", payload)
	}
	if err = application.Store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened := openSandboxEnvironment(t, dir, "debug-trace", sandboxTestRuntime{})
	defer reopened.Store.Close()
	state, err := reopened.Sandbox.SimulationState(context.Background())
	if err != nil || state.Model != nil {
		t.Fatalf("debug mode remained enabled after one-shot trace: %#v err=%v", state, err)
	}
}

func containsJSONField(payload []byte, field string) bool {
	var value any
	if json.Unmarshal(payload, &value) != nil {
		return false
	}
	var walk func(any) bool
	walk = func(current any) bool {
		switch typed := current.(type) {
		case map[string]any:
			if _, ok := typed[field]; ok {
				return true
			}
			for _, child := range typed {
				if walk(child) {
					return true
				}
			}
		case []any:
			for _, child := range typed {
				if walk(child) {
					return true
				}
			}
		}
		return false
	}
	return walk(value)
}

func TestSandboxPreservesUnrelatedSimulationNamespace(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	first := openSandboxEnvironment(t, dir, "namespace-isolation", sandboxTestRuntime{})
	unrelatedTime := time.Date(2022, 7, 17, 23, 0, 0, 0, time.UTC)
	unrelatedPayload := []byte(`{"environment":"namespace-isolation","account_id":"unrelated","current_time":"2022-07-17T23:00:00Z","granularity":"hour","generated_hours":0,"fact_count":0,"seed_start":"2022-07-04","seed_end":"2022-07-17"}`)
	if err := first.Store.SaveSandboxAdvance(ctx, "namespace-isolation", unrelatedTime, unrelatedTime, unrelatedPayload, nil); err != nil {
		t.Fatal(err)
	}
	if err := first.Store.Close(); err != nil {
		t.Fatal(err)
	}
	second := openSandboxEnvironment(t, dir, "namespace-isolation", sandboxTestRuntime{})
	defer second.Store.Close()
	state, err := second.Sandbox.SimulationState(ctx)
	if err != nil || state.SeedEnd != "2026-09-03" || state.GeneratedHours != 0 {
		t.Fatalf("new simulation state=%#v err=%v", state, err)
	}
	preserved, _, err := second.Store.SandboxSimulation(ctx, "namespace-isolation")
	if err != nil || string(preserved) != string(unrelatedPayload) {
		t.Fatalf("unrelated simulation was not preserved: %s err=%v", preserved, err)
	}
}

func TestSandboxAgentApprovalRoundTrip(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	application := openSandboxEnvironment(t, dir, "approval", sandboxDraftRuntime{})
	defer application.Store.Close()
	application.Host.AutomaticMemoryCapture = false
	result, err := application.Host.Run(context.Background(), "agent-acceptance", "Change campaign_prospect_us budget to 860 USD", nil)
	if err != nil || result.Status != "completed" {
		t.Fatalf("turn status=%s err=%v", result.Status, err)
	}
	changes, err := application.Store.Changes(context.Background(), "agent-acceptance")
	if err != nil || len(changes) != 1 || changes[0].State != ads.Staged {
		t.Fatalf("changes=%#v err=%v", changes, err)
	}
	change, err := application.Changes.Apply(context.Background(), "agent-acceptance", changes[0].ID, "sandbox-operator")
	if err != nil || change.State != ads.Applied {
		t.Fatalf("apply state=%s err=%v", change.State, err)
	}
}

func TestSandboxAgentStagesAndApprovesCreation(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	application := openSandboxEnvironment(t, dir, "agent-create", sandboxCreateRuntime{})
	defer application.Store.Close()
	application.Host.AutomaticMemoryCapture = false
	result, err := application.Host.Run(context.Background(), "create-session", "Create a disabled traffic campaign named Agent launch with an 80 USD total budget", nil)
	if err != nil || result.Status != "completed" {
		t.Fatalf("turn=%#v err=%v", result, err)
	}
	changes, err := application.Store.Changes(context.Background(), "create-session")
	if err != nil || len(changes) != 1 || changes[0].Kind != ads.CreateChange || changes[0].Created != nil {
		t.Fatalf("drafts=%#v err=%v", changes, err)
	}
	change, err := application.Changes.Apply(context.Background(), "create-session", changes[0].ID, "sandbox-operator")
	if err != nil || change.State != ads.Applied || change.Created == nil || change.Created.Name != "Agent launch" {
		t.Fatalf("change=%#v err=%v", change, err)
	}
	created, err := application.Backend.Get(context.Background(), ads.Campaign, change.Created.ID)
	if err != nil || created.Name != "Agent launch" {
		t.Fatalf("created=%#v err=%v", created, err)
	}
}

func TestManagerSandboxPersistsAndIsolatesAdvertisers(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	first, err := OpenManagerSandboxRuntime(dir, "manager-persist", managerDraftRuntime{})
	if err != nil {
		t.Fatal(err)
	}
	result, err := first.Host.Run(context.Background(), "manager-session", "Set only Northstar's campaign budget to 860 USD", nil)
	if err != nil || result.Status != "completed" {
		t.Fatalf("turn=%#v err=%v", result, err)
	}
	changes, err := first.Store.Changes(context.Background(), "manager-session")
	if err != nil || len(changes) != 1 || changes[0].Source.AccountID != "sandbox_adv_north" {
		t.Fatalf("changes=%#v err=%v", changes, err)
	}
	if _, err = first.Scope.Apply(context.Background(), "manager-session", changes[0].ID, "sandbox-operator"); err != nil {
		t.Fatal(err)
	}
	if err = first.Store.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := OpenManagerSandboxRuntime(dir, "manager-persist", sandboxTestRuntime{})
	if err != nil {
		t.Fatal(err)
	}
	defer second.Store.Close()
	north, err := second.Scope.Get(context.Background(), "sandbox_adv_north", ads.Campaign, "campaign_prospect_us")
	if err != nil || north.Budget == nil || !north.Budget.Equal(decimal.NewFromInt(860)) {
		t.Fatalf("north=%#v err=%v", north, err)
	}
	home, err := second.Scope.Get(context.Background(), "sandbox_adv_home", ads.Campaign, "campaign_prospect_us")
	if err != nil || home.Budget == nil || home.Budget.Equal(decimal.NewFromInt(860)) {
		t.Fatalf("home advertiser was not isolated: %#v err=%v", home, err)
	}
}
