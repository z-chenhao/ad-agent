package sandbox

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/z-chenhao/ad-agent/internal/ads"
)

const (
	MaxAdvanceHours   = 24 * 31
	MaxGeneratedHours = 24 * 365
)

type HourFact struct {
	AdID              string               `json:"ad_id"`
	Hour              time.Time            `json:"hour"`
	ModelVersion      string               `json:"model_version,omitempty"`
	Metrics           ads.Metrics          `json:"metrics"`
	TrueMetrics       ads.Metrics          `json:"true_metrics,omitempty"`
	Attribution       AttributionBreakdown `json:"attribution,omitempty"`
	ReportAvailableAt time.Time            `json:"report_available_at,omitempty"`
	Trace             *CausalTrace         `json:"debug_trace,omitempty"`
}

type SimulationState struct {
	Environment    string                `json:"environment"`
	AccountID      string                `json:"account_id"`
	CurrentTime    time.Time             `json:"current_time"`
	Granularity    string                `json:"granularity"`
	GeneratedHours int64                 `json:"generated_hours"`
	FactCount      int64                 `json:"fact_count"`
	SeedStart      string                `json:"seed_start"`
	SeedEnd        string                `json:"seed_end"`
	Limitations    []string              `json:"limitations"`
	Model          *SimulationModelState `json:"model,omitempty"`
}

type AdvanceResult struct {
	PreviousTime time.Time       `json:"previous_time"`
	AdvancedBy   int             `json:"advanced_by_hours"`
	State        SimulationState `json:"state"`
	FactsCreated int             `json:"facts_created"`
}

func (f *Backend) SimulationState(ctx context.Context) (SimulationState, error) {
	if err := ctx.Err(); err != nil {
		return SimulationState{}, err
	}
	f.mu.RLock()
	defer f.mu.RUnlock()
	state := f.simulationStateLocked()
	state.Model = nil
	return state, nil
}

func (f *Backend) simulationStateLocked() SimulationState {
	model := f.model
	return SimulationState{
		Environment:    f.environment,
		AccountID:      f.account.ID,
		CurrentTime:    f.clock,
		Granularity:    "hour",
		GeneratedHours: countGeneratedHours(f.hourFacts),
		FactCount:      int64(len(f.hourFacts)),
		SeedStart:      f.seedStart,
		SeedEnd:        f.seedEnd,
		Limitations:    []string{"Delivery is produced by a seeded generic auction abstraction; its formulas are configurable modeling assumptions, not TikTok or Meta algorithms."},
		Model:          &model,
	}
}

func countGeneratedHours(facts []HourFact) int64 {
	seen := map[string]struct{}{}
	for _, fact := range facts {
		seen[fact.Hour.UTC().Format(time.RFC3339)] = struct{}{}
	}
	return int64(len(seen))
}

// RestoreSimulation installs the persisted virtual clock and immutable facts.
// It is called after entity overrides so future generation observes current state.
func (f *Backend) RestoreSimulation(state *SimulationState, facts []HourFact) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if state != nil {
		if state.Environment != f.environment || state.AccountID != f.account.ID || state.Granularity != "hour" || state.CurrentTime.Before(f.clock) {
			return errors.New("invalid sandbox simulation state")
		}
		if state.Model == nil || state.Model.Version != causalModelVersion {
			return errors.New("unsupported sandbox simulation schema")
		}
		if err := state.Model.validate(); err != nil {
			return err
		}
		f.clock = state.CurrentTime.UTC()
		f.model = *state.Model
	}
	seen := map[string]bool{}
	for _, fact := range facts {
		if !f.validHourFact(fact) {
			return errors.New("invalid sandbox hour fact")
		}
		key := fact.AdID + "/" + fact.Hour.UTC().Format(time.RFC3339)
		if seen[key] {
			return errors.New("duplicate sandbox hour fact")
		}
		seen[key] = true
		fact.Hour = fact.Hour.UTC()
		f.hourFacts = append(f.hourFacts, fact)
	}
	sort.Slice(f.hourFacts, func(i, j int) bool {
		if f.hourFacts[i].Hour.Equal(f.hourFacts[j].Hour) {
			return f.hourFacts[i].AdID < f.hourFacts[j].AdID
		}
		return f.hourFacts[i].Hour.Before(f.hourFacts[j].Hour)
	})
	f.account.LatestDate = f.clock.In(f.location).Format(time.DateOnly)
	return nil
}

