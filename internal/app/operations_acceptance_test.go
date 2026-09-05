package app

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/z-chenhao/ad-agent/internal/ads"
	"github.com/z-chenhao/ad-agent/internal/sandbox"
	"github.com/z-chenhao/ad-agent/internal/store"
)

func TestReadOnlyCompositionCanPlanButCannotExecuteOperations(t *testing.T) {
	backend, err := sandbox.NewEnvironment("read-only-operations")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err = os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	application, err := OpenBackendRuntime(dir, backend, sandboxTestRuntime{})
	if err != nil {
		t.Fatal(err)
	}
	defer application.Store.Close()
	if application.Changes.Planner == nil || application.Changes.Operator != nil || application.Changes.Creator != nil || application.Writable {
		t.Fatalf("read-only authority leak: %#v", application.Changes)
	}
	account, _ := application.Backend.Account(context.Background())
	session := store.Session{ID: "read-only-session", Source: account.Source, Provenance: map[string]store.Seen{}}
	change, err := application.Changes.StageOperation(context.Background(), session, ads.OperationRequest{Kind: ads.CreateEventSource, EventSource: &ads.EventSourceCreateSpec{Name: "Planning only", Kind: "pixel"}}, "prove read-only planning")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = application.Changes.Apply(context.Background(), session.ID, change.ID, "operator"); err == nil {
		t.Fatal("read-only composition executed an operation")
	}
}

