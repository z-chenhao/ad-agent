package tiktokmapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/z-chenhao/ad-agent/internal/ads"
)

type rawObject map[string]json.RawMessage

func rawString(object rawObject, keys ...string) string {
	for _, key := range keys {
		var value string
		if raw := object[key]; len(raw) > 0 && json.Unmarshal(raw, &value) == nil && value != "" {
			return value
		}
	}
	return ""
}

func rawInt64(object rawObject, keys ...string) (int64, bool) {
	for _, key := range keys {
		raw := object[key]
		if len(raw) == 0 || string(raw) == "null" {
			continue
		}
		var value int64
		if json.Unmarshal(raw, &value) == nil {
			return value, true
		}
		var text string
		if json.Unmarshal(raw, &text) == nil {
			value, err := strconv.ParseInt(text, 10, 64)
			if err == nil {
				return value, true
			}
		}
	}
	return 0, false
}

func rawTime(object rawObject, keys ...string) *time.Time {
	value := rawString(object, keys...)
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", time.DateOnly} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return &parsed
		}
	}
	return nil
}

func rawStrings(object rawObject, keys ...string) []string {
	for _, key := range keys {
		var values []string
		if raw := object[key]; len(raw) > 0 && json.Unmarshal(raw, &values) == nil {
			return values
		}
	}
	return nil
}

func rawFieldNames(object rawObject, keys ...string) []string {
	for _, key := range keys {
		raw := object[key]
		if len(raw) == 0 {
			continue
		}
		var direct []string
		if json.Unmarshal(raw, &direct) == nil {
			return direct
		}
		var fields []rawObject
		if json.Unmarshal(raw, &fields) == nil {
			out := make([]string, 0, len(fields))
			for _, field := range fields {
				if name := rawString(field, "field_name", "name", "type"); name != "" {
					out = append(out, name)
				}
			}
			return out
		}
		var encoded string
		if json.Unmarshal(raw, &encoded) == nil && json.Unmarshal([]byte(encoded), &direct) == nil {
			return direct
		}
	}
	return nil
}

func (b *Backend) pagedObjects(ctx context.Context, path string, query url.Values) ([]rawObject, error) {
	const pageSize = 100
	result := []rawObject{}
	for page := 1; page <= b.client.maxPages; page++ {
		q := url.Values{}
		for key, values := range query {
			q[key] = append([]string(nil), values...)
		}
		putInt(q, "page", page)
		putInt(q, "page_size", pageSize)
		var data struct {
			List     []rawObject `json:"list"`
			PageInfo pageInfo    `json:"page_info"`
		}
		if _, err := b.client.get(ctx, path, q, &data); err != nil {
			return nil, err
		}
		result = append(result, data.List...)
		if !morePages(data.PageInfo, page, len(data.List), pageSize) {
			return result, nil
		}
	}
	return nil, errors.New("TikTok resource result exceeded the configured page limit")
}

func (b *Backend) accountQuery() url.Values {
	return url.Values{"advertiser_id": {b.client.advertiserID}}
}

func (b *Backend) checkReturnedAccount(object rawObject) error {
	if accountID := rawString(object, "advertiser_id", "account_id"); accountID != "" && accountID != b.client.advertiserID {
		return errors.New("TikTok returned a resource from a different advertiser")
	}
	return nil
}

func (b *Backend) ListIdentities(ctx context.Context) ([]ads.Identity, error) {
	objects, err := b.pagedObjects(ctx, "/open_api/v1.3/identity/get/", b.accountQuery())
	if err != nil {
		return nil, err
	}
	values := make([]ads.Identity, 0, len(objects))
	for _, object := range objects {
		if err := b.checkReturnedAccount(object); err != nil {
			return nil, err
		}
		value := ads.Identity{ID: rawString(object, "identity_id"), AccountID: b.client.advertiserID, Name: rawString(object, "display_name", "identity_name", "name"), Kind: strings.ToLower(rawString(object, "identity_type", "type")), Status: rawString(object, "identity_status", "status")}
		if value.ID == "" || value.Name == "" {
			return nil, errors.New("TikTok identity response is missing required fields")
		}
		values = append(values, value)
	}
	sort.Slice(values, func(i, j int) bool { return values[i].ID < values[j].ID })
	return values, nil
}

