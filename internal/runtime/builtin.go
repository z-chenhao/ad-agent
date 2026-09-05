package runtime

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"

	loop "github.com/z-chenhao/J/J-agent/agent"
)

const (
	builtinCheckpointVersion = 1
	builtinFrameLimit        = 8 << 20
	builtinTextLimit         = 64 << 10
)

// Builtin runs the provider-neutral agent loop using the project's lightweight loop dependency. A small model-only
// process owns provider-native OAuth and reasoning state; business tools remain
// in this Go process behind the same Runtime contract used by other runtimes.
type Builtin struct {
	Entry string
	Node  string
}

type builtinCheckpoint struct {
	Version       int             `json:"version"`
	SystemSHA256  string          `json:"system_sha256"`
	Provider      string          `json:"provider,omitempty"`
	Model         string          `json:"model,omitempty"`
	History       []loop.Message  `json:"history"`
	ProviderState json.RawMessage `json:"provider_state"`
}

type builtinBridgeFrame struct {
	Type          string              `json:"type"`
	ID            string              `json:"id,omitempty"`
	Delta         *loop.ModelDelta    `json:"delta,omitempty"`
	Response      *loop.ModelResponse `json:"response,omitempty"`
	ProviderState json.RawMessage     `json:"provider_state,omitempty"`
	Error         string              `json:"error,omitempty"`
}

type builtinBridgeModel struct {
	cmd       *exec.Cmd
	in        io.WriteCloser
	scan      *bufio.Scanner
	state     json.RawMessage
	nextID    int
	textBytes int
	mu        sync.Mutex
}

func (j Builtin) Run(ctx context.Context, r Request, h Hooks) (Result, error) {
	selection, err := NormalizeModel(r.Model)
	if err != nil {
		return Result{}, err
	}
	r.Model = selection
	if r.MaxRounds < 0 || r.MaxRounds > 16 {
		return Result{}, errors.New("invalid round budget")
	}
	if !filepath.IsAbs(j.Entry) {
		return Result{}, errors.New("Builtin model bridge entry must be absolute")
	}
	if r.SessionDir != "" {
		if !filepath.IsAbs(r.SessionDir) {
			return Result{}, errors.New("session directory must be absolute")
		}
		if err := os.MkdirAll(r.SessionDir, 0700); err != nil {
			return Result{}, err
		}
	}

	checkpoint, err := loadBuiltinCheckpoint(r)
	if err != nil {
		return Result{}, errors.New("runtime_checkpoint_invalid")
	}
	model, err := startBuiltinBridge(ctx, j, checkpoint.ProviderState, r.Model)
	if err != nil {
		return Result{}, err
	}
	defer model.close()

	state := &builtinToolState{hooks: h, maxRounds: r.MaxRounds, seen: make(map[string]bool)}
	tools := make([]loop.Tool, 0, len(r.Tools))
	for _, tool := range r.Tools {
		tools = append(tools, &builtinTool{spec: loop.ToolSpec{
			Name: tool.Name, Description: tool.Description, InputSchema: append(json.RawMessage(nil), tool.Parameters...),
		}, state: state})
	}
	bounded := &builtinBoundedModel{inner: model, maxRounds: r.MaxRounds, state: state, model: r.Model}
	options := []loop.Option{loop.WithTools(tools...)}
	if len(checkpoint.History) > 0 {
		options = append(options, loop.WithHistory(checkpoint.History...))
	} else {
		options = append(options, loop.WithSystemPrompt(r.System))
	}
	runner, err := loop.New(bounded, options...)
	if err != nil {
		return Result{}, errors.New("invalid Built-in Runtime checkpoint")
	}
	run, err := runner.Run(ctx, r.Prompt, func(event loop.Event) {
		switch event.Type {
		case loop.EventTurnStarted:
			state.round++
		case loop.EventToolStarted:
			if event.ToolCall != nil {
				state.current = cloneBuiltinCall(*event.ToolCall)
			}
		case loop.EventMessageDelta:
			if event.Delta != nil && h.Emit != nil {
				switch event.Delta.Type {
				case loop.DeltaText:
					h.Emit(Event{Type: "text.delta", ID: fmt.Sprintf("message-%d", state.round), Text: event.Delta.Delta})
				case loop.DeltaToolCall:
					h.Emit(Event{Type: "tool.delta", ID: event.Delta.ToolCallID, Name: event.Delta.ToolName, Arguments: json.RawMessage(event.Delta.Delta)})
				}
			}
		case loop.EventMessageCompleted:
			if event.Message != nil {
				state.prefetch(ctx, event.Message.ToolCalls())
			}
		}
	})
	if err != nil {
		if ctx.Err() != nil {
			return Result{}, ctx.Err()
		}
		return Result{}, errors.New("Builtin runtime failed (authentication, transport, model response, or checkpoint state): " + safeBuiltinBridgeError(err))
	}
	if state.err != nil {
		return Result{}, state.err
	}
	text := run.Message.Text()
	if len(text) > builtinTextLimit {
		return Result{}, errors.New("model_text_limit_exceeded")
	}
	checkpointPath := ""
	if r.SessionDir != "" {
		checkpointPath, err = saveBuiltinCheckpoint(r.SessionDir, builtinCheckpoint{
			Version: builtinCheckpointVersion, SystemSHA256: systemDigest(r.System),
			History: runner.History(), ProviderState: model.providerState(), Provider: r.Model.Provider, Model: r.Model.Model,
		})
		if err != nil {
			return Result{}, err
		}
	}
	usage := Usage{}
	if run.Usage != nil {
		usage.Input = run.Usage.InputTokens
		usage.Output = run.Usage.OutputTokens
		if run.Usage.CachedInputTokens != nil {
			usage.CacheRead = *run.Usage.CachedInputTokens
		}
	}
	stop := "stop"
	if bounded.budget {
		stop = "budget"
	}
	return Result{Text: text, Stop: stop, Checkpoint: checkpointPath, Usage: usage}, nil
}

