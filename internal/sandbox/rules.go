package sandbox

import (
	"sort"
	"time"

	"github.com/shopspring/decimal"
	"github.com/z-chenhao/ad-agent/internal/ads"
)

// SetBudgetPolicy keeps scheduled rule execution inside operator budget limits.
func (f *Backend) SetBudgetPolicy(policy ads.Policy) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.budgetPolicy = &policy
}

// evaluateRules runs approved definitions against reported observations available
// at this evaluation time. It is called under the Sandbox lock every half-hour.
func (f *Backend) evaluateRules(at time.Time) {
	ids := make([]string, 0, len(f.operations.RuleDefinitions))
	for id := range f.operations.RuleDefinitions {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		rule := f.operations.RuleDefinitions[id]
		if f.operations.Rules[id].Status != "ENABLED" || !at.After(f.operations.RuleEvaluatedAt[id]) {
			continue
		}
		f.operations.RuleEvaluatedAt[id] = at
		result := ads.AutomatedRuleResult{ID: id + "/" + at.UTC().Format(time.RFC3339), RuleID: id, ExecutedAt: at, Status: "NO_ACTION"}
		for _, targetID := range rule.TargetIDs {
			entity, ok := f.entities[targetID]
			if !ok || entity.Level != rule.TargetLevel {
				result.Status = "REJECTED"
				continue
			}
			matched := true
			for _, condition := range rule.Conditions {
				metrics := f.ruleMetrics(entity, condition.Window, at)
				value, available := ruleMetricValue(metrics, condition.Metric)
				matched = matched && available && (condition.Operator == "GT" && value.GreaterThan(condition.Value) || condition.Operator == "LT" && value.LessThan(condition.Value))
			}
			if !matched {
				continue
			}
			after := entity
			switch rule.Action {
			case "PAUSE":
				if entity.Status == "DISABLE" {
					continue
				}
				after.Status = "DISABLE"
			case "CHANGE_BUDGET":
				if rule.ActionValue == nil {
					result.Status = "REJECTED"
					continue
				}
				if entity.Budget != nil && entity.Budget.Equal(*rule.ActionValue) {
					continue
				}
				after.Budget = rule.ActionValue
				policy := ads.SandboxPolicy()
				if f.budgetPolicy != nil {
					policy = *f.budgetPolicy
				}
				if err := policy.Validate(entity, after, ads.BudgetChange); err != nil {
					result.Status = "REJECTED"
					continue
				}
			case "NOTIFY":
			default:
				result.Status = "REJECTED"
				continue
			}
			f.entities[targetID] = after
			result.AffectedCount++
			if result.Status != "REJECTED" {
				result.Status = "SUCCEEDED"
			}
		}
		f.operations.RuleResults = append(f.operations.RuleResults, result)
	}
}

func (f *Backend) ruleMetrics(target ads.Entity, window string, at time.Time) ads.Metrics {
	days := 1
	if window == "LAST_3_DAYS" {
		days = 3
	}
	if window == "LAST_7_DAYS" {
		days = 7
	}
	end := at.In(f.location).Format(time.DateOnly)
	start := at.In(f.location).AddDate(0, 0, 1-days).Format(time.DateOnly)
	matches := func(id string) bool {
		entity, ok := f.entities[id]
		if !ok {
			return false
		}
		for entity.Level != target.Level {
			entity, ok = f.entities[entity.ParentID]
			if !ok {
				return false
			}
		}
		return entity.ID == target.ID
	}
	total := ads.ZeroMetrics()
	for _, row := range f.rows {
		if row.Date >= start && row.Date <= end && matches(row.EntityID) {
			total = total.Add(row.Metrics)
		}
	}
	for _, fact := range f.hourFacts {
		day := fact.Hour.In(f.location).Format(time.DateOnly)
		if day < start || day > end || fact.Hour.After(at) || !matches(fact.AdID) {
			continue
		}
		metrics := fact.Metrics
		if at.Before(fact.ReportAvailableAt) {
			zero := decimal.Zero
			metrics.Conversions = &zero
			metrics.Revenue = &zero
		}
		total = total.Add(metrics)
	}
	return total
}

func ruleMetricValue(m ads.Metrics, name string) (decimal.Decimal, bool) {
	switch name {
	case "SPEND":
		return m.Spend, true
	case "CONVERSIONS":
		if m.Conversions != nil {
			return *m.Conversions, true
		}
	case "CPA":
		if m.Conversions != nil && m.Conversions.IsPositive() {
			return m.Spend.Div(*m.Conversions), true
		}
	case "CTR":
		if m.Impressions > 0 {
			return decimal.NewFromInt(m.Clicks).Div(decimal.NewFromInt(m.Impressions)).Mul(decimal.NewFromInt(100)), true
		}
	}
	return decimal.Zero, false
}

func (f *Backend) settleSpend(at time.Time, facts []HourFact) {
	spend := decimal.Zero
	for _, fact := range facts {
		spend = spend.Add(fact.Metrics.Spend)
	}
	if spend.IsZero() {
		return
	}
	f.operations.Balance.Cash = f.operations.Balance.Cash.Sub(spend)
	f.operations.Balance.Available = f.operations.Balance.Available.Sub(spend)
	f.operations.Balance.AsOf = at
	id := "delivery/" + at.In(f.location).Format(time.DateOnly)
	for i := range f.operations.Transactions {
		if f.operations.Transactions[i].ID == id {
			f.operations.Transactions[i].Amount = f.operations.Transactions[i].Amount.Sub(spend)
			f.operations.Transactions[i].OccurredAt = at
			return
		}
	}
	f.operations.Transactions = append(f.operations.Transactions, ads.BillingTransaction{ID: id, AccountID: f.account.ID, Type: "AD_SPEND", Amount: spend.Neg(), Currency: f.account.Currency, OccurredAt: at, Status: "SETTLED"})
}
