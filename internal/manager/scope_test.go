package manager

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/z-chenhao/ad-agent/internal/ads"
	"github.com/z-chenhao/ad-agent/internal/agenthost"
	ar "github.com/z-chenhao/ad-agent/internal/runtime"
	"github.com/z-chenhao/ad-agent/internal/sandbox"
	"github.com/z-chenhao/ad-agent/internal/store"
)

type fakeRuntime func(context.Context, ar.Request, ar.Hooks) (ar.Result, error)

func (f fakeRuntime) Run(ctx context.Context, request ar.Request, hooks ar.Hooks) (ar.Result, error) {
	return f(ctx, request, hooks)
}

func tool(name, arguments string) ar.Call {
	return ar.Call{ID: store.ID("call"), Name: name, Arguments: json.RawMessage(arguments), Round: 1}
}

func TestManagerAgentStagesAndAppliesIndependentAdvertiserChanges(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	one, _ := sandbox.NewAccountEnvironment("manager-test", "adv_one", "Advertiser One")
	two, _ := sandbox.NewAccountEnvironment("manager-test", "adv_two", "Advertiser Two")
	p, err := NewScope("manager-test", "Test manager", s, []Binding{
		{Backend: one, Writer: one, Creator: one, Policy: ads.SandboxPolicy()},
		{Backend: two, Writer: two, Creator: two, Policy: ads.SandboxPolicy()},
	})
	if err != nil {
		t.Fatal(err)
	}
	model := fakeRuntime(func(ctx context.Context, request ar.Request, hooks ar.Hooks) (ar.Result, error) {
		if request.MaxRounds != 0 {
			t.Fatalf("manager main agent round limit = %d, want 0", request.MaxRounds)
		}
		for _, candidate := range request.Tools {
			if candidate.Name == "apply_change" {
				t.Fatal("runtime received apply authority")
			}
		}
		if !hooks.Execute(ctx, tool("list_advertisers", `{}`)).OK {
			t.Fatal("manager listing failed")
		}
		if !hooks.Execute(ctx, tool("get_manager_performance", `{"start_date":"2026-08-28","end_date":"2026-09-03"}`)).OK {
			t.Fatal("manager report failed")
		}
		for _, account := range []string{"adv_one", "adv_two"} {
			read := `{"advertiser_id":"` + account + `","level":"campaign","id":"campaign_prospect_us"}`
			if !hooks.Execute(ctx, tool("get_account_entity", read)).OK {
				t.Fatal("account drill-down failed", account)
			}
			stage := `{"advertiser_id":"` + account + `","level":"campaign","id":"campaign_prospect_us","budget":"860","currency":"USD","reason":"bounded manager test"}`
			if !hooks.Execute(ctx, tool("stage_account_budget_change", stage)).OK {
				t.Fatal("account staging failed", account)
			}
		}
		return ar.Result{Text: "Two advertiser drafts await approval.", Stop: "stop"}, nil
	})
	host, err := NewHost(p, model)
	if err != nil {
		t.Fatal(err)
	}
	result, err := host.Run(context.Background(), "manager_session", "Raise both campaign budgets to 860 USD", nil)
	if err != nil || result.Status != "completed" {
		t.Fatalf("run=%#v err=%v", result, err)
	}
	if len(result.Cards) != 2 || result.Cards[0].Change == nil || result.Cards[1].Change == nil {
		t.Fatalf("manager stages did not render exact previews: %#v", result.Cards)
	}
	changes, err := s.Changes(context.Background(), "manager_session")
	if err != nil || len(changes) != 2 || changes[0].Source.AccountID == changes[1].Source.AccountID {
		t.Fatalf("changes=%#v err=%v", changes, err)
	}
	for _, backend := range []*sandbox.Backend{one, two} {
		entity, _ := backend.Get(context.Background(), ads.Campaign, "campaign_prospect_us")
		if entity.Budget.String() != "850" {
			t.Fatal("model turn applied a write")
		}
	}
	for _, change := range changes {
		applied, applyErr := p.Apply(context.Background(), "manager_session", change.ID, "test-operator")
		if applyErr != nil || applied.State != ads.Applied {
			t.Fatalf("apply=%#v err=%v", applied, applyErr)
		}
	}
	for _, backend := range []*sandbox.Backend{one, two} {
		entity, _ := backend.Get(context.Background(), ads.Campaign, "campaign_prospect_us")
		if entity.Budget.String() != "860" {
			t.Fatal("approved account change was not applied")
		}
	}
	if _, err := p.Get(context.Background(), "outside", ads.Campaign, "campaign_prospect_us"); err == nil {
		t.Fatal("out-of-scope advertiser was accepted")
	}
}

