package runtime

import (
	"context"
	"path/filepath"
	"testing"
)

func TestCodexModelConnections(t *testing.T) {
	if err := ValidateCodexModel(DefaultModelSelection()); err != nil {
		t.Fatal(err)
	}
	model := ModelSelection{Provider: "local", Model: "test", Reasoning: "medium", AuthMode: APIKeyAuth, API: OpenAIResponses, BaseURL: "http://127.0.0.1:9911/v1", APIKeyEnv: "AD_AGENT_TEST_KEY", ContextWindow: 128000, MaxOutputTokens: 4096}
	if err := ValidateCodexModel(model); err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{OpenAICompletions, AnthropicMessages} {
		model.API = api
		if err := ValidateCodexModel(model); err == nil || err.Error() != "codex_requires_responses_protocol" {
			t.Fatal("unsupported protocol accepted", err)
		}
	}
}

func TestCodexMainLoopHasNoFixedRoundCeiling(t *testing.T) {
	path, _ := filepath.Abs("testdata/bridge.mjs")
	calls := 0
	result, err := (Codex{Entry: path}).Run(context.Background(), Request{Prompt: "budget"}, Hooks{
		Execute: func(context.Context, Call) ToolResult { calls++; return Value("data") },
	})
	if err != nil || result.Stop != "stop" || calls != 1 {
		t.Fatal("main loop stopped by an internal-pass allowance", result, err)
	}
	if _, err = SelectPeer(Codex{Entry: path}, "j"); err == nil {
		t.Fatal("retired runtime is still selectable")
	}
}
