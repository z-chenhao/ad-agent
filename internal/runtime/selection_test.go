package runtime

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

type credentialProbe struct{ env []string }

func (p *credentialProbe) Run(ctx context.Context, _ Request, _ Hooks) (Result, error) {
	p.env = processEnv(ctx)
	return Result{}, nil
}

func TestCredentialRuntimeIsRequestScopedAndNotSerialized(t *testing.T) {
	const name = "AD_AGENT_WEB_MODEL_KEY"
	t.Setenv(name, "ambient-old")
	probe := &credentialProbe{}
	adapter := CredentialRuntime{Runtime: probe, Env: name, Key: "private-session-value"}
	request := Request{Model: ModelSelection{APIKeyEnv: name}}
	if _, err := adapter.Run(context.Background(), request, Hooks{}); err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, value := range probe.env {
		if strings.HasPrefix(value, name+"=") {
			count++
			if value != name+"=private-session-value" {
				t.Fatal("stale key won")
			}
		}
	}
	if count != 1 {
		t.Fatal("duplicate process credential")
	}
	raw, _ := json.Marshal(request)
	if strings.Contains(string(raw), "private-session-value") {
		t.Fatal("key entered model request")
	}
	if _, err := adapter.Run(context.Background(), Request{}, Hooks{}); err != nil {
		t.Fatal(err)
	}
	for _, value := range probe.env {
		if strings.Contains(value, "private-session-value") {
			t.Fatal("key crossed connection boundary")
		}
	}
}
