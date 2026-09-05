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

// Claude runs the Anthropic Claude Agent SDK as a peer runtime. The SDK owns
// its model/tool loop, while every advertising tool still calls back into Go.
type Claude struct {
	Entry string
	Node  string
}

func (c Claude) Run(ctx context.Context, r Request, h Hooks) (Result, error) {
	model, err := NormalizeModel(r.Model)
	if err != nil {
		return Result{}, err
	}
	if model.AuthMode != APIKeyAuth || model.Provider != "anthropic" || model.API != AnthropicMessages {
		return Result{}, errors.New("Claude runtime requires an Anthropic Messages API-key model")
	}
	r.Model = model
	if r.MaxRounds < 0 || r.MaxRounds > 16 {
		return Result{}, errors.New("invalid round budget")
	}
	if !filepath.IsAbs(c.Entry) {
		return Result{}, errors.New("Claude entry must be absolute")
	}
	if r.SessionDir == "" || !filepath.IsAbs(r.SessionDir) {
		return Result{}, errors.New("Claude session directory must be absolute")
	}
	if err := os.MkdirAll(r.SessionDir, 0700); err != nil {
		return Result{}, err
	}
	node := c.Node
	if node == "" {
		node = "node"
	}
	cmd := exec.CommandContext(ctx, node, c.Entry)
	cmd.Env = processEnv(ctx)
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
		return Result{}, errors.New("cannot start Claude bridge")
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
	count, textBytes, lastRound := 0, 0, 0
	scan := bufio.NewScanner(out)
	scan.Buffer(make([]byte, 8192), 1<<20)
	for scan.Scan() {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		var f frame
		if json.Unmarshal(scan.Bytes(), &f) != nil {
			return Result{}, errors.New("invalid Claude protocol frame")
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
				return Result{}, errors.New("Claude tool reply failed")
			}
		case "done":
			if len(f.Text) > 65536 {
				return Result{}, errors.New("model_text_limit_exceeded")
			}
			if f.Stop != "stop" && f.Stop != "budget" {
				return Result{}, fmt.Errorf("Claude turn stopped: %s", safeStop(f.Stop))
			}
			if f.Checkpoint == "" {
				return Result{}, errors.New("missing Claude checkpoint")
			}
			rel, e := filepath.Rel(r.SessionDir, f.Checkpoint)
			if e != nil || filepath.IsAbs(rel) || rel == ".." || len(rel) > 3 && rel[:3] == "../" {
				return Result{}, errors.New("invalid checkpoint location")
			}
			return Result{Text: f.Text, Stop: f.Stop, Checkpoint: f.Checkpoint, Usage: f.Usage}, nil
		case "error":
			return Result{}, errors.New("Claude runtime failed (authentication, transport, model response, or SDK state)")
		default:
			return Result{}, errors.New("unexpected Claude protocol event")
		}
	}
	if ctx.Err() != nil {
		return Result{}, ctx.Err()
	}
	if scan.Err() != nil {
		return Result{}, errors.New("Claude protocol read failed")
	}
	return Result{}, errors.New("Claude exited before a terminal result")
}
