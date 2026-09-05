package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/z-chenhao/ad-agent/internal/ads"
	"github.com/z-chenhao/ad-agent/internal/agenthost"
	"github.com/z-chenhao/ad-agent/internal/app"
	ar "github.com/z-chenhao/ad-agent/internal/runtime"
	"github.com/z-chenhao/ad-agent/internal/store"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

const webKeyEnv = "AD_AGENT_WEB_MODEL_KEY"

// Let short report reads finish, but never queue a settings change behind a
// running agent turn or long write. No services are touched before this succeeds.
func (s *Server) lockSettings(ctx context.Context) bool {
	deadline := time.NewTimer(300 * time.Millisecond)
	defer deadline.Stop()
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for {
		if s.appMu.TryLock() {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-deadline.C:
			return false
		case <-tick.C:
		}
	}
}

func (s *Server) initSettings() error {
	if s.App == nil {
		connection := "http"
		if s.Manager.Host.DefaultModel().AuthMode == ar.ChatGPTOAuth {
			connection = "chatgpt_oauth"
		}
		s.settings = app.WorkspaceSettings{Runtime: s.Manager.Runtime, Model: s.Manager.Host.DefaultModel(), Connection: connection, Backend: app.BackendSettings{Kind: "manager"}, Skills: []agenthost.CustomSkill{}}
		raw, err := s.Manager.Store.WorkspaceSettings(context.Background())
		if err != nil {
			return err
		}
		if len(raw) > 0 {
			saved, err := app.DecodeSettings(raw)
			if err != nil {
				return err
			}
			next, err := s.Manager.Reconfigure(saved, "")
			if err != nil {
				return errors.New("saved_workspace_configuration_invalid")
			}
			s.Manager, s.settings = next, saved
		}
		return nil
	}
	value, err := s.App.Settings(context.Background())
	if err != nil {
		return err
	}
	s.settings = value
	raw, err := s.App.Store.WorkspaceSettings(context.Background())
	if err != nil {
		return err
	}
	if len(raw) > 0 {
		saved, e := app.DecodeSettings(raw)
		if e != nil {
			return e
		}
		next, e := s.App.Reconfigure(context.Background(), saved, "")
		if e != nil {
			return errors.New("saved_workspace_configuration_invalid")
		}
		s.App = next
		s.settings = saved
	}
	return nil
}

// Service consumers retain a read lease for the full operation/stream. Settings
// updates are rejected rather than queued while a turn, write, or advance is active.
func (s *Server) configurationGate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && (r.URL.Path == "/api/v1/settings" || strings.HasPrefix(r.URL.Path, "/api/v1/settings/")) {
			next.ServeHTTP(w, r)
			return
		}
		s.appMu.RLock()
		defer s.appMu.RUnlock()
		next.ServeHTTP(w, r)
	})
}

func (s *Server) settingsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/settings", s.authorize(s.getSettings))
	mux.HandleFunc("POST /api/v1/settings", s.authorize(s.saveSettings))
	mux.HandleFunc("POST /api/v1/settings/skills/preview", s.authorize(s.previewSkill))
	mux.HandleFunc("POST /api/v1/settings/openrouter/start", s.authorize(s.startOpenRouter))
	mux.HandleFunc("POST /api/v1/settings/openrouter/complete", s.authorize(s.completeOpenRouter))
}

