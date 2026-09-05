package agenthost

import (
	"context"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/z-chenhao/ad-agent/internal/ads"
)

func TestCampaignBundlePolicyChecksBothBudgetLevels(t *testing.T) {
	service := Changes{Policy: ads.SandboxPolicy()}
	valid := decimal.NewFromInt(100)
	tooHigh := service.Policy.MaxBudget.Add(decimal.NewFromInt(1))
	tooLow := decimal.RequireFromString("0.50")
	for _, test := range []struct {
		name     string
		campaign *decimal.Decimal
		group    decimal.Decimal
		valid    bool
	}{
		{"ad-group allocation", nil, valid, true},
		{"campaign allocation", &valid, valid, true},
		{"campaign over cap", &tooHigh, valid, false},
		{"group over cap", nil, tooHigh, false},
		{"campaign under minimum", &tooLow, valid, false},
		{"group under minimum", nil, tooLow, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := ads.OperationRequest{Kind: ads.CreateCampaignBundle, CampaignBundle: &ads.CampaignBundleSpec{Campaign: ads.CampaignSpec{Budget: test.campaign}, AdGroup: ads.AdGroupSpec{Budget: test.group}}}
			err := service.validateOperationPolicy(context.Background(), request)
			if (err == nil) != test.valid {
				t.Fatalf("policy error = %v", err)
			}
		})
	}
}
