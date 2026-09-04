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

	jagent "github.com/z-chenhao/J/J-agent/agent"
)

const (
	jCheckpointVersion = 1
	jFrameLimit        = 8 << 20
	jTextLimit         = 64 << 10
)

// J runs the provider-neutral agent loop from J-agent. A small model-only
// process owns provider-native OAuth and reasoning state; business tools remain
// in this Go process behind the same Runtime contract used by other runtimes.
type J struct {
	Entry string
	Node  string
}

type jCheckpoint struct {
	Version       int              `json:"version"`
	SystemSHA256  string           `json:"system_sha256"`
	Provider      string           `json:"provider,omitempty"`
	Model         string           `json:"model,omitempty"`
	History       []jagent.Message `json:"history"`
	ProviderState json.RawMessage  `json:"provider_state"`
}

type jBridgeFrame struct {
	Type          string                `json:"type"`
	ID            string                `json:"id,omitempty"`
	Delta         *jagent.ModelDelta    `json:"delta,omitempty"`
	Response      *jagent.ModelResponse `json:"response,omitempty"`
	ProviderState json.RawMessage       `json:"provider_state,omitempty"`
	Error         string                `json:"error,omitempty"`
}

type jBridgeModel struct {
	cmd       *exec.Cmd
	in        io.WriteCloser
	scan      *bufio.Scanner
	state     json.RawMessage
	nextID    int
	textBytes int
	mu        sync.Mutex
}

