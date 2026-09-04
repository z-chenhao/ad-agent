// Package app is the composition root shared by CLI and the local HTTP host.
package app

import (
	"context"
	"errors"
	"github.com/z-chenhao/ad-agent/internal/ads"
	"github.com/z-chenhao/ad-agent/internal/agenthost"
	"github.com/z-chenhao/ad-agent/internal/portfolio"
	ar "github.com/z-chenhao/ad-agent/internal/runtime"
	"github.com/z-chenhao/ad-agent/internal/sandbox"
	"github.com/z-chenhao/ad-agent/internal/store"
	"path/filepath"
	"sort"
)

type App struct {
	Store    *store.Store
	Backend  ads.Reader
	Host     *agenthost.Host
	Changes  agenthost.Changes
	Runtime  string
	Writable bool
}

type PortfolioApp struct {
	Store   *store.Store
	Host    *portfolio.Host
	Scope   *portfolio.Portfolio
	Runtime string
}

func Open(root, stateDir string) (*App, error) {
	return OpenRuntime(stateDir, ar.Pi{Entry: filepath.Join(root, "runtime", "pi-bridge", "dist", "main.js")})
}

// OpenRuntime composes the baseline sandbox with an explicitly selected runtime.
func OpenRuntime(stateDir string, runtime ar.Runtime) (*App, error) {
	return OpenSandboxRuntime(stateDir, "default", runtime)
}

// OpenSandboxRuntime composes one persistent, isolated fictional environment.
func OpenSandboxRuntime(stateDir, environment string, runtime ar.Runtime) (*App, error) {
	if runtime == nil {
		return nil, errors.New("runtime is required")
	}
	if !sandbox.ValidEnvironment(environment) {
		return nil, errors.New("invalid sandbox environment")
	}
	s, err := store.Open(stateDir)
	if err != nil {
		return nil, err
	}
	b := persistentSandbox{s: s, environment: environment}
	return compose(s, b, b, ads.SandboxPolicy(), runtime)
}

// OpenPortfolioSandboxRuntime composes several persistent fictional advertisers
// behind one host-authorized portfolio scope and the same runtime port.
func OpenPortfolioSandboxRuntime(stateDir, environment string, runtime ar.Runtime) (*PortfolioApp, error) {
	if runtime == nil || !sandbox.ValidEnvironment(environment) {
		return nil, errors.New("invalid portfolio sandbox configuration")
	}
	s, err := store.Open(stateDir)
	if err != nil {
		return nil, err
	}
	profiles := []struct{ id, name string }{
		{"sandbox_adv_north", "Northstar Apps"},
		{"sandbox_adv_home", "Hearth Commerce"},
		{"sandbox_adv_fit", "Momentum Fitness"},
	}
	bindings := make([]portfolio.Binding, 0, len(profiles))
	for _, profile := range profiles {
		backend := persistentSandbox{s: s, environment: environment, accountID: profile.id, accountName: profile.name}
		bindings = append(bindings, portfolio.Binding{Backend: backend, Writer: backend, Creator: backend, Policy: ads.SandboxPolicy()})
	}
	scope, err := portfolio.NewPortfolio(environment, "Sandbox advertiser portfolio", s, bindings)
	if err != nil {
		s.Close()
		return nil, err
	}
	host, err := portfolio.NewHost(scope, runtime)
	if err != nil {
		s.Close()
		return nil, err
	}
	return &PortfolioApp{Store: s, Host: host, Scope: scope, Runtime: runtimeName(runtime)}, nil
}

// OpenBackend composes one already-bound real backend. Real writes stay disabled
// and no Writer is accepted until endpoint semantics have been live-verified.
func OpenBackend(root, stateDir string, backend ads.Reader) (*App, error) {
	return OpenBackendRuntime(stateDir, backend, ar.Pi{Entry: filepath.Join(root, "runtime", "pi-bridge", "dist", "main.js")})
}

// OpenBackendRuntime composes a real read-only backend with an explicitly selected runtime.
func OpenBackendRuntime(stateDir string, backend ads.Reader, runtime ar.Runtime) (*App, error) {
	if backend == nil {
		return nil, errors.New("backend is required")
	}
	if runtime == nil {
		return nil, errors.New("runtime is required")
	}
	s, err := store.Open(stateDir)
	if err != nil {
		return nil, err
	}
	return compose(s, backend, nil, ads.ReadOnlyPolicy(), runtime)
}

