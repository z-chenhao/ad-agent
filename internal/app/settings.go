package app

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/shopspring/decimal"
	"github.com/z-chenhao/ad-agent/internal/ads"
	"github.com/z-chenhao/ad-agent/internal/agenthost"
	ar "github.com/z-chenhao/ad-agent/internal/runtime"
	"github.com/z-chenhao/ad-agent/internal/sandbox"
	"github.com/z-chenhao/ad-agent/internal/tiktokmapi"
	"path/filepath"
)

type BudgetSettings struct {
	Min   string `json:"min_budget"`
	Max   string `json:"max_budget"`
	Delta string `json:"max_delta_percent"`
}
type BackendSettings struct {
	Kind         string `json:"kind"`
	Environment  string `json:"environment"`
	AdvertiserID string `json:"advertiser_id,omitempty"`
}
type WorkspaceSettings struct {
	Runtime    string                  `json:"runtime"`
	Model      ar.ModelSelection       `json:"model"`
	Connection string                  `json:"connection"`
	Backend    BackendSettings         `json:"backend"`
	Guardrails BudgetSettings          `json:"guardrails"`
	Skills     []agenthost.CustomSkill `json:"skills"`
}

func (a *App) Settings(ctx context.Context) (WorkspaceSettings, error) {
	account, err := a.Backend.Account(ctx)
	if err != nil {
		return WorkspaceSettings{}, err
	}
	policy := a.Changes.Policy
	connection := "http"
	if a.Host.ModelConfig(a.Runtime).Default.AuthMode == ar.ChatGPTOAuth {
		connection = "chatgpt_oauth"
	}
	kind := account.Source.Backend
	if kind == "tiktok-mapi" {
		kind = "tiktok"
	}
	return WorkspaceSettings{Runtime: a.Runtime, Model: a.Host.ModelConfig(a.Runtime).Default, Connection: connection,
		Backend:    BackendSettings{Kind: kind, Environment: account.Source.Environment, AdvertiserID: account.ID},
		Guardrails: BudgetSettings{Min: policy.MinBudget.String(), Max: policy.MaxBudget.String(), Delta: policy.MaxDeltaPercent.String()}, Skills: []agenthost.CustomSkill{}}, nil
}

func (b BudgetSettings) Policy(base ads.Policy) (ads.Policy, error) {
	min, e1 := decimal.NewFromString(b.Min)
	max, e2 := decimal.NewFromString(b.Max)
	delta, e3 := decimal.NewFromString(b.Delta)
	// An explicitly read-only deployment may retain its unconfigured zero policy.
	if e1 == nil && e2 == nil && e3 == nil && min.IsZero() && max.IsZero() && delta.IsZero() && base.MinBudget.IsZero() && base.MaxBudget.IsZero() && !base.LiveWrites {
		return base, nil
	}
	if e1 != nil || e2 != nil || e3 != nil || !min.IsPositive() || max.LessThan(min) || max.GreaterThan(decimal.NewFromInt(10000000)) || !delta.IsPositive() || delta.GreaterThan(decimal.NewFromInt(100)) {
		return base, errors.New("invalid_budget_guardrails")
	}
	base.MinBudget, base.MaxBudget, base.MaxDeltaPercent = min, max, delta
	return base, nil
}

