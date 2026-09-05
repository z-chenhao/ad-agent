package ads

import (
	"errors"
	"github.com/shopspring/decimal"
	"sort"
	"time"
)

type Contribution struct {
	EntityID   string           `json:"entity_id"`
	Previous   Metrics          `json:"previous"`
	Current    Metrics          `json:"current"`
	ROASPoints *decimal.Decimal `json:"roas_points"`
}
type Comparison struct {
	Source           Source           `json:"source"`
	CurrentQuery     ReportQuery      `json:"current_query"`
	PreviousQuery    ReportQuery      `json:"previous_query"`
	Currency         string           `json:"currency"`
	Timezone         string           `json:"timezone"`
	Attribution      string           `json:"attribution"`
	ID               string           `json:"id"`
	CurrentReportID  string           `json:"current_report_id"`
	PreviousReportID string           `json:"previous_report_id"`
	Previous         Metrics          `json:"previous"`
	Current          Metrics          `json:"current"`
	PreviousROAS     *decimal.Decimal `json:"previous_roas"`
	CurrentROAS      *decimal.Decimal `json:"current_roas"`
	DeltaROAS        *decimal.Decimal `json:"delta_roas"`
	Contributions    []Contribution   `json:"contributions"`
	Method           string           `json:"method"`
	Limitations      []string         `json:"limitations"`
}

func Compare(previous, current Report) (Comparison, error) {
	if !previous.Complete || !current.Complete {
		return Comparison{}, errors.New("comparison_requires_complete_reports")
	}
	if previous.Source != current.Source || previous.Currency != current.Currency || previous.Timezone != current.Timezone || previous.Attribution != current.Attribution || previous.Query.Level != current.Query.Level || previous.Query.EntityID != current.Query.EntityID {
		return Comparison{}, errors.New("incompatible_report_semantics")
	}
	if err := previous.Query.Validate(); err != nil {
		return Comparison{}, err
	}
	if err := current.Query.Validate(); err != nil {
		return Comparison{}, err
	}
	ps, _ := time.Parse(time.DateOnly, previous.Query.Start)
	pe, _ := time.Parse(time.DateOnly, previous.Query.End)
	cs, _ := time.Parse(time.DateOnly, current.Query.Start)
	ce, _ := time.Parse(time.DateOnly, current.Query.End)
	if pe.Sub(ps) != ce.Sub(cs) || !pe.Before(cs) {
		return Comparison{}, errors.New("comparison_requires_equal_nonoverlapping_windows")
	}
	p, err := Analyze(previous)
	if err != nil {
		return Comparison{}, err
	}
	c, err := Analyze(current)
	if err != nil {
		return Comparison{}, err
	}
	out := Comparison{PreviousReportID: previous.ID, CurrentReportID: current.ID, Previous: p.Totals, Current: c.Totals, PreviousROAS: p.ROAS, CurrentROAS: c.ROAS, Contributions: []Contribution{}, Method: "Entity contribution = current entity revenue / current total spend - previous entity revenue / previous total spend. Contributions sum to total ROAS change; includes mix and spend-denominator effects, not causal attribution.", Limitations: append(append([]string{}, previous.Limitations...), current.Limitations...)}
	out.Source = current.Source
	out.CurrentQuery = current.Query
	out.PreviousQuery = previous.Query
	out.Currency = current.Currency
	out.Timezone = current.Timezone
	out.Attribution = current.Attribution
	if p.ROAS != nil && c.ROAS != nil {
		v := c.ROAS.Sub(*p.ROAS)
		out.DeltaROAS = &v
	}
	groups := map[string]Contribution{}
	for _, r := range p.Ranking {
		groups[r.EntityID] = Contribution{EntityID: r.EntityID, Previous: r.Metrics, Current: ZeroMetrics()}
	}
	for _, r := range c.Ranking {
		v, ok := groups[r.EntityID]
		if !ok {
			v = Contribution{EntityID: r.EntityID, Previous: ZeroMetrics()}
		}
		v.Current = r.Metrics
		groups[r.EntityID] = v
	}
	for _, v := range groups {
		a := Ratio(v.Current.Revenue, c.Totals.Spend)
		b := Ratio(v.Previous.Revenue, p.Totals.Spend)
		if out.DeltaROAS != nil && a != nil && b != nil {
			d := a.Sub(*b)
			v.ROASPoints = &d
		}
		out.Contributions = append(out.Contributions, v)
	}
	sort.Slice(out.Contributions, func(i, j int) bool { return out.Contributions[i].EntityID < out.Contributions[j].EntityID })
	return out, nil
}