func (b *Backend) creativeObjects(ctx context.Context, path, kind string) ([]ads.CreativeAsset, error) {
	objects, err := b.pagedObjects(ctx, path, b.accountQuery())
	if err != nil {
		return nil, err
	}
	values := make([]ads.CreativeAsset, 0, len(objects))
	for _, object := range objects {
		if err := b.checkReturnedAccount(object); err != nil {
			return nil, err
		}
		value := ads.CreativeAsset{ID: rawString(object, kind+"_id", "material_id", "id"), AccountID: b.client.advertiserID, Name: rawString(object, kind+"_name", "file_name", "name"), Kind: kind, Status: rawString(object, "status", "material_status"), ReviewStatus: rawString(object, "review_status", "audit_status"), UpdatedAt: time.Time{}}
		if width, ok := rawInt64(object, "width"); ok {
			value.Width = int(width)
		}
		if height, ok := rawInt64(object, "height"); ok {
			value.Height = int(height)
		}
		if duration, ok := rawInt64(object, "duration_ms"); ok {
			value.DurationMS = duration
		} else if duration, ok := rawInt64(object, "duration"); ok {
			value.DurationMS = duration * 1000
		}
		if updated := rawTime(object, "modify_time", "update_time"); updated != nil {
			value.UpdatedAt = *updated
		}
		if value.ID == "" || value.Name == "" {
			return nil, errors.New("TikTok creative response is missing required fields")
		}
		values = append(values, value)
	}
	return values, nil
}

func (b *Backend) ListCreativeAssets(ctx context.Context) ([]ads.CreativeAsset, error) {
	images, err := b.creativeObjects(ctx, "/open_api/v1.3/file/image/ad/search/", "image")
	if err != nil {
		return nil, err
	}
	videos, err := b.creativeObjects(ctx, "/open_api/v1.3/file/video/ad/search/", "video")
	if err != nil {
		return nil, err
	}
	values := append(images, videos...)
	sort.Slice(values, func(i, j int) bool { return values[i].ID < values[j].ID })
	return values, nil
}

func (b *Backend) GetCreativeAsset(ctx context.Context, id string) (ads.CreativeAsset, error) {
	values, err := b.ListCreativeAssets(ctx)
	if err != nil {
		return ads.CreativeAsset{}, err
	}
	for _, value := range values {
		if value.ID == id {
			return value, nil
		}
	}
	return ads.CreativeAsset{}, ads.ErrNotFound
}

func decodeAudience(accountID string, object rawObject, defaultKind string) (ads.Audience, error) {
	value := ads.Audience{ID: rawString(object, "audience_id", "custom_audience_id", "saved_audience_id"), AccountID: accountID, Name: rawString(object, "audience_name", "custom_audience_name", "saved_audience_name", "name"), Kind: strings.ToLower(rawString(object, "audience_type", "type")), Status: rawString(object, "status", "audience_status"), Source: rawString(object, "source", "data_source"), UpdatedAt: time.Time{}, PrivacyLimited: true}
	if value.Kind == "" {
		value.Kind = defaultKind
	}
	if size, ok := rawInt64(object, "audience_size", "approximate_size", "size"); ok {
		value.ApproximateSize = &size
	}
	if updated := rawTime(object, "modify_time", "update_time", "create_time"); updated != nil {
		value.UpdatedAt = *updated
	}
	if value.ID == "" || value.Name == "" {
		return ads.Audience{}, errors.New("TikTok audience response is missing required fields")
	}
	return value, nil
}

func (b *Backend) ListAudiences(ctx context.Context) ([]ads.Audience, error) {
	values := []ads.Audience{}
	for _, endpoint := range []struct{ path, kind string }{{"/open_api/v1.3/dmp/custom_audience/list/", "custom"}, {"/open_api/v1.3/dmp/saved_audience/list/", "saved"}} {
		objects, err := b.pagedObjects(ctx, endpoint.path, b.accountQuery())
		if err != nil {
			return nil, err
		}
		for _, object := range objects {
			if err := b.checkReturnedAccount(object); err != nil {
				return nil, err
			}
			value, err := decodeAudience(b.client.advertiserID, object, endpoint.kind)
			if err != nil {
				return nil, err
			}
			values = append(values, value)
		}
	}
	sort.Slice(values, func(i, j int) bool { return values[i].ID < values[j].ID })
	return values, nil
}

