package sandbox

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/z-chenhao/ad-agent/internal/ads"
)

func TestSavedAudienceDefinitionControlsMembership(t *testing.T) {
	b, err := NewEnvironment("audience-definition")
	if err != nil {
		t.Fatal(err)
	}
	b.operations.AudienceDefinitions["new-us"] = ads.AudienceCreateSpec{Kind: "saved", LocationIDs: []string{"US"}, Languages: []string{"en"}}
	if !b.audienceMatches("new-us", Cohort{Geo: "US", Language: "en"}) || b.audienceMatches("new-us", Cohort{Geo: "GB", Language: "en"}) || b.audienceMatches("unknown", Cohort{Geo: "US", Language: "en"}) {
		t.Fatal("membership ignores definitions")
	}
	settings := ads.AdGroupUpdateSpec{ExcludedAudienceIDs: []string{"new-us"}}
	b.operations.AdGroups["adgroup_broad_us"] = settings
	ad := b.entities["ad_prospect_creator"]
	group := b.entities[ad.ParentID]
	campaign := b.entities[group.ParentID]
	cohort := b.model.Config.Cohorts[0]
	if ok, reason := b.eligibility(ad, group, campaign, cohort, b.clock); ok || reason != "audience_exclusion" {
		t.Fatalf("exclusion=%v %s", ok, reason)
	}
	cohort.Geo = "GB"
	if ok, reason := b.eligibility(ad, group, campaign, cohort, b.clock); !ok {
		t.Fatalf("excluded unrelated cohort: %s", reason)
	}
}

func TestLifetimeBudgetPersistsAcrossDayBoundaries(t *testing.T) {
	b, err := NewEnvironment("lifetime-cap")
	if err != nil {
		t.Fatal(err)
	}
	b.model = newSimulationModel(DefaultSimulationConfig())
	budget := decimal.NewFromInt(10)
	campaign := b.entities["campaign_prospect_us"]
	campaign.Budget = &budget
	campaign.BudgetMode = "BUDGET_MODE_TOTAL"
	b.entities[campaign.ID] = campaign
	group := b.entities["adgroup_broad_us"]
	b.recordSpend(group, campaign, b.clock, 9)
	for _, days := range []int{1, 31, 90} {
		if got := b.remainingEntityBudget(campaign, b.clock.AddDate(0, 0, days)); got != 1 {
			t.Fatalf("lifetime remaining=%v", got)
		}
	}
	b.recordSpend(group, campaign, b.clock.AddDate(0, 0, 31), 1)
	if got := b.remainingBudget(group, campaign, b.clock.AddDate(0, 0, 32)); got != 0 {
		t.Fatalf("exhausted lifetime remaining=%v", got)
	}
}

func TestReachUsesQueryWindowAndDeduplicatesCohorts(t *testing.T) {
	b, err := NewEnvironment("reach-window")
	if err != nil {
		t.Fatal(err)
	}
	b.model.ReportExposure = map[string]map[string]int64{
		"ad_prospect_creator/2026-09-01": {"broad_home": 100000},
		"ad_prospect_creator/2026-09-02": {"broad_home": 100000},
	}
	one := b.reportReach(ads.Ad, "ad_prospect_creator", "2026-09-01", "2026-09-01")
	two := b.reportReach(ads.Ad, "ad_prospect_creator", "2026-09-02", "2026-09-02")
	both := b.reportReach(ads.Ad, "ad_prospect_creator", "2026-09-01", "2026-09-02")
	if one == nil || two == nil || both == nil || *one != *two || *both <= *one || *both >= *one+*two {
		t.Fatalf("invalid window reach: %v %v %v", one, two, both)
	}
}

func TestApprovedRulesExecuteOnVirtualClock(t *testing.T) {
	b, err := NewEnvironment("rule-execution")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	request := ads.OperationRequest{Kind: ads.CreateAutomatedRule, Rule: &ads.AutomatedRuleCreateSpec{Name: "Pause after spend", TargetLevel: ads.AdGroup, TargetIDs: []string{"adgroup_broad_us"}, Conditions: []ads.RuleCondition{{Metric: "SPEND", Operator: "GT", Value: decimal.Zero, Window: "LAST_7_DAYS"}}, Action: "PAUSE", Schedule: "EVERY_30_MINUTES"}}
	plan, err := b.PrepareOperation(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	outcome := b.ApplyOperation(ctx, plan)
	if outcome.State != "acknowledged" {
		t.Fatal(outcome)
	}
	_, _, err = b.Advance(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	group, _ := b.Get(ctx, ads.AdGroup, "adgroup_broad_us")
	results, err := b.ListAutomatedRuleResults(ctx, outcome.Resources[0].ID)
	if err != nil || group.Status != "DISABLE" || len(results) != 2 {
		t.Fatalf("rule failed: group=%s results=%+v err=%v", group.Status, results, err)
	}
	if results[0].ExecutedAt.Sub(results[1].ExecutedAt) != 30*time.Minute {
		t.Fatal("rule cadence is not half hourly")
	}
}

func TestSourceCreationDoesNotInventEvents(t *testing.T) {
	b, err := NewEnvironment("empty-source")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	plan, err := b.PrepareOperation(ctx, ads.OperationRequest{Kind: ads.CreateEventSource, EventSource: &ads.EventSourceCreateSpec{Name: "Unused pixel", Kind: "pixel", EventTypes: []string{"Purchase"}}})
	if err != nil {
		t.Fatal(err)
	}
	created := b.ApplyOperation(ctx, plan)
	if created.State != "acknowledged" {
		t.Fatal(created)
	}
	stats, err := b.GetEventStats(ctx, created.Resources[0].ID, "2026-08-28", "2026-09-03")
	if err != nil || stats.Complete || stats.Events["Purchase"] != 0 {
		t.Fatalf("invented telemetry: %+v %v", stats, err)
	}
}

func TestSimulationRestoreRejectsMissingOrUnknownSchema(t *testing.T) {
	b, err := NewEnvironment("schema-validation")
	if err != nil {
		t.Fatal(err)
	}
	state := b.simulationStateLocked()
	state.Model = nil
	if err := b.RestoreSimulation(&state, nil); err == nil {
		t.Fatal("missing model silently reset causal state")
	}
	state = b.simulationStateLocked()
	state.Model.Version = "unsupported-schema"
	if err := b.RestoreSimulation(&state, nil); err == nil {
		t.Fatal("unknown model silently reset causal state")
	}
}