func (j J) Run(ctx context.Context, r Request, h Hooks) (Result, error) {
	selection, err := NormalizeModel(r.Model)
	if err != nil {
		return Result{}, err
	}
	r.Model = selection
	if r.MaxRounds < 1 || r.MaxRounds > 16 {
		return Result{}, errors.New("invalid round budget")
	}
	if !filepath.IsAbs(j.Entry) {
		return Result{}, errors.New("J model bridge entry must be absolute")
	}
	if r.SessionDir != "" {
		if !filepath.IsAbs(r.SessionDir) {
			return Result{}, errors.New("session directory must be absolute")
		}
		if err := os.MkdirAll(r.SessionDir, 0700); err != nil {
			return Result{}, err
		}
	}

	checkpoint, err := loadJCheckpoint(r)
	if err != nil {
		return Result{}, err
	}
	model, err := startJBridge(ctx, j, checkpoint.ProviderState, r.Model)
	if err != nil {
		return Result{}, err
	}
	defer model.close()

	state := &jToolState{hooks: h, maxRounds: r.MaxRounds, seen: make(map[string]bool)}
	tools := make([]jagent.Tool, 0, len(r.Tools))
	for _, tool := range r.Tools {
		tools = append(tools, &jTool{spec: jagent.ToolSpec{
			Name: tool.Name, Description: tool.Description, InputSchema: append(json.RawMessage(nil), tool.Parameters...),
		}, state: state})
	}
	bounded := &jBoundedModel{inner: model, maxRounds: r.MaxRounds, state: state, model: r.Model}
	options := []jagent.Option{jagent.WithTools(tools...)}
	if len(checkpoint.History) > 0 {
		options = append(options, jagent.WithHistory(checkpoint.History...))
	} else {
		options = append(options, jagent.WithSystemPrompt(r.System))
	}
	runner, err := jagent.New(bounded, options...)
	if err != nil {
		return Result{}, errors.New("invalid J-agent checkpoint")
	}
	run, err := runner.Run(ctx, r.Prompt, func(event jagent.Event) {
		switch event.Type {
		case jagent.EventTurnStarted:
			state.round++
		case jagent.EventToolStarted:
			if event.ToolCall != nil {
				state.current = cloneJCall(*event.ToolCall)
			}
		case jagent.EventMessageDelta:
			if event.Delta != nil && h.Emit != nil {
				switch event.Delta.Type {
				case jagent.DeltaText:
					h.Emit(Event{Type: "text.delta", Text: event.Delta.Delta})
				case jagent.DeltaToolCall:
					h.Emit(Event{Type: "tool.delta", ID: event.Delta.ToolCallID, Name: event.Delta.ToolName, Arguments: json.RawMessage(event.Delta.Delta)})
				}
			}
		case jagent.EventMessageCompleted:
			if event.Message != nil {
				state.prefetch(ctx, event.Message.ToolCalls())
			}
		}
	})
	if err != nil {
		if ctx.Err() != nil {
			return Result{}, ctx.Err()
		}
		return Result{}, errors.New("J runtime failed (authentication, transport, model response, or checkpoint state): " + safeJBridgeError(err))
	}
	if state.err != nil {
		return Result{}, state.err
	}
	text := run.Message.Text()
	if len(text) > jTextLimit {
		return Result{}, errors.New("model_text_limit_exceeded")
	}
	checkpointPath := ""
	if r.SessionDir != "" {
		checkpointPath, err = saveJCheckpoint(r.SessionDir, jCheckpoint{
			Version: jCheckpointVersion, SystemSHA256: systemDigest(r.System),
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

type jBoundedModel struct {
	inner     jagent.Model
	maxRounds int
	calls     int
	budget    bool
	state     *jToolState
	model     ModelSelection
}

func (m *jBoundedModel) Complete(ctx context.Context, req jagent.ModelRequest, emit func(jagent.ModelDelta)) (jagent.ModelResponse, error) {
	m.calls++
	if m.calls > m.maxRounds+1 {
		return jagent.ModelResponse{}, errors.New("model round limit exceeded")
	}
	if m.calls > m.maxRounds {
		m.budget = true
		req.Tools = nil
	} else if m.state.close {
		req.Tools = nil
	}
	response, err := m.inner.Complete(ctx, req, emit)
	if err != nil {
		return response, err
	}
	if response.Provider != m.model.Provider || response.Model != m.model.Model {
		return jagent.ModelResponse{}, errors.New("unexpected provider or model")
	}
	calls := response.Message.ToolCalls()
	if len(calls) > 0 && response.StopReason != jagent.StopReasonToolCalls {
		return jagent.ModelResponse{}, errors.New("inconsistent tool stop reason")
	}
	if len(calls) == 0 && response.StopReason != jagent.StopReasonStop {
		return jagent.ModelResponse{}, errors.New("incomplete model response")
	}
	if m.budget && len(calls) > 0 {
		return jagent.ModelResponse{}, errors.New("model requested a tool after budget exhaustion")
	}
	return response, nil
}

type jToolState struct {
	hooks     Hooks
	maxRounds int
	round     int
	calls     int
	current   *jagent.ToolCall
	seen      map[string]bool
	err       error
	close     bool
	mu        sync.Mutex
	prepared  map[string]ToolResult
}

// prefetch executes calls emitted together as one model response concurrently. Calls in
// one response cannot depend on each other's results; dependent work must remain in
// separate model rounds. The Go host independently serializes every mutation family.
func (s *jToolState) prefetch(ctx context.Context, calls []jagent.ToolCall) {
	if len(calls) < 2 || s.round < 1 || s.round > s.maxRounds || s.calls+len(calls) > 64 {
		return
	}
	var wait sync.WaitGroup
	for _, original := range calls {
		call := cloneJCall(original)
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

type jTool struct {
	spec  jagent.ToolSpec
	state *jToolState
}

func (t *jTool) Spec() jagent.ToolSpec { return t.spec }

func (t *jTool) Call(ctx context.Context, arguments json.RawMessage) (string, error) {
	s := t.state
	call := s.current
	s.current = nil
	if call == nil || call.Name != t.spec.Name || !sameJSON(call.Arguments, arguments) || call.ID == "" || s.seen[call.ID] {
		s.err = errors.New("invalid J tool correlation")
		return "tool_correlation_failed", s.err
	}
	s.seen[call.ID] = true
	s.calls++
	if s.calls > 64 || s.round < 1 || s.round > s.maxRounds {
		s.err = errors.New("J tool budget exceeded")
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
	if err != nil || len(b) > jTextLimit {
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

func startJBridge(ctx context.Context, j J, state json.RawMessage, selection ModelSelection) (*jBridgeModel, error) {
	node := j.Node
	if node == "" {
		node = "node"
	}
	cmd := exec.CommandContext(ctx, node, j.Entry)
	cmd.Env = modelProcessEnv()
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
		return nil, errors.New("cannot start J model bridge")
	}
	m := &jBridgeModel{cmd: cmd, in: in, scan: bufio.NewScanner(out), state: append(json.RawMessage(nil), state...)}
	m.scan.Buffer(make([]byte, 8192), jFrameLimit)
	if err = json.NewEncoder(in).Encode(struct {
		Type          string          `json:"type"`
		ProviderState json.RawMessage `json:"provider_state,omitempty"`
		Model         ModelSelection  `json:"model"`
	}{Type: "start", ProviderState: state, Model: selection}); err != nil {
		m.close()
		return nil, errors.New("J model bridge start failed")
	}
	if !m.scan.Scan() {
		m.close()
		return nil, errors.New("J model bridge exited during start")
	}
	var frame jBridgeFrame
	if json.Unmarshal(m.scan.Bytes(), &frame) != nil || frame.Type != "ready" {
		m.close()
		return nil, errors.New("invalid J model bridge handshake")
	}
	return m, nil
}

func (m *jBridgeModel) Complete(ctx context.Context, req jagent.ModelRequest, emit func(jagent.ModelDelta)) (jagent.ModelResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextID++
	id := fmt.Sprintf("model-%d", m.nextID)
	if err := json.NewEncoder(m.in).Encode(struct {
		Type    string              `json:"type"`
		ID      string              `json:"id"`
		Request jagent.ModelRequest `json:"request"`
	}{Type: "complete", ID: id, Request: req}); err != nil {
		return jagent.ModelResponse{}, errors.New("J model request failed")
	}
	for m.scan.Scan() {
		if err := ctx.Err(); err != nil {
			return jagent.ModelResponse{}, err
		}
		var frame jBridgeFrame
		if json.Unmarshal(m.scan.Bytes(), &frame) != nil || frame.ID != id {
			return jagent.ModelResponse{}, errors.New("invalid J model protocol frame")
		}
		switch frame.Type {
		case "delta":
			if frame.Delta == nil {
				return jagent.ModelResponse{}, errors.New("invalid J model delta")
			}
			if frame.Delta.Type == jagent.DeltaText {
				m.textBytes += len(frame.Delta.Delta)
				if m.textBytes > jTextLimit {
					return jagent.ModelResponse{}, errors.New("model_text_limit_exceeded")
				}
			}
			emit(*frame.Delta)
		case "done":
			if frame.Response == nil || len(frame.ProviderState) > jFrameLimit {
				return jagent.ModelResponse{}, errors.New("invalid J model result")
			}
			m.state = append(json.RawMessage(nil), frame.ProviderState...)
			return *frame.Response, nil
		case "error":
			return jagent.ModelResponse{}, errors.New("J model bridge failed: " + safeJBridgeCode(frame.Error))
		default:
			return jagent.ModelResponse{}, errors.New("unexpected J model protocol event")
		}
	}
	if ctx.Err() != nil {
		return jagent.ModelResponse{}, ctx.Err()
	}
	return jagent.ModelResponse{}, errors.New("J model bridge exited before a terminal result")
}

func (m *jBridgeModel) providerState() json.RawMessage {
	return append(json.RawMessage(nil), m.state...)
}

func (m *jBridgeModel) close() {
	_ = m.in.Close()
	if m.cmd.Process != nil {
		_ = m.cmd.Process.Kill()
	}
	_ = m.cmd.Wait()
}

func loadJCheckpoint(r Request) (jCheckpoint, error) {
	empty := jCheckpoint{Version: jCheckpointVersion, SystemSHA256: systemDigest(r.System), Provider: r.Model.Provider, Model: r.Model.Model, ProviderState: json.RawMessage(`{"version":1,"assistants":[]}`)}
	if r.Checkpoint == "" {
		return empty, nil
	}
	if r.SessionDir == "" || !filepath.IsAbs(r.Checkpoint) {
		return jCheckpoint{}, errors.New("invalid J checkpoint location")
	}
	root := filepath.Dir(filepath.Clean(r.SessionDir))
	rel, err := filepath.Rel(root, filepath.Clean(r.Checkpoint))
	if err != nil || filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return jCheckpoint{}, errors.New("invalid J checkpoint location")
	}
	info, err := os.Lstat(r.Checkpoint)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() > jFrameLimit {
		return jCheckpoint{}, errors.New("invalid J checkpoint file")
	}
	f, err := os.Open(r.Checkpoint)
	if err != nil {
		return jCheckpoint{}, errors.New("cannot read J checkpoint")
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !opened.Mode().IsRegular() || opened.Mode().Perm()&0077 != 0 || !os.SameFile(info, opened) {
		return jCheckpoint{}, errors.New("J checkpoint changed during open")
	}
	b, err := io.ReadAll(io.LimitReader(f, jFrameLimit+1))
	if err != nil || len(b) > jFrameLimit {
		return jCheckpoint{}, errors.New("cannot read J checkpoint")
	}
	var checkpoint jCheckpoint
	if json.Unmarshal(b, &checkpoint) != nil || checkpoint.Version != jCheckpointVersion || checkpoint.SystemSHA256 != systemDigest(r.System) || len(checkpoint.ProviderState) > jFrameLimit {
		return jCheckpoint{}, errors.New("invalid J checkpoint")
	}
	// v1 checkpoints created before model selection were pinned to Luna.
	if checkpoint.Provider == "" && checkpoint.Model == "" {
		checkpoint.Provider = CodexProvider
		checkpoint.Model = DefaultModel
	}
	if checkpoint.Provider != r.Model.Provider || checkpoint.Model != r.Model.Model {
		return jCheckpoint{}, errors.New("J checkpoint model mismatch")
	}
	return checkpoint, nil
}

func saveJCheckpoint(dir string, checkpoint jCheckpoint) (string, error) {
	b, err := json.Marshal(checkpoint)
	if err != nil || len(b) > jFrameLimit {
		return "", errors.New("J checkpoint too large")
	}
	tmp, err := os.OpenFile(filepath.Join(dir, ".j-checkpoint.tmp"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
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
	path := filepath.Join(dir, "j-checkpoint.json")
	if err = os.Rename(name, path); err != nil {
		return "", err
	}
	ok = true
	return path, nil
}

func systemDigest(system string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(system))) }

func cloneJCall(call jagent.ToolCall) *jagent.ToolCall {
	call.Arguments = append(json.RawMessage(nil), call.Arguments...)
	return &call
}

func sameJSON(a, b json.RawMessage) bool {
	var av, bv any
	return json.Unmarshal(a, &av) == nil && json.Unmarshal(b, &bv) == nil && reflect.DeepEqual(av, bv)
}

func safeJBridgeCode(code string) string {
	switch code {
	case "assistant_state_mismatch", "assistant_state_count_mismatch", "invalid_request", "invalid_state", "invalid_assistant", "invalid_message", "invalid_content", "invalid_system", "invalid_tool", "invalid_tool_result", "unexpected_non_text_content", "provider_failed", "provider_incomplete", "oauth_or_model_missing", "api_key_missing", "invalid_model":
		return code
	default:
		return "runtime_failed"
	}
}

func safeJBridgeError(err error) string {
	for _, code := range []string{"assistant_state_mismatch", "assistant_state_count_mismatch", "invalid_request", "invalid_state", "invalid_assistant", "invalid_message", "invalid_content", "invalid_system", "invalid_tool", "invalid_tool_result", "unexpected_non_text_content", "provider_failed", "provider_incomplete", "oauth_or_model_missing", "api_key_missing", "invalid_model"} {
		if strings.Contains(err.Error(), code) {
			return code
		}
	}
	return "runtime_failed"
}
