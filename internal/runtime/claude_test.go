package runtime

import (
	"context"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestClaudeBridgeUsesSameHostToolContract(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node required")
	}
	entry, _ := filepath.Abs("testdata/claude-bridge.mjs")
	dir := t.TempDir()
	t.Setenv("ANTHROPIC_API_KEY", "test-only")
	t.Setenv("CLAUDE_FAKE_SESSION_DIR", dir)
	selection := ModelSelection{Provider: "anthropic", Model: "claude-sonnet-4-6", Reasoning: "medium", AuthMode: APIKeyAuth, API: AnthropicMessages, BaseURL: "https://api.anthropic.com", APIKeyEnv: "ANTHROPIC_API_KEY", ContextWindow: 200000, MaxOutputTokens: 16000}
	calls := 0
	result, err := (Claude{Entry: entry}).Run(context.Background(), Request{System: "system", Prompt: "read", Model: selection, SessionDir: dir, MaxRounds: 2, Tools: []Tool{{Name: "read_data", Description: "read", Parameters: json.RawMessage(`{"type":"object"}`)}}}, Hooks{Execute: func(_ context.Context, call Call) ToolResult {
		calls++
		if call.Name != "read_data" || call.Round != 1 {
			t.Fatalf("call=%#v", call)
		}
		return Value("ok")
	}})
	if err != nil || calls != 1 || result.Text != "done" || result.Usage.CacheRead != 3 {
		t.Fatalf("result=%#v calls=%d err=%v", result, calls, err)
	}
}

func TestClaudeRejectsOAuthSelection(t *testing.T) {
	_, err := (Claude{Entry: "/tmp/unused"}).Run(context.Background(), Request{Model: DefaultModelSelection(), SessionDir: t.TempDir(), MaxRounds: 1}, Hooks{})
	if err == nil {
		t.Fatal("Claude accepted ChatGPT OAuth")
	}
}
