package store

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/z-chenhao/ad-agent/internal/ads"
)

func TestSessionLeaseCheckpointAndEvents(t *testing.T) {
	dir := t.TempDir()
	os.Chmod(dir, 0700)
	s, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	ctx := context.Background()
	source := ads.Source{Backend: "sandbox", Environment: "baseline", AccountID: "a"}
	session, e := s.Session(ctx, "one", source)
	if e != nil {
		t.Fatal(e)
	}
	session.Checkpoint = "private-checkpoint"
	session.ExecutionContract = "private-contract"
	if e = s.SaveSession(ctx, session); e != nil {
		t.Fatal(e)
	}
	session, e = s.Session(ctx, "one", source)
	if e != nil || session.Checkpoint != "private-checkpoint" || session.ExecutionContract != "private-contract" {
		t.Fatal("checkpoint lost", e)
	}
	other := source
	other.AccountID = "b"
	if _, e = s.Session(ctx, "one", other); e == nil {
		t.Fatal("source rebind")
	}
	if e = s.Lease(ctx, "one", "owner", time.Now().Add(time.Minute)); e != nil {
		t.Fatal(e)
	}
	if e = s.Lease(ctx, "one", "second", time.Now().Add(time.Minute)); e == nil {
		t.Fatal("concurrent turn allowed")
	}
	s.Release("one", "wrong")
	if e = s.Lease(ctx, "one", "second", time.Now().Add(time.Minute)); e == nil {
		t.Fatal("wrong owner released lease")
	}
	s.Release("one", "owner")
	if e = s.Lease(ctx, "one", "second", time.Now().Add(time.Minute)); e != nil {
		t.Fatal(e)
	}
	c := ads.Change{ID: "x", SessionID: "one", State: ads.Staged}
	if e = s.InsertChange(ctx, c); e != nil {
		t.Fatal(e)
	}
	c.State = ads.Applied
	if e = s.Transition(ctx, ads.Staged, c); e == nil {
		t.Fatal("approval bypass transition")
	}
}

func TestMemoryLifecycleIsSourceScoped(t *testing.T) {
	dir := t.TempDir()
	os.Chmod(dir, 0700)
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	source := ads.Source{Backend: "sandbox", Environment: "baseline", AccountID: "account-a"}
	m, err := s.SaveMemory(ctx, source, MemoryConstraint, "Limit each budget change to 10%")
	if err != nil {
		t.Fatal(err)
	}
	memories, err := s.Memories(ctx, source, 50)
	if err != nil || len(memories) != 1 || memories[0] != m {
		t.Fatalf("memory not recalled: %#v %v", memories, err)
	}
	other := source
	other.AccountID = "account-b"
	if memories, err = s.Memories(ctx, other, 50); err != nil || len(memories) != 0 {
		t.Fatalf("memory crossed source: %#v %v", memories, err)
	}
	if _, err = s.DeleteMemory(ctx, other, m.ID); err == nil {
		t.Fatal("cross-source delete allowed")
	}
	deleted, err := s.DeleteMemory(ctx, source, m.ID)
	if err != nil || deleted.ID != m.ID {
		t.Fatal("memory not deleted", err)
	}
	if _, err = s.DeleteMemory(ctx, source, m.ID); err == nil {
		t.Fatal("memory replay delete allowed")
	}
}

func TestMemoryValidation(t *testing.T) {
	dir := t.TempDir()
	os.Chmod(dir, 0700)
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	source := ads.Source{Backend: "sandbox", Environment: "baseline", AccountID: "account-a"}
	for _, tc := range []struct {
		kind MemoryKind
		text string
	}{
		{"unknown", "valid text"},
		{MemoryGoal, ""},
		{MemoryGoal, "line one\nline two"},
	} {
		if _, err = s.SaveMemory(context.Background(), source, tc.kind, tc.text); err == nil {
			t.Fatalf("invalid memory accepted: %#v", tc)
		}
	}
	if _, err = s.Memories(context.Background(), source, 51); err == nil {
		t.Fatal("invalid limit accepted")
	}
}

