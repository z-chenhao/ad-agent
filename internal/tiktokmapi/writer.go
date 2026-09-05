package tiktokmapi

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/z-chenhao/ad-agent/internal/ads"
)

// Write implements the narrow host-only mutation contract used after a human
// approval. It sends exactly one object update and never retries a write.
func (b *Backend) Write(ctx context.Context, request ads.WriteRequest) ads.WriteOutcome {
	if err := ads.CheckContext(ctx); err != nil {
		return ads.WriteOutcome{State: "not_sent", Message: "request_cancelled"}
	}
	if err := ads.CheckEntity(request.Target, b.client.advertiserID); err != nil {
		return ads.WriteOutcome{State: "not_sent", Message: "invalid_target"}
	}
	path, body, err := b.writeRequest(request)
	if err != nil {
		return ads.WriteOutcome{State: "not_sent", Message: err.Error()}
	}
	requestID, err := b.client.post(ctx, path, body, nil)
	if err == nil {
		return ads.WriteOutcome{State: "acknowledged", RequestID: requestID}
	}
	var apiErr *Error
	if errors.As(err, &apiErr) {
		switch apiErr.Kind {
		case "credential":
			return ads.WriteOutcome{State: "not_sent", Message: "credential_unavailable"}
		case "business":
			return ads.WriteOutcome{State: "rejected", RequestID: apiErr.RequestID, Message: "platform_rejected"}
		default:
			return ads.WriteOutcome{State: "unknown", RequestID: apiErr.RequestID, Message: "remote_outcome_unknown"}
		}
	}
	return ads.WriteOutcome{State: "unknown", Message: "remote_outcome_unknown"}
}

func (b *Backend) writeRequest(request ads.WriteRequest) (string, any, error) {
	base := map[string]any{"advertiser_id": b.client.advertiserID}
	switch request.Kind {
	case string(ads.BudgetChange):
		if request.Budget == nil || !request.Budget.IsPositive() || request.Target.Level == ads.Ad {
			return "", nil, errors.New("budget_not_supported")
		}
		// json.RawMessage preserves decimal precision while encoding the API's numeric field.
		base["budget"] = json.RawMessage(request.Budget.String())
		switch request.Target.Level {
		case ads.Campaign:
			base["campaign_id"] = request.Target.ID
			return "/open_api/v1.3/campaign/update/", base, nil
		case ads.AdGroup:
			base["adgroup_id"] = request.Target.ID
			return "/open_api/v1.3/adgroup/update/", base, nil
		}
	case string(ads.StatusChange):
		if request.Status != "ENABLE" && request.Status != "DISABLE" {
			return "", nil, errors.New("unsupported_operation_status")
		}
		base["operation_status"] = request.Status
		switch request.Target.Level {
		case ads.Campaign:
			base["campaign_ids"] = []string{request.Target.ID}
			return "/open_api/v1.3/campaign/status/update/", base, nil
		case ads.AdGroup:
			base["adgroup_ids"] = []string{request.Target.ID}
			base["allow_partial_success"] = false
			return "/open_api/v1.3/adgroup/status/update/", base, nil
		case ads.Ad:
			base["ad_ids"] = []string{request.Target.ID}
			return "/open_api/v1.3/ad/status/update/", base, nil
		}
	}
	return "", nil, errors.New("unsupported_change")
}

var _ ads.Writer = (*Backend)(nil)
