// Package app is the composition root shared by CLI and the local HTTP host.
package app

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/z-chenhao/ad-agent/internal/ads"
	"github.com/z-chenhao/ad-agent/internal/agenthost"
	"github.com/z-chenhao/ad-agent/internal/manager"
	ar "github.com/z-chenhao/ad-agent/internal/runtime"
	"github.com/z-chenhao/ad-agent/internal/sandbox"
	"github.com/z-chenhao/ad-agent/internal/store"
	"path/filepath"
	"sort"
	"time"
)

type App struct {
	Store    *store.Store
	Backend  ads.Reader
	Host     *agenthost.Host
	Changes  agenthost.Changes
	Sandbox  SandboxControl
	Runtime  string
	Writable bool
}

type SandboxControl interface {
	SimulationState(context.Context) (sandbox.SimulationState, error)
	Advance(context.Context, int) (sandbox.AdvanceResult, error)
	AdvanceDebug(context.Context, int) (sandbox.AdvanceResult, []sandbox.CausalTrace, error)
}

type ManagerApp struct {
	Store   *store.Store
	Host    *manager.Host
	Scope   *manager.Scope
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
	a, err := compose(s, b, b, ads.SandboxPolicy(), runtime)
	if err != nil {
		return nil, err
	}
	a.Sandbox = b
	return a, nil
}

