package agenthost

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/z-chenhao/ad-agent/internal/ads"
	ar "github.com/z-chenhao/ad-agent/internal/runtime"
)

type countedReportReader struct {
	ads.Reader
	calls atomic.Int32
}

func (r *countedReportReader) Report(ctx context.Context, q ads.ReportQuery) (ads.Report, error) {
	r.calls.Add(1)
	a, err := r.Account(ctx)
	return ads.Report{Source: a.Source, Query: q}, err
}

func TestReportCapacityReservedBeforeIOAndCompletedQueriesReused(t *testing.T) {
	model := fakeRuntime(func(ctx context.Context, _ ar.Request, hooks ar.Hooks) (ar.Result, error) {
		var mu sync.Mutex
		var results []ads.Report
		var wg sync.WaitGroup
		for i := 1; i <= 20; i++ {
			wg.Add(1)
			go func(day int) {
				defer wg.Done()
				result := hooks.Execute(ctx, call("get_performance_report", fmt.Sprintf(`{"level":"campaign","start_date":"2026-08-%02d","end_date":"2026-08-31"}`, day)))
				if !result.OK {
					if result.Error != "report_budget_exceeded" {
						t.Errorf("unexpected failure: %s", result.Error)
					}
					return
				}
				var report ads.Report
				if err := json.Unmarshal(result.Data, &report); err != nil {
					t.Error(err)
				}
				mu.Lock()
				results = append(results, report)
				mu.Unlock()
			}(i)
		}
		wg.Wait()
		if len(results) != 8 {
			t.Fatalf("retained %d reports", len(results))
		}
		query, _ := json.Marshal(results[0].Query)
		repeated := hooks.Execute(ctx, call("get_performance_report", string(query)))
		var cached ads.Report
		if !repeated.OK || json.Unmarshal(repeated.Data, &cached) != nil || cached.ID != results[0].ID {
			t.Fatal("identical completed query was not reused at capacity")
		}
		return ar.Result{Stop: "stop", Text: "Evidence retained."}, nil
	})
	h, backend := testHost(t, model)
	reader := &countedReportReader{Reader: backend}
	h.Backend = reader
	for i := 1; i <= 2; i++ {
		if _, err := h.Run(context.Background(), "report-capacity", "Inspect setup", nil); err != nil {
			t.Fatal(err)
		}
		if got := reader.calls.Load(); got != int32(8*i) {
			t.Fatalf("upstream requests=%d; expected %d", got, 8*i)
		}
	}
}
