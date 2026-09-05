package store

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestToolTraceRetainsDurationAndAllowlistedFailureOnly(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for i, message := range []string{"analysis_missing_submission", "budget_delta_exceeded", "provider_token_private_value", "https://provider.test/private?key=secret", "report_budget_exceeded"} {
		data, _ := json.Marshal(map[string]any{"id": "call", "name": "run_analysis", "ok": false, "error": message, "duration_ms": 1234})
		if err := s.AddEvent(context.Background(), Event{Version: "1", Type: "tool.finished", TurnID: "turn", Seq: int64(i + 1), Data: data}); err != nil {
			t.Fatal(err)
		}
	}
	content, err := os.ReadFile(filepath.Join(dir, "logs", "agent-trace.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(content), "private") || strings.Contains(string(content), "secret") {
		t.Fatal("raw error leaked into metadata log")
	}
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	for i, want := range []string{"analysis_missing_submission", "budget_delta_exceeded", "tool_failed", "tool_failed", "report_budget_exceeded"} {
		var diagnostic Diagnostic
		if err := json.Unmarshal([]byte(lines[i]), &diagnostic); err != nil {
			t.Fatal(err)
		}
		if diagnostic.ErrorCode != want || diagnostic.DurationMS != 1234 || diagnostic.Success == nil || *diagnostic.Success {
			t.Fatalf("diagnostic=%#v", diagnostic)
		}
	}
}

func TestTurnFailureTraceRetainsSafeCategory(t *testing.T) {
	dir := t.TempDir()
	_ = os.Chmod(dir, 0700)
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for i, code := range []string{"provider_history_rejected", "private_provider_token"} {
		data, _ := json.Marshal(map[string]any{"status": "failed", "error_code": code, "elapsed_ms": 120})
		if err := s.AddEvent(context.Background(), Event{Version: "1", Type: "turn.completed", TurnID: "failed", Seq: int64(i + 1), Data: data}); err != nil {
			t.Fatal(err)
		}
	}
	content, err := os.ReadFile(filepath.Join(dir, "logs", "agent-trace.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(content), "private_provider") {
		t.Fatal("raw failure leaked")
	}
	for i, line := range strings.Split(strings.TrimSpace(string(content)), "\n") {
		var diagnostic Diagnostic
		if err := json.Unmarshal([]byte(line), &diagnostic); err != nil {
			t.Fatal(err)
		}
		want := []string{"provider_history_rejected", "runtime_failed"}[i]
		if diagnostic.ErrorCode != want || diagnostic.Outcome != "failed" || diagnostic.DurationMS != 120 {
			t.Fatalf("unexpected diagnostic: %+v", diagnostic)
		}
	}
}