// OpenManagerSandboxRuntime composes several persistent fictional advertisers
// behind one application-authorized Manager scope and one runtime adapter.
func OpenManagerSandboxRuntime(stateDir, environment string, runtime ar.Runtime) (*ManagerApp, error) {
	if runtime == nil || !sandbox.ValidEnvironment(environment) {
		return nil, errors.New("invalid manager sandbox configuration")
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
	bindings := make([]manager.Binding, 0, len(profiles))
	for _, profile := range profiles {
		backend := persistentSandbox{s: s, environment: environment, accountID: profile.id, accountName: profile.name}
		bindings = append(bindings, manager.Binding{Backend: backend, Writer: backend, Creator: backend, Planner: backend, Operator: backend, Policy: ads.SandboxPolicy()})
	}
	scope, err := manager.NewScope(environment, "Sandbox manager workspace", s, bindings)
	if err != nil {
		s.Close()
		return nil, err
	}
	host, err := manager.NewHost(scope, runtime)
	if err != nil {
		s.Close()
		return nil, err
	}
	return &ManagerApp{Store: s, Host: host, Scope: scope, Runtime: runtimeName(runtime)}, nil
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
	planner, _ := backend.(ads.OperationPlanner)
	var creator ads.Creator
	var operator ads.Operations
	if writer != nil {
		creator, _ = backend.(ads.Creator)
		operator, _ = backend.(ads.Operations)
	}
	changes := agenthost.Changes{Backend: backend, Writer: writer, Creator: creator, Planner: planner, Operator: operator, Store: s, Policy: policy}
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
	case ar.Builtin, *ar.Builtin:
		return "builtin"
	case ar.Codex, *ar.Codex:
		return "codex"
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
	policy      *ads.Policy
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
	account, accountErr := b.Account(ctx)
	if accountErr != nil {
		return nil, accountErr
	}
	fallback := ads.SandboxPolicy()
	if p.policy != nil {
		fallback = *p.policy
	}
	policy, policyErr := p.s.BudgetPolicy(ctx, account.Source, fallback)
	if policyErr != nil {
		return nil, policyErr
	}
	b.SetBudgetPolicy(policy)
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
	statePayload, factPayloads, e := p.s.SandboxSimulation(ctx, p.simulationStorageScope())
	if e != nil {
		return nil, e
	}
	var state *sandbox.SimulationState
	if len(statePayload) > 0 {
		var value sandbox.SimulationState
		if e = json.Unmarshal(statePayload, &value); e != nil {
			return nil, e
		}
		state = &value
	}
	facts := make([]sandbox.HourFact, 0, len(factPayloads))
	for _, payload := range factPayloads {
		var fact sandbox.HourFact
		if e = json.Unmarshal(payload, &fact); e != nil {
			return nil, e
		}
		facts = append(facts, fact)
	}
	if e = b.RestoreSimulation(state, facts); e != nil {
		return nil, e
	}
	operationPayload, e := p.s.SandboxOperationState(ctx, p.storageScope())
	if e != nil {
		return nil, e
	}
	if len(operationPayload) > 0 {
		var operationState sandbox.OperationState
		if e = json.Unmarshal(operationPayload, &operationState); e != nil {
			return nil, e
		}
		if e = b.RestoreOperationState(operationState); e != nil {
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

// Simulation facts have a separate schema namespace from entities and approvals.
// Never interpret an unrelated namespace as delivery state or delete it during startup.
func (p persistentSandbox) simulationStorageScope() string {
	return p.storageScope() + "__simulation_v1"
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

func (p persistentSandbox) PrepareOperation(ctx context.Context, request ads.OperationRequest) (ads.OperationPlan, error) {
	b, err := p.load(ctx)
	if err != nil {
		return ads.OperationPlan{}, err
	}
	return b.PrepareOperation(ctx, request)
}

func (p persistentSandbox) ApplyOperation(ctx context.Context, plan ads.OperationPlan) ads.OperationOutcome {
	b, err := p.load(ctx)
	if err != nil {
		return ads.OperationOutcome{State: "not_sent", Message: "sandbox_load_failed"}
	}
	outcome := b.ApplyOperation(ctx, plan)
	if outcome.State != "acknowledged" && outcome.State != "partial" {
		return outcome
	}
	payload, err := json.Marshal(b.OperationState())
	if err != nil {
		return ads.OperationOutcome{State: "unknown", RequestIDs: outcome.RequestIDs, Resources: outcome.Resources, Message: "sandbox_state_encode_failed"}
	}
	if err = p.s.SaveSandboxResources(ctx, store.SandboxAdvanceResources{Environment: p.storageScope(), Entities: b.AllEntities(), Operations: payload}); err != nil {
		return ads.OperationOutcome{State: "unknown", RequestIDs: outcome.RequestIDs, Resources: outcome.Resources, Message: "sandbox_state_persistence_failed"}
	}
	return outcome
}

func (p persistentSandbox) ReconcileOperation(ctx context.Context, plan ads.OperationPlan, outcome ads.OperationOutcome) (bool, error) {
	b, err := p.load(ctx)
	if err != nil {
		return false, err
	}
	return b.ReconcileOperation(ctx, plan, outcome)
}

func (p persistentSandbox) SimulationState(ctx context.Context) (sandbox.SimulationState, error) {
	b, err := p.load(ctx)
	if err != nil {
		return sandbox.SimulationState{}, err
	}
	return b.SimulationState(ctx)
}

func (p persistentSandbox) Advance(ctx context.Context, hours int) (sandbox.AdvanceResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	release, err := p.lockAdvance(ctx)
	if err != nil {
		return sandbox.AdvanceResult{}, err
	}
	defer release()
	b, err := p.load(ctx)
	if err != nil {
		return sandbox.AdvanceResult{}, err
	}
	result, facts, err := b.Advance(ctx, hours)
	if err != nil {
		return sandbox.AdvanceResult{}, err
	}
	if err = p.persistAdvance(ctx, b, result, facts); err != nil {
		return sandbox.AdvanceResult{}, err
	}
	result.State.Model = nil
	return result, nil
}

func (p persistentSandbox) AdvanceDebug(ctx context.Context, hours int) (sandbox.AdvanceResult, []sandbox.CausalTrace, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	release, err := p.lockAdvance(ctx)
	if err != nil {
		return sandbox.AdvanceResult{}, nil, err
	}
	defer release()
	b, err := p.load(ctx)
	if err != nil {
		return sandbox.AdvanceResult{}, nil, err
	}
	b.SetSimulationDebug(true)
	result, facts, err := b.Advance(ctx, hours)
	b.SetSimulationDebug(false)
	if err != nil {
		return sandbox.AdvanceResult{}, nil, err
	}
	// Advance returned the complete persistence snapshot. Disable debug in that
	// snapshot before storing it, then redact the model from the caller result.
	result.State.Model.Config.Debug = false
	if err = p.persistAdvance(ctx, b, result, facts); err != nil {
		return sandbox.AdvanceResult{}, nil, err
	}
	traces := make([]sandbox.CausalTrace, 0, len(facts))
	for _, fact := range facts {
		if fact.Trace != nil {
			traces = append(traces, *fact.Trace)
		}
	}
	result.State.Model = nil
	return result, traces, nil
}

// Share the approval writer's account lease before loading mutable state.
// Expiration exceeds the bounded local simulation request; clock CAS remains a second guard.
func (p persistentSandbox) lockAdvance(ctx context.Context) (func(), error) {
	a, err := p.Account(ctx)
	if err != nil {
		return nil, err
	}
	id := "apply:" + a.Source.Backend + ":" + a.Source.Environment + ":" + a.ID
	owner := store.ID("advance")
	if err := p.s.Lease(ctx, id, owner, time.Now().Add(10*time.Minute)); err != nil {
		return nil, err
	}
	return func() { p.s.Release(id, owner) }, nil
}

func (p persistentSandbox) persistAdvance(ctx context.Context, backend *sandbox.Backend, result sandbox.AdvanceResult, facts []sandbox.HourFact) error {
	statePayload, err := json.Marshal(result.State)
	if err != nil {
		return err
	}
	factPayloads := make([]store.SandboxFactPayload, 0, len(facts))
	for _, fact := range facts {
		payload, marshalErr := json.Marshal(fact)
		if marshalErr != nil {
			return marshalErr
		}
		factPayloads = append(factPayloads, store.SandboxFactPayload{AdID: fact.AdID, Hour: fact.Hour, Payload: payload})
	}
	operations, err := json.Marshal(backend.OperationState())
	if err != nil {
		return err
	}
	resources := store.SandboxAdvanceResources{Environment: p.storageScope(), Entities: backend.AllEntities(), Operations: operations}
	if err = p.s.SaveSandboxAdvance(ctx, p.simulationStorageScope(), result.PreviousTime, result.State.CurrentTime, statePayload, factPayloads, resources); err != nil {
		return err
	}
	return nil
}
