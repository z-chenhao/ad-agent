package tiktokmapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/z-chenhao/ad-agent/internal/ads"
)

func TestCreationReadbackRequiresExactSubmittedFields(t *testing.T) {
	for _, mode := range []string{"match", "missing", "mismatch", "wrong_account"} {
		t.Run(mode, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" || r.URL.Query().Get("advertiser_id") != "adv-1" {
					t.Error("readback must be an account-bound read")
				}
				row := map[string]any{"campaign_id": "c1", "advertiser_id": "adv-1", "campaign_name": "Approved launch", "operation_status": "DISABLE", "budget": "120"}
				switch mode {
				case "missing":
					delete(row, "budget")
				case "mismatch":
					row["operation_status"] = "ENABLE"
				case "wrong_account":
					row["advertiser_id"] = "adv-2"
				}
				writeEnvelope(t, w, 200, 0, "readback", map[string]any{"list": []any{row}})
			}))
			defer server.Close()
			b := newTestBackend(t, server, 1)
			ok, err := b.verifyCreatedRecord(context.Background(), "campaign", "campaign_id", "c1", map[string]any{"campaign_name": "Approved launch", "operation_status": "DISABLE", "budget": 120})
			if ok != (mode == "match") || mode == "match" && err != nil {
				t.Fatalf("mode=%s ok=%v err=%v", mode, ok, err)
			}
		})
	}
}

func TestAudienceReadbackRejectsExistenceOnlyAndWrongDefinition(t *testing.T) {
	for _, mode := range []string{"match", "missing", "mismatch"} {
		t.Run(mode, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/open_api/v1.3/dmp/saved_audience/list/" {
					t.Error(r.URL.Path)
				}
				row := map[string]any{"saved_audience_id": "a1", "saved_audience_name": "US audience", "location_ids": []string{"US"}}
				if mode == "missing" {
					delete(row, "location_ids")
				}
				if mode == "mismatch" {
					row["location_ids"] = []string{"JP"}
				}
				writeEnvelope(t, w, 200, 0, "audience-read", map[string]any{"list": []any{row}})
			}))
			defer server.Close()
			b := newTestBackend(t, server, 1)
			request := ads.OperationRequest{Kind: ads.CreateAudience, Audience: &ads.AudienceCreateSpec{Name: "US audience", Kind: "saved", LocationIDs: []string{"US"}}}
			ok, err := b.ReconcileOperation(context.Background(), ads.OperationPlan{Request: request}, ads.OperationOutcome{State: "acknowledged", Resources: []ads.OperationResource{{Kind: "audience", ID: "a1"}}})
			if err != nil || ok != (mode == "match") {
				t.Fatalf("ok=%v err=%v", ok, err)
			}
		})
	}
}