type builtinBoundedModel struct {
	inner       loop.Model
	maxRounds   int
	calls       int
	budget      bool
	state       *builtinToolState
	model       ModelSelection
	publicBytes int
}

func (m *builtinBoundedModel) Complete(ctx context.Context, req loop.ModelRequest, emit func(loop.ModelDelta)) (loop.ModelResponse, error) {
	m.calls++
	if m.maxRounds > 0 && m.calls > m.maxRounds+1 {
		return loop.ModelResponse{}, errors.New("model round limit exceeded")
	}
	if m.maxRounds > 0 && m.calls > m.maxRounds {
		m.budget = true
		req.Tools = nil
	} else if m.state.close {
		req.Tools = nil
	}
	streamed := make(map[int]string)
	response, err := m.inner.Complete(ctx, req, func(delta loop.ModelDelta) {
		if delta.Type == loop.DeltaText {
			streamed[delta.Index] += delta.Delta
		}
		if emit != nil {
			emit(delta)
		}
	})
	if err != nil {
		return response, err
	}
	m.publicBytes += len(response.Message.Text())
	if m.publicBytes > builtinTextLimit {
		return loop.ModelResponse{}, errors.New("model_text_limit_exceeded")
	}
	// Some providers return public text only in the completed assistant message,
	// including messages that also select tools. Emit its missing suffix before
	// the loop dispatches those tools, without exposing reasoning blocks.
	for index, block := range response.Message.Content {
		if block.Type != loop.ContentText {
			continue
		}
		if !strings.HasPrefix(block.Text, streamed[index]) {
			return loop.ModelResponse{}, errors.New("model_text_mismatch")
		}
		if suffix := strings.TrimPrefix(block.Text, streamed[index]); suffix != "" && emit != nil {
			emit(loop.ModelDelta{Type: loop.DeltaText, Index: index, Delta: suffix})
		}
	}
	if response.Provider != m.model.Provider || response.Model != m.model.Model {
		return loop.ModelResponse{}, errors.New("unexpected provider or model")
	}
	calls := response.Message.ToolCalls()
	if len(calls) > 0 && response.StopReason != loop.StopReasonToolCalls {
		return loop.ModelResponse{}, errors.New("inconsistent tool stop reason")
	}
	if len(calls) == 0 && response.StopReason != loop.StopReasonStop {
		return loop.ModelResponse{}, errors.New("incomplete model response")
	}
	if m.budget && len(calls) > 0 {
		return loop.ModelResponse{}, errors.New("model requested a tool after budget exhaustion")
	}
	return response, nil
}