func TestSandboxRuleAndBalanceEffectsSurviveRestart(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	a := openSandboxEnvironment(t, dir, "rule-restart", sandboxTestRuntime{})
	account, _ := a.Backend.Account(ctx)
	session := store.Session{ID: "rule-session", Source: account.Source}
	request := ads.OperationRequest{Kind: ads.CreateAutomatedRule, Rule: &ads.AutomatedRuleCreateSpec{Name: "Pause on spend", TargetLevel: ads.AdGroup, TargetIDs: []string{"adgroup_broad_us"}, Conditions: []ads.RuleCondition{{Metric: "SPEND", Operator: "GT", Value: decimal.Zero, Window: "LAST_7_DAYS"}}, Action: "PAUSE", Schedule: "EVERY_30_MINUTES"}}
	draft, err := a.Changes.StageOperation(ctx, session, request, "reviewed rule")
	if err != nil {
		t.Fatal(err)
	}
	applied, err := a.Changes.Apply(ctx, session.ID, draft.ID, "operator")
	if err != nil || applied.State != ads.Applied {
		t.Fatalf("apply=%+v err=%v", applied, err)
	}
	ruleID := applied.OperationOutcome.Resources[0].ID
	lease := "apply:" + account.Source.Backend + ":" + account.Source.Environment + ":" + account.ID
	if err := a.Store.Lease(ctx, lease, "held", time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Sandbox.Advance(ctx, 1); err == nil {
		t.Fatal("advance bypassed writer lease")
	}
	a.Store.Release(lease, "held")
	if _, err := a.Sandbox.Advance(ctx, 1); err != nil {
		t.Fatal(err)
	}
	before, err := a.Backend.(ads.OperationsReader).GetBillingBalance(ctx)
	if err != nil {
		t.Fatal(err)
	}
	a.Store.Close()
	b := openSandboxEnvironment(t, dir, "rule-restart", sandboxTestRuntime{})
	defer b.Store.Close()
	group, err := b.Backend.Get(ctx, ads.AdGroup, "adgroup_broad_us")
	if err != nil || group.Status != "DISABLE" {
		t.Fatalf("lost rule side effect: %v %v", group, err)
	}
	results, err := b.Backend.(ads.CommonAdsReader).ListAutomatedRuleResults(ctx, ruleID)
	if err != nil || len(results) != 2 {
		t.Fatalf("lost rule results: %v %v", results, err)
	}
	after, err := b.Backend.(ads.OperationsReader).GetBillingBalance(ctx)
	if err != nil || !after.Available.Equal(before.Available) {
		t.Fatal("balance did not persist")
	}
}

func TestSandboxDailyOperationsPersistAndReconcile(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	application := openSandboxEnvironment(t, dir, "daily-operations", sandboxTestRuntime{})
	account, _ := application.Backend.Account(ctx)
	session := store.Session{ID: "operations-session", Source: account.Source, Provenance: map[string]store.Seen{}}

	reader := application.Backend.(ads.OperationsReader)
	comments, err := reader.ListComments(ctx, "ad_prospect_creator", 20)
	if err != nil || len(comments) == 0 {
		t.Fatalf("comments=%#v err=%v", comments, err)
	}
	balance, err := reader.GetBillingBalance(ctx)
	if err != nil || !balance.Available.Equal(decimal.RequireFromString("7421.80")) {
		t.Fatalf("balance=%#v err=%v", balance, err)
	}

	budget := decimal.RequireFromString("120.00")
	bundle := ads.OperationRequest{Kind: ads.CreateCampaignBundle, CampaignBundle: &ads.CampaignBundleSpec{
		Campaign: ads.CampaignSpec{Name: "Autumn acquisition", Objective: "WEB_CONVERSIONS", Status: "DISABLE"},
		AdGroup:  ads.AdGroupSpec{Name: "US broad | Purchase", Budget: budget, BudgetMode: "BUDGET_MODE_DAY", BillingEvent: "OCPM", OptimizationGoal: "CONVERT", OptimizationEvent: "Purchase", Pacing: "PACING_MODE_SMOOTH", ScheduleType: "SCHEDULE_START_END", ScheduleStart: "2026-09-06 00:00:00", ScheduleEnd: "2026-09-30 23:59:59", Placements: []string{"PLACEMENT_TIKTOK"}, LocationIDs: []string{"US"}, AudienceIDs: []string{"audience_prospecting"}, ExcludedAudienceIDs: []string{"audience_purchasers"}, PixelID: "pixel_aster_web", Status: "DISABLE"},
		Ads:      []ads.AdCreativeSpec{{Name: "Room reset | creator", IdentityID: "identity_aster_pine", IdentityType: "CUSTOMIZED_USER", AssetID: "creative_creator_pov", AssetKind: "video", PrimaryText: "A calmer room starts with one shelf.", CallToAction: "SHOP_NOW", DestinationURL: "https://asterandpine.example/collections/modular", Status: "DISABLE"}},
	}}
	change, err := application.Changes.StageOperation(ctx, session, bundle, "Launch a fully reviewed disabled campaign bundle")
	if err != nil || len(change.Operation.Lines) < 20 {
		t.Fatalf("bundle draft=%#v err=%v", change, err)
	}
	change, err = application.Changes.Apply(ctx, session.ID, change.ID, "sandbox-operator")
	if err != nil || change.State != ads.Applied || len(change.OperationOutcome.Resources) != 3 {
		t.Fatalf("bundle apply=%#v err=%v", change, err)
	}
	createdAd := change.OperationOutcome.Resources[2].ID
	if detail, detailErr := application.Backend.(ads.AdDetailsReader).GetAdDetail(ctx, createdAd); detailErr != nil || detail.PrimaryText == "" || detail.Ad.Status != "DISABLE" {
		t.Fatalf("created ad detail=%#v err=%v", detail, detailErr)
	}

	operations := []ads.OperationRequest{
		{Kind: ads.CreateAudience, Audience: &ads.AudienceCreateSpec{Name: "US consideration", Kind: "saved", LocationIDs: []string{"US"}, Languages: []string{"en"}}},
		{Kind: ads.CreateAutomatedRule, Rule: &ads.AutomatedRuleCreateSpec{Name: "Notify on high CPA", TargetLevel: ads.AdGroup, TargetIDs: []string{"adgroup_broad_us"}, Conditions: []ads.RuleCondition{{Metric: "CPA", Operator: "GT", Value: decimal.RequireFromString("60"), Window: "LAST_3_DAYS"}}, Action: "NOTIFY", Schedule: "EVERY_30_MINUTES"}},
		{Kind: ads.ModerateComment, Comment: &ads.CommentActionSpec{CommentID: "comment_shipping", AdID: "ad_prospect_creator", TikTokItemID: "item_warm_loft", Action: "hide"}},
		{Kind: ads.CreateEventSource, EventSource: &ads.EventSourceCreateSpec{Name: "Pop-up store sales", Kind: "offline", EventTypes: []string{"Purchase"}}},
	}
	for index, request := range operations {
		draft, stageErr := application.Changes.StageOperation(ctx, session, request, "daily operation acceptance")
		if stageErr != nil {
			t.Fatalf("stage operation %d: %v", index, stageErr)
		}
		applied, applyErr := application.Changes.Apply(ctx, session.ID, draft.ID, "sandbox-operator")
		if applyErr != nil || applied.State != ads.Applied {
			t.Fatalf("apply operation %d=%#v err=%v", index, applied, applyErr)
		}
	}
	if err = application.Store.Close(); err != nil {
		t.Fatal(err)
	}

	reopened := openSandboxEnvironment(t, dir, "daily-operations", sandboxTestRuntime{})
	defer reopened.Store.Close()
	if _, err = reopened.Backend.Get(ctx, ads.Ad, createdAd); err != nil {
		t.Fatal("created bundle did not persist", err)
	}
	common := reopened.Backend.(ads.CommonAdsReader)
	audiences, _ := common.ListAudiences(ctx)
	rules, _ := common.ListAutomatedRules(ctx)
	sources, _ := common.ListEventSources(ctx)
	if len(audiences) != 4 || len(rules) != 3 || len(sources) != 3 {
		t.Fatalf("persisted resources audiences=%d rules=%d sources=%d", len(audiences), len(rules), len(sources))
	}
	comments, _ = reopened.Backend.(ads.OperationsReader).ListComments(ctx, "ad_prospect_creator", 20)
	foundHidden := false
	for _, item := range comments {
		foundHidden = foundHidden || item.ID == "comment_shipping" && item.Status == "HIDDEN"
	}
	if !foundHidden {
		t.Fatalf("comment action did not persist: %#v", comments)
	}
}
