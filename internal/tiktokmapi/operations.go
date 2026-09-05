package tiktokmapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	addomain "github.com/z-chenhao/ad-agent/internal/ads"
)

func operationFingerprint(values ...any) string {
	b, _ := json.Marshal(values)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func (b *Backend) PrepareOperation(ctx context.Context, request addomain.OperationRequest) (addomain.OperationPlan, error) {
	if err := request.Validate(); err != nil {
		return addomain.OperationPlan{}, err
	}
	plan := addomain.OperationPlan{Request: request, Lines: []addomain.ChangeLine{}}
	var reviewBefore any
	var conditions []any
	switch request.Kind {
	case addomain.CreateCampaignBundle:
		v := request.CampaignBundle
		identities, err := b.ListIdentities(ctx)
		if err != nil {
			return plan, err
		}
		assets, err := b.ListCreativeAssets(ctx)
		if err != nil {
			return plan, err
		}
		for _, ad := range v.Ads {
			if !identityUsable(identities, ad.IdentityID) || !assetUsable(assets, ad.AssetID) {
				return plan, errors.New("campaign bundle references an unavailable identity or asset")
			}
		}
		for _, id := range append(append([]string{}, v.AdGroup.AudienceIDs...), v.AdGroup.ExcludedAudienceIDs...) {
			if _, err = b.GetAudience(ctx, id); err != nil {
				return plan, errors.New("campaign bundle references an unavailable audience")
			}
		}
		if v.AdGroup.PixelID != "" {
			sources, e := b.ListEventSources(ctx)
			if e != nil {
				return plan, e
			}
			found := false
			for _, source := range sources {
				found = found || source.ID == v.AdGroup.PixelID
			}
			if !found {
				return plan, errors.New("campaign bundle references an unavailable event source")
			}
			conditions = append(conditions, sources)
		}
		conditions = append(conditions, identities, assets, v.AdGroup.AudienceIDs, v.AdGroup.ExcludedAudienceIDs)
	case addomain.UpdateAdGroup:
		v := request.AdGroupUpdate
		current, err := b.Get(ctx, addomain.AdGroup, v.AdGroupID)
		if err != nil {
			return plan, err
		}
		settings, err := b.getAdGroupOperation(ctx, v.AdGroupID)
		if err != nil {
			return plan, err
		}
		reviewBefore = addomain.OperationRequest{Kind: addomain.UpdateAdGroup, AdGroupUpdate: &settings}
		for _, id := range append(append([]string{}, v.AudienceIDs...), v.ExcludedAudienceIDs...) {
			if _, err = b.GetAudience(ctx, id); err != nil {
				return plan, errors.New("ad group update references an unavailable audience")
			}
		}
		if v.Budget != nil {
			plan.SpendIncreasing = current.Budget != nil && v.Budget.GreaterThan(*current.Budget)
		}
		conditions = []any{current, settings}
	case addomain.UpdateAdCreative:
		v := request.AdUpdate
		current, err := b.GetAdDetail(ctx, v.AdID)
		if err != nil {
			return plan, err
		}
		if v.IdentityID != "" {
			ids, e := b.ListIdentities(ctx)
			if e != nil || !identityUsable(ids, v.IdentityID) {
				return plan, errors.New("identity unavailable")
			}
		}
		if v.AssetID != "" {
			assets, e := b.ListCreativeAssets(ctx)
			if e != nil || !assetUsable(assets, v.AssetID) {
				return plan, errors.New("creative asset unavailable")
			}
		}
		reviewBefore = addomain.CreativeReviewBefore(current)
		conditions = []any{current}
	case addomain.CreateAudience:
		v := request.Audience
		if v.Kind == "lookalike" {
			source, err := b.GetAudience(ctx, v.SourceAudienceID)
			if err != nil || source.Kind != "custom" || source.Status != "READY" {
				return plan, errors.New("lookalike source unavailable")
			}
			conditions = []any{source}
		}
	case addomain.CreateAutomatedRule:
		v := request.Rule
		for _, id := range v.TargetIDs {
			entity, err := b.Get(ctx, v.TargetLevel, id)
			if err != nil {
				return plan, err
			}
			conditions = append(conditions, entity)
		}
		plan.SpendIncreasing = v.Action == "CHANGE_BUDGET"
	case addomain.ModerateComment:
		v := request.Comment
		items, err := b.ListComments(ctx, v.AdID, 100)
		if err != nil {
			return plan, err
		}
		var found *addomain.Comment
		for i := range items {
			if items[i].ID == v.CommentID {
				found = &items[i]
				break
			}
		}
		if found == nil || found.TikTokItemID != v.TikTokItemID {
			return plan, errors.New("comment not found in ad scope")
		}
		conditions = []any{found}
	case addomain.CreateEventSource:
		if len(request.EventSource.EventTypes) > 0 {
			return plan, errors.New("event types cannot be configured by source creation; configure tracking separately")
		}
	default:
		return plan, errors.New("unsupported TikTok operation")
	}
	plan.Lines = addomain.OperationReviewLines(request, reviewBefore)
	plan.PreconditionHash = operationFingerprint(conditions...)
	return plan, nil
}

func (b *Backend) ApplyOperation(ctx context.Context, plan addomain.OperationPlan) addomain.OperationOutcome {
	if err := ctx.Err(); err != nil {
		return addomain.OperationOutcome{State: "not_sent", Message: "request_cancelled"}
	}
	prepared, err := b.PrepareOperation(ctx, plan.Request)
	if err != nil || prepared.Version() != plan.Version() {
		return addomain.OperationOutcome{State: "not_sent", Message: "operation_revalidation_failed"}
	}
	switch plan.Request.Kind {
	case addomain.CreateCampaignBundle:
		return b.createCampaignBundle(ctx, *plan.Request.CampaignBundle)
	case addomain.UpdateAdGroup:
		v := plan.Request.AdGroupUpdate
		body := adGroupUpdateBody(b.client.advertiserID, *v)
		return b.singleOperation(ctx, "/open_api/v1.3/adgroup/update/", body, "ad_group", v.AdGroupID, "")
	case addomain.UpdateAdCreative:
		v := plan.Request.AdUpdate
		ad, readErr := b.Get(ctx, addomain.Ad, v.AdID)
		if readErr != nil {
			return addomain.OperationOutcome{State: "not_sent", Message: "ad_read_failed"}
		}
		creative := map[string]any{"ad_id": v.AdID}
		putIf(creative, "identity_id", v.IdentityID)
		putIf(creative, "identity_type", v.IdentityType)
		putIf(creative, "ad_text", v.PrimaryText)
		putIf(creative, "call_to_action", v.CallToAction)
		putIf(creative, "landing_page_url", v.DestinationURL)
		if v.AssetID != "" {
			if v.AssetKind == "video" {
				creative["video_id"] = v.AssetID
			} else {
				creative["image_ids"] = []string{v.AssetID}
			}
		}
		body := map[string]any{"advertiser_id": b.client.advertiserID, "adgroup_id": ad.ParentID, "creatives": []any{creative}, "patch_update": true}
		return b.singleOperation(ctx, "/open_api/v1.3/ad/update/", body, "ad", v.AdID, "")
	case addomain.CreateAudience:
		return b.createAudience(ctx, *plan.Request.Audience)
	case addomain.CreateAutomatedRule:
		return b.createRule(ctx, *plan.Request.Rule)
	case addomain.ModerateComment:
		return b.applyComment(ctx, *plan.Request.Comment)
	case addomain.CreateEventSource:
		v := plan.Request.EventSource
		path := "/open_api/v1.3/pixel/create/"
		body := map[string]any{"advertiser_id": b.client.advertiserID, "pixel_name": v.Name}
		idKey := "pixel_id"
		if v.Kind == "offline" {
			path = "/open_api/v1.3/offline/create/"
			body = map[string]any{"advertiser_id": b.client.advertiserID, "name": v.Name, "description": "Created through Ad Agent approval", "auto_tracking": false}
			idKey = "offline_id"
		}
		return b.createSingle(ctx, path, body, "event_source", v.Name, idKey)
	default:
		return addomain.OperationOutcome{State: "not_sent", Message: "unsupported_operation"}
	}
}

func adGroupUpdateBody(advertiserID string, v addomain.AdGroupUpdateSpec) map[string]any {
	body := map[string]any{"advertiser_id": advertiserID, "adgroup_id": v.AdGroupID}
	putDecimal(body, "budget", v.Budget)
	putDecimal(body, "bid_price", v.Bid)
	putIf(body, "schedule_start_time", v.ScheduleStart)
	putIf(body, "schedule_end_time", v.ScheduleEnd)
	putSlice(body, "placements", v.Placements)
	putSlice(body, "audience_ids", v.AudienceIDs)
	putSlice(body, "excluded_audience_ids", v.ExcludedAudienceIDs)
	putSlice(body, "location_ids", v.LocationIDs)
	putSlice(body, "languages", v.Languages)
	return body
}

func (b *Backend) createCampaignBundle(ctx context.Context, v addomain.CampaignBundleSpec) addomain.OperationOutcome {
	result := addomain.OperationOutcome{State: "acknowledged"}
	campaignBody := map[string]any{"advertiser_id": b.client.advertiserID, "campaign_name": v.Campaign.Name, "objective_type": v.Campaign.Objective, "operation_status": "DISABLE"}
	putIf(campaignBody, "budget_mode", v.Campaign.BudgetMode)
	putDecimal(campaignBody, "budget", v.Campaign.Budget)
	campaignID, req, err := b.postCreated(ctx, "/open_api/v1.3/campaign/create/", campaignBody, "campaign_id")
	if err != nil {
		return classifyOperationError(err)
	}
	result.RequestIDs = append(result.RequestIDs, req)
	result.Resources = append(result.Resources, addomain.OperationResource{Kind: "campaign", ID: campaignID, Name: v.Campaign.Name})
	groupBody := map[string]any{"advertiser_id": b.client.advertiserID, "campaign_id": campaignID, "adgroup_name": v.AdGroup.Name, "budget": json.RawMessage(v.AdGroup.Budget.String()), "budget_mode": v.AdGroup.BudgetMode, "billing_event": v.AdGroup.BillingEvent, "optimization_goal": v.AdGroup.OptimizationGoal, "pacing": v.AdGroup.Pacing, "schedule_type": v.AdGroup.ScheduleType, "schedule_start_time": v.AdGroup.ScheduleStart, "placements": v.AdGroup.Placements, "location_ids": v.AdGroup.LocationIDs, "operation_status": "DISABLE"}
	putIf(groupBody, "optimization_event", v.AdGroup.OptimizationEvent)
	putIf(groupBody, "bid_type", v.AdGroup.BidType)
	putDecimal(groupBody, "bid_price", v.AdGroup.Bid)
	putIf(groupBody, "schedule_end_time", v.AdGroup.ScheduleEnd)
	putSlice(groupBody, "languages", v.AdGroup.Languages)
	putSlice(groupBody, "age_groups", v.AdGroup.AgeGroups)
	putIf(groupBody, "gender", v.AdGroup.Gender)
	putSlice(groupBody, "audience_ids", v.AdGroup.AudienceIDs)
	putSlice(groupBody, "excluded_audience_ids", v.AdGroup.ExcludedAudienceIDs)
	putIf(groupBody, "pixel_id", v.AdGroup.PixelID)
	groupID, req, err := b.postCreated(ctx, "/open_api/v1.3/adgroup/create/", groupBody, "adgroup_id")
	if err != nil {
		return makePartial(result, err)
	}
	result.RequestIDs = append(result.RequestIDs, req)
	result.Resources = append(result.Resources, addomain.OperationResource{Kind: "ad_group", ID: groupID, Name: v.AdGroup.Name})
	creatives := make([]any, 0, len(v.Ads))
	for _, ad := range v.Ads {
		item := map[string]any{"ad_name": ad.Name, "identity_id": ad.IdentityID, "identity_type": ad.IdentityType, "ad_text": ad.PrimaryText, "call_to_action": ad.CallToAction, "landing_page_url": ad.DestinationURL, "operation_status": "DISABLE"}
		if ad.AssetKind == "video" {
			item["video_id"] = ad.AssetID
		} else {
			item["image_ids"] = []string{ad.AssetID}
		}
		creatives = append(creatives, item)
	}
	var data struct {
		AdIDs []string `json:"ad_ids"`
		Ads   []struct {
			AdID string `json:"ad_id"`
		} `json:"ads"`
	}
	req, err = b.client.post(ctx, "/open_api/v1.3/ad/create/", map[string]any{"advertiser_id": b.client.advertiserID, "adgroup_id": groupID, "creatives": creatives}, &data)
	if err != nil {
		return makePartial(result, err)
	}
	result.RequestIDs = append(result.RequestIDs, req)
	ids := data.AdIDs
	for _, item := range data.Ads {
		ids = append(ids, item.AdID)
	}
	if len(ids) != len(v.Ads) {
		result.State = "partial"
		result.Message = "ad create response did not identify every requested ad"
		return result
	}
	for i, id := range ids {
		result.Resources = append(result.Resources, addomain.OperationResource{Kind: "ad", ID: id, Name: v.Ads[i].Name})
	}
	return result
}

func (b *Backend) createAudience(ctx context.Context, v addomain.AudienceCreateSpec) addomain.OperationOutcome {
	if v.Kind == "saved" {
		body := map[string]any{"advertiser_id": b.client.advertiserID, "saved_audience_name": v.Name, "location_ids": v.LocationIDs}
		putSlice(body, "languages", v.Languages)
		putSlice(body, "age_groups", v.AgeGroups)
		putIf(body, "gender", v.Gender)
		return b.createSingle(ctx, "/open_api/v1.3/dmp/saved_audience/create/", body, "audience", v.Name, "saved_audience_id")
	}
	size := "BROAD"
	if v.LookalikeRatio <= 2 {
		size = "NARROW"
	} else if v.LookalikeRatio <= 5 {
		size = "BALANCED"
	}
	body := map[string]any{"advertiser_id": b.client.advertiserID, "custom_audience_name": v.Name, "lookalike_spec": map[string]any{"source_audience_id": v.SourceAudienceID, "audience_size": size, "location_ids": v.LocationIDs, "placements": []string{"PLACEMENT_TIKTOK"}, "mobile_os": "ALL", "include_source": false}}
	return b.createSingle(ctx, "/open_api/v1.3/dmp/custom_audience/lookalike/create/", body, "audience", v.Name, "custom_audience_id")
}

func ruleWireDefinition(v addomain.AutomatedRuleCreateSpec) map[string]any {
	conditions := make([]any, 0, len(v.Conditions))
	for _, c := range v.Conditions {
		conditions = append(conditions, map[string]any{
			"subject_type": ruleMetric(c.Metric), "calculation_type": "DEFAULT",
			"match_type": c.Operator, "range_type": ruleWindow(c.Window),
			"values": []string{c.Value.String()},
		})
	}
	action := map[string]any{"subject_type": ruleAction(v.Action)}
	if v.ActionValue != nil {
		action["action_type"] = "ADJUST_TO"
		action["value_type"] = "EXACT"
		action["value"] = map[string]any{"value": json.RawMessage(v.ActionValue.String())}
	}
	// The official v1.3 SDK create model has no status member. Do not invent an
	// undocumented wire field; live activation semantics remain an explicit
	// platform-validation item.
	return map[string]any{
		"name":          v.Name,
		"apply_objects": []any{map[string]any{"dimension": ruleDimension(v.TargetLevel), "dimension_ids": v.TargetIDs, "pre_condition_type": "SELECTED"}},
		"conditions":    conditions, "actions": []any{action},
		"notification":   map[string]any{"notification_type": "NOT_NOTIFICATION"},
		"rule_exec_info": map[string]any{"exec_time_type": "PER_HALF_HOUR"},
	}
}

func (b *Backend) createRule(ctx context.Context, v addomain.AutomatedRuleCreateSpec) addomain.OperationOutcome {
	rule := ruleWireDefinition(v)
	var data map[string]json.RawMessage
	requestID, err := b.client.post(ctx, "/open_api/v1.3/optimizer/rule/create/", map[string]any{"advertiser_id": b.client.advertiserID, "rules": []any{rule}}, &data)
	if err != nil {
		return classifyOperationError(err)
	}
	id := firstCreatedID(data, "rule_id", "rule_ids", "rules")
	if id == "" {
		return addomain.OperationOutcome{State: "unknown", RequestIDs: []string{requestID}, Message: "rule create response omitted resource ID"}
	}
	return addomain.OperationOutcome{State: "acknowledged", RequestIDs: []string{requestID}, Resources: []addomain.OperationResource{{Kind: "automated_rule", ID: id, Name: v.Name}}}
}

func ruleMetric(value string) string {
	return map[string]string{"SPEND": "COST", "CPA": "CPA", "CTR": "CTR", "CONVERSIONS": "CONVERSION"}[value]
}
func ruleWindow(value string) string {
	return map[string]string{"TODAY": "TODAY", "LAST_3_DAYS": "PAST_THREE_DAYS", "LAST_7_DAYS": "PAST_SEVEN_DAYS"}[value]
}
func ruleAction(value string) string {
	return map[string]string{"NOTIFY": "MESSAGE", "PAUSE": "TURN_OFF", "CHANGE_BUDGET": "DAILY_BUDGET"}[value]
}
func ruleDimension(value addomain.Level) string {
	return map[addomain.Level]string{addomain.Campaign: "CAMPAIGN", addomain.AdGroup: "ADGROUP", addomain.Ad: "AD"}[value]
}

func firstCreatedID(data map[string]json.RawMessage, single, list, objects string) string {
	if value := wireString(data, single); value != "" {
		return value
	}
	var ids []string
	if json.Unmarshal(data[list], &ids) == nil && len(ids) > 0 {
		return ids[0]
	}
	var values []map[string]json.RawMessage
	if json.Unmarshal(data[objects], &values) == nil && len(values) > 0 {
		return wireString(values[0], single)
	}
	return ""
}

func (b *Backend) applyComment(ctx context.Context, v addomain.CommentActionSpec) addomain.OperationOutcome {
	body := map[string]any{"advertiser_id": b.client.advertiserID}
	path := ""
	switch v.Action {
	case "reply":
		path = "/open_api/v1.3/comment/post/"
		body["ad_id"] = v.AdID
		body["comment_id"] = v.CommentID
		body["comment_type"] = "COMMENT_TYPE_NORMAL"
		body["identity_id"] = v.IdentityID
		body["identity_type"] = v.IdentityType
		body["text"] = v.Text
		body["tiktok_item_id"] = v.TikTokItemID
	case "hide", "unhide":
		path = "/open_api/v1.3/comment/status/update/"
		body["comment_ids"] = []string{v.CommentID}
		if v.Action == "hide" {
			body["operation"] = "HIDE"
		} else {
			body["operation"] = "UNHIDE"
		}
	case "delete":
		path = "/open_api/v1.3/comment/delete/"
		body["ad_id"] = v.AdID
		body["comment_id"] = v.CommentID
		body["identity_id"] = v.IdentityID
		body["identity_type"] = v.IdentityType
		body["tiktok_item_id"] = v.TikTokItemID
	}
	return b.singleOperation(ctx, path, body, "comment", v.CommentID, "")
}

func (b *Backend) ReconcileOperation(ctx context.Context, plan addomain.OperationPlan, outcome addomain.OperationOutcome) (bool, error) {
	if outcome.State != "acknowledged" || len(outcome.Resources) == 0 {
		return false, nil
	}
	if plan.Request.Kind == addomain.CreateCampaignBundle {
		return b.reconcileCampaignBundle(ctx, *plan.Request.CampaignBundle, outcome)
	}
	if plan.Request.Kind == addomain.CreateAudience || plan.Request.Kind == addomain.CreateAutomatedRule || plan.Request.Kind == addomain.CreateEventSource {
		return b.reconcileCreatedResource(ctx, plan.Request, outcome)
	}
	for _, item := range outcome.Resources {
		switch item.Kind {
		case "campaign":
			if _, err := b.Get(ctx, addomain.Campaign, item.ID); err != nil {
				return false, nil
			}
		case "ad_group":
			if _, err := b.Get(ctx, addomain.AdGroup, item.ID); err != nil {
				return false, nil
			}
			if plan.Request.Kind == addomain.UpdateAdGroup {
				current, e := b.getAdGroupOperation(ctx, item.ID)
				if e != nil || !adGroupMatches(current, *plan.Request.AdGroupUpdate) {
					return false, nil
				}
			}
		case "ad":
			if plan.Request.Kind == addomain.UpdateAdCreative {
				current, e := b.GetAdDetail(ctx, item.ID)
				if e != nil || !adMatches(current, *plan.Request.AdUpdate) {
					return false, nil
				}
			} else if _, e := b.Get(ctx, addomain.Ad, item.ID); e != nil {
				return false, nil
			}
		case "audience":
			if _, e := b.GetAudience(ctx, item.ID); e != nil {
				return false, nil
			}
		case "automated_rule":
			rules, e := b.ListAutomatedRules(ctx)
			found := false
			for _, rule := range rules {
				found = found || rule.ID == item.ID
			}
			if e != nil || !found {
				return false, nil
			}
		case "comment":
			comments, e := b.ListComments(ctx, plan.Request.Comment.AdID, 100)
			if e != nil || !commentMatches(comments, *plan.Request.Comment) {
				return false, nil
			}
		case "event_source":
			sources, e := b.ListEventSources(ctx)
			if e != nil || !sourceExists(sources, item.ID) {
				return false, nil
			}
		default:
			return false, nil
		}
	}
	return true, nil
}

func (b *Backend) GetAdDetail(ctx context.Context, id string) (addomain.AdDetail, error) {
	entity, err := b.Get(ctx, addomain.Ad, id)
	if err != nil {
		return addomain.AdDetail{}, err
	}
	q := url.Values{"advertiser_id": {b.client.advertiserID}}
	fields, _ := jsonQuery([]string{"ad_id", "adgroup_id", "ad_name", "identity_id", "identity_type", "video_id", "image_ids", "ad_text", "call_to_action", "landing_page_url", "operation_status"})
	filter, _ := jsonQuery(map[string][]string{"ad_ids": {id}})
	q.Set("fields", fields)
	q.Set("filtering", filter)
	q.Set("page", "1")
	q.Set("page_size", "2")
	var data struct {
		List []struct {
			AdID         string   `json:"ad_id"`
			IdentityID   string   `json:"identity_id"`
			IdentityType string   `json:"identity_type"`
			VideoID      string   `json:"video_id"`
			ImageIDs     []string `json:"image_ids"`
			AdText       string   `json:"ad_text"`
			CTA          string   `json:"call_to_action"`
			URL          string   `json:"landing_page_url"`
		} `json:"list"`
	}
	_, err = b.client.get(ctx, "/open_api/v1.3/ad/get/", q, &data)
	if err != nil {
		return addomain.AdDetail{}, err
	}
	for _, v := range data.List {
		if v.AdID != id {
			continue
		}
		detail := addomain.AdDetail{Ad: entity, PrimaryText: v.AdText, CallToAction: v.CTA, DestinationURL: v.URL}
		if v.IdentityID != "" {
			detail.Identity = &addomain.Identity{ID: v.IdentityID, AccountID: b.client.advertiserID, Kind: v.IdentityType}
		}
		assetID, kind := "", ""
		if v.VideoID != "" {
			assetID, kind = v.VideoID, "video"
		} else if len(v.ImageIDs) > 0 {
			assetID, kind = v.ImageIDs[0], "image"
		}
		if assetID != "" {
			detail.Creative = &addomain.CreativeAsset{ID: assetID, AccountID: b.client.advertiserID, Kind: kind}
		}
		return detail, nil
	}
	return addomain.AdDetail{}, addomain.ErrNotFound
}

func (b *Backend) getAdGroupOperation(ctx context.Context, id string) (addomain.AdGroupUpdateSpec, error) {
	q := url.Values{"advertiser_id": {b.client.advertiserID}}
	fields, _ := jsonQuery([]string{"adgroup_id", "budget", "bid_price", "schedule_start_time", "schedule_end_time", "placements", "audience_ids", "excluded_audience_ids", "location_ids", "languages"})
	filter, _ := jsonQuery(map[string][]string{"adgroup_ids": {id}})
	q.Set("fields", fields)
	q.Set("filtering", filter)
	q.Set("page", "1")
	q.Set("page_size", "2")
	var data struct {
		List []struct {
			ID                  string           `json:"adgroup_id"`
			Budget              *decimal.Decimal `json:"budget"`
			Bid                 *decimal.Decimal `json:"bid_price"`
			ScheduleStart       string           `json:"schedule_start_time"`
			ScheduleEnd         string           `json:"schedule_end_time"`
			Placements          []string         `json:"placements"`
			AudienceIDs         []string         `json:"audience_ids"`
			ExcludedAudienceIDs []string         `json:"excluded_audience_ids"`
			LocationIDs         []string         `json:"location_ids"`
			Languages           []string         `json:"languages"`
		} `json:"list"`
	}
	_, err := b.client.get(ctx, "/open_api/v1.3/adgroup/get/", q, &data)
	if err != nil {
		return addomain.AdGroupUpdateSpec{}, err
	}
	for _, v := range data.List {
		if v.ID == id {
			return addomain.AdGroupUpdateSpec{AdGroupID: id, Budget: v.Budget, Bid: v.Bid, ScheduleStart: v.ScheduleStart, ScheduleEnd: v.ScheduleEnd, Placements: v.Placements, AudienceIDs: v.AudienceIDs, ExcludedAudienceIDs: v.ExcludedAudienceIDs, LocationIDs: v.LocationIDs, Languages: v.Languages}, nil
		}
	}
	return addomain.AdGroupUpdateSpec{}, addomain.ErrNotFound
}

func (b *Backend) ListComments(ctx context.Context, adID string, limit int) ([]addomain.Comment, error) {
	if adID == "" || limit < 1 || limit > 100 {
		return nil, errors.New("ad_id and comment limit are required")
	}
	account, err := b.Account(ctx)
	if err != nil {
		return nil, err
	}
	loc, _ := time.LoadLocation(account.Timezone)
	end := time.Now().In(loc)
	start := end.AddDate(0, 0, -30)
	q := url.Values{"advertiser_id": {b.client.advertiserID}, "start_time": {start.Format("2006-01-02 15:04:05")}, "end_time": {end.Format("2006-01-02 15:04:05")}, "search_field": {"AD_ID"}, "search_value": {adID}, "sort_field": {"CREATE_TIME"}, "sort_type": {"DESC"}, "page": {"1"}, "page_size": {strconv.Itoa(limit)}}
	var data struct {
		List []struct {
			ID      string `json:"comment_id"`
			AdID    string `json:"ad_id"`
			ItemID  string `json:"tiktok_item_id"`
			Author  string `json:"user_name"`
			Text    string `json:"text"`
			Status  string `json:"comment_status"`
			Created int64  `json:"create_time"`
			Replies int64  `json:"reply_count"`
		} `json:"list"`
	}
	_, err = b.client.get(ctx, "/open_api/v1.3/comment/list/", q, &data)
	if err != nil {
		return nil, err
	}
	out := make([]addomain.Comment, 0, len(data.List))
	for _, v := range data.List {
		if v.ID == "" || v.AdID != adID {
			return nil, errors.New("TikTok comment response is outside requested scope")
		}
		out = append(out, addomain.Comment{ID: v.ID, AccountID: b.client.advertiserID, AdID: v.AdID, TikTokItemID: v.ItemID, Author: v.Author, Text: v.Text, Status: v.Status, CreatedAt: time.Unix(v.Created, 0).UTC(), ReplyCount: v.Replies})
	}
	return out, nil
}

func (b *Backend) GetBillingBalance(ctx context.Context) (addomain.BillingBalance, error) {
	if b.client.businessCenterID == "" {
		return addomain.BillingBalance{}, errors.New("TikTok Business Center ID is required for billing reads")
	}
	filter, _ := jsonQuery(map[string]any{"keyword": b.client.advertiserID})
	q := url.Values{"bc_id": {b.client.businessCenterID}, "filtering": {filter}, "page": {"1"}, "page_size": {"100"}}
	var data struct {
		List []map[string]json.RawMessage `json:"list"`
	}
	_, err := b.client.get(ctx, "/open_api/v1.3/advertiser/balance/get/", q, &data)
	if err != nil {
		return addomain.BillingBalance{}, err
	}
	for _, item := range data.List {
		if wireString(item, "advertiser_id") != b.client.advertiserID {
			continue
		}
		currency := wireString(item, "currency")
		available, e := rawDecimal(item, "balance")
		if e != nil {
			return addomain.BillingBalance{}, e
		}
		cash, _ := rawDecimal(item, "cash_balance")
		voucher, _ := rawDecimal(item, "grant_balance")
		return addomain.BillingBalance{AccountID: b.client.advertiserID, Currency: currency, Available: available, Cash: cash, Voucher: voucher, AsOf: time.Now().UTC()}, nil
	}
	return addomain.BillingBalance{}, addomain.ErrNotFound
}

func (b *Backend) ListBillingTransactions(ctx context.Context, start, end string) ([]addomain.BillingTransaction, error) {
	if err := addomain.ValidateDateRange(start, end, 93); err != nil {
		return nil, err
	}
	if b.client.businessCenterID == "" {
		return nil, errors.New("TikTok Business Center ID is required for billing reads")
	}
	filter, _ := jsonQuery(map[string]any{"keyword": b.client.advertiserID, "start_date": start, "end_date": end, "summary_by_account": false})
	q := url.Values{"bc_id": {b.client.businessCenterID}, "filtering": {filter}, "page": {"1"}, "page_size": {"1000"}}
	var data struct {
		List []map[string]json.RawMessage `json:"list"`
	}
	_, err := b.client.get(ctx, "/open_api/v1.3/advertiser/transaction/get/", q, &data)
	if err != nil {
		return nil, err
	}
	out := []addomain.BillingTransaction{}
	for _, item := range data.List {
		if id := wireString(item, "advertiser_id"); id != "" && id != b.client.advertiserID {
			continue
		}
		amount, e := rawDecimal(item, "amount")
		if e != nil {
			return nil, e
		}
		at, _ := time.Parse(time.RFC3339, wireString(item, "create_time"))
		out = append(out, addomain.BillingTransaction{ID: wireString(item, "transaction_id"), AccountID: b.client.advertiserID, Type: wireString(item, "transaction_type"), Amount: amount, Currency: wireString(item, "currency"), OccurredAt: at, Status: wireString(item, "status")})
	}
	return out, nil
}

func (b *Backend) postCreated(ctx context.Context, path string, body any, idKey string) (string, string, error) {
	var data map[string]json.RawMessage
	requestID, err := b.client.post(ctx, path, body, &data)
	if err != nil {
		return "", requestID, err
	}
	id := wireString(data, idKey)
	if id == "" {
		return "", requestID, errors.New("TikTok create response omitted resource ID")
	}
	return id, requestID, nil
}
func (b *Backend) createSingle(ctx context.Context, path string, body any, kind, name, idKey string) addomain.OperationOutcome {
	id, requestID, err := b.postCreated(ctx, path, body, idKey)
	if err != nil {
		return classifyOperationError(err)
	}
	return addomain.OperationOutcome{State: "acknowledged", RequestIDs: []string{requestID}, Resources: []addomain.OperationResource{{Kind: kind, ID: id, Name: name}}}
}
func (b *Backend) singleOperation(ctx context.Context, path string, body any, kind, id, name string) addomain.OperationOutcome {
	requestID, err := b.client.post(ctx, path, body, nil)
	if err != nil {
		return classifyOperationError(err)
	}
	return addomain.OperationOutcome{State: "acknowledged", RequestIDs: []string{requestID}, Resources: []addomain.OperationResource{{Kind: kind, ID: id, Name: name}}}
}
func classifyOperationError(err error) addomain.OperationOutcome {
	var apiErr *Error
	if errors.As(err, &apiErr) {
		switch apiErr.Kind {
		case "credential":
			return addomain.OperationOutcome{State: "not_sent", Message: "credential_unavailable"}
		case "business":
			return addomain.OperationOutcome{State: "rejected", RequestIDs: []string{apiErr.RequestID}, Message: "platform_rejected"}
		default:
			return addomain.OperationOutcome{State: "unknown", RequestIDs: []string{apiErr.RequestID}, Message: "remote_outcome_unknown"}
		}
	}
	return addomain.OperationOutcome{State: "unknown", Message: "remote_outcome_unknown"}
}
func makePartial(base addomain.OperationOutcome, err error) addomain.OperationOutcome {
	classified := classifyOperationError(err)
	base.State = "partial"
	base.Message = "campaign bundle partially completed: " + classified.Message
	base.RequestIDs = append(base.RequestIDs, classified.RequestIDs...)
	return base
}
func putIf(body map[string]any, key, value string) {
	if value != "" {
		body[key] = value
	}
}
func putSlice(body map[string]any, key string, value []string) {
	if len(value) > 0 {
		body[key] = value
	}
}
func putDecimal(body map[string]any, key string, value *decimal.Decimal) {
	if value != nil {
		body[key] = json.RawMessage(value.String())
	}
}
func identityUsable(values []addomain.Identity, id string) bool {
	for _, v := range values {
		if v.ID == id && (v.Status == "ACTIVE" || v.Status == "AUTHORIZED") {
			return true
		}
	}
	return false
}
func assetUsable(values []addomain.CreativeAsset, id string) bool {
	for _, v := range values {
		if v.ID == id && v.ReviewStatus == "APPROVED" && (v.Status == "READY" || v.Status == "AVAILABLE") {
			return true
		}
	}
	return false
}
func adGroupMatches(current, requested addomain.AdGroupUpdateSpec) bool {
	if requested.Budget != nil && (current.Budget == nil || !current.Budget.Equal(*requested.Budget)) {
		return false
	}
	if requested.Bid != nil && (current.Bid == nil || !current.Bid.Equal(*requested.Bid)) {
		return false
	}
	if requested.ScheduleEnd != "" && current.ScheduleEnd != requested.ScheduleEnd {
		return false
	}
	if requested.ScheduleStart != "" && current.ScheduleStart != requested.ScheduleStart {
		return false
	}
	return sameWhenSet(current.Placements, requested.Placements) && sameWhenSet(current.AudienceIDs, requested.AudienceIDs) && sameWhenSet(current.ExcludedAudienceIDs, requested.ExcludedAudienceIDs) && sameWhenSet(current.LocationIDs, requested.LocationIDs) && sameWhenSet(current.Languages, requested.Languages)
}
func adMatches(current addomain.AdDetail, requested addomain.AdCreativeUpdateSpec) bool {
	if requested.IdentityID != "" && (current.Identity == nil || current.Identity.ID != requested.IdentityID) {
		return false
	}
	if requested.AssetID != "" && (current.Creative == nil || current.Creative.ID != requested.AssetID) {
		return false
	}
	if requested.PrimaryText != "" && current.PrimaryText != requested.PrimaryText {
		return false
	}
	if requested.CallToAction != "" && current.CallToAction != requested.CallToAction {
		return false
	}
	return requested.DestinationURL == "" || current.DestinationURL == requested.DestinationURL
}
func sameWhenSet(current, want []string) bool {
	if len(want) == 0 {
		return true
	}
	a, b := append([]string{}, current...), append([]string{}, want...)
	sort.Strings(a)
	sort.Strings(b)
	return strings.Join(a, "\x00") == strings.Join(b, "\x00")
}
func ruleExists(values []addomain.AutomatedRule, id string) bool {
	for _, v := range values {
		if v.ID == id {
			return true
		}
	}
	return false
}
func sourceExists(values []addomain.EventSource, id string) bool {
	for _, v := range values {
		if v.ID == id {
			return true
		}
	}
	return false
}
func commentMatches(values []addomain.Comment, request addomain.CommentActionSpec) bool {
	for _, v := range values {
		if v.ID != request.CommentID {
			continue
		}
		if request.Action == "hide" {
			return v.Status == "HIDDEN"
		}
		if request.Action == "unhide" {
			return v.Status == "VISIBLE"
		}
		if request.Action == "delete" {
			return false
		}
		if request.Action == "reply" {
			return v.ReplyCount > 0
		}
	}
	return request.Action == "delete"
}
func wireString(values map[string]json.RawMessage, key string) string {
	var value string
	_ = json.Unmarshal(values[key], &value)
	return value
}
func rawDecimal(values map[string]json.RawMessage, key string) (decimal.Decimal, error) {
	raw := values[key]
	if len(raw) == 0 {
		return decimal.Zero, nil
	}
	var s string
	if json.Unmarshal(raw, &s) != nil {
		s = string(raw)
	}
	value, err := decimal.NewFromString(s)
	if err != nil {
		return decimal.Zero, errors.New("TikTok money field is invalid")
	}
	return value, nil
}

var _ addomain.Operations = (*Backend)(nil)
var _ addomain.OperationsReader = (*Backend)(nil)
var _ addomain.AdDetailsReader = (*Backend)(nil)
