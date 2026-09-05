package tiktokmapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/z-chenhao/ad-agent/internal/ads"
)

func TestBackendAccountEntitiesAndPagination(t *testing.T) {
	var mu sync.Mutex
	seenPages := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Access-Token") != testToken {
			t.Errorf("missing token")
		}
		switch r.URL.Path {
		case "/open_api/v1.3/advertiser/info/":
			if r.URL.Query().Get("advertiser_ids") != `["adv-1"]` {
				t.Errorf("advertiser_ids=%q", r.URL.Query().Get("advertiser_ids"))
			}
			writeEnvelope(t, w, 200, 0, "acct", map[string]any{"list": []any{map[string]any{"advertiser_id": "adv-1", "name": "Local Shop", "currency": "USD", "timezone": "Asia/Shanghai"}}})
		case "/open_api/v1.3/campaign/get/":
			page := r.URL.Query().Get("page")
			mu.Lock()
			seenPages = append(seenPages, page)
			mu.Unlock()
			item := map[string]any{"advertiser_id": "adv-1", "campaign_id": "c" + page, "campaign_name": "Campaign " + page, "operation_status": "ENABLE", "budget": "50.25", "budget_mode": "BUDGET_MODE_TOTAL", "objective_type": "TRAFFIC"}
			pageNumber := 1
			if page == "2" {
				pageNumber = 2
			}
			writeEnvelope(t, w, 200, 0, "campaign-"+page, map[string]any{"list": []any{item}, "page_info": map[string]any{"page": pageNumber, "page_size": 1000, "total_page": 2, "total_number": 2}})
		case "/open_api/v1.3/adgroup/get/":
			var filtering map[string][]string
			if json.Unmarshal([]byte(r.URL.Query().Get("filtering")), &filtering) != nil || !reflect.DeepEqual(filtering["campaign_ids"], []string{"c1"}) {
				t.Errorf("filtering=%q", r.URL.Query().Get("filtering"))
			}
			item := map[string]any{"advertiser_id": "adv-1", "campaign_id": "c1", "adgroup_id": "g1", "adgroup_name": "Group", "operation_status": "STATUS_DISABLE", "budget": 12.5, "budget_mode": "BUDGET_MODE_DAY", "objective_type": "TRAFFIC"}
			writeEnvelope(t, w, 200, 0, "group", map[string]any{"list": []any{item}, "page_info": map[string]any{"total_page": 1}})
		case "/open_api/v1.3/ad/get/":
			item := map[string]any{"advertiser_id": "adv-1", "campaign_id": "c1", "adgroup_id": "g1", "ad_id": "a1", "ad_name": "Ad", "operation_status": "ENABLE"}
			writeEnvelope(t, w, 200, 0, "ad", map[string]any{"list": []any{item}, "page_info": map[string]any{"total_page": 1}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	b := newTestBackend(t, server, 3)
	account, err := b.Account(context.Background())
	if err != nil || account.ID != "adv-1" || account.Currency != "USD" || account.Source.Environment != "sandbox" {
		t.Fatalf("account=%#v err=%v", account, err)
	}
	campaigns, err := b.List(context.Background(), ads.EntityQuery{Level: ads.Campaign})
	if err != nil || len(campaigns) != 2 || campaigns[0].Budget == nil || campaigns[0].Budget.String() != "50.25" {
		t.Fatalf("campaigns=%#v err=%v", campaigns, err)
	}
	if !reflect.DeepEqual(seenPages, []string{"1", "2"}) {
		t.Fatalf("pages=%v", seenPages)
	}
	groups, err := b.List(context.Background(), ads.EntityQuery{Level: ads.AdGroup, ParentID: "c1"})
	if err != nil || len(groups) != 1 || groups[0].Status != "DISABLE" || groups[0].ParentID != "c1" {
		t.Fatalf("groups=%#v err=%v", groups, err)
	}
	ad, err := b.Get(context.Background(), ads.Ad, "a1")
	if err != nil || ad.ParentID != "g1" {
		t.Fatalf("ad=%#v err=%v", ad, err)
	}
}

func TestBackendReportPreservesQueriesTotalsAndMissingMetrics(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/open_api/v1.3/advertiser/info/":
			writeEnvelope(t, w, 200, 0, "acct", map[string]any{"list": []any{map[string]any{"advertiser_id": "adv-1", "name": "Shop", "currency": "USD", "timezone": "UTC"}}})
		case "/open_api/v1.3/report/integrated/get/":
			q := r.URL.Query()
			if q.Get("service_type") != "AUCTION" || q.Get("report_type") != "BASIC" || q.Get("data_level") != "AUCTION_CAMPAIGN" {
				t.Errorf("query=%v", q)
			}
			assertJSONArray(t, q, "dimensions", []string{"campaign_id", "stat_time_day"})
			assertJSONArray(t, q, "metrics", []string{"spend", "impressions", "clicks", "conversion", "total_purchase_value"})
			rows := []any{
				map[string]any{"dimensions": map[string]string{"campaign_id": "c1", "stat_time_day": "2026-09-01 00:00:00"}, "metrics": map[string]any{"spend": "10.10", "impressions": "100", "clicks": "5", "conversion": "2", "total_purchase_value": "30.3"}},
				map[string]any{"dimensions": map[string]string{"campaign_id": "c2", "stat_time_day": "2026-09-01"}, "metrics": map[string]any{"spend": "20.20", "impressions": "200", "clicks": "8", "conversion": nil, "total_purchase_value": nil}},
			}
			writeEnvelope(t, w, 200, 0, "report-1", map[string]any{"list": rows, "page_info": map[string]any{"total_page": 1}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	b := newTestBackendWithRevenue(t, server, 2, "total_purchase_value")
	report, err := b.Report(context.Background(), ads.ReportQuery{Level: ads.Campaign, Start: "2026-09-01", End: "2026-09-01"})
	if err != nil {
		t.Fatal(err)
	}
	if report.Source.Backend != "tiktok-mapi" || !report.Complete || len(report.Rows) != 2 || report.Totals.Spend.String() != "30.3" {
		t.Fatalf("report=%#v", report)
	}
	if report.Totals.Conversions != nil || report.Totals.Revenue != nil {
		t.Fatalf("missing metrics must remain unavailable: %#v", report.Totals)
	}
	if !reflect.DeepEqual(report.RequestIDs, []string{"report-1"}) {
		t.Fatalf("request IDs=%v", report.RequestIDs)
	}
}

func TestBackendRejectsDailyReportOverThirtyDaysBeforeNetwork(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("overlong report must be rejected before an HTTP request")
	}))
	defer server.Close()
	_, err := newTestBackend(t, server, 2).Report(context.Background(), ads.ReportQuery{Level: ads.Campaign, Start: "2026-08-01", End: "2026-08-31"})
	if err == nil || !strings.Contains(err.Error(), "1–30") {
		t.Fatalf("err=%v", err)
	}
}

func TestBackendRejectsCrossAccountAndPageTruncation(t *testing.T) {
	t.Run("cross account", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			item := map[string]any{"advertiser_id": "adv-other", "campaign_id": "c1", "campaign_name": "Wrong", "operation_status": "ENABLE"}
			writeEnvelope(t, w, 200, 0, "x", map[string]any{"list": []any{item}, "page_info": map[string]any{"total_page": 1}})
		}))
		defer server.Close()
		_, err := newTestBackend(t, server, 1).List(context.Background(), ads.EntityQuery{Level: ads.Campaign})
		if err == nil || !strings.Contains(err.Error(), "different advertiser") {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("page cap", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			item := map[string]any{"advertiser_id": "adv-1", "campaign_id": "c1", "campaign_name": "One", "operation_status": "ENABLE"}
			writeEnvelope(t, w, 200, 0, "x", map[string]any{"list": []any{item}, "page_info": map[string]any{"total_page": 2}})
		}))
		defer server.Close()
		_, err := newTestBackend(t, server, 1).List(context.Background(), ads.EntityQuery{Level: ads.Campaign})
		if err == nil || !strings.Contains(err.Error(), "page limit") {
			t.Fatalf("err=%v", err)
		}
	})
}

