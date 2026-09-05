package runtime

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// Name identifies checkpoint ownership, not the provider transport.
func Name(r Runtime) string {
	if n, ok := r.(interface{ RuntimeName() string }); ok {
		return n.RuntimeName()
	}
	switch r.(type) {
	case Pi, *Pi:
		return "pi"
	case Builtin, *Builtin:
		return "builtin"
	case Codex, *Codex:
		return "codex"
	case Claude, *Claude:
		return "claude"
	default:
		return "custom"
	}
}

func SelectPeer(current Runtime, name string) (Runtime, error) {
	if c, ok := current.(CredentialRuntime); ok {
		current = c.Runtime
	}
	var entry, node string
	switch r := current.(type) {
	case Pi:
		entry, node = r.Entry, r.Node
	case *Pi:
		entry, node = r.Entry, r.Node
	case Builtin:
		entry, node = r.Entry, r.Node
	case *Builtin:
		entry, node = r.Entry, r.Node
	case Codex:
		entry, node = r.Entry, r.Node
	case *Codex:
		entry, node = r.Entry, r.Node
	case Claude:
		entry, node = r.Entry, r.Node
	case *Claude:
		entry, node = r.Entry, r.Node
	default:
		if name == Name(current) {
			return current, nil
		}
		return nil, errors.New("runtime_selection_unavailable")
	}
	root := filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(entry))))
	var next Runtime
	switch name {
	case "pi":
		entry = filepath.Join(root, "runtime", "pi-bridge", "dist", "main.js")
		next = Pi{Entry: entry, Node: node}
	case "builtin":
		entry = filepath.Join(root, "runtime", "builtin-model-bridge", "dist", "main.js")
		next = Builtin{Entry: entry, Node: node}
	case "codex":
		entry = filepath.Join(root, "runtime", "codex-bridge", "dist", "main.js")
		next = Codex{Entry: entry, Node: node}
	case "claude":
		entry = filepath.Join(root, "runtime", "claude-bridge", "dist", "main.js")
		next = Claude{Entry: entry, Node: node}
	default:
		return nil, errors.New("unknown_runtime")
	}
	if info, err := os.Stat(entry); err != nil || !info.Mode().IsRegular() {
		return nil, errors.New("runtime_bridge_not_built")
	}
	return next, nil
}

type credentialContextKey struct{}
type processCredential struct{ Name, Value string }

// CredentialRuntime injects an operator-entered key only into this adapter's child
// process environment. It never changes global environment or serialized requests.
type CredentialRuntime struct {
	Runtime  Runtime
	Env, Key string
}

func (r CredentialRuntime) RuntimeName() string { return Name(r.Runtime) }
func (r CredentialRuntime) Run(ctx context.Context, req Request, hooks Hooks) (Result, error) {
	if req.Model.APIKeyEnv == r.Env && r.Key != "" {
		ctx = context.WithValue(ctx, credentialContextKey{}, processCredential{r.Env, r.Key})
	}
	return r.Runtime.Run(ctx, req, hooks)
}

func processEnv(ctx context.Context) []string {
	env := modelProcessEnv()
	if c, ok := ctx.Value(credentialContextKey{}).(processCredential); ok {
		filtered := env[:0]
		for _, value := range env {
			if !strings.HasPrefix(value, c.Name+"=") {
				filtered = append(filtered, value)
			}
		}
		env = filtered
		env = append(env, c.Name+"="+c.Value)
	}
	return env
}
