package tiktokmapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"github.com/z-chenhao/ad-agent/internal/ads"
)

type Backend struct{ client *Client }

func NewBackend(client *Client) (*Backend, error) {
	if client == nil {
		return nil, errors.New("TikTok MAPI client is required")
	}
	return &Backend{client: client}, nil
}

func (b *Backend) source() ads.Source {
	return ads.Source{Backend: "tiktok-mapi", Environment: b.client.environment, AccountID: b.client.advertiserID}
}

func (b *Backend) Account(ctx context.Context) (ads.Account, error) {
	ids, _ := jsonQuery([]string{b.client.advertiserID})
	fields, _ := jsonQuery([]string{"advertiser_id", "name", "currency", "timezone"})
	q := url.Values{"advertiser_ids": {ids}, "fields": {fields}}
	var data struct {
		List []struct {
			AdvertiserID string `json:"advertiser_id"`
			Name         string `json:"name"`
			Currency     string `json:"currency"`
			Timezone     string `json:"timezone"`
		} `json:"list"`
	}
	_, err := b.client.get(ctx, "/open_api/v1.3/advertiser/info/", q, &data)
	if err != nil {
		return ads.Account{}, err
	}
	for _, item := range data.List {
		if item.AdvertiserID != b.client.advertiserID {
			continue
		}
		if item.Name == "" || item.Currency == "" || item.Timezone == "" {
			return ads.Account{}, errors.New("TikTok advertiser response is missing required fields")
		}
		loc, locErr := time.LoadLocation(item.Timezone)
		if locErr != nil {
			return ads.Account{}, errors.New("TikTok advertiser timezone is invalid")
		}
		return ads.Account{
			ID: item.AdvertiserID, Name: item.Name, Currency: item.Currency, Timezone: item.Timezone,
			Source: b.source(), LatestDate: time.Now().In(loc).Format(time.DateOnly),
			Limitations: []string{"MAPI reports can lag or be revised; latest_date is the current advertiser-local date, not a freshness guarantee."},
		}, nil
	}
	return ads.Account{}, ads.ErrNotFound
}

type listData struct {
	List     []json.RawMessage `json:"list"`
	PageInfo pageInfo          `json:"page_info"`
}

func (b *Backend) List(ctx context.Context, query ads.EntityQuery) ([]ads.Entity, error) {
	if query.Level != ads.Campaign && query.Level != ads.AdGroup && query.Level != ads.Ad {
		return nil, errors.New("TikTok entity level must be campaign, ad_group, or ad")
	}
	const pageSize = 1000
	result := []ads.Entity{}
	for page := 1; page <= b.client.maxPages; page++ {
		q := url.Values{"advertiser_id": {b.client.advertiserID}}
		putInt(q, "page", page)
		putInt(q, "page_size", pageSize)
		fields, _ := jsonQuery(fieldsFor(query.Level))
		q.Set("fields", fields)
		if query.ParentID != "" {
			filter := map[string][]string{}
			switch query.Level {
			case ads.AdGroup:
				filter["campaign_ids"] = []string{query.ParentID}
			case ads.Ad:
				filter["adgroup_ids"] = []string{query.ParentID}
			default:
				return nil, errors.New("campaign does not accept a parent ID")
			}
			encoded, _ := jsonQuery(filter)
			q.Set("filtering", encoded)
		}
		var data listData
		_, err := b.client.get(ctx, listPath(query.Level), q, &data)
		if err != nil {
			return nil, err
		}
		for _, raw := range data.List {
			entity, parseErr := decodeEntity(query.Level, b.client.advertiserID, raw)
			if parseErr != nil {
				return nil, parseErr
			}
			if query.ParentID != "" && entity.ParentID != query.ParentID {
				return nil, errors.New("TikTok returned an entity outside the requested parent")
			}
			result = append(result, entity)
		}
		if !morePages(data.PageInfo, page, len(data.List), pageSize) {
			sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
			return result, nil
		}
	}
	return nil, errors.New("TikTok entity result exceeded the configured page limit")
}

