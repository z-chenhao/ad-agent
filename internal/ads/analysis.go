package ads

import (
	"errors"
	"github.com/shopspring/decimal"
	"sort"
)

type Finding struct {
	EntityID   string           `json:"entity_id"`
	Metrics    Metrics          `json:"metrics"`
	ROAS       *decimal.Decimal `json:"roas"`
	SpendShare *decimal.Decimal `json:"spend_share"`
}
type Calculation struct {
	Source      Source           `json:"source"`
	Query       ReportQuery      `json:"query"`
	Currency    string           `json:"currency"`
	Timezone    string           `json:"timezone"`
	Attribution string           `json:"attribution"`
	ID          string           `json:"id"`
	ReportID    string           `json:"report_id"`
	Method      string           `json:"method"`
	Totals      Metrics          `json:"totals"`
	ROAS        *decimal.Decimal `json:"roas"`
	Ranking     []Finding        `json:"ranking"`
	Limitations []string         `json:"limitations"`
}

func Analyze(r Report) (Calculation, error) {
	if !r.Complete {
		return Calculation{}, errors.New("incomplete report cannot establish an account-wide ranking")
	}
	groups := make(map[string]Metrics)
	actual := ZeroMetrics()
	for _, row := range r.Rows {
		actual = actual.Add(row.Metrics)
		v, ok := groups[row.EntityID]
		if !ok {
			v = ZeroMetrics()
		}
		groups[row.EntityID] = v.Add(row.Metrics)
	}
	if !actual.Spend.Equal(r.Totals.Spend) || actual.Impressions != r.Totals.Impressions || actual.Clicks != r.Totals.Clicks || !equalNullable(actual.Revenue, r.Totals.Revenue) || !equalNullable(actual.Conversions, r.Totals.Conversions) {
		return Calculation{}, errors.New("report_totals_do_not_match_rows")
	}
	c := Calculation{ReportID: r.ID, Method: "Sum additive metrics per entity; ROAS=sum(revenue)/sum(spend); rank ascending with unavailable ratios last. Correlation is not causation.", Totals: r.Totals, ROAS: r.Totals.ROAS(), Limitations: append([]string{}, r.Limitations...), Ranking: []Finding{}}
	c.Source = r.Source
	c.Query = r.Query
	c.Currency = r.Currency
	c.Timezone = r.Timezone
	c.Attribution = r.Attribution
	for id, m := range groups {
		c.Ranking = append(c.Ranking, Finding{EntityID: id, Metrics: m, ROAS: m.ROAS(), SpendShare: Ratio(&m.Spend, r.Totals.Spend)})
	}
	sort.Slice(c.Ranking, func(i, j int) bool {
		a, b := c.Ranking[i], c.Ranking[j]
		if a.ROAS == nil && b.ROAS == nil {
			return a.EntityID < b.EntityID
		}
		if a.ROAS == nil {
			return false
		}
		if b.ROAS == nil {
			return true
		}
		if a.ROAS.Equal(*b.ROAS) {
			return a.EntityID < b.EntityID
		}
		return a.ROAS.LessThan(*b.ROAS)
	})
	if c.ROAS == nil {
		c.Limitations = append(c.Limitations, "ROAS unavailable: missing revenue or zero spend")
	}
	return c, nil
}
func equalNullable(a, b *decimal.Decimal) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Equal(*b)
}