func (b *Backend) GetAudience(ctx context.Context, id string) (ads.Audience, error) {
	if id == "" {
		return ads.Audience{}, ads.ErrNotFound
	}
	q := b.accountQuery()
	filter, _ := jsonQuery(map[string][]string{"custom_audience_ids": {id}})
	q.Set("filtering", filter)
	objects, err := b.pagedObjects(ctx, "/open_api/v1.3/dmp/custom_audience/get/", q)
	if err != nil {
		return ads.Audience{}, err
	}
	for _, object := range objects {
		if err := b.checkReturnedAccount(object); err != nil {
			return ads.Audience{}, err
		}
		value, err := decodeAudience(b.client.advertiserID, object, "custom")
		if err != nil {
			return ads.Audience{}, err
		}
		if value.ID == id {
			return value, nil
		}
	}
	return ads.Audience{}, ads.ErrNotFound
}

func (b *Backend) GetAudienceOverlap(ctx context.Context, left, right string) (ads.AudienceOverlap, error) {
	if left == "" || right == "" || left == right {
		return ads.AudienceOverlap{}, errors.New("two distinct audiences are required")
	}
	q := b.accountQuery()
	ids, _ := jsonQuery([]string{left, right})
	q.Set("audience_ids", ids)
	var data rawObject
	_, err := b.client.get(ctx, "/open_api/v1.3/audience/insight/overlap/", q, &data)
	if err != nil {
		return ads.AudienceOverlap{}, err
	}
	overlap, hasOverlap := rawInt64(data, "overlap_users", "overlap_size")
	leftRate, hasLeft := rawFloat(data, "left_rate", "audience_1_overlap_rate")
	rightRate, hasRight := rawFloat(data, "right_rate", "audience_2_overlap_rate")
	value := ads.AudienceOverlap{LeftID: left, RightID: right, Complete: hasOverlap || hasLeft || hasRight}
	if hasOverlap {
		value.OverlapUsers = &overlap
	}
	if hasLeft {
		value.LeftRate = &leftRate
	}
	if hasRight {
		value.RightRate = &rightRate
	}
	if !value.Complete {
		value.Limitations = []string{"TikTok did not return an overlap value for these audiences."}
	}
	return value, nil
}

func rawFloat(object rawObject, keys ...string) (float64, bool) {
	for _, key := range keys {
		raw := object[key]
		if len(raw) == 0 || string(raw) == "null" {
			continue
		}
		var value float64
		if json.Unmarshal(raw, &value) == nil {
			return value, true
		}
		var text string
		if json.Unmarshal(raw, &text) == nil {
			value, err := strconv.ParseFloat(text, 64)
			if err == nil {
				return value, true
			}
		}
	}
	return 0, false
}

func (b *Backend) ListTargetingOptions(ctx context.Context, kind string) ([]ads.TargetingOption, error) {
	if kind == "" {
		return nil, errors.New("targeting option kind is required")
	}
	q := b.accountQuery()
	q.Set("targeting_type", strings.ToUpper(kind))
	objects, err := b.pagedObjects(ctx, "/open_api/v1.3/tool/targeting/list/", q)
	if err != nil {
		return nil, err
	}
	values := make([]ads.TargetingOption, 0, len(objects))
	for _, object := range objects {
		value := ads.TargetingOption{ID: rawString(object, "targeting_id", "id"), Kind: kind, Name: rawString(object, "targeting_name", "name"), ParentID: rawString(object, "parent_id"), Enabled: rawString(object, "status") != "DISABLED"}
		if value.ID == "" || value.Name == "" {
			return nil, errors.New("TikTok targeting response is missing required fields")
		}
		values = append(values, value)
	}
	return values, nil
}

func (b *Backend) sourceObjects(ctx context.Context, path, kind string) ([]ads.EventSource, error) {
	objects, err := b.pagedObjects(ctx, path, b.accountQuery())
	if err != nil {
		return nil, err
	}
	values := make([]ads.EventSource, 0, len(objects))
	for _, object := range objects {
		if err := b.checkReturnedAccount(object); err != nil {
			return nil, err
		}
		value := ads.EventSource{ID: rawString(object, "pixel_id", "app_id", "offline_id", "offline_event_set_id", "id"), AccountID: b.client.advertiserID, Name: rawString(object, "pixel_name", "app_name", "offline_name", "name"), Kind: kind, Status: rawString(object, "status"), LastEventAt: rawTime(object, "last_event_time", "last_active_time"), EventTypes: rawStrings(object, "event_types", "optimization_events")}
		if value.ID == "" || value.Name == "" {
			return nil, errors.New("TikTok event source response is missing required fields")
		}
		values = append(values, value)
	}
	return values, nil
}