func (b *Backend) Get(ctx context.Context, level ads.Level, id string) (ads.Entity, error) {
	if id == "" {
		return ads.Entity{}, ads.ErrNotFound
	}
	q := url.Values{"advertiser_id": {b.client.advertiserID}}
	putInt(q, "page", 1)
	putInt(q, "page_size", 2)
	fields, _ := jsonQuery(fieldsFor(level))
	q.Set("fields", fields)
	filterName := map[ads.Level]string{ads.Campaign: "campaign_ids", ads.AdGroup: "adgroup_ids", ads.Ad: "ad_ids"}[level]
	if filterName == "" {
		return ads.Entity{}, ads.ErrNotFound
	}
	filter, _ := jsonQuery(map[string][]string{filterName: {id}})
	q.Set("filtering", filter)
	var data listData
	_, err := b.client.get(ctx, listPath(level), q, &data)
	if err != nil {
		return ads.Entity{}, err
	}
	for _, raw := range data.List {
		entity, parseErr := decodeEntity(level, b.client.advertiserID, raw)
		if parseErr != nil {
			return ads.Entity{}, parseErr
		}
		if entity.ID == id {
			return entity, nil
		}
	}
	return ads.Entity{}, ads.ErrNotFound
}

func listPath(level ads.Level) string {
	return map[ads.Level]string{ads.Campaign: "/open_api/v1.3/campaign/get/", ads.AdGroup: "/open_api/v1.3/adgroup/get/", ads.Ad: "/open_api/v1.3/ad/get/"}[level]
}

func fieldsFor(level ads.Level) []string {
	switch level {
	case ads.Campaign:
		return []string{"advertiser_id", "campaign_id", "campaign_name", "operation_status", "budget", "budget_mode", "objective_type"}
	case ads.AdGroup:
		return []string{"advertiser_id", "campaign_id", "adgroup_id", "adgroup_name", "operation_status", "budget", "budget_mode", "objective_type"}
	case ads.Ad:
		return []string{"advertiser_id", "campaign_id", "adgroup_id", "ad_id", "ad_name", "operation_status"}
	default:
		return nil
	}
}

func decodeEntity(level ads.Level, accountID string, raw []byte) (ads.Entity, error) {
	var v map[string]json.RawMessage
	if json.Unmarshal(raw, &v) != nil {
		return ads.Entity{}, errors.New("invalid TikTok entity response")
	}
	stringValue := func(key string) string {
		var s string
		_ = json.Unmarshal(v[key], &s)
		return s
	}
	idKey, nameKey, parentKey := "", "", ""
	switch level {
	case ads.Campaign:
		idKey, nameKey = "campaign_id", "campaign_name"
	case ads.AdGroup:
		idKey, nameKey, parentKey = "adgroup_id", "adgroup_name", "campaign_id"
	case ads.Ad:
		idKey, nameKey, parentKey = "ad_id", "ad_name", "adgroup_id"
	default:
		return ads.Entity{}, errors.New("invalid TikTok entity level")
	}
	returnedAccount := stringValue("advertiser_id")
	if returnedAccount != "" && returnedAccount != accountID {
		return ads.Entity{}, errors.New("TikTok returned an entity from a different advertiser")
	}
	e := ads.Entity{ID: stringValue(idKey), AccountID: accountID, Level: level, ParentID: stringValue(parentKey), Name: stringValue(nameKey), Status: normalizeStatus(stringValue("operation_status")), BudgetMode: stringValue("budget_mode"), Objective: stringValue("objective_type")}
	if e.ID == "" || e.Name == "" || e.Status == "" || (level != ads.Campaign && e.ParentID == "") {
		return ads.Entity{}, errors.New("TikTok entity response is missing required fields")
	}
	if rawBudget := v["budget"]; len(rawBudget) > 0 && string(rawBudget) != "null" {
		var s string
		if json.Unmarshal(rawBudget, &s) != nil {
			var number json.Number
			if json.Unmarshal(rawBudget, &number) == nil {
				s = number.String()
			}
		}
		if s != "" {
			d, err := decimal.NewFromString(s)
			if err != nil || d.IsNegative() {
				return ads.Entity{}, errors.New("TikTok entity budget is invalid")
			}
			e.Budget = &d
		}
	}
	return e, nil
}

