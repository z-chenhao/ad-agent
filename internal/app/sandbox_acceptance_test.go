package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"

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
	read := hooks.Execute(ctx, ar.Call{ID: "read", Name: "get_entity", Arguments: json.RawMessage(`{"level":"campaign","id":"campaign_example_1"}`), Round: 1})
	if !read.OK {
		return ar.Result{}, errors.New(read.Error)
	}
	staged := hooks.Execute(ctx, ar.Call{ID: "stage", Name: "stage_budget_change", Arguments: json.RawMessage(`{"level":"campaign","id":"campaign_example_1","budget":"55","currency":"USD","reason":"sandbox agent acceptance"}`), Round: 2})
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

type portfolioDraftRuntime struct{}

func (portfolioDraftRuntime) Run(ctx context.Context, _ ar.Request, hooks ar.Hooks) (ar.Result, error) {
	read := hooks.Execute(ctx, ar.Call{ID: "read", Name: "get_account_entity", Arguments: json.RawMessage(`{"advertiser_id":"sandbox_adv_north","level":"campaign","id":"campaign_example_1"}`), Round: 1})
	if !read.OK {
		return ar.Result{}, errors.New(read.Error)
	}
	staged := hooks.Execute(ctx, ar.Call{ID: "stage", Name: "stage_account_budget_change", Arguments: json.RawMessage(`{"advertiser_id":"sandbox_adv_north","level":"campaign","id":"campaign_example_1","budget":"60","currency":"USD","reason":"portfolio persistence acceptance"}`), Round: 2})
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

func TestSandboxAgentApprovalRoundTrip(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	application := openSandboxEnvironment(t, dir, "approval", sandboxDraftRuntime{})
	defer application.Store.Close()
	application.Host.AutomaticMemoryCapture = false
	result, err := application.Host.Run(context.Background(), "agent-acceptance", "Change campaign_example_1 budget to 55 USD", nil)
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

func TestPortfolioSandboxPersistsAndIsolatesAdvertisers(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	first, err := OpenPortfolioSandboxRuntime(dir, "portfolio-persist", portfolioDraftRuntime{})
	if err != nil {
		t.Fatal(err)
	}
	result, err := first.Host.Run(context.Background(), "portfolio-session", "Set only Northstar's campaign budget to 60 USD", nil)
	if err != nil || result.Status != "completed" {
		t.Fatalf("turn=%#v err=%v", result, err)
	}
	changes, err := first.Store.Changes(context.Background(), "portfolio-session")
	if err != nil || len(changes) != 1 || changes[0].Source.AccountID != "sandbox_adv_north" {
		t.Fatalf("changes=%#v err=%v", changes, err)
	}
	if _, err = first.Scope.Apply(context.Background(), "portfolio-session", changes[0].ID, "sandbox-operator"); err != nil {
		t.Fatal(err)
	}
	if err = first.Store.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := OpenPortfolioSandboxRuntime(dir, "portfolio-persist", sandboxTestRuntime{})
	if err != nil {
		t.Fatal(err)
	}
	defer second.Store.Close()
	north, err := second.Scope.Get(context.Background(), "sandbox_adv_north", ads.Campaign, "campaign_example_1")
	if err != nil || north.Budget == nil || !north.Budget.Equal(decimal.NewFromInt(60)) {
		t.Fatalf("north=%#v err=%v", north, err)
	}
	home, err := second.Scope.Get(context.Background(), "sandbox_adv_home", ads.Campaign, "campaign_example_1")
	if err != nil || home.Budget == nil || home.Budget.Equal(decimal.NewFromInt(60)) {
		t.Fatalf("home advertiser was not isolated: %#v err=%v", home, err)
	}
}
