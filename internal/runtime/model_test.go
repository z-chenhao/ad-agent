package runtime

import "testing"

func TestSupportedModelsAreExplicitAndValid(t *testing.T) {
	options := SupportedModels()
	if len(options) != 7 {
		t.Fatalf("supported model count=%d", len(options))
	}
	for _, option := range options {
		if err := ValidateModel(ModelSelection{Provider: option.Provider, Model: option.Model, Reasoning: "medium"}); err != nil {
			t.Fatalf("option %#v is invalid: %v", option, err)
		}
	}
	options[0].Model = "mutated"
	if SupportedModels()[0].Model == "mutated" {
		t.Fatal("SupportedModels returned mutable shared state")
	}
}

func TestModelValidationFailsClosed(t *testing.T) {
	for _, selection := range []ModelSelection{
		{Provider: "openai", Model: DefaultModel, Reasoning: "medium"},
		{Provider: CodexProvider, Model: "unknown", Reasoning: "medium"},
		{Provider: CodexProvider, Model: DefaultModel, Reasoning: "high"},
	} {
		if ValidateModel(selection) == nil {
			t.Fatalf("selection %#v should be rejected", selection)
		}
	}
	if got, err := NormalizeModel(ModelSelection{}); err != nil || got != DefaultModelSelection() {
		t.Fatalf("default=%#v err=%v", got, err)
	}
}