func normalizeStatus(v string) string {
	switch v {
	case "ENABLE", "STATUS_ENABLE":
		return "ENABLE"
	case "DISABLE", "STATUS_DISABLE":
		return "DISABLE"
	default:
		return ""
	}
}

type reportData struct {
	List []struct {
		Dimensions map[string]string          `json:"dimensions"`
		Metrics    map[string]json.RawMessage `json:"metrics"`
	} `json:"list"`
	PageInfo pageInfo `json:"page_info"`
}

func (b *Backend) Report(ctx context.Context, query ads.ReportQuery) (ads.Report, error) {
	if err := query.Validate(); err != nil {
		return ads.Report{}, err
	}
	startDate, _ := time.Parse(time.DateOnly, query.Start)
	endDate, _ := time.Parse(time.DateOnly, query.End)
	if endDate.Sub(startDate) > 29*24*time.Hour {
		return ads.Report{}, errors.New("TikTok daily report window must be 1–30 inclusive days")
	}
	account, err := b.Account(ctx)
	if err != nil {
		return ads.Report{}, err
	}
	dataLevel := map[ads.Level]string{ads.Advertiser: "AUCTION_ADVERTISER", ads.Campaign: "AUCTION_CAMPAIGN", ads.AdGroup: "AUCTION_ADGROUP", ads.Ad: "AUCTION_AD"}[query.Level]
	dimensionID := map[ads.Level]string{ads.Advertiser: "advertiser_id", ads.Campaign: "campaign_id", ads.AdGroup: "adgroup_id", ads.Ad: "ad_id"}[query.Level]
	const pageSize = 1000
	rows := []ads.Row{}
	requestIDs := []string{}
	for page := 1; page <= b.client.maxPages; page++ {
		dimensions, _ := jsonQuery([]string{dimensionID, "stat_time_day"})
		metricNames := []string{"spend", "impressions", "clicks", "conversion"}
		if b.client.revenueMetric != "" {
			metricNames = append(metricNames, b.client.revenueMetric)
		}
		metrics, _ := jsonQuery(metricNames)
		q := url.Values{
			"advertiser_id": {b.client.advertiserID}, "service_type": {"AUCTION"}, "report_type": {"BASIC"},
			"data_level": {dataLevel}, "dimensions": {dimensions}, "metrics": {metrics},
			"start_date": {query.Start}, "end_date": {query.End},
		}
		putInt(q, "page", page)
		putInt(q, "page_size", pageSize)
		var data reportData
		requestID, pageErr := b.client.get(ctx, "/open_api/v1.3/report/integrated/get/", q, &data)
		if pageErr != nil {
			return ads.Report{}, pageErr
		}
		if requestID != "" {
			requestIDs = append(requestIDs, requestID)
		}
		for _, wire := range data.List {
			entityID := wire.Dimensions[dimensionID]
			if query.Level == ads.Advertiser && entityID == "" {
				entityID = b.client.advertiserID
			}
			if entityID == "" || (query.Level == ads.Advertiser && entityID != b.client.advertiserID) {
				return ads.Report{}, errors.New("TikTok report row has an invalid entity identity")
			}
			if query.EntityID != "" && entityID != query.EntityID {
				continue
			}
			date := wire.Dimensions["stat_time_day"]
			if len(date) >= 10 {
				date = date[:10]
			}
			if date < query.Start || date > query.End {
				return ads.Report{}, errors.New("TikTok report row is outside the requested date range")
			}
			metric, parseErr := decodeMetrics(wire.Metrics, b.client.revenueMetric)
			if parseErr != nil {
				return ads.Report{}, parseErr
			}
			rows = append(rows, ads.Row{EntityID: entityID, Date: date, Metrics: metric})
		}
		if !morePages(data.PageInfo, page, len(data.List), pageSize) {
			sort.Slice(rows, func(i, j int) bool {
				if rows[i].Date == rows[j].Date {
					return rows[i].EntityID < rows[j].EntityID
				}
				return rows[i].Date < rows[j].Date
			})
			totals := totalMetrics(rows)
			attribution := "TikTok MAPI BASIC report; conversions map to the selected optimization event. Revenue is unavailable until an advertiser-specific value metric is configured."
			if b.client.revenueMetric != "" {
				attribution = "TikTok MAPI BASIC report; conversions map to conversion and revenue maps explicitly to " + b.client.revenueMetric + "."
			}
			return ads.Report{
				ID: reportID(b.client.advertiserID, query, requestIDs), Source: b.source(), Query: query,
				Currency: account.Currency, Timezone: account.Timezone,
				Attribution: attribution,
				FetchedAt:   time.Now().UTC(), Complete: true,
				Limitations: []string{"MAPI results can lag or be revised; no transactional snapshot is guaranteed across pages."},
				Rows:        rows, Totals: totals, RequestIDs: requestIDs,
			}, nil
		}
	}
	return ads.Report{}, errors.New("TikTok report exceeded the configured page limit")
}