func (b *Backend) ListEventSources(ctx context.Context) ([]ads.EventSource, error) {
	values := []ads.EventSource{}
	for _, endpoint := range []struct{ path, kind string }{{"/open_api/v1.3/pixel/list/", "pixel"}, {"/open_api/v1.3/app/list/", "app"}, {"/open_api/v1.3/offline/get/", "offline"}} {
		items, err := b.sourceObjects(ctx, endpoint.path, endpoint.kind)
		if err != nil {
			return nil, err
		}
		values = append(values, items...)
	}
	sort.Slice(values, func(i, j int) bool { return values[i].ID < values[j].ID })
	return values, nil
}

func (b *Backend) GetEventStats(ctx context.Context, sourceID, start, end string) (ads.EventStats, error) {
	if err := ads.ValidateDateRange(start, end, 31); err != nil {
		return ads.EventStats{}, err
	}
	q := b.accountQuery()
	q.Set("pixel_id", sourceID)
	q.Set("start_date", start)
	q.Set("end_date", end)
	objects, err := b.pagedObjects(ctx, "/open_api/v1.3/pixel/event/stats/", q)
	if err != nil {
		return ads.EventStats{}, err
	}
	events := map[string]int64{}
	for _, object := range objects {
		name := rawString(object, "event", "event_name", "event_type")
		count, ok := rawInt64(object, "count", "event_count")
		if name == "" || !ok || count < 0 {
			return ads.EventStats{}, errors.New("TikTok event stats response is missing required fields")
		}
		events[name] += count
	}
	return ads.EventStats{SourceID: sourceID, Start: start, End: end, Events: events, Complete: true, Limitations: []string{"Pixel event statistics can lag and do not expose user-level rows."}}, nil
}

func (b *Backend) GetAttributionSettings(ctx context.Context) (ads.AttributionSettings, error) {
	if err := ctx.Err(); err != nil {
		return ads.AttributionSettings{}, err
	}
	return ads.AttributionSettings{Basis: "TikTok account and ad-group configured attribution", Limitations: []string{"No account-wide MAPI endpoint proves one click/view window; inspect the relevant ad group before optimization conclusions."}}, nil
}

func (b *Backend) ListLeadForms(ctx context.Context) ([]ads.LeadForm, error) {
	objects, err := b.pagedObjects(ctx, "/open_api/v1.3/page/library/get/", b.accountQuery())
	if err != nil {
		return nil, err
	}
	values := make([]ads.LeadForm, 0, len(objects))
	for _, object := range objects {
		if err := b.checkReturnedAccount(object); err != nil {
			return nil, err
		}
		value := ads.LeadForm{ID: rawString(object, "page_id", "form_id"), AccountID: b.client.advertiserID, Name: rawString(object, "page_name", "form_name", "name"), Status: rawString(object, "status"), UpdatedAt: time.Time{}}
		if updated := rawTime(object, "modify_time", "update_time", "create_time"); updated != nil {
			value.UpdatedAt = *updated
		}
		if value.ID == "" || value.Name == "" {
			return nil, errors.New("TikTok lead form response is missing required fields")
		}
		values = append(values, value)
	}
	return values, nil
}

func (b *Backend) GetLeadForm(ctx context.Context, id string) (ads.LeadForm, error) {
	q := b.accountQuery()
	q.Set("page_id", id)
	var data rawObject
	_, err := b.client.get(ctx, "/open_api/v1.3/page/field/get/", q, &data)
	if err != nil {
		return ads.LeadForm{}, err
	}
	if err := b.checkReturnedAccount(data); err != nil {
		return ads.LeadForm{}, err
	}
	value := ads.LeadForm{ID: rawString(data, "page_id"), AccountID: b.client.advertiserID, Name: rawString(data, "page_name", "name"), Status: rawString(data, "status"), FieldNames: rawFieldNames(data, "fields", "field_list"), UpdatedAt: time.Time{}}
	if updated := rawTime(data, "modify_time", "create_time"); updated != nil {
		value.UpdatedAt = *updated
	}
	if value.ID != id || value.Name == "" {
		return ads.LeadForm{}, errors.New("TikTok lead form response is missing required fields")
	}
	return value, nil
}