func newTestBackend(t *testing.T, server *httptest.Server, maxPages int) *Backend {
	return newTestBackendWithRevenue(t, server, maxPages, "")
}

func newTestBackendWithRevenue(t *testing.T, server *httptest.Server, maxPages int, revenueMetric string) *Backend {
	t.Helper()
	c, err := NewClient(Config{
		BaseURL: server.URL, AdvertiserID: "adv-1", Environment: "sandbox",
		HTTPClient: server.Client(), MaxPages: maxPages, RevenueMetric: revenueMetric,
		Tokens: TokenResolverFunc(func(context.Context, string) (string, error) { return testToken, nil }),
	})
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewBackend(c)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestBackendLeavesRevenueUnavailableWithoutExplicitMetric(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/open_api/v1.3/advertiser/info/":
			writeEnvelope(t, w, 200, 0, "acct", map[string]any{"list": []any{map[string]any{"advertiser_id": "adv-1", "name": "Shop", "currency": "USD", "timezone": "UTC"}}})
		case "/open_api/v1.3/report/integrated/get/":
			var metrics []string
			_ = json.Unmarshal([]byte(r.URL.Query().Get("metrics")), &metrics)
			for _, metric := range metrics {
				if strings.Contains(metric, "value") || strings.Contains(metric, "payment_rate") {
					t.Fatalf("unconfigured revenue metric requested: %s", metric)
				}
			}
			row := map[string]any{"dimensions": map[string]string{"campaign_id": "c1", "stat_time_day": "2026-09-01"}, "metrics": map[string]any{"spend": "10", "impressions": "100", "clicks": "5", "conversion": "2"}}
			writeEnvelope(t, w, 200, 0, "report", map[string]any{"list": []any{row}, "page_info": map[string]any{"total_page": 1}})
		}
	}))
	defer server.Close()
	report, err := newTestBackend(t, server, 1).Report(context.Background(), ads.ReportQuery{Level: ads.Campaign, Start: "2026-09-01", End: "2026-09-01"})
	if err != nil {
		t.Fatal(err)
	}
	if report.Totals.Revenue != nil || !strings.Contains(report.Attribution, "unavailable") {
		t.Fatalf("report=%#v", report)
	}
}

func assertJSONArray(t *testing.T, q url.Values, key string, want []string) {
	t.Helper()
	var got []string
	if json.Unmarshal([]byte(q.Get(key)), &got) != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("%s=%q", key, q.Get(key))
	}
}