func decodeMetrics(v map[string]json.RawMessage, revenueMetric string) (ads.Metrics, error) {
	spend, ok, err := decimalMetric(v, "spend")
	if err != nil || !ok || spend.IsNegative() {
		return ads.Metrics{}, errors.New("TikTok report spend is missing or invalid")
	}
	impressions, err := integerMetric(v, "impressions")
	if err != nil || impressions < 0 {
		return ads.Metrics{}, errors.New("TikTok report impressions are invalid")
	}
	clicks, err := integerMetric(v, "clicks")
	if err != nil || clicks < 0 {
		return ads.Metrics{}, errors.New("TikTok report clicks are invalid")
	}
	conversion, hasConversion, err := decimalMetric(v, "conversion")
	if err != nil || (hasConversion && conversion.IsNegative()) {
		return ads.Metrics{}, errors.New("TikTok report conversion is invalid")
	}
	revenue, hasRevenue := decimal.Zero, false
	if revenueMetric != "" {
		revenue, hasRevenue, err = decimalMetric(v, revenueMetric)
		if err != nil || (hasRevenue && revenue.IsNegative()) {
			return ads.Metrics{}, errors.New("TikTok report purchase value is invalid")
		}
	}
	m := ads.Metrics{Spend: spend, Impressions: impressions, Clicks: clicks}
	if hasConversion {
		m.Conversions = &conversion
	}
	if hasRevenue {
		m.Revenue = &revenue
	}
	return m, nil
}

func decimalMetric(v map[string]json.RawMessage, key string) (decimal.Decimal, bool, error) {
	raw, ok := v[key]
	if !ok || len(raw) == 0 || string(raw) == "null" || string(raw) == `"-"` {
		return decimal.Zero, false, nil
	}
	var s string
	if json.Unmarshal(raw, &s) != nil {
		var n json.Number
		if json.Unmarshal(raw, &n) != nil {
			return decimal.Zero, false, errors.New("invalid metric")
		}
		s = n.String()
	}
	d, err := decimal.NewFromString(s)
	return d, true, err
}

func integerMetric(v map[string]json.RawMessage, key string) (int64, error) {
	d, ok, err := decimalMetric(v, key)
	if err != nil || !ok || !d.Equal(d.Truncate(0)) {
		return 0, errors.New("missing or non-integer metric")
	}
	return d.IntPart(), nil
}

func totalMetrics(rows []ads.Row) ads.Metrics {
	t := ads.Metrics{}
	hasConversion, hasRevenue := true, true
	conversion, revenue := decimal.Zero, decimal.Zero
	for _, row := range rows {
		t.Spend = t.Spend.Add(row.Metrics.Spend)
		t.Impressions += row.Metrics.Impressions
		t.Clicks += row.Metrics.Clicks
		if row.Metrics.Conversions == nil {
			hasConversion = false
		} else {
			conversion = conversion.Add(*row.Metrics.Conversions)
		}
		if row.Metrics.Revenue == nil {
			hasRevenue = false
		} else {
			revenue = revenue.Add(*row.Metrics.Revenue)
		}
	}
	if hasConversion {
		t.Conversions = &conversion
	}
	if hasRevenue {
		t.Revenue = &revenue
	}
	return t
}

func reportID(account string, query ads.ReportQuery, requestIDs []string) string {
	return fmt.Sprintf("tiktok:%s:%s:%s:%s:%s:%s", account, query.Level, query.Start, query.End, query.EntityID, strings.Join(requestIDs, ","))
}