func TestExtractedMemoryUpsertsBySourceAndKey(t *testing.T) {
	dir := t.TempDir()
	os.Chmod(dir, 0700)
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	source := ads.Source{Backend: "sandbox", Environment: "baseline", AccountID: "account-a"}
	first, err := s.UpsertMemory(ctx, source, "budget guardrail", MemoryConstraint, "Keep budget changes below 10%")
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.UpsertMemory(ctx, source, "budget_guardrail", MemoryConstraint, "Keep each budget change at or below 8%")
	if err != nil {
		t.Fatal(err)
	}
	memories, err := s.Memories(ctx, source, 50)
	if err != nil || len(memories) != 1 {
		t.Fatalf("memories = %#v, err = %v", memories, err)
	}
	if first.ID != second.ID || memories[0].Text != "Keep each budget change at or below 8%" || memories[0].Key != "budget_guardrail" {
		t.Fatalf("keyed memory did not replace: first=%#v second=%#v stored=%#v", first, second, memories[0])
	}
	other := source
	other.AccountID = "account-b"
	third, err := s.UpsertMemory(ctx, other, "budget_guardrail", MemoryConstraint, "Keep changes below 5%")
	if err != nil || third.ID == first.ID {
		t.Fatalf("source-scoped memory collided: %#v %v", third, err)
	}
}

func TestOpenRejectsSymlinkState(t *testing.T) {
	target := t.TempDir()
	os.Chmod(target, 0700)
	linked := filepath.Join(t.TempDir(), "linked-state")
	if err := os.Symlink(target, linked); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(linked); err == nil {
		t.Fatal("state directory symlink accepted")
	}
	dir := t.TempDir()
	os.Chmod(dir, 0700)
	databaseTarget := filepath.Join(t.TempDir(), "database")
	if err := os.WriteFile(databaseTarget, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(databaseTarget, filepath.Join(dir, "state.db")); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(dir); err == nil {
		t.Fatal("state database symlink accepted")
	}
}

func TestSandboxAdvanceUsesAtomicClockCompareAndSwap(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	environment := "concurrent-clock"
	epoch := time.Date(2022, 7, 17, 23, 0, 0, 0, time.UTC)
	first := epoch.Add(time.Hour)
	initialState, _ := json.Marshal(map[string]any{"current_time": first})
	initialFact, _ := json.Marshal(map[string]any{"ad_id": "ad_1", "hour": first})
	if err = s.SaveSandboxAdvance(ctx, environment, epoch, first, initialState, []SandboxFactPayload{{AdID: "ad_1", Hour: first, Payload: initialFact}}); err != nil {
		t.Fatal(err)
	}
	var persistedClock string
	if err = s.db.QueryRowContext(ctx, `SELECT "current_time" FROM sandbox_clock WHERE environment=?`, environment).Scan(&persistedClock); err != nil {
		t.Fatal(err)
	}
	if expected := first.UTC().Format(time.RFC3339); persistedClock != expected {
		t.Fatalf("initial clock=%q expected=%q", persistedClock, expected)
	}

	next := first.Add(time.Hour)
	nextState, _ := json.Marshal(map[string]any{"current_time": next})
	nextFact, _ := json.Marshal(map[string]any{"ad_id": "ad_1", "hour": next})
	start := make(chan struct{})
	errs := make(chan error, 2)
	var ready sync.WaitGroup
	ready.Add(2)
	for range 2 {
		go func() {
			ready.Done()
			<-start
			errs <- s.SaveSandboxAdvance(ctx, environment, first, next, nextState, []SandboxFactPayload{{AdID: "ad_1", Hour: next, Payload: nextFact}})
		}()
	}
	ready.Wait()
	close(start)
	var successes, conflicts int
	for range 2 {
		if saveErr := <-errs; saveErr == nil {
			successes++
		} else if saveErr.Error() == "sandbox_clock_conflict" {
			conflicts++
		} else {
			t.Fatalf("unexpected save error: %v", saveErr)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("CAS results: successes=%d conflicts=%d", successes, conflicts)
	}
	state, facts, err := s.SandboxSimulation(ctx, environment)
	if err != nil || string(state) != string(nextState) || len(facts) != 2 {
		t.Fatalf("persisted simulation: state=%s facts=%d err=%v", state, len(facts), err)
	}
}
