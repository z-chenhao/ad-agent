package agenthost

import (
	"context"
	"errors"

	"github.com/shopspring/decimal"
	"github.com/z-chenhao/ad-agent/internal/ads"
)

// validateOperationPolicy applies the same deployment limits to every budget entry.
// It runs when staging and again immediately before an approved operation is sent.
func (s Changes) validateOperationPolicy(ctx context.Context, request ads.OperationRequest) error {
	checkNew := func(value *decimal.Decimal) error {
		if value == nil {
			return nil
		}
		if !s.Policy.MinBudget.IsPositive() || !s.Policy.MaxBudget.IsPositive() || !s.Policy.MaxDeltaPercent.IsPositive() {
			return errors.New("budget_policy_not_configured")
		}
		if value.LessThan(s.Policy.MinBudget) || value.GreaterThan(s.Policy.MaxBudget) {
			return errors.New("budget_outside_limits")
		}
		return nil
	}
	checkUpdate := func(level ads.Level, id string, budget *decimal.Decimal) error {
		if budget == nil {
			return nil
		}
		before, err := s.Backend.Get(ctx, level, id)
		if err != nil {
			return err
		}
		after := before
		after.Budget = budget
		if before.Budget != nil && before.Budget.Equal(*budget) {
			return checkNew(budget)
		}
		return s.Policy.Validate(before, after, ads.BudgetChange)
	}
	switch request.Kind {
	case ads.UpdateAdGroup:
		return checkUpdate(ads.AdGroup, request.AdGroupUpdate.AdGroupID, request.AdGroupUpdate.Budget)
	case ads.CreateCampaignBundle:
		if err := checkNew(request.CampaignBundle.Campaign.Budget); err != nil {
			return err
		}
		return checkNew(&request.CampaignBundle.AdGroup.Budget)
	case ads.CreateAutomatedRule:
		rule := request.Rule
		if rule.Action == "CHANGE_BUDGET" {
			for _, id := range rule.TargetIDs {
				if err := checkUpdate(rule.TargetLevel, id, rule.ActionValue); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