type builtinToolState struct {
	hooks     Hooks
	maxRounds int
	round     int
	calls     int
	current   *loop.ToolCall
	seen      map[string]bool
	err       error
	close     bool
	mu        sync.Mutex
	prepared  map[string]ToolResult
}

// prefetch executes calls emitted together as one model response concurrently. Calls in
// one response cannot depend on each other's results; dependent work must remain in
// separate model rounds. The Go host independently serializes every mutation family.
func (s *builtinToolState) prefetch(ctx context.Context, calls []loop.ToolCall) {
	if len(calls) < 2 || s.round < 1 || s.maxRounds > 0 && s.round > s.maxRounds || s.calls+len(calls) > 64 {
		return
	}
	var wait sync.WaitGroup
	for _, original := range calls {
		call := cloneBuiltinCall(original)
		wait.Go(func() {
			result := Failure("tool_unavailable")
			runtimeCall := Call{ID: call.ID, Name: call.Name, Arguments: append(json.RawMessage(nil), call.Arguments...), Round: s.round}
			if s.hooks.Execute != nil {
				result = s.hooks.Execute(ctx, runtimeCall)
			}
			closes := result.OK && s.hooks.CloseAfter != nil && s.hooks.CloseAfter(runtimeCall, result)
			if closes {
				result.Close = true
			}
			s.mu.Lock()
			if s.prepared == nil {
				s.prepared = make(map[string]ToolResult)
			}
			s.prepared[call.ID] = result
			s.close = s.close || closes
			s.mu.Unlock()
		})
	}
	wait.Wait()
}

type builtinTool struct {
	spec  loop.ToolSpec
	state *builtinToolState
}

func (t *builtinTool) Spec() loop.ToolSpec { return t.spec }

func (t *builtinTool) Call(ctx context.Context, arguments json.RawMessage) (string, error) {
	s := t.state
	call := s.current
	s.current = nil
	if call == nil || call.Name != t.spec.Name || !sameJSON(call.Arguments, arguments) || call.ID == "" || s.seen[call.ID] {
		s.err = errors.New("invalid Builtin tool correlation")
		return "tool_correlation_failed", s.err
	}
	s.seen[call.ID] = true
	s.calls++
	if s.calls > 64 || s.round < 1 || s.maxRounds > 0 && s.round > s.maxRounds {
		s.err = errors.New("Builtin tool budget exceeded")
		return "tool_budget_exhausted", s.err
	}
	s.mu.Lock()
	result, prepared := s.prepared[call.ID]
	delete(s.prepared, call.ID)
	s.mu.Unlock()
	if !prepared {
		result = Failure("tool_unavailable")
		runtimeCall := Call{ID: call.ID, Name: call.Name, Arguments: append(json.RawMessage(nil), arguments...), Round: s.round}
		if s.hooks.Execute != nil {
			result = s.hooks.Execute(ctx, runtimeCall)
		}
		if result.OK && s.hooks.CloseAfter != nil && s.hooks.CloseAfter(runtimeCall, result) {
			result.Close = true
			s.mu.Lock()
			s.close = true
			s.mu.Unlock()
		}
	}
	b, err := json.Marshal(result)
	if err != nil || len(b) > builtinTextLimit {
		result = Failure("tool_result_too_large")
		b, _ = json.Marshal(result)
	}
	output := "<untrusted_tool_data>\n" + string(b) + "\n</untrusted_tool_data>"
	if !result.OK {
		code := result.Error
		if code == "" {
			code = "tool_failed"
		}
		return output, errors.New(code)
	}
	return output, nil
}

