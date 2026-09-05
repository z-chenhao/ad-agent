package sandbox

import (
	"context"
	"encoding/json"
	"reflect"

	"github.com/z-chenhao/ad-agent/internal/ads"
)

func equalJSON(a, b any) bool {
	left, _ := json.Marshal(a)
	right, _ := json.Marshal(b)
	return string(left) == string(right)
}

func reviewMatches(request, current ads.OperationRequest) bool {
	for _, line := range ads.OperationReviewLines(request, current) {
		if line.Before != line.After {
			return false
		}
	}
	return true
}

func (f *Backend) reconcileBundle(ctx context.Context, spec ads.CampaignBundleSpec, outcome ads.OperationOutcome) (bool, error) {
	if len(outcome.Resources) != 2+len(spec.Ads) {
		return false, nil
	}
	r := outcome.Resources
	if r[0].Kind != "campaign" || r[1].Kind != "ad_group" {
		return false, nil
	}
	c, err := f.Get(ctx, ads.Campaign, r[0].ID)
	if err != nil {
		return false, err
	}
	if c.Name != spec.Campaign.Name || c.Objective != spec.Campaign.Objective || c.Status != "DISABLE" || c.BudgetMode != spec.Campaign.BudgetMode || !equalJSON(c.Budget, spec.Campaign.Budget) {
		return false, nil
	}
	g, err := f.Get(ctx, ads.AdGroup, r[1].ID)
	if err != nil {
		return false, err
	}
	if g.ParentID != c.ID || g.Name != spec.AdGroup.Name || g.Status != "DISABLE" || g.BudgetMode != spec.AdGroup.BudgetMode || !equalJSON(g.Budget, &spec.AdGroup.Budget) {
		return false, nil
	}
	state := f.OperationState()
	if !equalJSON(state.AdGroupDefinitions[g.ID], spec.AdGroup) {
		return false, nil
	}
	seen := map[string]bool{c.ID: true, g.ID: true}
	for i, want := range spec.Ads {
		resource := r[i+2]
		if resource.Kind != "ad" || seen[resource.ID] {
			return false, nil
		}
		seen[resource.ID] = true
		ad, err := f.Get(ctx, ads.Ad, resource.ID)
		if err != nil {
			return false, err
		}
		if ad.ParentID != g.ID || ad.Name != want.Name || ad.Status != "DISABLE" {
			return false, nil
		}
		actual := state.Ads[ad.ID]
		expected := ads.AdCreativeUpdateSpec{AdID: ad.ID, IdentityID: want.IdentityID, IdentityType: want.IdentityType, AssetID: want.AssetID, AssetKind: want.AssetKind, PrimaryText: want.PrimaryText, CallToAction: want.CallToAction, DestinationURL: want.DestinationURL}
		if !reflect.DeepEqual(actual, expected) {
			return false, nil
		}
	}
	return true, nil
}
