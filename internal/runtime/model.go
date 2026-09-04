package runtime

import (
	"errors"
	"net"
	"net/url"
	"regexp"
)

const (
	CodexProvider = "openai-codex"
	DefaultModel  = "gpt-5.6-luna"
	ChatGPTOAuth  = "chatgpt_oauth"
	APIKeyAuth    = "api_key"
)

const (
	AnthropicMessages = "anthropic-messages"
	OpenAIResponses   = "openai-responses"
	OpenAICompletions = "openai-completions"
)

type ModelSelection struct {
	Provider        string `json:"provider"`
	Model           string `json:"model"`
	Reasoning       string `json:"reasoning"`
	AuthMode        string `json:"auth_mode"`
	API             string `json:"api,omitempty"`
	BaseURL         string `json:"base_url,omitempty"`
	APIKeyEnv       string `json:"api_key_env,omitempty"`
	ContextWindow   int    `json:"context_window,omitempty"`
	MaxOutputTokens int    `json:"max_output_tokens,omitempty"`
}

type ModelOption struct {
	Provider        string `json:"provider"`
	Model           string `json:"model"`
	Label           string `json:"label"`
	AuthMode        string `json:"auth_mode"`
	API             string `json:"api,omitempty"`
	BaseURL         string `json:"base_url,omitempty"`
	APIKeyEnv       string `json:"api_key_env,omitempty"`
	ContextWindow   int    `json:"context_window,omitempty"`
	MaxOutputTokens int    `json:"max_output_tokens,omitempty"`
}

var supportedModels = []ModelOption{
	{Provider: CodexProvider, Model: "gpt-5.3-codex-spark", Label: "GPT-5.3 Codex Spark", AuthMode: ChatGPTOAuth},
	{Provider: CodexProvider, Model: "gpt-5.4", Label: "GPT-5.4", AuthMode: ChatGPTOAuth},
	{Provider: CodexProvider, Model: "gpt-5.4-mini", Label: "GPT-5.4 Mini", AuthMode: ChatGPTOAuth},
	{Provider: CodexProvider, Model: "gpt-5.5", Label: "GPT-5.5", AuthMode: ChatGPTOAuth},
	{Provider: CodexProvider, Model: DefaultModel, Label: "GPT-5.6 Luna", AuthMode: ChatGPTOAuth},
	{Provider: CodexProvider, Model: "gpt-5.6-sol", Label: "GPT-5.6 Sol", AuthMode: ChatGPTOAuth},
	{Provider: CodexProvider, Model: "gpt-5.6-terra", Label: "GPT-5.6 Terra", AuthMode: ChatGPTOAuth},
}

func DefaultModelSelection() ModelSelection {
	return ModelSelection{Provider: CodexProvider, Model: DefaultModel, Reasoning: "medium", AuthMode: ChatGPTOAuth}
}

func SupportedModels() []ModelOption {
	return append([]ModelOption(nil), supportedModels...)
}

func ValidateModel(selection ModelSelection) error {
	if selection.Reasoning != "medium" {
		return errors.New("unsupported reasoning policy")
	}
	switch selection.AuthMode {
	case ChatGPTOAuth:
		if selection.API != "" || selection.BaseURL != "" || selection.APIKeyEnv != "" || selection.ContextWindow != 0 || selection.MaxOutputTokens != 0 {
			return errors.New("OAuth model cannot include direct HTTP configuration")
		}
		for _, option := range supportedModels {
			if selection.Provider == option.Provider && selection.Model == option.Model {
				return nil
			}
		}
		return errors.New("unsupported OAuth provider or model")
	case APIKeyAuth:
		if !modelName.MatchString(selection.Provider) || !modelName.MatchString(selection.Model) || !envName.MatchString(selection.APIKeyEnv) {
			return errors.New("invalid direct model identity or API key environment variable")
		}
		if selection.API != AnthropicMessages && selection.API != OpenAIResponses && selection.API != OpenAICompletions {
			return errors.New("unsupported direct model API protocol")
		}
		if !safeModelBaseURL(selection.BaseURL) {
			return errors.New("model base URL must be HTTPS or loopback HTTP")
		}
		if selection.ContextWindow < 4096 || selection.ContextWindow > 4_000_000 || selection.MaxOutputTokens < 256 || selection.MaxOutputTokens > selection.ContextWindow {
			return errors.New("invalid direct model token limits")
		}
		return nil
	default:
		return errors.New("unsupported model authentication mode")
	}
}

func NormalizeModel(selection ModelSelection) (ModelSelection, error) {
	if selection.Provider == "" && selection.Model == "" && selection.Reasoning == "" {
		selection = DefaultModelSelection()
	}
	// Older stored sessions predate the explicit auth axis and are ChatGPT OAuth sessions.
	if selection.AuthMode == "" && selection.Provider == CodexProvider {
		selection.AuthMode = ChatGPTOAuth
	}
	return selection, ValidateModel(selection)
}

var (
	modelName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/-]{0,127}$`)
	envName   = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,127}$`)
)

func safeModelBaseURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	if u.Scheme == "https" {
		return true
	}
	if u.Scheme != "http" {
		return false
	}
	host := u.Hostname()
	return host == "localhost" || net.ParseIP(host) != nil && net.ParseIP(host).IsLoopback()
}
