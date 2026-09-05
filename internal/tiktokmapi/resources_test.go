package tiktokmapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestCommonAdvertisingResourcesUseTikTokMAPIEndpoints(t *testing.T) {
	called := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called[r.URL.Path]++
		if r.URL.Query().Get("advertiser_id") != "adv-1" {
			t.Errorf("%s missing advertiser binding", r.URL.Path)
		}
		paged := func(items ...any) {
			writeEnvelope(t, w, 200, 0, "resource", map[string]any{"list": items, "page_info": map[string]any{"total_page": 1}})
		}
		switch r.URL.Path {
		case "/open_api/v1.3/identity/get/":
			paged(map[string]any{"identity_id": "identity-1", "display_name": "Brand", "identity_type": "BC_AUTH_TT", "identity_status": "AUTHORIZED"})
		case "/open_api/v1.3/file/image/ad/search/":
			paged(map[string]any{"image_id": "image-1", "image_name": "Square", "width": 1080, "height": 1080, "status": "READY"})
		case "/open_api/v1.3/file/video/ad/search/":
			paged(map[string]any{"video_id": "video-1", "video_name": "Story", "width": "1080", "height": "1920", "duration": 15, "review_status": "APPROVED"})
		case "/open_api/v1.3/dmp/custom_audience/list/":
			paged(map[string]any{"custom_audience_id": "audience-1", "custom_audience_name": "Purchasers", "audience_size": "2500", "status": "READY"})
		case "/open_api/v1.3/dmp/saved_audience/list/":
			paged(map[string]any{"saved_audience_id": "saved-1", "saved_audience_name": "Prospecting", "status": "READY"})
		case "/open_api/v1.3/audience/insight/overlap/":
			writeEnvelope(t, w, 200, 0, "overlap", map[string]any{"overlap_users": "200", "left_rate": 0.08, "right_rate": "0.04"})
		case "/open_api/v1.3/tool/targeting/list/":
			if r.URL.Query().Get("targeting_type") != "INTEREST" {
				t.Errorf("targeting type=%q", r.URL.Query().Get("targeting_type"))
			}
			paged(map[string]any{"targeting_id": "interest-1", "targeting_name": "Fitness", "status": "ENABLED"})
		case "/open_api/v1.3/pixel/list/":
			paged(map[string]any{"pixel_id": "pixel-1", "pixel_name": "Checkout", "status": "ACTIVE", "event_types": []string{"Purchase"}})
		case "/open_api/v1.3/app/list/":
			paged(map[string]any{"app_id": "app-1", "app_name": "Mobile App", "status": "ACTIVE"})
		case "/open_api/v1.3/offline/get/":
			paged(map[string]any{"offline_event_set_id": "offline-1", "offline_name": "Retail", "status": "ACTIVE"})
		case "/open_api/v1.3/pixel/event/stats/":
			if r.URL.Query().Get("pixel_id") != "pixel-1" {
				t.Errorf("pixel=%q", r.URL.Query().Get("pixel_id"))
			}
			paged(map[string]any{"event_name": "Purchase", "event_count": "42"})
		case "/open_api/v1.3/page/library/get/":
			paged(map[string]any{"page_id": "page-1", "page_name": "Demo request", "status": "ACTIVE"})
		case "/open_api/v1.3/page/field/get/":
			if r.URL.Query().Get("page_id") != "page-1" {
				t.Errorf("page=%q", r.URL.Query().Get("page_id"))
			}
			writeEnvelope(t, w, 200, 0, "fields", map[string]any{"page_id": "page-1", "page_name": "Demo request", "status": "ACTIVE", "fields": []any{map[string]any{"field_name": "email"}, map[string]any{"field_name": "full_name"}}})
		case "/open_api/v1.3/catalog/get/":
			paged(map[string]any{"catalog_id": "catalog-1", "catalog_name": "Products", "currency": "USD", "status": "ACTIVE", "product_count": "80", "issue_count": 2})
		case "/open_api/v1.3/catalog/set/get/":
			if r.URL.Query().Get("catalog_id") != "catalog-1" {
				t.Errorf("catalog=%q", r.URL.Query().Get("catalog_id"))
			}
			paged(map[string]any{"product_set_id": "set-1", "product_set_name": "Bestsellers", "product_count": 12})
		case "/open_api/v1.3/optimizer/rule/list/":
			paged(map[string]any{"rule_id": "rule-1", "rule_name": "Budget guard", "status": "ENABLED", "target_level": "ADGROUP", "action_type": "NOTIFY", "schedule_type": "HOURLY"})
		case "/open_api/v1.3/optimizer/rule/result/list/":
			if r.URL.Query().Get("rule_id") != "rule-1" {
				t.Errorf("rule=%q", r.URL.Query().Get("rule_id"))
			}
			paged(map[string]any{"result_id": "result-1", "rule_id": "rule-1", "status": "SUCCEEDED", "affected_count": 1, "execute_time": "2026-09-04 01:00:00"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	b := newTestBackend(t, server, 2)
	ctx := context.Background()
	identities, err := b.ListIdentities(ctx)
	if err != nil || len(identities) != 1 {
		t.Fatalf("identities=%#v err=%v", identities, err)
	}
	creative, err := b.ListCreativeAssets(ctx)
	if err != nil || len(creative) != 2 || creative[1].DurationMS != 15000 {
		t.Fatalf("creative=%#v err=%v", creative, err)
	}
	audiences, err := b.ListAudiences(ctx)
	if err != nil || len(audiences) != 2 {
		t.Fatalf("audiences=%#v err=%v", audiences, err)
	}
	overlap, err := b.GetAudienceOverlap(ctx, "audience-1", "saved-1")
	if err != nil || !overlap.Complete || overlap.OverlapUsers == nil || *overlap.OverlapUsers != 200 {
		t.Fatalf("overlap=%#v err=%v", overlap, err)
	}
	targeting, err := b.ListTargetingOptions(ctx, "interest")
	if err != nil || len(targeting) != 1 {
		t.Fatalf("targeting=%#v err=%v", targeting, err)
	}
	sources, err := b.ListEventSources(ctx)
	if err != nil || len(sources) != 3 {
		t.Fatalf("sources=%#v err=%v", sources, err)
	}
	stats, err := b.GetEventStats(ctx, "pixel-1", "2026-09-01", "2026-09-04")
	if err != nil || stats.Events["Purchase"] != 42 {
		t.Fatalf("stats=%#v err=%v", stats, err)
	}
	forms, err := b.ListLeadForms(ctx)
	if err != nil || len(forms) != 1 {
		t.Fatalf("forms=%#v err=%v", forms, err)
	}
	form, err := b.GetLeadForm(ctx, "page-1")
	if err != nil || !reflect.DeepEqual(form.FieldNames, []string{"email", "full_name"}) {
		t.Fatalf("form=%#v err=%v", form, err)
	}
	catalogs, err := b.ListCatalogs(ctx)
	if err != nil || len(catalogs) != 1 || catalogs[0].IssueCount != 2 {
		t.Fatalf("catalogs=%#v err=%v", catalogs, err)
	}
	sets, err := b.ListProductSets(ctx, "catalog-1")
	if err != nil || len(sets) != 1 {
		t.Fatalf("sets=%#v err=%v", sets, err)
	}
	rules, err := b.ListAutomatedRules(ctx)
	if err != nil || len(rules) != 1 {
		t.Fatalf("rules=%#v err=%v", rules, err)
	}
	results, err := b.ListAutomatedRuleResults(ctx, "rule-1")
	if err != nil || len(results) != 1 || results[0].AffectedCount != 1 {
		t.Fatalf("results=%#v err=%v", results, err)
	}

	wantPaths := []string{
		"/open_api/v1.3/identity/get/", "/open_api/v1.3/file/image/ad/search/", "/open_api/v1.3/file/video/ad/search/",
		"/open_api/v1.3/dmp/custom_audience/list/", "/open_api/v1.3/dmp/saved_audience/list/", "/open_api/v1.3/audience/insight/overlap/",
		"/open_api/v1.3/tool/targeting/list/", "/open_api/v1.3/pixel/list/", "/open_api/v1.3/app/list/", "/open_api/v1.3/offline/get/",
		"/open_api/v1.3/pixel/event/stats/", "/open_api/v1.3/page/library/get/", "/open_api/v1.3/page/field/get/",
		"/open_api/v1.3/catalog/get/", "/open_api/v1.3/catalog/set/get/", "/open_api/v1.3/optimizer/rule/list/", "/open_api/v1.3/optimizer/rule/result/list/",
	}
	for _, path := range wantPaths {
		if called[path] != 1 {
			t.Errorf("%s calls=%d", path, called[path])
		}
	}
}
