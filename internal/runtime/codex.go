package runtime

import (
	"context"
	"errors"
)

// Codex is a peer adapter, not the application server. Its isolated App Server
// subprocess owns native model sessions; advertising authority stays in Hooks.
type Codex struct{ Entry, Node string }

func ValidateCodexModel(model ModelSelection) error {
	if err := ValidateModel(model); err != nil {
		return err
	}
	if model.AuthMode == APIKeyAuth && model.API != OpenAIResponses {
		return errors.New("codex_requires_responses_protocol")
	}
	return nil
}

func (c Codex) Run(ctx context.Context, request Request, hooks Hooks) (Result, error) {
	model, err := NormalizeModel(request.Model)
	if err != nil {
		return Result{}, err
	}
	if err = ValidateCodexModel(model); err != nil {
		return Result{}, err
	}
	request.Model = model
	return runSDKBridge(ctx, c.Entry, c.Node, "Codex", request, hooks)
}
