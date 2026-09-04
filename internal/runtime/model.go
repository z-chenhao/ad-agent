package runtime

import "errors"

const (
	CodexProvider = "openai-codex"
	DefaultModel  = "gpt-5.6-luna"
)

type ModelSelection struct {
	Provider  string `json:"provider"`
	Model     string `json:"model"`
	Reasoning string `json:"reasoning"`
}

type ModelOption struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Label    string `json:"label"`
}

var supportedModels = []ModelOption{
	{Provider: CodexProvider, Model: "gpt-5.3-codex-spark", Label: "GPT-5.3 Codex Spark"},
	{Provider: CodexProvider, Model: "gpt-5.4", Label: "GPT-5.4"},
	{Provider: CodexProvider, Model: "gpt-5.4-mini", Label: "GPT-5.4 Mini"},
	{Provider: CodexProvider, Model: "gpt-5.5", Label: "GPT-5.5"},
	{Provider: CodexProvider, Model: DefaultModel, Label: "GPT-5.6 Luna"},
	{Provider: CodexProvider, Model: "gpt-5.6-sol", Label: "GPT-5.6 Sol"},
	{Provider: CodexProvider, Model: "gpt-5.6-terra", Label: "GPT-5.6 Terra"},
}

func DefaultModelSelection() ModelSelection {
	return ModelSelection{Provider: CodexProvider, Model: DefaultModel, Reasoning: "medium"}
}

func SupportedModels() []ModelOption {
	return append([]ModelOption(nil), supportedModels...)
}

func ValidateModel(selection ModelSelection) error {
	if selection.Reasoning != "medium" {
		return errors.New("unsupported reasoning policy")
	}
	for _, option := range supportedModels {
		if selection.Provider == option.Provider && selection.Model == option.Model {
			return nil
		}
	}
	return errors.New("unsupported provider or model")
}

func NormalizeModel(selection ModelSelection) (ModelSelection, error) {
	if selection.Provider == "" && selection.Model == "" && selection.Reasoning == "" {
		selection = DefaultModelSelection()
	}
	return selection, ValidateModel(selection)
}