func TestManagerReportPreservesAccountBoundaries(t *testing.T) {
	dir := t.TempDir()
	os.Chmod(dir, 0700)
	s, _ := store.Open(dir)
	defer s.Close()
	one, _ := sandbox.NewAccountEnvironment("manager-report", "adv_one", "Advertiser One")
	two, _ := sandbox.NewAccountEnvironment("manager-report", "adv_two", "Advertiser Two")
	p, _ := NewScope("manager-report", "Test manager", s, []Binding{{Backend: one}, {Backend: two}})
	report, err := p.Performance(context.Background(), "2026-08-28", "2026-09-03")
	if err != nil || len(report.Accounts) != 2 || len(report.Limitations) == 0 {
		t.Fatalf("report=%#v err=%v", report, err)
	}
	if report.Accounts[0].Account.Currency == "" || report.Accounts[0].Account.Timezone == "" {
		t.Fatal("account reporting semantics were lost")
	}
	if report.Accounts[0].Metrics.Spend.Equal(report.Accounts[1].Metrics.Spend) {
		t.Fatal("independent advertiser bindings reused identical delivery facts")
	}
}

func TestManagerCreationStaysInSelectedAdvertiser(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	one, _ := sandbox.NewAccountEnvironment("manager-create", "adv_one", "Advertiser One")
	two, _ := sandbox.NewAccountEnvironment("manager-create", "adv_two", "Advertiser Two")
	p, err := NewScope("manager-create", "Test manager", s, []Binding{
		{Backend: one, Writer: one, Creator: one, Policy: ads.SandboxPolicy()},
		{Backend: two, Writer: two, Creator: two, Policy: ads.SandboxPolicy()},
	})
	if err != nil {
		t.Fatal(err)
	}
	session := store.Session{ID: "manager_create_session", Source: p.Source(), Provenance: map[string]store.Seen{}}
	change, err := p.StageCreate(context.Background(), session, "adv_one", ads.CreateRequest{Level: ads.Campaign, Name: "Manager launch", Status: "DISABLE"}, "isolated creation test")
	if err != nil || change.State != ads.Staged {
		t.Fatalf("change=%#v err=%v", change, err)
	}
	applied, err := p.Apply(context.Background(), session.ID, change.ID, "test-operator")
	if err != nil || applied.Created == nil {
		t.Fatalf("applied=%#v err=%v", applied, err)
	}
	if _, err = one.Get(context.Background(), ads.Campaign, applied.Created.ID); err != nil {
		t.Fatal("selected advertiser did not receive creation", err)
	}
	if _, err = two.Get(context.Background(), ads.Campaign, applied.Created.ID); !errors.Is(err, ads.ErrNotFound) {
		t.Fatal("creation leaked into another advertiser", err)
	}
}

