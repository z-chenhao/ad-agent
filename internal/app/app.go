// Package app is the composition root shared by CLI and the local HTTP host.
package app

import (
	"context"
	"errors"
	"github.com/z-chenhao/ad-agent/internal/ads"
	"github.com/z-chenhao/ad-agent/internal/agenthost"
	"github.com/z-chenhao/ad-agent/internal/fixture"
	ar "github.com/z-chenhao/ad-agent/internal/runtime"
	"github.com/z-chenhao/ad-agent/internal/store"
	"path/filepath"
)

type App struct {
	Store   *store.Store
	Backend ads.Backend
	Host    *agenthost.Host
	Changes agenthost.Changes
}

func Open(root, stateDir string) (*App, error) {
	s, err := store.Open(stateDir)
	if err != nil {
		return nil, err
	}
	b := persistentFixture{s: s}
	return compose(root, s, b, b, ads.FixturePolicy())
}

// OpenBackend composes one already-bound real backend. Real writes stay disabled
// and no Writer is accepted until endpoint semantics have been live-verified.
func OpenBackend(root, stateDir string, backend ads.Backend) (*App, error) {
	if backend == nil {
		return nil, errors.New("backend is required")
	}
	s, err := store.Open(stateDir)
	if err != nil {
		return nil, err
	}
	return compose(root, s, backend, nil, ads.ReadOnlyPolicy())
}

func compose(root string, s *store.Store, backend ads.Backend, writer ads.Writer, policy ads.Policy) (*App, error) {
	changes := agenthost.Changes{Backend: backend, Writer: writer, Store: s, Policy: policy}
	h, err := agenthost.New(backend, ar.Pi{Entry: filepath.Join(root, "runtime", "pi-bridge", "dist", "main.js")}, s, changes)
	if err != nil {
		s.Close()
		return nil, err
	}
	return &App{Store: s, Backend: backend, Host: h, Changes: changes}, nil
}

// Fixture state lives in the database so independent CLI invocations see the same lab world.
type persistentFixture struct{ s *store.Store }

func (p persistentFixture) load(ctx context.Context) (*fixture.Backend, error) {
	b, e := fixture.New()
	if e != nil {
		return nil, e
	}
	overrides, e := p.s.FixtureEntities(ctx)
	if e != nil {
		return nil, e
	}
	for _, v := range overrides {
		if e = b.Restore(v); e != nil {
			return nil, e
		}
	}
	return b, nil
}
func (p persistentFixture) Account(ctx context.Context) (ads.Account, error) {
	b, e := p.load(ctx)
	if e != nil {
		return ads.Account{}, e
	}
	return b.Account(ctx)
}
func (p persistentFixture) List(ctx context.Context, q ads.EntityQuery) ([]ads.Entity, error) {
	b, e := p.load(ctx)
	if e != nil {
		return nil, e
	}
	return b.List(ctx, q)
}
func (p persistentFixture) Get(ctx context.Context, l ads.Level, id string) (ads.Entity, error) {
	b, e := p.load(ctx)
	if e != nil {
		return ads.Entity{}, e
	}
	return b.Get(ctx, l, id)
}
func (p persistentFixture) Report(ctx context.Context, q ads.ReportQuery) (ads.Report, error) {
	b, e := p.load(ctx)
	if e != nil {
		return ads.Report{}, e
	}
	return b.Report(ctx, q)
}
func (p persistentFixture) Write(ctx context.Context, w ads.WriteRequest) ads.WriteOutcome {
	b, e := p.load(ctx)
	if e != nil {
		return ads.WriteOutcome{State: "not_sent", Message: "fixture_load_failed"}
	}
	result := b.Write(ctx, w)
	if result.State != "acknowledged" {
		return result
	}
	entity, e := b.Get(ctx, w.Target.Level, w.Target.ID)
	if e != nil {
		return ads.WriteOutcome{State: "unknown", Message: "fixture_read_failed"}
	}
	if e = p.s.SaveFixture(ctx, entity); e != nil {
		return ads.WriteOutcome{State: "unknown", Message: "fixture_persistence_failed"}
	}
	return result
}
