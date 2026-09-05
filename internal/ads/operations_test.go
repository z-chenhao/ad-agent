package ads

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestOperationValidationRejectsUnsafeOrAmbiguousDrafts(t *testing.T) {
	budget := decimal.NewFromInt(100)
	base := OperationRequest{Kind: CreateCampaignBundle, CampaignBundle: &CampaignBundleSpec{
		Campaign: CampaignSpec{Name: "Launch", Objective: "TRAFFIC", Status: "DISABLE"},
		AdGroup:  AdGroupSpec{Name: "Prospecting", Budget: budget, BudgetMode: "BUDGET_MODE_DAY", BillingEvent: "CPC", OptimizationGoal: "CLICK", Pacing: "PACING_MODE_SMOOTH", ScheduleType: "SCHEDULE_START_END", ScheduleStart: "2026-09-06 00:00:00", Placements: []string{"PLACEMENT_TIKTOK"}, LocationIDs: []string{"US"}, AudienceIDs: []string{"aud-1"}, ExcludedAudienceIDs: []string{"aud-1"}, Status: "DISABLE"},
		Ads:      []AdCreativeSpec{{Name: "Creator", IdentityID: "identity-1", IdentityType: "CUSTOMIZED_USER", AssetID: "video-1", AssetKind: "video", PrimaryText: "A real product claim needs review.", CallToAction: "SHOP_NOW", DestinationURL: "https://example.com", Status: "DISABLE"}},
	}}
	if err := base.Validate(); err == nil {
		t.Fatal("expected overlapping audience validation failure")
	}
	base.CampaignBundle.AdGroup.ExcludedAudienceIDs = nil
	base.CampaignBundle.Ads[0].Status = "ENABLE"
	if err := base.Validate(); err == nil {
		t.Fatal("expected enabled creative validation failure")
	}
	base.CampaignBundle.Ads[0].Status = "DISABLE"
	base.CampaignBundle.Ads[0].DestinationURL = "http://example.com"
	if err := base.Validate(); err == nil {
		t.Fatal("expected non-HTTPS destination validation failure")
	}
}

func TestAdGroupScheduleUpdateRequiresOrderedTimes(t *testing.T) {
	request := OperationRequest{Kind: UpdateAdGroup, AdGroupUpdate: &AdGroupUpdateSpec{
		AdGroupID: "group-1", ScheduleStart: "2026-09-07 00:00:00", ScheduleEnd: "2026-09-06 00:00:00",
	}}
	if err := request.Validate(); err == nil {
		t.Fatal("accepted schedule end before start")
	}
	request.AdGroupUpdate.ScheduleEnd = "2026-09-08 00:00:00"
	request.AdGroupUpdate.Placements = []string{"PLACEMENT_TIKTOK"}
	if err := request.Validate(); err != nil {
		t.Fatalf("valid schedule and placement update rejected: %v", err)
	}
}
