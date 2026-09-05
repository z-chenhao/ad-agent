package tiktokmapi

import (
	"context"
	"encoding/json"
	"github.com/z-chenhao/ad-agent/internal/ads"
)

// Read submitted definitions, not just resource existence. Unavailable fields or
// eventual consistency leave the operation unconfirmed for explicit reconciliation.
func (b *Backend) reconcileCreatedResource(ctx context.Context, request ads.OperationRequest, outcome ads.OperationOutcome) (bool, error) {
	if len(outcome.Resources) != 1 || outcome.Resources[0].ID == "" {
		return false, nil
	}
	r := outcome.Resources[0]
	path, key := "", ""
	want := map[string]any{}
	switch request.Kind {
	case ads.CreateAudience:
		if r.Kind != "audience" {
			return false, nil
		}
		v := request.Audience
		if v.Kind == "saved" {
			path, key = "/open_api/v1.3/dmp/saved_audience/list/", "saved_audience_id"
			want = map[string]any{"saved_audience_name": v.Name, "location_ids": v.LocationIDs}
			putSlice(want, "languages", v.Languages)
			putSlice(want, "age_groups", v.AgeGroups)
			putIf(want, "gender", v.Gender)
		} else {
			path, key = "/open_api/v1.3/dmp/custom_audience/get/", "custom_audience_id"
			size := "BROAD"
			if v.LookalikeRatio <= 2 {
				size = "NARROW"
			} else if v.LookalikeRatio <= 5 {
				size = "BALANCED"
			}
			want = map[string]any{"custom_audience_name": v.Name, "lookalike_spec": map[string]any{"source_audience_id": v.SourceAudienceID, "audience_size": size, "location_ids": v.LocationIDs, "placements": []string{"PLACEMENT_TIKTOK"}, "mobile_os": "ALL", "include_source": false}}
		}
	case ads.CreateAutomatedRule:
		if r.Kind != "automated_rule" {
			return false, nil
		}
		path, key = "/open_api/v1.3/optimizer/rule/list/", "rule_id"
		want = ruleWireDefinition(*request.Rule)
	case ads.CreateEventSource:
		if r.Kind != "event_source" {
			return false, nil
		}
		if request.EventSource.Kind == "pixel" {
			path, key = "/open_api/v1.3/pixel/list/", "pixel_id"
			want["pixel_name"] = request.EventSource.Name
		} else {
			path, key = "/open_api/v1.3/offline/get/", "offline_id"
			want["name"] = request.EventSource.Name
		}
	default:
		return false, nil
	}
	query := b.accountQuery()
	if key == "custom_audience_id" {
		filter, _ := jsonQuery(map[string][]string{"custom_audience_ids": {r.ID}})
		query.Set("filtering", filter)
	}
	rows, err := b.pagedObjects(ctx, path, query)
	if err != nil {
		return false, err
	}
	for _, row := range rows {
		if err := b.checkReturnedAccount(row); err != nil {
			return false, err
		}
		if rawString(row, key) != r.ID {
			continue
		}
		for field, value := range want {
			encoded, _ := json.Marshal(value)
			if !sameWireValue(row[field], encoded) {
				return false, nil
			}
		}
		return true, nil
	}
	return false, nil
}
