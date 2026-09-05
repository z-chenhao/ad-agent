//go:build !windows

package runtime

import (
	"context"
	"encoding/json"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestBridgeCancellationTerminatesNativeChild(t *testing.T) {
	path, _ := filepath.Abs("testdata/bridge-child.mjs")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pid := 0
	_, err := (Codex{Entry: path}).Run(ctx, Request{}, Hooks{Execute: func(_ context.Context, call Call) ToolResult {
		var args struct {
			PID int `json:"pid"`
		}
		if err := json.Unmarshal(call.Arguments, &args); err != nil {
			t.Fatal(err)
		}
		pid = args.PID
		cancel()
		return Value("cancelled")
	}})
	if err == nil || pid <= 0 {
		t.Fatal("expected cancellation after child start", err, pid)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if syscall.Kill(pid, 0) == syscall.ESRCH {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("native child survived cancellation")
}
