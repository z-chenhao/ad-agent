package agenthost

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/z-chenhao/ad-agent/internal/ads"
	ar "github.com/z-chenhao/ad-agent/internal/runtime"
	"github.com/z-chenhao/ad-agent/internal/store"
)

func TestMetricScopeUsesEvidenceNotNavigation(t *testing.T) {
	h, b := testHost(t, nil)
	ctx := context.Background()
	a, err := b.Account(ctx)
	if err != nil {
		t.Fatal(err)
	}
	task := &turn{host: h, session: store.Session{Source: a.Source}}
	for _, level := range []ads.Level{ads.Campaign, ads.AdGroup, ads.Ad} {
		entities, err := b.List(ctx, ads.EntityQuery{Level: level})
		if err != nil || len(entities) == 0 {
			t.Fatalf("entities: %v %v", entities, err)
		}
		entity := entities[0]
		query := ads.ReportQuery{Level: level, EntityID: entity.ID}
		for _, card := range []Card{
			{Report: &ads.Report{Source: a.Source, Query: query}},
			{Calculation: &ads.Calculation{Source: a.Source, Query: query}},
			{Comparison: &ads.Comparison{Source: a.Source, CurrentQuery: query}},
		} {
			scope := task.metricScope(ctx, card)
			if scope.AccountID != a.ID || scope.AccountName != a.Name || scope.Level != level || scope.EntityID != entity.ID || scope.EntityName != entity.Name {
				t.Fatalf("wrong scope: %+v", scope)
			}
		}
		// Aggregates remain aggregates, even when the record contains only one row.
		scope := task.metricScope(ctx, Card{Report: &ads.Report{Source: a.Source, Query: ads.ReportQuery{Level: level}, Rows: []ads.Row{{EntityID: entity.ID}}}})
		if scope.EntityID != "" || scope.EntityName != "" {
			t.Fatalf("invented filter: %+v", scope)
		}
	}
	if len(task.session.Provenance) != 0 {
		t.Fatal("presentation lookup granted write provenance")
	}
}

type unavailableScopeReader struct {
	ads.Reader
	foreign bool
}

func (b unavailableScopeReader) Account(ctx context.Context) (ads.Account, error) {
	if !b.foreign {
		return ads.Account{}, errors.New("unavailable")
	}
	a, err := b.Reader.Account(ctx)
	a.ID, a.Name = "another-account", "Wrong account"
	return a, err
}
func (b unavailableScopeReader) Get(ctx context.Context, l ads.Level, id string) (ads.Entity, error) {
	if !b.foreign {
		return ads.Entity{}, errors.New("unavailable")
	}
	e, err := b.Reader.Get(ctx, l, id)
	e.AccountID, e.Name = "another-account", "Wrong entity"
	return e, err
}

func TestMetricScopeFailsClosedOnUnavailableOrForeignNames(t *testing.T) {
	h, b := testHost(t, nil)
	a, _ := b.Account(context.Background())
	card := Card{Report: &ads.Report{Source: a.Source, Query: ads.ReportQuery{Level: ads.Campaign, EntityID: "campaign_prospect_us"}}}
	for _, foreign := range []bool{false, true} {
		h.Backend = unavailableScopeReader{Reader: b, foreign: foreign}
		task := &turn{host: h, session: store.Session{Source: a.Source}}
		scope := task.metricScope(context.Background(), card)
		if scope.AccountName != "" || scope.EntityName != "" || scope.EntityID != "campaign_prospect_us" || scope.AccountID != a.ID {
			t.Fatalf("unsafe scope: %+v", scope)
		}
	}
	h.Backend = b
	task := &turn{host: h, session: store.Session{Source: ads.Source{AccountID: "different-bound-account"}}}
	scope := task.metricScope(context.Background(), card)
	if scope.AccountName != "" || scope.EntityName != "" {
		t.Fatalf("cross-source lookup: %+v", scope)
	}
}

func TestPresentedMetricScopePersistsWithoutGrantingProvenance(t *testing.T) {
	model := fakeRuntime(func(ctx context.Context, _ ar.Request, hooks ar.Hooks) (ar.Result, error) {
		result := hooks.Execute(ctx, call("get_performance_report", `{"level":"campaign","entity_id":"campaign_prospect_us","start_date":"2026-08-28","end_date":"2026-09-03"}`))
		if !result.OK {
			t.Fatal(result.Error)
		}
		var report ads.Report
		if err := json.Unmarshal(result.Data, &report); err != nil {
			t.Fatal(err)
		}
		args, _ := json.Marshal(map[string]string{"record_id": report.ID})
		result = hooks.Execute(ctx, ar.Call{ID: "present", Name: "present_metrics", Arguments: args})
		if !result.OK {
			t.Fatal(result.Error)
		}
		if hooks.Execute(ctx, call("stage_status_change", `{"level":"campaign","id":"campaign_prospect_us","status":"ENABLE","reason":"shown on card"}`)).OK {
			t.Fatal("card lookup granted mutation authority")
		}
		return ar.Result{Stop: "stop", Text: "Report ready."}, nil
	})
	h, b := testHost(t, model)
	out, err := h.Run(context.Background(), "scope-card", "Read performance", nil)
	if err != nil {
		t.Fatal(err)
	}
	entity, _ := b.Get(context.Background(), ads.Campaign, "campaign_prospect_us")
	if len(out.Cards) != 1 || out.Cards[0].MetricScope == nil || out.Cards[0].MetricScope.EntityName != entity.Name {
		t.Fatalf("cards: %+v", out.Cards)
	}
	events, err := h.Store.Events(context.Background(), out.TurnID, 0)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, event := range events {
		if event.Type != "ui.upsert" {
			continue
		}
		var card Card
		if err := json.Unmarshal(event.Data, &card); err != nil {
			t.Fatal(err)
		}
		if card.MetricScope != nil && *card.MetricScope == *out.Cards[0].MetricScope {
			found = true
		}
	}
	if !found {
		t.Fatal("saved event omitted metric scope")
	}
}
