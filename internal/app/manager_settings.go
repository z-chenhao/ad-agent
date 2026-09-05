package app

import (
	"errors"
	"github.com/z-chenhao/ad-agent/internal/manager"
)

// Manager account bindings remain deployment-owned. Model execution and
// operator guidance can change without rebinding accounts or their authority.
func (a *ManagerApp) Reconfigure(settings WorkspaceSettings, key string) (*ManagerApp, error) {
	if settings.Backend != (BackendSettings{Kind: "manager"}) || settings.Guardrails != (BudgetSettings{}) {
		return nil, errors.New("manager_account_settings_are_account_scoped")
	}
	runtime, err := configuredRuntime(a.Host.Runtime, settings, key)
	if err != nil {
		return nil, err
	}
	host, err := manager.NewHost(a.Scope, runtime, settings.Skills...)
	if err != nil {
		return nil, err
	}
	if err = host.ConfigureModel(settings.Model); err != nil {
		return nil, err
	}
	return &ManagerApp{Store: a.Store, Host: host, Scope: a.Scope, Runtime: settings.Runtime}, nil
}
