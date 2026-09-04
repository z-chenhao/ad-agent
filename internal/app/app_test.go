package app

import (
	"context"
	"github.com/shopspring/decimal"
	"github.com/z-chenhao/ad-agent/internal/ads"
	"github.com/z-chenhao/ad-agent/internal/store"
	"os"
	"testing"
	"time"
)

func TestSandboxSurvivesReopen(t *testing.T) {
	dir := t.TempDir()
	os.Chmod(dir, 0700)
	ctx := context.Background()
	a, e := Open(t.TempDir(), dir)
	if e != nil {
		t.Fatal(e)
	}
	account, _ := a.Backend.Account(ctx)
	before, _ := a.Backend.Get(ctx, ads.Campaign, "campaign_example_1")
	after := before
	v := decimal.NewFromInt(55)
	after.Budget = &v
	s := store.Session{ID: "s", Source: account.Source, Provenance: map[string]store.Seen{before.ID: {Entity: before, At: time.Now()}}}
	c, e := a.Changes.Stage(ctx, s, before, after, ads.BudgetChange, "test")
	if e != nil {
		t.Fatal(e)
	}
	c, e = a.Changes.Apply(ctx, s.ID, c.ID, "operator")
	if e != nil || c.State != ads.Applied {
		t.Fatal("apply", e, c.State)
	}
	a.Store.Close()
	b, e := Open(t.TempDir(), dir)
	if e != nil {
		t.Fatal(e)
	}
	defer b.Store.Close()
	entity, e := b.Backend.Get(ctx, ads.Campaign, before.ID)
	if e != nil || entity.Budget.String() != "55" {
		t.Fatal("lost persisted sandbox change", e)
	}
	persisted, e := b.Store.Change(ctx, c.ID)
	if e != nil || persisted.State != ads.Applied {
		t.Fatal("lost ledger", e)
	}
}
