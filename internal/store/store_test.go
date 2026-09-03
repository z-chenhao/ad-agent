package store

import (
	"context"
	"github.com/z-chenhao/ad-agent/internal/ads"
	"os"
	"path/filepath"
	"testing"
	"time"
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
	source := ads.Source{Backend: "fixture", Environment: "fixture", AccountID: "a"}
	session, e := s.Session(ctx, "one", source)
	if e != nil {
		t.Fatal(e)
	}
	session.Checkpoint = "private-checkpoint"
	if e = s.SaveSession(ctx, session); e != nil {
		t.Fatal(e)
	}
	session, e = s.Session(ctx, "one", source)
	if e != nil || session.Checkpoint != "private-checkpoint" {
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
	source := ads.Source{Backend: "fixture", Environment: "fixture", AccountID: "account-a"}
	m, err := s.SaveMemory(ctx, source, MemoryConstraint, "预算调整一次不超过 10%")
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
	source := ads.Source{Backend: "fixture", Environment: "fixture", AccountID: "account-a"}
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
