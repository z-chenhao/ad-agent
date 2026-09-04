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

// Pi runs one bounded turn in an isolated SDK process. No default coding tools exist.
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
	model, err := NormalizeModel(r.Model)
	if err != nil {
		return Result{}, err
	}
	r.Model = model
	if r.MaxRounds < 1 || r.MaxRounds > 16 {
		return Result{}, errors.New("invalid round budget")
	}
	if !filepath.IsAbs(p.Entry) {
		return Result{}, errors.New("Pi entry must be absolute")
	}
	if r.SessionDir != "" {
		if !filepath.IsAbs(r.SessionDir) {
			return Result{}, errors.New("session directory must be absolute")
		}
		if err := os.MkdirAll(r.SessionDir, 0700); err != nil {
			return Result{}, err
		}
	}
	node := p.Node
	if node == "" {
		node = "node"
	}
	cmd := exec.CommandContext(ctx, node, p.Entry)
	cmd.Env = modelProcessEnv()
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
		return Result{}, errors.New("cannot start Pi bridge")
	}
	defer func() { in.Close(); _ = cmd.Process.Kill(); _ = cmd.Wait() }()
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
			return Result{}, errors.New("invalid Pi protocol frame")
		}
		switch f.Type {
		case "text_delta":
			textBytes += len(f.Text)
			if textBytes > 65536 {
				return Result{}, errors.New("model_text_limit_exceeded")
			}
			if h.Emit != nil {
				h.Emit(Event{Type: "text.delta", Text: f.Text})
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
			if f.Round <= r.MaxRounds && h.Execute != nil {
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
				return Result{}, errors.New("Pi tool reply failed")
			}
		case "done":
			if len(f.Text) > 65536 {
				return Result{}, errors.New("model_text_limit_exceeded")
			}
			if f.Stop != "stop" && f.Stop != "budget" {
				return Result{}, fmt.Errorf("Pi turn stopped: %s", safeStop(f.Stop))
			}
			if f.Checkpoint != "" {
				rel, e := filepath.Rel(r.SessionDir, f.Checkpoint)
				if r.SessionDir == "" || e != nil || filepath.IsAbs(rel) || rel == ".." || len(rel) > 3 && rel[:3] == "../" {
					return Result{}, errors.New("invalid checkpoint location")
				}
			}
			return Result{Text: f.Text, Stop: f.Stop, Checkpoint: f.Checkpoint, Usage: f.Usage}, nil
		case "error":
			return Result{}, errors.New("Pi runtime failed (authentication, transport, or model response); run the local readiness probe")
		default:
			return Result{}, errors.New("unexpected Pi protocol event")
		}
	}
	if ctx.Err() != nil {
		return Result{}, ctx.Err()
	}
	if scan.Err() != nil {
		return Result{}, errors.New("Pi protocol read failed")
	}
	return Result{}, errors.New("Pi exited before a terminal result")
}
func safeStop(s string) string {
	switch s {
	case "error", "aborted", "length", "toolUse":
		return s
	default:
		return "unexpected"
	}
}
