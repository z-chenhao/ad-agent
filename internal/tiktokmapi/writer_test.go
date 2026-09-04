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

func TestWriterUsesTypedSingleObjectEndpoints(t *testing.T) {
	cases := []struct {
		name string
		req  ads.WriteRequest
		path string
		body map[string]any
	}{
		{"campaign budget", budgetWrite(ads.Campaign, "c1"), "/open_api/v1.3/campaign/update/", map[string]any{"advertiser_id": "adv-1", "campaign_id": "c1", "budget": json.Number("60.25")}},
		{"ad group budget", budgetWrite(ads.AdGroup, "g1"), "/open_api/v1.3/adgroup/update/", map[string]any{"advertiser_id": "adv-1", "adgroup_id": "g1", "budget": json.Number("60.25")}},
		{"campaign status", statusWrite(ads.Campaign, "c1"), "/open_api/v1.3/campaign/status/update/", map[string]any{"advertiser_id": "adv-1", "campaign_ids": []any{"c1"}, "operation_status": "DISABLE"}},
		{"ad group status", statusWrite(ads.AdGroup, "g1"), "/open_api/v1.3/adgroup/status/update/", map[string]any{"advertiser_id": "adv-1", "adgroup_ids": []any{"g1"}, "allow_partial_success": false, "operation_status": "DISABLE"}},
		{"ad status", statusWrite(ads.Ad, "a1"), "/open_api/v1.3/ad/status/update/", map[string]any{"advertiser_id": "adv-1", "ad_ids": []any{"a1"}, "operation_status": "DISABLE"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != tc.path || r.Header.Get("Access-Token") != testToken {
					t.Fatalf("request %s %s", r.Method, r.URL.Path)
				}
				decoder := json.NewDecoder(r.Body)
				decoder.UseNumber()
				var got map[string]any
				if decoder.Decode(&got) != nil || !reflect.DeepEqual(got, tc.body) {
					t.Fatalf("body=%#v want=%#v", got, tc.body)
				}
				writeEnvelope(t, w, 200, 0, "write-1", map[string]any{})
			}))
			defer server.Close()
			outcome := newTestBackend(t, server, 1).Write(context.Background(), tc.req)
			if outcome.State != "acknowledged" || outcome.RequestID != "write-1" {
				t.Fatalf("outcome=%#v", outcome)
			}
		})
	}
}

func TestWriterClassifiesRejectedUnknownAndNotSent(t *testing.T) {
	t.Run("business rejection", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeEnvelope(t, w, 200, 40001, "reject-1", nil)
		}))
		defer server.Close()
		got := newTestBackend(t, server, 1).Write(context.Background(), statusWrite(ads.Campaign, "c1"))
		if got.State != "rejected" || got.RequestID != "reject-1" {
			t.Fatalf("outcome=%#v", got)
		}
	})
	t.Run("server result unknown", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeEnvelope(t, w, 503, 0, "unknown-1", nil)
		}))
		defer server.Close()
		got := newTestBackend(t, server, 1).Write(context.Background(), statusWrite(ads.Ad, "a1"))
		if got.State != "unknown" || got.RequestID != "unknown-1" {
			t.Fatalf("outcome=%#v", got)
		}
	})
	t.Run("invalid request not sent", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("unexpected request") }))
		defer server.Close()
		got := newTestBackend(t, server, 1).Write(context.Background(), budgetWrite(ads.Ad, "a1"))
		if got.State != "not_sent" || got.Message != "budget_not_supported" {
			t.Fatalf("outcome=%#v", got)
		}
	})
}

func budgetWrite(level ads.Level, id string) ads.WriteRequest {
	value := decimal.RequireFromString("60.25")
	return ads.WriteRequest{Target: ads.Entity{ID: id, AccountID: "adv-1", Level: level, Name: "Object", Status: "ENABLE", Budget: ptrDecimal("50")}, Kind: string(ads.BudgetChange), Budget: &value}
}

func statusWrite(level ads.Level, id string) ads.WriteRequest {
	return ads.WriteRequest{Target: ads.Entity{ID: id, AccountID: "adv-1", Level: level, Name: "Object", Status: "ENABLE"}, Kind: string(ads.StatusChange), Status: "DISABLE"}
}

func ptrDecimal(value string) *decimal.Decimal {
	v := decimal.RequireFromString(value)
	return &v
}
