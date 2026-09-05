package tiktokmapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/z-chenhao/ad-agent/internal/ads"
)

func TestCampaignBundleUsesOfficialSequentialCreateEndpoints(t *testing.T) {
	paths := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		var body map[string]any
		decoder := json.NewDecoder(r.Body)
		decoder.UseNumber()
		if decoder.Decode(&body) != nil || body["advertiser_id"] != "adv-1" {
			t.Fatalf("invalid body %#v", body)
		}
		switch r.URL.Path {
		case "/open_api/v1.3/campaign/create/":
			if body["operation_status"] != "DISABLE" || body["objective_type"] != "WEB_CONVERSIONS" {
				t.Fatalf("campaign body=%#v", body)
			}
			writeEnvelope(t, w, 200, 0, "req-campaign", map[string]any{"campaign_id": "campaign-new"})
		case "/open_api/v1.3/adgroup/create/":
			if body["campaign_id"] != "campaign-new" || body["pixel_id"] != "pixel-1" || body["operation_status"] != "DISABLE" {
				t.Fatalf("ad group body=%#v", body)
			}
			writeEnvelope(t, w, 200, 0, "req-group", map[string]any{"adgroup_id": "group-new"})
		case "/open_api/v1.3/ad/create/":
			if body["adgroup_id"] != "group-new" {
				t.Fatalf("ad body=%#v", body)
			}
			writeEnvelope(t, w, 200, 0, "req-ad", map[string]any{"ad_ids": []string{"ad-new"}})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	b := newTestBackend(t, server, 1)
	budget := decimal.RequireFromString("120")
	outcome := b.createCampaignBundle(context.Background(), ads.CampaignBundleSpec{
		Campaign: ads.CampaignSpec{Name: "Launch", Objective: "WEB_CONVERSIONS", Status: "DISABLE"},
		AdGroup:  ads.AdGroupSpec{Name: "Purchase", Budget: budget, BudgetMode: "BUDGET_MODE_DAY", BillingEvent: "OCPM", OptimizationGoal: "CONVERT", OptimizationEvent: "Purchase", Pacing: "PACING_MODE_SMOOTH", ScheduleType: "SCHEDULE_START_END", ScheduleStart: "2026-09-06 00:00:00", Placements: []string{"PLACEMENT_TIKTOK"}, LocationIDs: []string{"US"}, PixelID: "pixel-1", Status: "DISABLE"},
		Ads:      []ads.AdCreativeSpec{{Name: "Creative", IdentityID: "identity-1", IdentityType: "CUSTOMIZED_USER", AssetID: "video-1", AssetKind: "video", PrimaryText: "Text", CallToAction: "SHOP_NOW", DestinationURL: "https://example.com", Status: "DISABLE"}},
	})
	if outcome.State != "acknowledged" || len(outcome.Resources) != 3 || !reflect.DeepEqual(paths, []string{"/open_api/v1.3/campaign/create/", "/open_api/v1.3/adgroup/create/", "/open_api/v1.3/ad/create/"}) {
		t.Fatalf("outcome=%#v paths=%#v", outcome, paths)
	}
}

func TestCampaignBundleReportsPartialWithoutRetry(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path == "/open_api/v1.3/campaign/create/" {
			writeEnvelope(t, w, 200, 0, "req-1", map[string]any{"campaign_id": "campaign-new"})
			return
		}
		writeEnvelope(t, w, 200, 40001, "req-rejected", nil)
	}))
	defer server.Close()
	b := newTestBackend(t, server, 1)
	budget := decimal.NewFromInt(20)
	outcome := b.createCampaignBundle(context.Background(), ads.CampaignBundleSpec{Campaign: ads.CampaignSpec{Name: "Partial", Objective: "TRAFFIC", Status: "DISABLE"}, AdGroup: ads.AdGroupSpec{Name: "Group", Budget: budget, BudgetMode: "BUDGET_MODE_DAY", BillingEvent: "CPC", OptimizationGoal: "CLICK", Pacing: "PACING_MODE_SMOOTH", ScheduleType: "SCHEDULE_FROM_NOW", ScheduleStart: "2026-09-06 00:00:00", Placements: []string{"PLACEMENT_TIKTOK"}, LocationIDs: []string{"US"}, Status: "DISABLE"}, Ads: []ads.AdCreativeSpec{{Name: "Ad"}}})
	if outcome.State != "partial" || calls != 2 || len(outcome.Resources) != 1 {
		t.Fatalf("outcome=%#v calls=%d", outcome, calls)
	}
}

