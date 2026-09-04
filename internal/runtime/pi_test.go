package runtime

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBridgeProtocol(t *testing.T) {
	if _, e := exec.LookPath("node"); e != nil {
		t.Skip("node required")
	}
	path, _ := filepath.Abs("testdata/bridge.mjs")
	pi := Pi{Entry: path}
	for _, scenario := range []string{"ok", "close", "partial", "budget", "duplicate", "malformed", "crash", "wait"} {
		t.Run(scenario, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			if scenario == "wait" {
				cancel()
				ctx, cancel = context.WithTimeout(context.Background(), 150*time.Millisecond)
			}
			defer cancel()
			calls := 0
			partials := 0
			r, e := pi.Run(ctx, Request{Prompt: scenario, MaxRounds: 1}, Hooks{
				Execute:    func(context.Context, Call) ToolResult { calls++; return Value("data") },
				CloseAfter: func(Call, ToolResult) bool { return scenario == "close" },
				Emit: func(event Event) {
					if event.Type == "tool.delta" && event.Name == "present_metrics" {
						partials++
					}
				},
			})
			switch scenario {
			case "ok":
				if e != nil || calls != 1 {
					t.Fatal(e, calls)
				}
			case "close":
				if e != nil || calls != 1 || !strings.Contains(r.Text, `"close":true`) {
					t.Fatal("close signal missing", e, r)
				}
			case "partial":
				if e != nil || calls != 1 || partials != 1 {
					t.Fatal("partial tool event missing", e, r, partials)
				}
			case "budget":
				if e != nil || calls != 0 || !strings.Contains(r.Text, "tool_budget_exhausted") {
					t.Fatal("budget bypass", e, r)
				}
			default:
				if e == nil {
					t.Fatal("expected protocol failure")
				}
			}
		})
	}
}