func (b *Backend) ListCatalogs(ctx context.Context) ([]ads.Catalog, error) {
	objects, err := b.pagedObjects(ctx, "/open_api/v1.3/catalog/get/", b.accountQuery())
	if err != nil {
		return nil, err
	}
	values := make([]ads.Catalog, 0, len(objects))
	for _, object := range objects {
		if err := b.checkReturnedAccount(object); err != nil {
			return nil, err
		}
		value := ads.Catalog{ID: rawString(object, "catalog_id"), AccountID: b.client.advertiserID, Name: rawString(object, "catalog_name", "name"), Currency: rawString(object, "currency"), Status: rawString(object, "status")}
		value.ProductCount, _ = rawInt64(object, "product_count", "total_product_count")
		value.IssueCount, _ = rawInt64(object, "issue_count", "problem_product_count")
		if value.ID == "" || value.Name == "" {
			return nil, errors.New("TikTok catalog response is missing required fields")
		}
		values = append(values, value)
	}
	return values, nil
}

func (b *Backend) ListProductSets(ctx context.Context, catalogID string) ([]ads.ProductSet, error) {
	q := b.accountQuery()
	q.Set("catalog_id", catalogID)
	objects, err := b.pagedObjects(ctx, "/open_api/v1.3/catalog/set/get/", q)
	if err != nil {
		return nil, err
	}
	values := make([]ads.ProductSet, 0, len(objects))
	for _, object := range objects {
		value := ads.ProductSet{ID: rawString(object, "product_set_id", "set_id"), CatalogID: catalogID, Name: rawString(object, "product_set_name", "set_name", "name")}
		value.ProductCount, _ = rawInt64(object, "product_count")
		if value.ID == "" || value.Name == "" {
			return nil, errors.New("TikTok product set response is missing required fields")
		}
		values = append(values, value)
	}
	return values, nil
}

func (b *Backend) ListAutomatedRules(ctx context.Context) ([]ads.AutomatedRule, error) {
	objects, err := b.pagedObjects(ctx, "/open_api/v1.3/optimizer/rule/list/", b.accountQuery())
	if err != nil {
		return nil, err
	}
	values := make([]ads.AutomatedRule, 0, len(objects))
	for _, object := range objects {
		if err := b.checkReturnedAccount(object); err != nil {
			return nil, err
		}
		level := ads.Level(strings.ToLower(rawString(object, "target_level", "dimension")))
		if level == "adgroup" {
			level = ads.AdGroup
		}
		value := ads.AutomatedRule{ID: rawString(object, "rule_id"), AccountID: b.client.advertiserID, Name: rawString(object, "rule_name", "name"), Status: rawString(object, "status"), TargetLevel: level, Action: rawString(object, "action_type", "action"), Schedule: rawString(object, "schedule_type", "schedule")}
		if value.ID == "" || value.Name == "" || !value.TargetLevel.Valid() || value.TargetLevel == ads.Advertiser {
			return nil, errors.New("TikTok automated rule response is missing required fields")
		}
		values = append(values, value)
	}
	return values, nil
}

func (b *Backend) ListAutomatedRuleResults(ctx context.Context, ruleID string) ([]ads.AutomatedRuleResult, error) {
	q := b.accountQuery()
	q.Set("rule_id", ruleID)
	objects, err := b.pagedObjects(ctx, "/open_api/v1.3/optimizer/rule/result/list/", q)
	if err != nil {
		return nil, err
	}
	values := make([]ads.AutomatedRuleResult, 0, len(objects))
	for _, object := range objects {
		value := ads.AutomatedRuleResult{ID: rawString(object, "result_id", "execution_id"), RuleID: rawString(object, "rule_id"), Status: rawString(object, "status", "result_status")}
		value.AffectedCount, _ = rawInt64(object, "affected_count", "object_count")
		if executed := rawTime(object, "execute_time", "execution_time", "create_time"); executed != nil {
			value.ExecutedAt = *executed
		}
		if value.RuleID == "" {
			value.RuleID = ruleID
		}
		if value.ID == "" || value.RuleID != ruleID {
			return nil, errors.New("TikTok automated rule result is missing required fields")
		}
		values = append(values, value)
	}
	return values, nil
}

var _ ads.CommonAdsReader = (*Backend)(nil)