func (s *Server) getSettings(w http.ResponseWriter, r *http.Request, _ loginSession) {
	var current ar.Runtime
	var toolNames []string
	liveWrites := false
	if s.App != nil {
		current, toolNames, liveWrites = s.App.Host.Runtime, s.App.Host.ToolNames(), s.App.Changes.Policy.LiveWrites
	} else {
		current, toolNames = s.Manager.Host.Runtime, s.Manager.Host.ToolNames()
	}
	runtimes := []string{}
	for _, name := range []string{"builtin", "pi", "codex", "claude", "custom"} {
		if _, err := ar.SelectPeer(current, name); err == nil {
			runtimes = append(runtimes, name)
		}
	}
	ready := s.settings.Model.AuthMode == ar.ChatGPTOAuth || s.modelKey != "" || os.Getenv(s.settings.Model.APIKeyEnv) != ""
	writeJSON(w, 200, struct {
		Settings        app.WorkspaceSettings `json:"settings"`
		Runtimes        []string              `json:"runtimes"`
		KeyReady        bool                  `json:"key_ready"`
		LiveWrites      bool                  `json:"live_writes"`
		Tools           []string              `json:"tools"`
		OpenRouterReady bool                  `json:"openrouter_ready"`
	}{s.settings, runtimes, ready, liveWrites, toolNames, s.openRouterKey != ""})
}

func (s *Server) saveSettings(w http.ResponseWriter, r *http.Request, _ loginSession) {
	var input struct {
		Settings  app.WorkspaceSettings `json:"settings"`
		APIKey    string                `json:"api_key,omitempty"`
		SessionID string                `json:"session_id,omitempty"`
	}
	if !readSettingsJSON(w, r, &input) {
		return
	}
	if !s.lockSettings(r.Context()) {
		writeError(w, 409, "workspace_busy_retry_when_idle")
		return
	}
	defer s.appMu.Unlock()
	var sessionSource ads.Source
	if input.SessionID != "" {
		if !validSession(input.SessionID) {
			writeError(w, 400, "invalid_session")
			return
		}
		if s.App != nil {
			account, err := s.App.Backend.Account(r.Context())
			if err != nil {
				settingsError(w, err)
				return
			}
			sessionSource = account.Source
		} else {
			sessionSource = s.Manager.Scope.Source()
		}
		if _, err := s.diagnosticStore().Session(r.Context(), input.SessionID, sessionSource); err != nil {
			settingsError(w, err)
			return
		}
		owner := store.ID("settings")
		if err := s.diagnosticStore().Lease(r.Context(), input.SessionID, owner, time.Now().Add(time.Minute)); err != nil {
			writeError(w, 409, "workspace_busy_retry_when_idle")
			return
		}
		defer s.diagnosticStore().Release(input.SessionID, owner)
	}
	key := input.APIKey
	if len(key) > 8192 || strings.ContainsAny(key, "\r\n\x00") {
		writeError(w, 400, "invalid_api_key")
		return
	}
	if (input.Settings.Model.AuthMode == ar.ChatGPTOAuth || input.Settings.Connection == "openrouter_oauth") && key != "" {
		writeError(w, 400, "oauth_does_not_accept_api_key")
		return
	}
	if key != "" {
		input.Settings.Model.APIKeyEnv = webKeyEnv
	}
	sameDestination := input.Settings.Model.Provider == s.settings.Model.Provider && input.Settings.Model.BaseURL == s.settings.Model.BaseURL && input.Settings.Model.API == s.settings.Model.API
	if key == "" && sameDestination && input.Settings.Model.APIKeyEnv == webKeyEnv {
		key = s.modelKey
	}
	if input.Settings.Connection == "openrouter_oauth" {
		key = s.openRouterKey
		input.Settings.Model.APIKeyEnv = webKeyEnv
	}
	if input.Settings.Connection == "openrouter_oauth" && key == "" {
		writeError(w, 400, "connect_openrouter_first")
		return
	}
	nextSource, err := s.replaceSettings(r.Context(), input.Settings, key)
	if err != nil {
		settingsError(w, err)
		return
	}
	sessionID := input.SessionID
	if sessionID == "" || sessionSource != nextSource {
		sessionID = store.ID("web")
	}
	writeJSON(w, 200, struct {
		SessionID string `json:"session_id"`
	}{sessionID})
}

