package app

import (
	"context"
	"os"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/z-chenhao/ad-agent/internal/ads"
	"github.com/z-chenhao/ad-agent/internal/store"
)

func TestOperationsEnforceBudgetPolicyAtStageAndApply(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	a := openSandboxEnvironment(t, dir, "budget-policy", sandboxTestRuntime{})
	defer a.Store.Close()
	ctx := context.Background()
	account, _ := a.Backend.Account(ctx)
	session := store.Session{ID: "policy", Source: account.Source}
	huge := decimal.NewFromInt(1000000)
	for _, request := range []ads.OperationRequest{
		{Kind: ads.UpdateAdGroup, AdGroupUpdate: &ads.AdGroupUpdateSpec{AdGroupID: "adgroup_broad_us", Budget: &huge}},
		{Kind: ads.CreateAutomatedRule, Rule: &ads.AutomatedRuleCreateSpec{Name: "Unsafe", TargetLevel: ads.AdGroup, TargetIDs: []string{"adgroup_broad_us"}, Conditions: []ads.RuleCondition{{Metric: "SPEND", Operator: "GT", Value: decimal.Zero, Window: "TODAY"}}, Action: "CHANGE_BUDGET", ActionValue: &huge, Schedule: "EVERY_30_MINUTES"}},
	} {
		if _, err := a.Changes.StageOperation(ctx, session, request, "review"); err == nil {
			t.Fatal("accepted out-of-policy operation")
		}
	}
	before, _ := a.Backend.Get(ctx, ads.AdGroup, "adgroup_broad_us")
	excessDelta := before.Budget.Mul(decimal.RequireFromString("1.21"))
	if _, err := a.Changes.StageOperation(ctx, session, ads.OperationRequest{Kind: ads.UpdateAdGroup, AdGroupUpdate: &ads.AdGroupUpdateSpec{AdGroupID: before.ID, Budget: &excessDelta}}, "over percentage cap"); err == nil {
		t.Fatal("typed update bypassed percentage guardrail")
	}
	valid := before.Budget.Mul(decimal.RequireFromString("1.1"))
	request := ads.OperationRequest{Kind: ads.UpdateAdGroup, AdGroupUpdate: &ads.AdGroupUpdateSpec{AdGroupID: before.ID, Budget: &valid}}
	draft, err := a.Changes.StageOperation(ctx, session, request, "approved increase")
	if err != nil {
		t.Fatal(err)
	}
	a.Changes.Policy.MaxBudget = decimal.NewFromInt(1)
	result, err := a.Changes.Apply(ctx, session.ID, draft.ID, "operator")
	if err != nil {
		t.Fatal(err)
	}
	observed, _ := a.Backend.Get(ctx, ads.AdGroup, before.ID)
	if result.State != ads.Failed || !observed.Budget.Equal(*before.Budget) {
		t.Fatal("apply ignored tightened policy")
	}
}