func TestManagerAnalysisDelegateHasNoMutationAuthority(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	backend, _ := sandbox.NewAccountEnvironment("manager-analysis", "adv_one", "Advertiser One")
	p, _ := NewScope("manager-analysis", "Test manager", s, []Binding{{Backend: backend, Writer: backend, Creator: backend, Policy: ads.SandboxPolicy()}})
	runtime := fakeRuntime(func(ctx context.Context, request ar.Request, hooks ar.Hooks) (ar.Result, error) {
		if len(request.Tools) == 1 && request.Tools[0].Name == "submit_manager_analysis" {
			result := hooks.Execute(ctx, tool("submit_manager_analysis", `{"summary":"Advertiser One needs campaign drill-down.","prioritized_accounts":["adv_one"],"limitations":["Sandbox evidence only."]}`))
			if !result.OK {
				t.Fatal("analysis submission failed", result.Error)
			}
			return ar.Result{Stop: "tool"}, nil
		}
		for _, candidate := range request.Tools {
			if candidate.Name == "submit_manager_analysis" {
				t.Fatal("analysis submission tool leaked into parent")
			}
		}
		reportResult := hooks.Execute(ctx, tool("get_manager_performance", `{"start_date":"2026-08-28","end_date":"2026-09-03"}`))
		if !reportResult.OK {
			t.Fatal(reportResult.Error)
		}
		var report AccountSummaryReport
		if err := json.Unmarshal(reportResult.Data, &report); err != nil {
			t.Fatal(err)
		}
		analysis := hooks.Execute(ctx, tool("run_manager_analysis", `{"question":"Which account needs attention?","report_id":"`+report.ID+`"}`))
		if !analysis.OK || !strings.Contains(string(analysis.Data), "adv_one") {
			t.Fatal("bounded analysis failed", analysis.Error, string(analysis.Data))
		}
		return ar.Result{Text: "Analysis complete.", Stop: "stop"}, nil
	})
	host, err := NewHost(p, runtime)
	if err != nil {
		t.Fatal(err)
	}
	result, err := host.Run(context.Background(), "analysis_session", "Prioritize this manager", nil)
	if err != nil || result.Status != "completed" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func TestManagerPromptIndexesAndLoadsSkillOnDemand(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	backend, _ := sandbox.NewAccountEnvironment("manager-prompt", "adv_one", "Advertiser One")
	p, err := NewScope("manager-prompt", "Prompt manager", s, []Binding{{Backend: backend, Writer: backend, Creator: backend, Policy: ads.SandboxPolicy()}})
	if err != nil {
		t.Fatal(err)
	}
	runtime := fakeRuntime(func(ctx context.Context, request ar.Request, hooks ar.Hooks) (ar.Result, error) {
		for _, want := range []string{"Cards are already visible product results, not work to announce.", "never screen position.", "Do not claim a card exists when its presentation failed."} {
			if !strings.Contains(request.System, want) {
				t.Fatalf("manager system missing layout-independent presentation rule %q", want)
			}
		}
		for _, want := range []string{"# Ad Agent", "# Manager workspace", "`manager-operations`", "<runtime_context>", "<manager_data>", "<view_context>", `"account_id":"adv_one"`, "navigation hint only"} {
			if !strings.Contains(request.System+request.Prompt, want) {
				t.Fatalf("compiled manager request missing %q", want)
			}
		}
		if strings.Contains(request.System, "## Cross-account triage") {
			t.Fatal("manager skill body was preloaded into the stable system prompt")
		}
		loaded := hooks.Execute(ctx, tool("load_skill", `{"name":"manager-operations"}`))
		if !loaded.OK || !strings.Contains(string(loaded.Data), "## Cross-account triage") || strings.Contains(string(loaded.Data), "---\\nname:") {
			t.Fatalf("manager skill was not loaded on demand: %#v", loaded)
		}
		return ar.Result{Text: "ready", Stop: "stop"}, nil
	})
	host, err := NewHost(p, runtime)
	if err != nil {
		t.Fatal(err)
	}
	if words := len(strings.Fields(host.system)); words > 900 {
		t.Fatalf("manager system prompt has %d words, want at most 900", words)
	}
	view := agenthost.ViewContext{Page: "campaigns", AccountID: "adv_one", AccountName: "Advertiser One", StartDate: "2026-08-28", EndDate: "2026-09-03"}
	if _, err = host.RunWithModelAndView(context.Background(), "manager_prompt_session", "Inspect this account", ar.ModelSelection{}, view, nil); err != nil {
		t.Fatal(err)
	}
	if _, err = host.RunWithModelAndView(context.Background(), "manager_outside", "Inspect this account", ar.ModelSelection{}, agenthost.ViewContext{Page: "campaigns", AccountID: "outside"}, nil); err == nil || err.Error() != "view_account_outside_manager_scope" {
		t.Fatalf("outside view context err=%v", err)
	}
}