func (s *Server) replaceSettings(ctx context.Context, value app.WorkspaceSettings, key string) (ads.Source, error) {
	if s.App == nil {
		next, err := s.Manager.Reconfigure(value, key)
		if err != nil {
			return ads.Source{}, err
		}
		payload, err := json.Marshal(value)
		if err != nil {
			return ads.Source{}, err
		}
		if err = next.Store.SaveWorkspaceSettings(ctx, payload); err != nil {
			return ads.Source{}, errors.New("settings_save_failed")
		}
		s.Manager, s.settings, s.modelKey = next, value, key
		return next.Scope.Source(), nil
	}
	next, err := s.App.Reconfigure(ctx, value, key)
	if err != nil {
		return ads.Source{}, err
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return ads.Source{}, err
	}
	account, err := next.Backend.Account(ctx)
	if err != nil {
		return ads.Source{}, errors.New("ad_connection_unavailable")
	}
	leaseID := "apply:" + account.Source.Backend + ":" + account.Source.Environment + ":" + account.ID
	owner := store.ID("settings")
	if err = s.App.Store.Lease(ctx, leaseID, owner, time.Now().Add(time.Minute)); err != nil {
		return ads.Source{}, errors.New("workspace_busy_retry_when_idle")
	}
	defer s.App.Store.Release(leaseID, owner)
	if err = s.App.Store.SaveWorkspaceConfiguration(ctx, payload, account.Source, next.Changes.Policy); err != nil {
		return ads.Source{}, errors.New("settings_save_failed")
	}
	s.App = next
	s.settings = value
	s.modelKey = key
	if err := s.diagnostics.RecordDiagnostic("server", store.Diagnostic{Type: "workspace.settings_changed", RequestID: store.RequestID(ctx)}); err != nil {
		log.Print("private settings diagnostic write failed")
	}
	return account.Source, nil
}

// Preview parses operator guidance without installing it or changing any settings.
// The single settings save validates the complete draft and applies it atomically.
func (s *Server) previewSkill(w http.ResponseWriter, r *http.Request, _ loginSession) {
	var input struct {
		Content       string   `json:"content"`
		RequiredTools []string `json:"required_tools"`
		Scopes        []string `json:"scopes"`
	}
	if !readSettingsJSON(w, r, &input) {
		return
	}
	skill, err := agenthost.ParseCustomSkill(input.Content, input.RequiredTools, input.Scopes)
	if err != nil {
		settingsError(w, err)
		return
	}
	s.appMu.RLock()
	defer s.appMu.RUnlock()
	var names []string
	if s.App != nil {
		names = s.App.Host.ToolNames()
	} else {
		names = s.Manager.Host.ToolNames()
	}
	installed := map[string]bool{}
	for _, name := range names {
		installed[name] = true
	}
	for _, name := range skill.RequiredTools {
		if !installed[name] {
			writeError(w, 400, "skill_requires_unavailable_tool")
			return
		}
	}
	writeJSON(w, 200, skill)
}

func settingsError(w http.ResponseWriter, err error) {
	if err.Error() == "codex_requires_responses_protocol" {
		writeError(w, 400, "Codex requires an OpenAI Responses connection. Choose Responses in Model settings or use Built-in Runtime or Pi.")
		return
	}
	allowed := map[string]bool{"invalid_budget_guardrails": true, "tiktok_authorization_required": true, "ad_backend_not_implemented": true, "runtime_bridge_not_built": true, "claude_requires_anthropic_messages": true, "invalid_skill_document": true, "built_in_skill_cannot_be_overridden": true, "custom_skill_limit": true, "invalid_sandbox_environment": true}
	code := "invalid_workspace_settings"
	if allowed[err.Error()] {
		code = err.Error()
	}
	writeError(w, 400, code)
}
func readSettingsJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if r.Header.Get("Content-Type") != "application/json" {
		writeError(w, 415, "json_required")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1000000)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if d.Decode(v) != nil {
		writeError(w, 400, "invalid_json")
		return false
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		writeError(w, 400, "invalid_json")
		return false
	}
	return true
}

type oauthAttempt struct {
	Verifier, Owner string
	Until           time.Time
}