func startBuiltinBridge(ctx context.Context, j Builtin, state json.RawMessage, selection ModelSelection) (*builtinBridgeModel, error) {
	node := j.Node
	if node == "" {
		node = "node"
	}
	cmd := exec.CommandContext(ctx, node, j.Entry)
	configureBridgeProcess(cmd)
	cmd.Env = processEnv(ctx)
	cmd.Stderr = io.Discard
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err = cmd.Start(); err != nil {
		return nil, errors.New("cannot start Builtin model bridge")
	}
	m := &builtinBridgeModel{cmd: cmd, in: in, scan: bufio.NewScanner(out), state: append(json.RawMessage(nil), state...)}
	m.scan.Buffer(make([]byte, 8192), builtinFrameLimit)
	if err = json.NewEncoder(in).Encode(struct {
		Type          string          `json:"type"`
		ProviderState json.RawMessage `json:"provider_state,omitempty"`
		Model         ModelSelection  `json:"model"`
	}{Type: "start", ProviderState: state, Model: selection}); err != nil {
		m.close()
		return nil, errors.New("Builtin model bridge start failed")
	}
	if !m.scan.Scan() {
		m.close()
		return nil, errors.New("Builtin model bridge exited during start")
	}
	var frame builtinBridgeFrame
	if json.Unmarshal(m.scan.Bytes(), &frame) != nil || frame.Type != "ready" {
		m.close()
		return nil, errors.New("invalid Builtin model bridge handshake")
	}
	return m, nil
}

func (m *builtinBridgeModel) Complete(ctx context.Context, req loop.ModelRequest, emit func(loop.ModelDelta)) (loop.ModelResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextID++
	id := fmt.Sprintf("model-%d", m.nextID)
	if err := json.NewEncoder(m.in).Encode(struct {
		Type    string            `json:"type"`
		ID      string            `json:"id"`
		Request loop.ModelRequest `json:"request"`
	}{Type: "complete", ID: id, Request: req}); err != nil {
		return loop.ModelResponse{}, errors.New("Builtin model request failed")
	}
	for m.scan.Scan() {
		if err := ctx.Err(); err != nil {
			return loop.ModelResponse{}, err
		}
		var frame builtinBridgeFrame
		if json.Unmarshal(m.scan.Bytes(), &frame) != nil || frame.ID != id {
			return loop.ModelResponse{}, errors.New("invalid Builtin model protocol frame")
		}
		switch frame.Type {
		case "delta":
			if frame.Delta == nil {
				return loop.ModelResponse{}, errors.New("invalid Builtin model delta")
			}
			if frame.Delta.Type == loop.DeltaText {
				m.textBytes += len(frame.Delta.Delta)
				if m.textBytes > builtinTextLimit {
					return loop.ModelResponse{}, errors.New("model_text_limit_exceeded")
				}
			}
			emit(*frame.Delta)
		case "done":
			if frame.Response == nil || len(frame.ProviderState) > builtinFrameLimit {
				return loop.ModelResponse{}, errors.New("invalid Builtin model result")
			}
			m.state = append(json.RawMessage(nil), frame.ProviderState...)
			return *frame.Response, nil
		case "error":
			return loop.ModelResponse{}, errors.New("Builtin model bridge failed: " + safeBuiltinBridgeCode(frame.Error))
		default:
			return loop.ModelResponse{}, errors.New("unexpected Builtin model protocol event")
		}
	}
	if ctx.Err() != nil {
		return loop.ModelResponse{}, ctx.Err()
	}
	return loop.ModelResponse{}, errors.New("Builtin model bridge exited before a terminal result")
}

func (m *builtinBridgeModel) providerState() json.RawMessage {
	return append(json.RawMessage(nil), m.state...)
}

func (m *builtinBridgeModel) close() {
	_ = m.in.Close()
	if m.cmd.Process != nil {
		_ = killBridgeProcess(m.cmd)
	}
	_ = m.cmd.Wait()
}

