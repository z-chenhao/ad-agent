package runtime

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestJToolLoopCheckpointAndRestore(t *testing.T) {
	entry, err := filepath.Abs("testdata/j-model-bridge.mjs")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	runtime := J{Entry: entry}
	tool := Tool{Name: "read_data", Description: "read", Parameters: json.RawMessage(`{"type":"object"}`)}
	executed := 0
	var deltas strings.Builder
	first, err := runtime.Run(context.Background(), Request{
		System: "system", Prompt: "first", Tools: []Tool{tool}, MaxRounds: 3,
		SessionDir: filepath.Join(root, "turn-1"),
	}, Hooks{
		Execute: func(_ context.Context, call Call) ToolResult {
			executed++
			if call.ID != "call-1" || call.Name != "read_data" || call.Round != 1 {
				t.Fatalf("call=%#v", call)
			}
			return Value(map[string]string{"answer": "ok"})
		},
		Emit: func(event Event) { deltas.WriteString(event.Text) },
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.Text != "done" || first.Stop != "stop" || executed != 1 || deltas.String() != "done" {
		t.Fatalf("result=%#v executed=%d deltas=%q", first, executed, deltas.String())
	}
	info, err := os.Stat(first.Checkpoint)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("checkpoint stat=%#v err=%v", info, err)
	}
	second, err := runtime.Run(context.Background(), Request{
		System: "system", Prompt: "second", Tools: []Tool{tool}, MaxRounds: 3,
		Checkpoint: first.Checkpoint, SessionDir: filepath.Join(root, "turn-2"),
	}, Hooks{})
	if err != nil {
		t.Fatal(err)
	}
	if second.Text != "done" || second.Checkpoint == first.Checkpoint {
		t.Fatalf("second=%#v", second)
	}
}

func TestJMarksBackstopAsBudgetExhausted(t *testing.T) {
	entry, _ := filepath.Abs("testdata/j-model-bridge.mjs")
	result, err := (J{Entry: entry}).Run(context.Background(), Request{
		System: "system", Prompt: "first", MaxRounds: 1,
		Tools: []Tool{{Name: "read_data", Description: "read", Parameters: json.RawMessage(`{"type":"object"}`)}},
	}, Hooks{Execute: func(context.Context, Call) ToolResult { return Value("ok") }})
	if err != nil {
		t.Fatal(err)
	}
	if result.Stop != "budget" || result.Text != "done" {
		t.Fatalf("result=%#v", result)
	}
}

func TestJDispatchesIndependentCallsConcurrently(t *testing.T) {
	entry, _ := filepath.Abs("testdata/j-model-bridge.mjs")
	started := 0
	var mu sync.Mutex
	both := make(chan struct{})
	result, err := (J{Entry: entry}).Run(context.Background(), Request{
		System: "system", Prompt: "parallel", MaxRounds: 3,
		Tools: []Tool{{Name: "read_data", Description: "read", Parameters: json.RawMessage(`{"type":"object"}`)}},
	}, Hooks{Execute: func(ctx context.Context, call Call) ToolResult {
		mu.Lock()
		started++
		if started == 2 {
			close(both)
		}
		mu.Unlock()
		select {
		case <-both:
			return Value(call.ID)
		case <-time.After(time.Second):
			return Failure("calls_were_serial")
		}
	}})
	if err != nil || result.Stop != "stop" || started != 2 {
		t.Fatalf("result=%#v started=%d err=%v", result, started, err)
	}
}

func TestJCancellationStopsBridge(t *testing.T) {
	entry, _ := filepath.Abs("testdata/j-model-bridge.mjs")
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := (J{Entry: entry}).Run(ctx, Request{System: "system", Prompt: "hang", MaxRounds: 1}, Hooks{})
	if err == nil || ctx.Err() == nil {
		t.Fatalf("err=%v ctx=%v", err, ctx.Err())
	}
}

func TestJRejectsCheckpointOutsideRuntimeRoot(t *testing.T) {
	entry, _ := filepath.Abs("testdata/j-model-bridge.mjs")
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "checkpoint.json")
	if err := os.WriteFile(outside, []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := (J{Entry: entry}).Run(context.Background(), Request{
		System: "system", Prompt: "first", MaxRounds: 1,
		SessionDir: filepath.Join(root, "turn"), Checkpoint: outside,
	}, Hooks{})
	if err == nil {
		t.Fatal("expected checkpoint confinement error")
	}
}
