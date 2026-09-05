package runtime

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

// Pi runs one isolated SDK turn. No default coding tools exist.
type Pi struct {
	Entry string
	Node  string
}
type frame struct {
	Type string `json:"type"`
	Call
	Text       string `json:"text,omitempty"`
	Stop       string `json:"stop,omitempty"`
	Checkpoint string `json:"checkpoint,omitempty"`
	Usage      Usage  `json:"usage"`
	Error      string `json:"error,omitempty"`
}

func (p Pi) Run(ctx context.Context, r Request, h Hooks) (Result, error) {
	return runSDKBridge(ctx, p.Entry, p.Node, "Pi", r, h)
}

// runSDKBridge transports the private application protocol; the selected SDK
// process, not this transport, owns its model loop and native session.
func runSDKBridge(ctx context.Context, entry, node, label string, r Request, h Hooks) (Result, error) {
	model, err := NormalizeModel(r.Model)
	if err != nil {
		return Result{}, err
	}
	r.Model = model
	if r.MaxRounds < 0 || r.MaxRounds > 16 {
		return Result{}, errors.New("invalid round budget")
	}
	if !filepath.IsAbs(entry) {
		return Result{}, fmt.Errorf("%s entry must be absolute", label)
	}
	if r.SessionDir != "" {
		if !filepath.IsAbs(r.SessionDir) {
			return Result{}, errors.New("session directory must be absolute")
		}
		if err := os.MkdirAll(r.SessionDir, 0700); err != nil {
			return Result{}, err
		}
	}
	if node == "" {
		node = "node"
	}
	cmd := exec.CommandContext(ctx, node, entry)
	configureBridgeProcess(cmd)
	cmd.Env = processEnv(ctx)
	// Credentials and proxy configuration remain process-local; stderr never enters public logs.
	cmd.Stderr = io.Discard
	in, err := cmd.StdinPipe()
	if err != nil {
		return Result{}, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return Result{}, err
	}
	if err = cmd.Start(); err != nil {
		return Result{}, fmt.Errorf("cannot start %s bridge", label)
	}
	defer func() { in.Close(); _ = killBridgeProcess(cmd); _ = cmd.Wait() }()
	enc := json.NewEncoder(in)
	if err = enc.Encode(struct {
		Type string `json:"type"`
		Request
	}{"start", r}); err != nil {
		return Result{}, err
	}
	seen := map[string]bool{}
	count := 0
	textBytes := 0
	lastRound := 0
	scan := bufio.NewScanner(out)
	scan.Buffer(make([]byte, 8192), 1<<20)
	for scan.Scan() {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		var f frame
		if json.Unmarshal(scan.Bytes(), &f) != nil {
			return Result{}, fmt.Errorf("invalid %s protocol frame", label)
		}
		switch f.Type {
		case "text_delta":
			textBytes += len(f.Text)
			if textBytes > 65536 {
				return Result{}, errors.New("model_text_limit_exceeded")
			}
			if h.Emit != nil {
				h.Emit(Event{Type: "text.delta", ID: f.ID, Text: f.Text})
			}
		case "tool_delta":
			if h.Emit != nil && f.Name != "" {
				h.Emit(Event{Type: "tool.delta", ID: f.ID, Name: f.Name, Arguments: f.Arguments})
			}
		case "tool_call":
			if f.ID == "" || seen[f.ID] || f.Round < lastRound || f.Round < 1 {
				return Result{}, errors.New("invalid tool correlation or round")
			}
			seen[f.ID] = true
			lastRound = f.Round
			count++
			if count > 64 {
				return Result{}, errors.New("tool call limit exceeded")
			}
			result := Failure("tool_budget_exhausted")
			if (r.MaxRounds == 0 || f.Round <= r.MaxRounds) && h.Execute != nil {
				result = h.Execute(ctx, f.Call)
			}
			if result.OK && h.CloseAfter != nil && h.CloseAfter(f.Call, result) {
				result.Close = true
			}
			b, _ := json.Marshal(result)
			if len(b) > 65536 {
				result = Failure("tool_result_too_large")
			}
			if err = enc.Encode(struct {
				Type   string     `json:"type"`
				ID     string     `json:"id"`
				Result ToolResult `json:"result"`
			}{"tool_result", f.ID, result}); err != nil {
				return Result{}, fmt.Errorf("%s tool reply failed", label)
			}
		case "done":
			if len(f.Text) > 65536 {
				return Result{}, errors.New("model_text_limit_exceeded")
			}
			if f.Stop != "stop" && f.Stop != "budget" {
				return Result{}, fmt.Errorf("%s turn stopped: %s", label, safeStop(f.Stop))
			}
			if f.Checkpoint != "" {
				rel, e := filepath.Rel(r.SessionDir, f.Checkpoint)
				if r.SessionDir == "" || e != nil || filepath.IsAbs(rel) || rel == ".." || len(rel) > 3 && rel[:3] == "../" {
					return Result{}, errors.New("invalid checkpoint location")
				}
			}
			return Result{Text: f.Text, Stop: f.Stop, Checkpoint: f.Checkpoint, Usage: f.Usage}, nil
		case "error":
			return Result{}, fmt.Errorf("%s runtime failed: %s", label, SafeFailureCode(f.Error))
		default:
			return Result{}, fmt.Errorf("unexpected %s protocol event", label)
		}
	}
	if ctx.Err() != nil {
		return Result{}, ctx.Err()
	}
	if scan.Err() != nil {
		return Result{}, fmt.Errorf("%s protocol read failed", label)
	}
	return Result{}, fmt.Errorf("%s exited before a terminal result", label)
}
func safeStop(s string) string {
	switch s {
	case "error", "aborted", "length", "toolUse":
		return s
	default:
		return "unexpected"
	}
}
