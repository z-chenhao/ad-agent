package httpapi

import (
	"context"
	"encoding/json"
	"github.com/shopspring/decimal"
	"github.com/z-chenhao/ad-agent/internal/ads"
	"github.com/z-chenhao/ad-agent/internal/agenthost"
	"github.com/z-chenhao/ad-agent/internal/app"
	ar "github.com/z-chenhao/ad-agent/internal/runtime"
	"github.com/z-chenhao/ad-agent/internal/store"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkspaceSettingsPersistWithoutKeysAndRespectGate(t *testing.T) {
	s, ts, client, key := setup(t)
	code, raw := request(t, client, "POST", ts.URL+"/api/v1/login", ts.URL, "", map[string]string{"key": key})
	if code != 200 {
		t.Fatal(code)
	}
	var auth struct {
		CSRF string `json:"csrf"`
	}
	json.Unmarshal(raw, &auth)
	value := s.settings
	value.Runtime = "custom"
	value.Connection = "http"
	value.Model = ar.ModelSelection{Provider: "openai", Model: "operator-model", AuthMode: ar.APIKeyAuth, Reasoning: "medium", API: ar.OpenAIResponses, BaseURL: "https://api.openai.com/v1", APIKeyEnv: webKeyEnv, ContextWindow: 32000, MaxOutputTokens: 4096}
	input := map[string]any{"settings": value, "api_key": "sentinel-provider-key-not-persistent"}
	code, _ = request(t, client, "POST", ts.URL+"/api/v1/settings", ts.URL, "", input)
	if code != 403 {
		t.Fatal("settings bypassed CSRF", code)
	}
	s.appMu.RLock()
	code, _ = request(t, client, "POST", ts.URL+"/api/v1/settings", ts.URL, auth.CSRF, input)
	s.appMu.RUnlock()
	if code != 409 {
		t.Fatal("changed during active work", code)
	}
	code, raw = request(t, client, "POST", ts.URL+"/api/v1/settings", ts.URL, auth.CSRF, input)
	if code != 200 {
		t.Fatal(code, string(raw))
	}
	persisted, err := s.App.Store.WorkspaceSettings(context.Background())
	if err != nil || strings.Contains(string(persisted), "sentinel-provider-key") {
		t.Fatal("key persisted", err)
	}
	code, raw = request(t, client, "GET", ts.URL+"/api/v1/settings", "", "", nil)
	if code != 200 || strings.Contains(string(raw), "sentinel-provider-key") || !strings.Contains(string(raw), `"key_ready":true`) {
		t.Fatal("unsafe settings response")
	}
	for _, file := range []string{"server.jsonl", "agent-trace.jsonl"} {
		content, _ := os.ReadFile(filepath.Join(s.App.Store.Dir, "logs", file))
		if strings.Contains(string(content), "sentinel-provider-key") {
			t.Fatal("key logged")
		}
	}
	restored, err := New(s.App, s.Origin, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if restored.modelKey != "" || restored.settings.Model.Model != "operator-model" {
		t.Fatal("restart credentials/config mismatch")
	}
}

func TestSettingsTighteningRechecksAlreadyStagedDraft(t *testing.T) {
	s, ts, client, key := setup(t)
	_, raw := request(t, client, "POST", ts.URL+"/api/v1/login", ts.URL, "", map[string]string{"key": key})
	var auth struct {
		CSRF string `json:"csrf"`
	}
	json.Unmarshal(raw, &auth)
	original := s.App
	ctx := context.Background()
	account, _ := original.Backend.Account(ctx)
	budget := decimal.NewFromInt(374)
	draft, err := original.Changes.StageOperation(ctx, store.Session{ID: "settings-policy", Source: account.Source}, ads.OperationRequest{Kind: ads.UpdateAdGroup, AdGroupUpdate: &ads.AdGroupUpdateSpec{AdGroupID: "adgroup_broad_us", Budget: &budget}}, "test approval")
	if err != nil {
		t.Fatal(err)
	}
	value := s.settings
	value.Runtime = "custom"
	value.Guardrails.Delta = "5"
	code, raw := request(t, client, "POST", ts.URL+"/api/v1/settings", ts.URL, auth.CSRF, map[string]any{"settings": value})
	if code != 200 {
		t.Fatal(code, string(raw))
	}
	result, err := original.Changes.Apply(ctx, "settings-policy", draft.ID, "operator")
	if err != nil || result.State != ads.Failed {
		t.Fatal("stale service bypassed saved policy", err, result.State)
	}
	value.Guardrails.Max = "0"
	code, _ = request(t, client, "POST", ts.URL+"/api/v1/settings", ts.URL, auth.CSRF, map[string]any{"settings": value})
	if code != 400 {
		t.Fatal("disabled budget cap")
	}
}

func TestSettingsSkillAndBackendBoundaries(t *testing.T) {
	s, ts, client, key := setup(t)
	s.settings.Runtime = "custom"
	_, raw := request(t, client, "POST", ts.URL+"/api/v1/login", ts.URL, "", map[string]string{"key": key})
	var auth struct {
		CSRF string `json:"csrf"`
	}
	json.Unmarshal(raw, &auth)
	content := "---\nname: operator-audit\ndescription: Inspect a campaign before changing it.\n---\nRead the selected campaign. Separate facts from hypotheses."
	for _, tool := range []string{"shell_exec", "get_entity"} {
		code, b := request(t, client, "POST", ts.URL+"/api/v1/settings/skills/preview", ts.URL, auth.CSRF, map[string]any{"content": content, "required_tools": []string{tool}, "scopes": []string{"advertiser"}})
		if tool == "shell_exec" && code != 400 {
			t.Fatal("unknown skill tool accepted")
		}
		if tool == "get_entity" && code != 200 {
			t.Fatal(code, string(b))
		}
		if tool == "get_entity" {
			if len(s.settings.Skills) != 0 {
				t.Fatal("preview installed a skill")
			}
			var skill agenthost.CustomSkill
			if err := json.Unmarshal(b, &skill); err != nil {
				t.Fatal(err)
			}
			value := s.settings
			value.Skills = append(value.Skills, skill)
			code, b = request(t, client, "POST", ts.URL+"/api/v1/settings", ts.URL, auth.CSRF, map[string]any{"settings": value})
			if code != 200 {
				t.Fatal(code, string(b))
			}
		}
	}
	if !strings.Contains(s.App.Host.PublicHarness().Capabilities[len(s.App.Host.PublicHarness().Capabilities)-1].Name, "operator-audit") {
		t.Fatal("skill unavailable")
	}
	value := s.settings
	value.Backend = app.BackendSettings{Kind: "tiktok", Environment: "live", AdvertiserID: "not-authorized"}
	code, b := request(t, client, "POST", ts.URL+"/api/v1/settings", ts.URL, auth.CSRF, map[string]any{"settings": value})
	if code != 400 || !strings.Contains(string(b), "tiktok_authorization_required") {
		t.Fatal("unbound live account allowed", code, string(b))
	}
	value.Backend.Kind = "meta"
	code, _ = request(t, client, "POST", ts.URL+"/api/v1/settings", ts.URL, auth.CSRF, map[string]any{"settings": value})
	if code != 400 {
		t.Fatal("unimplemented backend accepted")
	}
	if s.App.Sandbox == nil || s.App.Changes.Policy.LiveWrites {
		t.Fatal("failed config changed authority")
	}
}

func TestOpenRouterUsesPKCEAndRejectsUnboundState(t *testing.T) {
	s, ts, client, key := setup(t)
	_, raw := request(t, client, "POST", ts.URL+"/api/v1/login", ts.URL, "", map[string]string{"key": key})
	var auth struct {
		CSRF string `json:"csrf"`
	}
	json.Unmarshal(raw, &auth)
	code, b := request(t, client, "POST", ts.URL+"/api/v1/settings/openrouter/start", ts.URL, auth.CSRF, map[string]any{})
	if code != 200 {
		t.Fatal(code)
	}
	var result struct {
		URL string `json:"url"`
	}
	json.Unmarshal(b, &result)
	u, _ := url.Parse(result.URL)
	if u.Host != "openrouter.ai" || u.Query().Get("code_challenge_method") != "S256" || len(u.Query().Get("code_challenge")) != 43 {
		t.Fatal("missing PKCE")
	}
	callback, _ := url.Parse(u.Query().Get("callback_url"))
	state := callback.Query().Get("state")
	if s.oauthAttempts[state].Owner != auth.CSRF || strings.Contains(string(b), s.oauthAttempts[state].Verifier) {
		t.Fatal("unsafe OAuth state")
	}
	code, _ = request(t, client, "POST", ts.URL+"/api/v1/settings/openrouter/complete", ts.URL, auth.CSRF, map[string]string{"code": "not-sent", "state": "wrong"})
	if code != 400 {
		t.Fatal("invalid OAuth state accepted")
	}
}

func TestManagerSettingsKeepAccountBindingSeparate(t *testing.T) {
	s, ts, client, key := setupManager(t)
	_, raw := request(t, client, "POST", ts.URL+"/api/v1/login", ts.URL, "", map[string]string{"key": key})
	var auth struct {
		CSRF string `json:"csrf"`
	}
	if err := json.Unmarshal(raw, &auth); err != nil {
		t.Fatal(err)
	}
	code, raw := request(t, client, "GET", ts.URL+"/api/v1/settings", "", "", nil)
	if code != 200 || !strings.Contains(string(raw), `"kind":"manager"`) {
		t.Fatal("manager settings missing", code)
	}
	value := s.settings
	value.Runtime = "custom"
	code, raw = request(t, client, "POST", ts.URL+"/api/v1/settings", ts.URL, auth.CSRF, map[string]any{"settings": value})
	if code != 200 {
		t.Fatal(code, string(raw))
	}
	value.Backend = app.BackendSettings{Kind: "sandbox", Environment: "foreign"}
	code, _ = request(t, client, "POST", ts.URL+"/api/v1/settings", ts.URL, auth.CSRF, map[string]any{"settings": value})
	if code != 400 {
		t.Fatal("manager rebound globally")
	}
	content := "---\nname: manager-reading\ndescription: Read each advertiser independently.\n---\nDo not combine currencies."
	code, raw = request(t, client, "POST", ts.URL+"/api/v1/settings/skills/preview", ts.URL, auth.CSRF, map[string]any{"content": content, "required_tools": []string{"list_advertisers"}, "scopes": []string{"manager"}})
	if code != 200 {
		t.Fatal(code, string(raw))
	}
	var skill agenthost.CustomSkill
	if err := json.Unmarshal(raw, &skill); err != nil {
		t.Fatal(err)
	}
	value = s.settings
	value.Skills = append(value.Skills, skill)
	code, raw = request(t, client, "POST", ts.URL+"/api/v1/settings", ts.URL, auth.CSRF, map[string]any{"settings": value})
	if code != 200 {
		t.Fatal(code, string(raw))
	}
}