// Reconfigure constructs a replacement without mutating active services. The API
// swaps it only after validation and persistence, under an exclusive request gate.
func (a *App) Reconfigure(ctx context.Context, settings WorkspaceSettings, key string) (*App, error) {
	runtime, err := configuredRuntime(a.Host.Runtime, settings, key)
	if err != nil {
		return nil, err
	}
	backend, writer, policy, control := a.Backend, a.Changes.Writer, a.Changes.Policy, a.Sandbox
	current, err := a.Backend.Account(ctx)
	if err != nil && settings.Backend.Kind != "sandbox" {
		return nil, err
	}
	switch settings.Backend.Kind {
	case "sandbox":
		if !sandbox.ValidEnvironment(settings.Backend.Environment) {
			return nil, errors.New("invalid_sandbox_environment")
		}
		if a.Sandbox == nil || current.Source.Environment != settings.Backend.Environment {
			b := persistentSandbox{s: a.Store, environment: settings.Backend.Environment}
			backend, writer, control, policy = b, b, b, ads.SandboxPolicy()
		}
	case "tiktok":
		if settings.Backend.Environment != "live" && settings.Backend.Environment != "sandbox" {
			return nil, errors.New("invalid_tiktok_environment")
		}
		if current.Source.Backend != "tiktok-mapi" || current.ID != settings.Backend.AdvertiserID || current.Source.Environment != settings.Backend.Environment {
			vault, e := tiktokmapi.NewFileVault(filepath.Join(a.Store.Dir, "credentials"))
			if e != nil {
				return nil, e
			}
			if _, e = vault.Resolve(ctx, settings.Backend.AdvertiserID); e != nil {
				return nil, errors.New("tiktok_authorization_required")
			}
			client, e := tiktokmapi.NewClient(tiktokmapi.Config{BaseURL: tiktokmapi.DefaultBaseURL, AdvertiserID: settings.Backend.AdvertiserID, Environment: settings.Backend.Environment, Tokens: vault})
			if e != nil {
				return nil, e
			}
			b, e := tiktokmapi.NewBackend(client)
			if e != nil {
				return nil, e
			}
			backend, writer, control, policy = b, nil, nil, ads.ReadOnlyPolicy()
		}
	default:
		return nil, errors.New("ad_backend_not_implemented")
	}
	policy, err = settings.Guardrails.Policy(policy)
	if err != nil {
		return nil, err
	}
	// The UI cannot grant live write authority; only deployment composition can.
	if b, ok := backend.(persistentSandbox); ok {
		b.policy = &policy
		backend, writer, control = b, b, b
	}
	planner, _ := backend.(ads.OperationPlanner)
	changes := agenthost.Changes{Backend: backend, Writer: writer, Planner: planner, Store: a.Store, Policy: policy}
	if writer != nil {
		changes.Creator, _ = backend.(ads.Creator)
		changes.Operator, _ = backend.(ads.Operations)
	}
	host, err := agenthost.New(backend, runtime, a.Store, changes, settings.Skills...)
	if err != nil {
		return nil, err
	}
	if err = host.ConfigureModel(settings.Model); err != nil {
		return nil, err
	}
	host.AutomaticMemoryCapture = a.Host.AutomaticMemoryCapture
	return &App{Store: a.Store, Backend: backend, Host: host, Changes: changes, Sandbox: control, Runtime: settings.Runtime, Writable: writer != nil}, nil
}

func DecodeSettings(raw []byte) (WorkspaceSettings, error) {
	var s WorkspaceSettings
	err := json.Unmarshal(raw, &s)
	return s, err
}

func configuredRuntime(current ar.Runtime, settings WorkspaceSettings, key string) (ar.Runtime, error) {
	runtime, err := ar.SelectPeer(current, settings.Runtime)
	if err != nil {
		return nil, err
	}
	if err = ar.ValidateModel(settings.Model); err != nil {
		return nil, err
	}
	if settings.Runtime == "codex" {
		if err = ar.ValidateCodexModel(settings.Model); err != nil {
			return nil, err
		}
	}
	if settings.Runtime == "claude" && (settings.Model.AuthMode != ar.APIKeyAuth || settings.Model.Provider != "anthropic" || settings.Model.API != ar.AnthropicMessages) {
		return nil, errors.New("claude_requires_anthropic_messages")
	}
	if settings.Connection != "http" && settings.Connection != "chatgpt_oauth" && settings.Connection != "openrouter_oauth" {
		return nil, errors.New("invalid_model_connection")
	}
	if (settings.Connection == "chatgpt_oauth") != (settings.Model.AuthMode == ar.ChatGPTOAuth) {
		return nil, errors.New("connection_model_mismatch")
	}
	if settings.Connection == "openrouter_oauth" && (settings.Model.Provider != "openrouter" || settings.Model.BaseURL != "https://openrouter.ai/api/v1" || settings.Model.API != ar.OpenAICompletions) {
		return nil, errors.New("invalid_openrouter_connection")
	}
	if key != "" {
		runtime = ar.CredentialRuntime{Runtime: runtime, Env: settings.Model.APIKeyEnv, Key: key}
	}
	return runtime, nil
}
