package portfolio

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/z-chenhao/ad-agent/internal/ads"
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

func TestPortfolioAgentStagesAndAppliesIndependentAdvertiserChanges(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	one, _ := sandbox.NewAccountEnvironment("portfolio-test", "adv_one", "Advertiser One")
	two, _ := sandbox.NewAccountEnvironment("portfolio-test", "adv_two", "Advertiser Two")
	p, err := NewPortfolio("portfolio-test", "Test portfolio", s, []Binding{
		{Backend: one, Writer: one, Creator: one, Policy: ads.SandboxPolicy()},
		{Backend: two, Writer: two, Creator: two, Policy: ads.SandboxPolicy()},
	})
	if err != nil {
		t.Fatal(err)
	}
	model := fakeRuntime(func(ctx context.Context, request ar.Request, hooks ar.Hooks) (ar.Result, error) {
		for _, candidate := range request.Tools {
			if candidate.Name == "apply_change" {
				t.Fatal("runtime received apply authority")
			}
		}
		if !hooks.Execute(ctx, tool("list_advertisers", `{}`)).OK {
			t.Fatal("portfolio listing failed")
		}
		if !hooks.Execute(ctx, tool("get_portfolio_performance", `{"start_date":"2022-07-11","end_date":"2022-07-17"}`)).OK {
			t.Fatal("portfolio report failed")
		}
		for _, account := range []string{"adv_one", "adv_two"} {
			read := `{"advertiser_id":"` + account + `","level":"campaign","id":"campaign_example_1"}`
			if !hooks.Execute(ctx, tool("get_account_entity", read)).OK {
				t.Fatal("account drill-down failed", account)
			}
			stage := `{"advertiser_id":"` + account + `","level":"campaign","id":"campaign_example_1","budget":"55","currency":"USD","reason":"bounded portfolio test"}`
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
	result, err := host.Run(context.Background(), "portfolio_session", "Raise both campaign budgets to 55 USD", nil)
	if err != nil || result.Status != "completed" {
		t.Fatalf("run=%#v err=%v", result, err)
	}
	changes, err := s.Changes(context.Background(), "portfolio_session")
	if err != nil || len(changes) != 2 || changes[0].Source.AccountID == changes[1].Source.AccountID {
		t.Fatalf("changes=%#v err=%v", changes, err)
	}
	for _, backend := range []*sandbox.Backend{one, two} {
		entity, _ := backend.Get(context.Background(), ads.Campaign, "campaign_example_1")
		if entity.Budget.String() != "50" {
			t.Fatal("model turn applied a write")
		}
	}
	for _, change := range changes {
		applied, applyErr := p.Apply(context.Background(), "portfolio_session", change.ID, "test-operator")
		if applyErr != nil || applied.State != ads.Applied {
			t.Fatalf("apply=%#v err=%v", applied, applyErr)
		}
	}
	for _, backend := range []*sandbox.Backend{one, two} {
		entity, _ := backend.Get(context.Background(), ads.Campaign, "campaign_example_1")
		if entity.Budget.String() != "55" {
			t.Fatal("approved account change was not applied")
		}
	}
	if _, err := p.Get(context.Background(), "outside", ads.Campaign, "campaign_example_1"); err == nil {
		t.Fatal("out-of-scope advertiser was accepted")
	}
}

func TestPortfolioReportPreservesAccountBoundaries(t *testing.T) {
	dir := t.TempDir()
	os.Chmod(dir, 0700)
	s, _ := store.Open(dir)
	defer s.Close()
	one, _ := sandbox.NewAccountEnvironment("portfolio-report", "adv_one", "Advertiser One")
	two, _ := sandbox.NewAccountEnvironment("portfolio-report", "adv_two", "Advertiser Two")
	p, _ := NewPortfolio("portfolio-report", "Test portfolio", s, []Binding{{Backend: one}, {Backend: two}})
	report, err := p.Performance(context.Background(), "2022-07-11", "2022-07-17")
	if err != nil || len(report.Accounts) != 2 || len(report.Limitations) == 0 {
		t.Fatalf("report=%#v err=%v", report, err)
	}
	if report.Accounts[0].Account.Currency == "" || report.Accounts[0].Account.Timezone == "" {
		t.Fatal("account reporting semantics were lost")
	}
}

func TestPortfolioCreationStaysInSelectedAdvertiser(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	one, _ := sandbox.NewAccountEnvironment("portfolio-create", "adv_one", "Advertiser One")
	two, _ := sandbox.NewAccountEnvironment("portfolio-create", "adv_two", "Advertiser Two")
	p, err := NewPortfolio("portfolio-create", "Test portfolio", s, []Binding{
		{Backend: one, Writer: one, Creator: one, Policy: ads.SandboxPolicy()},
		{Backend: two, Writer: two, Creator: two, Policy: ads.SandboxPolicy()},
	})
	if err != nil {
		t.Fatal(err)
	}
	session := store.Session{ID: "portfolio_create_session", Source: p.Source(), Provenance: map[string]store.Seen{}}
	change, err := p.StageCreate(context.Background(), session, "adv_one", ads.CreateRequest{Level: ads.Campaign, Name: "Portfolio launch", Status: "DISABLE"}, "isolated creation test")
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

func TestPortfolioAnalysisDelegateHasNoMutationAuthority(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	backend, _ := sandbox.NewAccountEnvironment("portfolio-analysis", "adv_one", "Advertiser One")
	p, _ := NewPortfolio("portfolio-analysis", "Test portfolio", s, []Binding{{Backend: backend, Writer: backend, Creator: backend, Policy: ads.SandboxPolicy()}})
	runtime := fakeRuntime(func(ctx context.Context, request ar.Request, hooks ar.Hooks) (ar.Result, error) {
		if len(request.Tools) == 1 && request.Tools[0].Name == "submit_portfolio_analysis" {
			result := hooks.Execute(ctx, tool("submit_portfolio_analysis", `{"summary":"Advertiser One needs campaign drill-down.","prioritized_accounts":["adv_one"],"limitations":["Sandbox evidence only."]}`))
			if !result.OK {
				t.Fatal("analysis submission failed", result.Error)
			}
			return ar.Result{Stop: "tool"}, nil
		}
		for _, candidate := range request.Tools {
			if candidate.Name == "submit_portfolio_analysis" {
				t.Fatal("analysis submission tool leaked into parent")
			}
		}
		reportResult := hooks.Execute(ctx, tool("get_portfolio_performance", `{"start_date":"2022-07-11","end_date":"2022-07-17"}`))
		if !reportResult.OK {
			t.Fatal(reportResult.Error)
		}
		var report PerformanceReport
		if err := json.Unmarshal(reportResult.Data, &report); err != nil {
			t.Fatal(err)
		}
		analysis := hooks.Execute(ctx, tool("run_portfolio_analysis", `{"question":"Which account needs attention?","report_id":"`+report.ID+`"}`))
		if !analysis.OK || !strings.Contains(string(analysis.Data), "adv_one") {
			t.Fatal("bounded analysis failed", analysis.Error, string(analysis.Data))
		}
		return ar.Result{Text: "Analysis complete.", Stop: "stop"}, nil
	})
	host, err := NewHost(p, runtime)
	if err != nil {
		t.Fatal(err)
	}
	result, err := host.Run(context.Background(), "analysis_session", "Prioritize this portfolio", nil)
	if err != nil || result.Status != "completed" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}