func (f *Backend) validHourFact(fact HourFact) bool {
	entity, ok := f.entities[fact.AdID]
	metrics := fact.Metrics
	return ok && entity.Level == ads.Ad && !fact.Hour.IsZero() &&
		fact.Hour.After(f.simulationStart) && !fact.Hour.After(f.clock) &&
		fact.Hour.Equal(fact.Hour.Truncate(time.Hour)) &&
		!metrics.Spend.IsNegative() && metrics.Impressions >= 0 &&
		metrics.Clicks >= 0 && metrics.Clicks <= metrics.Impressions &&
		metrics.Conversions != nil && !metrics.Conversions.IsNegative() &&
		metrics.Revenue != nil && !metrics.Revenue.IsNegative() &&
		fact.ModelVersion == causalModelVersion && validCausalMetrics(fact)
}

func validCausalMetrics(fact HourFact) bool {
	trueMetrics := fact.TrueMetrics
	return !fact.ReportAvailableAt.IsZero() && !fact.ReportAvailableAt.Before(fact.Hour) &&
		!trueMetrics.Spend.IsNegative() && trueMetrics.Impressions >= 0 && trueMetrics.Clicks >= 0 &&
		trueMetrics.Clicks <= trueMetrics.Impressions && trueMetrics.Conversions != nil &&
		!trueMetrics.Conversions.IsNegative() && trueMetrics.Revenue != nil && !trueMetrics.Revenue.IsNegative() &&
		fact.Metrics.Impressions == trueMetrics.Impressions && fact.Metrics.Clicks == trueMetrics.Clicks &&
		fact.Metrics.Spend.Equal(trueMetrics.Spend) && fact.Metrics.Conversions.LessThanOrEqual(*trueMetrics.Conversions) &&
		fact.Metrics.Revenue.LessThanOrEqual(*trueMetrics.Revenue)
}

// Advance moves virtual time forward and creates one immutable fact per ad per hour.
func (f *Backend) Advance(ctx context.Context, hours int) (AdvanceResult, []HourFact, error) {
	if hours < 1 || hours > MaxAdvanceHours {
		return AdvanceResult{}, nil, errors.New("advance must be between 1 and 744 hours")
	}
	if err := ctx.Err(); err != nil {
		return AdvanceResult{}, nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if countGeneratedHours(f.hourFacts)+int64(hours) > MaxGeneratedHours {
		return AdvanceResult{}, nil, errors.New("sandbox environment is limited to 8760 generated hours")
	}
	previous := f.clock
	adsList := make([]ads.Entity, 0)
	for _, entity := range f.entities {
		if entity.Level == ads.Ad {
			adsList = append(adsList, entity)
		}
	}
	sort.Slice(adsList, func(i, j int) bool { return adsList[i].ID < adsList[j].ID })
	created := make([]HourFact, 0, hours*len(adsList))
	for step := 0; step < hours; step++ {
		if err := ctx.Err(); err != nil {
			return AdvanceResult{}, nil, err
		}
		hour := f.clock.Add(time.Hour)
		f.evaluateRules(hour.Add(-30 * time.Minute))
		for i := range adsList {
			adsList[i] = f.entities[adsList[i].ID]
		}
		facts := f.generateCausalHoursJoint(adsList, hour)
		for _, fact := range facts {
			created = append(created, fact)
			f.hourFacts = append(f.hourFacts, fact)
		}
		f.clock = hour
		f.settleSpend(hour, facts)
		f.evaluateRules(hour)
	}
	f.account.LatestDate = f.clock.In(f.location).Format(time.DateOnly)
	state := f.simulationStateLocked()
	return AdvanceResult{PreviousTime: previous, AdvancedBy: hours, State: state, FactsCreated: len(created)}, created, nil
}

func (f *Backend) visibleFactMetrics(fact HourFact) ads.Metrics {
	if !f.clock.Before(fact.ReportAvailableAt) {
		return fact.Metrics
	}
	visible := fact.Metrics
	zero := ads.ZeroMetrics()
	visible.Conversions = zero.Conversions
	visible.Revenue = zero.Revenue
	return visible
}