// OpenAdBackendRuntime composes one complete backend. The reader is given to the
// host while the writer is given only to the approval service.
func OpenAdBackendRuntime(stateDir string, backend ads.Backend, policy ads.Policy, runtime ar.Runtime) (*App, error) {
	if backend == nil {
		return nil, errors.New("backend is required")
	}
	if runtime == nil {
		return nil, errors.New("runtime is required")
	}
	s, err := store.Open(stateDir)
	if err != nil {
		return nil, err
	}
	return compose(s, backend, backend, policy, runtime)
}

func compose(s *store.Store, backend ads.Reader, writer ads.Writer, policy ads.Policy, runtime ar.Runtime) (*App, error) {
	creator, _ := backend.(ads.Creator)
	changes := agenthost.Changes{Backend: backend, Writer: writer, Creator: creator, Store: s, Policy: policy}
	h, err := agenthost.New(backend, runtime, s, changes)
	if err != nil {
		s.Close()
		return nil, err
	}
	name := runtimeName(runtime)
	return &App{Store: s, Backend: backend, Host: h, Changes: changes, Runtime: name, Writable: writer != nil}, nil
}

func runtimeName(runtime ar.Runtime) string {
	switch runtime.(type) {
	case ar.Pi, *ar.Pi:
		return "pi"
	case ar.J, *ar.J:
		return "j"
	case ar.Claude, *ar.Claude:
		return "claude"
	default:
		return "custom"
	}
}

// Sandbox state is isolated by environment and persists across CLI invocations.
type persistentSandbox struct {
	s           *store.Store
	environment string
	accountID   string
	accountName string
}

func (p persistentSandbox) load(ctx context.Context) (*sandbox.Backend, error) {
	var b *sandbox.Backend
	var e error
	if p.accountID == "" {
		b, e = sandbox.NewEnvironment(p.environment)
	} else {
		b, e = sandbox.NewAccountEnvironment(p.environment, p.accountID, p.accountName)
	}
	if e != nil {
		return nil, e
	}
	overrides, e := p.s.SandboxEntities(ctx, p.storageScope())
	if e != nil {
		return nil, e
	}
	sort.SliceStable(overrides, func(i, j int) bool {
		order := map[ads.Level]int{ads.Campaign: 0, ads.AdGroup: 1, ads.Ad: 2}
		return order[overrides[i].Level] < order[overrides[j].Level]
	})
	for _, v := range overrides {
		if e = b.Restore(v); e != nil {
			return nil, e
		}
	}
	return b, nil
}

func (p persistentSandbox) storageScope() string {
	if p.accountID == "" {
		return p.environment
	}
	return p.environment + "__" + p.accountID
}
func (p persistentSandbox) Account(ctx context.Context) (ads.Account, error) {
	b, e := p.load(ctx)
	if e != nil {
		return ads.Account{}, e
	}
	return b.Account(ctx)
}
func (p persistentSandbox) List(ctx context.Context, q ads.EntityQuery) ([]ads.Entity, error) {
	b, e := p.load(ctx)
	if e != nil {
		return nil, e
	}
	return b.List(ctx, q)
}
func (p persistentSandbox) Get(ctx context.Context, l ads.Level, id string) (ads.Entity, error) {
	b, e := p.load(ctx)
	if e != nil {
		return ads.Entity{}, e
	}
	return b.Get(ctx, l, id)
}
func (p persistentSandbox) Report(ctx context.Context, q ads.ReportQuery) (ads.Report, error) {
	b, e := p.load(ctx)
	if e != nil {
		return ads.Report{}, e
	}
	return b.Report(ctx, q)
}
func (p persistentSandbox) Write(ctx context.Context, w ads.WriteRequest) ads.WriteOutcome {
	b, e := p.load(ctx)
	if e != nil {
		return ads.WriteOutcome{State: "not_sent", Message: "sandbox_load_failed"}
	}
	result := b.Write(ctx, w)
	entity, e := b.Get(ctx, w.Target.Level, w.Target.ID)
	if e != nil {
		return ads.WriteOutcome{State: "unknown", Message: "sandbox_read_failed"}
	}
	if entity.Version() == w.Target.Version() {
		return result
	}
	if e = p.s.SaveSandbox(ctx, p.storageScope(), entity); e != nil {
		return ads.WriteOutcome{State: "unknown", Message: "sandbox_persistence_failed"}
	}
	return result
}

func (p persistentSandbox) Create(ctx context.Context, request ads.CreateRequest) (ads.Entity, error) {
	b, err := p.load(ctx)
	if err != nil {
		return ads.Entity{}, err
	}
	entity, err := b.Create(ctx, request)
	if err != nil {
		return ads.Entity{}, err
	}
	if err := p.s.SaveSandbox(ctx, p.storageScope(), entity); err != nil {
		return ads.Entity{}, err
	}
	return entity, nil
}
