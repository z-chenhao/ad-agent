package agenthost

import (
	"context"
	"errors"
	"testing"

	ar "github.com/z-chenhao/ad-agent/internal/runtime"
)

func TestAnalysisFailureClassification(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		contextErr, runtimeErr error
		stop                   string
		submitted              bool
		want                   string
	}{
		{"submitted", nil, nil, "stop", true, ""},
		{"missing", nil, nil, "stop", false, "analysis_missing_submission"},
		{"runtime", nil, errors.New("private-provider-detail"), "", false, "analysis_runtime_failed"},
		{"deadline", context.DeadlineExceeded, errors.New("private-provider-detail"), "", false, "analysis_timeout"},
		{"wrapped-deadline", nil, context.DeadlineExceeded, "", false, "analysis_timeout"},
		{"cancelled", context.Canceled, nil, "stop", true, "analysis_cancelled"},
		{"budget", nil, nil, "budget", false, "analysis_budget_exhausted"},
		{"interrupted", nil, nil, "error", false, "analysis_interrupted"},
		{"submitted-then-error", nil, errors.New("private-provider-detail"), "stop", true, "analysis_runtime_failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := analysisFailure(tc.contextErr, tc.runtimeErr, tc.stop, tc.submitted); got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestUnsubmittedAnalysisReturnsSpecificFailure(t *testing.T) {
	model := fakeRuntime(func(_ context.Context, _ ar.Request, _ ar.Hooks) (ar.Result, error) {
		return ar.Result{Stop: "stop"}, nil
	})
	host, _ := testHost(t, model)
	task := &turn{host: host, ctx: context.Background(), id: "analysis-failure"}
	result := task.analyze(context.Background(), "analysis-parent", "Explain the supplied evidence", nil)
	if result.OK || result.Error != "analysis_missing_submission" {
		t.Fatalf("result=%#v", result)
	}
}
