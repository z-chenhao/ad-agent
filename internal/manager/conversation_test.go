package manager

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/z-chenhao/ad-agent/internal/ads"
	ar "github.com/z-chenhao/ad-agent/internal/runtime"
	"github.com/z-chenhao/ad-agent/internal/sandbox"
	"github.com/z-chenhao/ad-agent/internal/store"
)

type namedRuntime struct {
	fakeRuntime
	name string
}

func (r namedRuntime) RuntimeName() string { return r.name }

func TestManagerHandoffRestoresOnlyBoundConversation(t *testing.T) {
	dir := t.TempDir()
	os.Chmod(dir, 0700)
	s, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	backend, _ := sandbox.NewAccountEnvironment("continuity", "adv_one", "Advertiser One")
	scope, err := NewScope("continuity", "Manager", s, []Binding{{Backend: backend, Policy: ads.SandboxPolicy()}})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	session := store.Session{ID: "manager-continuity", Source: scope.Source(), Runtime: "pi", Model: ar.DefaultModelSelection(), Checkpoint: "private-old", Messages: []store.Message{{Role: "user", Text: "Keep the two currencies separate", TurnID: "previous", Status: "completed"}}}
	if err := s.SaveSession(ctx, session); err != nil {
		t.Fatal(err)
	}
	model := namedRuntime{name: "builtin", fakeRuntime: func(ctx context.Context, req ar.Request, hooks ar.Hooks) (ar.Result, error) {
		if req.Checkpoint != "" || !strings.Contains(req.Prompt, "Keep the two currencies separate") {
			t.Fatal("manager context not restored")
		}
		result := hooks.Execute(ctx, tool("read_conversation", `{}`))
		if !result.OK || !strings.Contains(string(result.Data), "Keep the two currencies separate") {
			t.Fatal("manager history unavailable")
		}
		if result := hooks.Execute(ctx, tool("read_conversation", `{"session_id":"foreign"}`)); result.OK {
			t.Fatal("model selected history session")
		}
		return ar.Result{Stop: "stop", Text: "Accounts stay separate.", Checkpoint: "private-new"}, nil
	}}
	host, err := NewHost(scope, model)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := host.Run(ctx, session.ID, "Continue", nil); err != nil {
		t.Fatal(err)
	}
	saved, err := s.Session(ctx, session.ID, session.Source)
	if err != nil || saved.Runtime != "builtin" || len(saved.Messages) != 3 {
		t.Fatal("manager session replaced", err)
	}
}