func TestCommentAudienceMeasurementAndFinanceWireMappings(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/open_api/v1.3/comment/status/update/":
			writeEnvelope(t, w, 200, 0, "comment-write", map[string]any{})
		case "/open_api/v1.3/dmp/saved_audience/create/":
			writeEnvelope(t, w, 200, 0, "audience-write", map[string]any{"saved_audience_id": "aud-new"})
		case "/open_api/v1.3/pixel/create/":
			writeEnvelope(t, w, 200, 0, "pixel-write", map[string]any{"pixel_id": "pixel-new"})
		case "/open_api/v1.3/advertiser/balance/get/":
			if r.URL.Query().Get("bc_id") != "bc-1" {
				t.Fatalf("missing bc scope")
			}
			writeEnvelope(t, w, 200, 0, "balance-read", map[string]any{"list": []any{map[string]any{"advertiser_id": "adv-1", "currency": "USD", "balance": "1250.25", "cash_balance": "1000", "grant_balance": "250.25"}}})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	b := newTestBackend(t, server, 1)
	b.client.businessCenterID = "bc-1"
	comment := b.applyComment(context.Background(), ads.CommentActionSpec{CommentID: "comment-1", AdID: "ad-1", TikTokItemID: "item-1", Action: "hide"})
	audience := b.createAudience(context.Background(), ads.AudienceCreateSpec{Name: "US", Kind: "saved", LocationIDs: []string{"US"}})
	pixel := b.createSingle(context.Background(), "/open_api/v1.3/pixel/create/", map[string]any{"advertiser_id": "adv-1", "pixel_name": "Web"}, "event_source", "Web", "pixel_id")
	balance, err := b.GetBillingBalance(context.Background())
	if comment.State != "acknowledged" || audience.Resources[0].ID != "aud-new" || pixel.Resources[0].ID != "pixel-new" || err != nil || !balance.Available.Equal(decimal.RequireFromString("1250.25")) {
		t.Fatalf("comment=%#v audience=%#v pixel=%#v balance=%#v err=%v", comment, audience, pixel, balance, err)
	}
}

func TestAutomatedRuleCreateMatchesOfficialSDKShape(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/open_api/v1.3/optimizer/rule/create/" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		var body struct {
			Rules []map[string]any `json:"rules"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.Rules) != 1 {
			t.Fatalf("body=%#v err=%v", body, err)
		}
		if _, invented := body.Rules[0]["status"]; invented {
			t.Fatal("official v1.3 create model has no status field")
		}
		rule := body.Rules[0]
		objects := rule["apply_objects"].([]any)
		object := objects[0].(map[string]any)
		conditions := rule["conditions"].([]any)
		condition := conditions[0].(map[string]any)
		actions := rule["actions"].([]any)
		action := actions[0].(map[string]any)
		execution := rule["rule_exec_info"].(map[string]any)
		notification := rule["notification"].(map[string]any)
		if object["dimension"] != "ADGROUP" || object["pre_condition_type"] != "SELECTED" || condition["subject_type"] != "CPA" || condition["match_type"] != "GT" || condition["range_type"] != "PAST_THREE_DAYS" || action["subject_type"] != "MESSAGE" || execution["exec_time_type"] != "PER_HALF_HOUR" || notification["notification_type"] != "NOT_NOTIFICATION" {
			t.Fatalf("rule does not match official enum shape: %#v", rule)
		}
		writeEnvelope(t, w, 200, 0, "rule-write", map[string]any{"rule_id": "rule-new"})
	}))
	defer server.Close()
	b := newTestBackend(t, server, 1)
	outcome := b.createRule(context.Background(), ads.AutomatedRuleCreateSpec{
		Name: "Notify high CPA", TargetLevel: ads.AdGroup, TargetIDs: []string{"group-1"},
		Conditions: []ads.RuleCondition{{Metric: "CPA", Operator: "GT", Value: decimal.NewFromInt(60), Window: "LAST_3_DAYS"}},
		Action:     "NOTIFY", Schedule: "EVERY_30_MINUTES",
	})
	if outcome.State != "acknowledged" || len(outcome.Resources) != 1 || outcome.Resources[0].ID != "rule-new" {
		t.Fatalf("outcome=%#v", outcome)
	}
}

func TestAdGroupReconciliationIncludesScheduleStartAndPlacements(t *testing.T) {
	current := ads.AdGroupUpdateSpec{
		AdGroupID: "group-1", ScheduleStart: "2026-09-06 00:00:00", ScheduleEnd: "2026-09-30 23:59:59",
		Placements: []string{"PLACEMENT_TIKTOK"},
	}
	if !adGroupMatches(current, current) {
		t.Fatal("matching delivery settings did not reconcile")
	}
	requested := current
	requested.ScheduleStart = "2026-09-07 00:00:00"
	if adGroupMatches(current, requested) {
		t.Fatal("schedule-start mismatch reconciled")
	}
	requested = current
	requested.Placements = []string{"PLACEMENT_PANGLE"}
	if adGroupMatches(current, requested) {
		t.Fatal("placement mismatch reconciled")
	}
}

func TestAdGroupUpdateMapsScheduleStartAndPlacements(t *testing.T) {
	body := adGroupUpdateBody("adv-1", ads.AdGroupUpdateSpec{
		AdGroupID: "group-1", ScheduleStart: "2026-09-06 00:00:00", ScheduleEnd: "2026-09-30 23:59:59",
		Placements: []string{"PLACEMENT_TIKTOK"},
	})
	if body["advertiser_id"] != "adv-1" || body["adgroup_id"] != "group-1" || body["schedule_start_time"] != "2026-09-06 00:00:00" || body["schedule_end_time"] != "2026-09-30 23:59:59" || !reflect.DeepEqual(body["placements"], []string{"PLACEMENT_TIKTOK"}) {
		t.Fatalf("incomplete ad-group update body: %#v", body)
	}
}