func loadBuiltinCheckpoint(r Request) (builtinCheckpoint, error) {
	empty := builtinCheckpoint{Version: builtinCheckpointVersion, SystemSHA256: systemDigest(r.System), Provider: r.Model.Provider, Model: r.Model.Model, ProviderState: json.RawMessage(`{"version":1,"assistants":[]}`)}
	if r.Checkpoint == "" {
		return empty, nil
	}
	if r.SessionDir == "" || !filepath.IsAbs(r.Checkpoint) {
		return builtinCheckpoint{}, errors.New("invalid Builtin checkpoint location")
	}
	root := filepath.Dir(filepath.Clean(r.SessionDir))
	rel, err := filepath.Rel(root, filepath.Clean(r.Checkpoint))
	if err != nil || filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return builtinCheckpoint{}, errors.New("invalid Builtin checkpoint location")
	}
	info, err := os.Lstat(r.Checkpoint)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() > builtinFrameLimit {
		return builtinCheckpoint{}, errors.New("invalid Builtin checkpoint file")
	}
	f, err := os.Open(r.Checkpoint)
	if err != nil {
		return builtinCheckpoint{}, errors.New("cannot read Builtin checkpoint")
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !opened.Mode().IsRegular() || opened.Mode().Perm()&0077 != 0 || !os.SameFile(info, opened) {
		return builtinCheckpoint{}, errors.New("Builtin checkpoint changed during open")
	}
	b, err := io.ReadAll(io.LimitReader(f, builtinFrameLimit+1))
	if err != nil || len(b) > builtinFrameLimit {
		return builtinCheckpoint{}, errors.New("cannot read Builtin checkpoint")
	}
	var checkpoint builtinCheckpoint
	if json.Unmarshal(b, &checkpoint) != nil || checkpoint.Version != builtinCheckpointVersion || checkpoint.SystemSHA256 != systemDigest(r.System) || len(checkpoint.ProviderState) > builtinFrameLimit {
		return builtinCheckpoint{}, errors.New("invalid Builtin checkpoint")
	}
	if checkpoint.Provider != r.Model.Provider || checkpoint.Model != r.Model.Model {
		return builtinCheckpoint{}, errors.New("Builtin checkpoint model mismatch")
	}
	return checkpoint, nil
}

func saveBuiltinCheckpoint(dir string, checkpoint builtinCheckpoint) (string, error) {
	b, err := json.Marshal(checkpoint)
	if err != nil || len(b) > builtinFrameLimit {
		return "", errors.New("Builtin checkpoint too large")
	}
	tmp, err := os.OpenFile(filepath.Join(dir, ".builtin-checkpoint.tmp"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return "", err
	}
	name := tmp.Name()
	ok := false
	defer func() {
		_ = tmp.Close()
		if !ok {
			_ = os.Remove(name)
		}
	}()
	if _, err = tmp.Write(b); err != nil {
		return "", err
	}
	if err = tmp.Sync(); err != nil {
		return "", err
	}
	if err = tmp.Close(); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "builtin-checkpoint.json")
	if err = os.Rename(name, path); err != nil {
		return "", err
	}
	ok = true
	return path, nil
}

func systemDigest(system string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(system))) }

func cloneBuiltinCall(call loop.ToolCall) *loop.ToolCall {
	call.Arguments = append(json.RawMessage(nil), call.Arguments...)
	return &call
}

func sameJSON(a, b json.RawMessage) bool {
	var av, bv any
	return json.Unmarshal(a, &av) == nil && json.Unmarshal(b, &bv) == nil && reflect.DeepEqual(av, bv)
}

func safeBuiltinBridgeCode(code string) string {
	if strings.HasPrefix(code, "provider_") && SafeFailureCode(code) == code {
		return code
	}
	switch code {
	case "assistant_state_mismatch", "assistant_state_count_mismatch", "invalid_request", "invalid_state", "invalid_assistant", "invalid_message", "invalid_content", "invalid_system", "invalid_tool", "invalid_tool_result", "unexpected_non_text_content", "provider_failed", "provider_incomplete", "oauth_or_model_missing", "api_key_missing", "invalid_model":
		return code
	default:
		return "runtime_failed"
	}
}

func safeBuiltinBridgeError(err error) string {
	if code := FailureCode(err); strings.HasPrefix(code, "provider_") {
		return code
	}
	for _, code := range []string{"assistant_state_mismatch", "assistant_state_count_mismatch", "invalid_request", "invalid_state", "invalid_assistant", "invalid_message", "invalid_content", "invalid_system", "invalid_tool", "invalid_tool_result", "unexpected_non_text_content", "provider_failed", "provider_incomplete", "oauth_or_model_missing", "api_key_missing", "invalid_model"} {
		if strings.Contains(err.Error(), code) {
			return code
		}
	}
	return "runtime_failed"
}
