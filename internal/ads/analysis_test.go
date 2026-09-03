package ads

import (
	"github.com/shopspring/decimal"
	"testing"
)

func money(v string) *decimal.Decimal { d := decimal.RequireFromString(v); return &d }
func TestWeightedAndUnavailable(t *testing.T) {
	a := Metrics{Spend: *money("100"), Revenue: money("200")}
	b := Metrics{Spend: *money("300"), Revenue: money("300")}
	r := Report{Complete: true, Rows: []Row{{EntityID: "a", Metrics: a}, {EntityID: "b", Metrics: b}}, Totals: ZeroMetrics().Add(a).Add(b)}
	c, e := Analyze(r)
	if e != nil {
		t.Fatal(e)
	}
	if c.ROAS.String() != "1.25" {
		t.Fatal("averaged ratios")
	}
	r.Rows[0].Metrics.Revenue = nil
	r.Totals = ZeroMetrics().Add(r.Rows[0].Metrics).Add(b)
	c, e = Analyze(r)
	if e != nil || c.ROAS != nil {
		t.Fatal("missing became zero")
	}
	if Ratio(money("10"), decimal.Zero) != nil {
		t.Fatal("zero division")
	}
	r.Totals.Spend = *money("999")
	if _, e = Analyze(r); e == nil {
		t.Fatal("untrusted totals accepted")
	}
}
func TestPolicyRejectsBroadOrOversizedChanges(t *testing.T) {
	before := Entity{ID: "x", AccountID: "a", Level: Campaign, Status: "DISABLE", Budget: money("50")}
	p := FixturePolicy()
	after := before
	after.Budget = money("55")
	if e := p.Validate(before, after, BudgetChange); e != nil {
		t.Fatal(e)
	}
	after.Status = "ENABLE"
	if e := p.Validate(before, after, BudgetChange); e == nil {
		t.Fatal("multi-field change")
	}
	after = before
	after.Budget = money("61")
	if e := p.Validate(before, after, BudgetChange); e == nil {
		t.Fatal("delta cap bypass")
	}
	if e := (Policy{}).Validate(before, after, BudgetChange); e == nil {
		t.Fatal("unconfigured policy allowed")
	}
}
