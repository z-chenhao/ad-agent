package agenthost

import (
	"context"
	"time"

	"github.com/z-chenhao/ad-agent/internal/ads"
)

// MetricScope is a saved presentation label, not navigation context or mutation
// provenance. IDs and aggregation scope come only from the evidence record.
type MetricScope struct {
	AccountID   string    `json:"account_id"`
	AccountName string    `json:"account_name,omitempty"`
	Level       ads.Level `json:"level"`
	EntityID    string    `json:"entity_id,omitempty"`
	EntityName  string    `json:"entity_name,omitempty"`
}

func (t *turn) metricScope(ctx context.Context, card Card) *MetricScope {
	var source ads.Source
	var query ads.ReportQuery
	switch {
	case card.Calculation != nil:
		source, query = card.Calculation.Source, card.Calculation.Query
	case card.Comparison != nil:
		source, query = card.Comparison.Source, card.Comparison.CurrentQuery
	case card.Report != nil:
		source, query = card.Report.Source, card.Report.Query
	default:
		return nil
	}
	scope := &MetricScope{AccountID: source.AccountID, Level: query.Level, EntityID: query.EntityID}
	if source != t.session.Source {
		return scope
	}
	// Names improve recognition but must never block evidence presentation or be
	// guessed from model prose. Keep ID fallback on unavailable/foreign metadata.
	lookup, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if account, err := t.host.Backend.Account(lookup); err == nil && account.Source == source && account.ID == source.AccountID {
		scope.AccountName = account.Name
	}
	if query.EntityID != "" && query.Level != ads.Advertiser {
		if entity, err := t.host.Backend.Get(lookup, query.Level, query.EntityID); err == nil && entity.AccountID == source.AccountID && entity.Level == query.Level && entity.ID == query.EntityID {
			scope.EntityName = entity.Name
		}
	}
	return scope
}
