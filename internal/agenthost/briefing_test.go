package agenthost

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	ar "github.com/z-chenhao/ad-agent/internal/runtime"
)

func TestBriefingContractRejectsIncompleteOrRepetitiveContentAtomically(t *testing.T) {
	valid := func() map[string]any {
		return map[string]any{
			"kind": "measurement", "headline": "Today's report is incomplete",
			"why":    "Only six hours of today's delivery are reported.",
			"action": "Recheck the complete day tomorrow before changing budgets.",
		}
	}
	model := fakeRuntime(func(ctx context.Context, _ ar.Request, hooks ar.Hooks) (ar.Result, error) {
		cases := []struct {
			name string
			edit func(map[string]any)
		}{
			{"missing evidence", func(v map[string]any) { delete(v, "why") }},
			{"missing next step", func(v map[string]any) { delete(v, "action") }},
			{"blank finding", func(v map[string]any) { v["headline"] = " \n " }},
			{"blank evidence", func(v map[string]any) { v["why"] = " \t " }},
			{"blank next step", func(v map[string]any) { v["action"] = "  " }},
			{"repeated recommendation", func(v map[string]any) { v["action"] = "  TODAY'S report is incomplete  " }},
			{"unseen subject", func(v map[string]any) { v["ref_id"] = "campaign_unseen" }},
			{"model supplied name", func(v map[string]any) { v["entity"] = map[string]string{"name": "Invented campaign"} }},
		}
		present := func(title string, items ...map[string]any) ar.ToolResult {
			arguments, _ := json.Marshal(map[string]any{"title": title, "items": items})
			return hooks.Execute(ctx, call("present_digest", string(arguments)))
		}
		for _, tc := range cases {
			item := valid()
			tc.edit(item)
			if result := present("Reporting readiness", valid(), item); result.OK {
				t.Errorf("accepted %s", tc.name)
			}
		}
		if result := present("Reporting readiness", valid(), valid()); result.OK || !strings.Contains(result.Error, "duplicate findings") {
			t.Errorf("duplicate findings were not rejected: %+v", result)
		}
		if result := present("Reporting readiness", valid(), valid(), valid(), valid()); result.OK {
			t.Error("accepted four items")
		}
		if result := present(" ", valid()); result.OK {
			t.Error("accepted blank title")
		}
		second, third := valid(), valid()
		second["headline"] = "Conversion reporting may backfill"
		third["headline"] = "No complete-day comparison is available yet"
		if result := present(" Reporting readiness ", valid(), second, third); !result.OK {
			t.Fatal(result.Error)
		}
		return ar.Result{Stop: "stop", Text: "Reporting readiness is summarized."}, nil
	})
	host, _ := testHost(t, model)
	result, err := host.Run(context.Background(), "briefing-contract", "Summarize reporting readiness", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Cards) != 1 || result.Cards[0].Digest == nil || result.Cards[0].Digest.Title != "Reporting readiness" || len(result.Cards[0].Digest.Items) != 3 {
		t.Fatalf("invalid briefing leaked a partial card or valid title was not trimmed: %+v", result.Cards)
	}
}
