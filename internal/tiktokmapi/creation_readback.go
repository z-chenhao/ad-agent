package tiktokmapi

import (
	"context"
	"encoding/json"
	"net/url"
	"reflect"
	"sort"

	"github.com/shopspring/decimal"
	"github.com/z-chenhao/ad-agent/internal/ads"
)

func (b *Backend) reconcileCampaignBundle(ctx context.Context, spec ads.CampaignBundleSpec, outcome ads.OperationOutcome) (bool, error) {
	if len(outcome.Resources) != 2+len(spec.Ads) {
		return false, nil
	}
	resources := outcome.Resources
	if resources[0].Kind != "campaign" || resources[1].Kind != "ad_group" {
		return false, nil
	}
	seen := map[string]bool{}
	for _, resource := range resources {
		if resource.ID == "" || seen[resource.ID] {
			return false, nil
		}
		seen[resource.ID] = true
	}
	campaign := map[string]any{"campaign_name": spec.Campaign.Name, "objective_type": spec.Campaign.Objective, "operation_status": "DISABLE"}
	putIf(campaign, "budget_mode", spec.Campaign.BudgetMode)
	putDecimal(campaign, "budget", spec.Campaign.Budget)
	if ok, err := b.verifyCreatedRecord(ctx, "campaign", "campaign_id", resources[0].ID, campaign); err != nil || !ok {
		return false, err
	}
	g := spec.AdGroup
	group := adGroupUpdateBody(b.client.advertiserID, ads.AdGroupUpdateSpec{AdGroupID: resources[1].ID, Budget: &g.Budget, Bid: g.Bid, ScheduleStart: g.ScheduleStart, ScheduleEnd: g.ScheduleEnd, Placements: g.Placements, LocationIDs: g.LocationIDs, Languages: g.Languages, AudienceIDs: g.AudienceIDs, ExcludedAudienceIDs: g.ExcludedAudienceIDs})
	delete(group, "advertiser_id")
	delete(group, "adgroup_id")
	group["campaign_id"] = resources[0].ID
	group["adgroup_name"] = g.Name
	group["operation_status"] = "DISABLE"
	group["budget_mode"] = g.BudgetMode
	group["billing_event"] = g.BillingEvent
	group["optimization_goal"] = g.OptimizationGoal
	group["pacing"] = g.Pacing
	group["schedule_type"] = g.ScheduleType
	putIf(group, "optimization_event", g.OptimizationEvent)
	putIf(group, "bid_type", g.BidType)
	putIf(group, "pixel_id", g.PixelID)
	putIf(group, "gender", g.Gender)
	putSlice(group, "age_groups", g.AgeGroups)
	if ok, err := b.verifyCreatedRecord(ctx, "adgroup", "adgroup_id", resources[1].ID, group); err != nil || !ok {
		return false, err
	}
	for i, ad := range spec.Ads {
		resource := resources[i+2]
		if resource.Kind != "ad" {
			return false, nil
		}
		want := map[string]any{"adgroup_id": resources[1].ID, "ad_name": ad.Name, "operation_status": "DISABLE", "identity_id": ad.IdentityID, "identity_type": ad.IdentityType, "ad_text": ad.PrimaryText, "call_to_action": ad.CallToAction, "landing_page_url": ad.DestinationURL}
		if ad.AssetKind == "video" {
			want["video_id"] = ad.AssetID
		} else {
			want["image_ids"] = []string{ad.AssetID}
		}
		if ok, err := b.verifyCreatedRecord(ctx, "ad", "ad_id", resource.ID, want); err != nil || !ok {
			return false, err
		}
	}
	return true, nil
}

// Missing or normalized-away fields remain unconfirmed instead of becoming success.
func (b *Backend) verifyCreatedRecord(ctx context.Context, family, idField, id string, want map[string]any) (bool, error) {
	fields := []string{idField}
	for key := range want {
		fields = append(fields, key)
	}
	sort.Strings(fields)
	fieldJSON, _ := jsonQuery(fields)
	filter, _ := jsonQuery(map[string][]string{idField + "s": {id}})
	query := url.Values{"advertiser_id": {b.client.advertiserID}, "fields": {fieldJSON}, "filtering": {filter}, "page": {"1"}, "page_size": {"2"}}
	var data struct {
		List []map[string]json.RawMessage `json:"list"`
	}
	_, err := b.client.get(ctx, "/open_api/v1.3/"+family+"/get/", query, &data)
	if err != nil {
		return false, err
	}
	for _, row := range data.List {
		if err := b.checkReturnedAccount(row); err != nil {
			return false, err
		}
		var observedID string
		_ = json.Unmarshal(row[idField], &observedID)
		if observedID != id {
			continue
		}
		for field, value := range want {
			actual, exists := row[field]
			if !exists {
				return false, nil
			}
			expected, _ := json.Marshal(value)
			if !sameWireValue(actual, expected) {
				return false, nil
			}
		}
		return true, nil
	}
	return false, nil
}

func sameWireValue(actual, expected []byte) bool {
	var a, b any
	if json.Unmarshal(actual, &a) != nil || json.Unmarshal(expected, &b) != nil {
		return false
	}
	// Monetary JSON numbers and provider decimal strings are equivalent.
	if number, ok := b.(float64); ok {
		if text, ok := a.(string); ok {
			parsed, err := decimal.NewFromString(text)
			return err == nil && parsed.Equal(decimal.NewFromFloat(number))
		}
	}
	if x, ok := a.([]any); ok {
		if y, ok := b.([]any); ok {
			sort.Slice(x, func(i, j int) bool {
				p, _ := json.Marshal(x[i])
				q, _ := json.Marshal(x[j])
				return string(p) < string(q)
			})
			sort.Slice(y, func(i, j int) bool {
				p, _ := json.Marshal(y[i])
				q, _ := json.Marshal(y[j])
				return string(p) < string(q)
			})
			return reflect.DeepEqual(x, y)
		}
	}
	return reflect.DeepEqual(a, b)
}
