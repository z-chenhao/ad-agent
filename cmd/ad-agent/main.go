package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/shopspring/decimal"
	"github.com/z-chenhao/ad-agent/internal/ads"
	"github.com/z-chenhao/ad-agent/internal/app"
	"github.com/z-chenhao/ad-agent/internal/httpapi"
	"github.com/z-chenhao/ad-agent/internal/oauthcallback"
	ar "github.com/z-chenhao/ad-agent/internal/runtime"
	"github.com/z-chenhao/ad-agent/internal/store"
	"github.com/z-chenhao/ad-agent/internal/tiktokmapi"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
func run(ctx context.Context, args []string, in io.Reader, out io.Writer) error {
	if len(args) == 0 {
		fmt.Fprintln(out, "Ad Agent — local advertising operations and approvals\nCommands: chat, inspect, report, changes, approve, discard, reconcile, serve, oauth-start, oauth-callback\nDefault source: fictional local sandbox data; no TikTok request or write is made.\nExample: ad-agent chat --message 'Which campaign contributed most to the latest ROAS change?'\nUse --help with any command for options.")
		return nil
	}
	command := args[0]
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(out)
	root := fs.String("root", ".", "project root containing the built runtime bridges")
	runtimeName := fs.String("runtime", "pi", "agent runtime: pi, j, or claude; do not switch within a session")
	provider := fs.String("provider", ar.CodexProvider, "model provider ID")
	model := fs.String("model", ar.DefaultModel, "model ID; use --help or the Web capability panel for supported models")
	modelAuth := fs.String("model-auth", ar.ChatGPTOAuth, "model authentication: chatgpt_oauth or api_key")
	modelAPI := fs.String("model-api", "", "direct HTTP protocol: anthropic-messages, openai-responses, or openai-completions")
	modelBaseURL := fs.String("model-base-url", "", "direct model HTTPS base URL (loopback HTTP is allowed)")
	modelAPIKeyEnv := fs.String("model-api-key-env", "", "environment variable containing the direct model API key")
	modelContextWindow := fs.Int("model-context-window", 128000, "direct model context window")
	modelMaxOutput := fs.Int("model-max-output-tokens", 16384, "direct model maximum output tokens")
	data := fs.String("data-dir", ".data", "private local state directory with mode 0700")
	session := fs.String("session", "local", "session ID")
	message := fs.String("message", "", "single-turn message; omit for interactive mode")
	asJSON := fs.Bool("json", false, "print structured JSON")
	events := fs.Bool("events", false, "print public lifecycle events as NDJSON")
	autoMemory := fs.Bool("auto-memory", true, "extract filtered durable operator facts after completed turns")
	level := fs.String("level", "campaign", "campaign / ad_group / ad / advertiser (reports only)")
	parent := fs.String("parent", "", "parent entity ID")
	start := fs.String("start", "2022-07-11", "inclusive start date")
	end := fs.String("end", "2022-07-17", "inclusive end date")
	id := fs.String("id", "", "exact entity or change ID")
	addr := fs.String("addr", "127.0.0.1:8080", "loopback Web address; port 3000 is reserved for OAuth callback")
	backendName := fs.String("backend", "sandbox", "sandbox or tiktok; TikTok writes require explicit enablement")
	scopeName := fs.String("scope", "single_advertiser", "operation scope: single_advertiser or portfolio")
	sandboxEnvironment := fs.String("sandbox-environment", "default", "isolated persistent local sandbox environment ID")
	tiktokAdvertiser := fs.String("tiktok-advertiser", "", "bound TikTok advertiser ID")
	tiktokEnvironment := fs.String("tiktok-environment", "sandbox", "TikTok environment: sandbox or live")
	tiktokBaseURL := fs.String("tiktok-base-url", tiktokmapi.DefaultBaseURL, "TikTok MAPI HTTPS base URL")
	tiktokRevenueMetric := fs.String("tiktok-revenue-metric", "", "validated revenue metric; empty disables live ROAS")
	enableTikTokWrites := fs.Bool("enable-tiktok-writes", false, "enable approval-gated TikTok budget and status writes")
	tiktokMinBudget := fs.String("tiktok-min-budget", "", "minimum approved TikTok budget; required when writes are enabled")
	tiktokMaxBudget := fs.String("tiktok-max-budget", "", "maximum approved TikTok budget; required when writes are enabled")
	tiktokMaxDelta := fs.String("tiktok-max-budget-delta-percent", "", "maximum approved budget delta percent; required when writes are enabled")
	redirectURL := fs.String("redirect-url", "", "exact registered HTTPS or localhost root callback URL")
	authorizationURL := fs.String("authorization-url", "", "advertiser authorization URL copied from TikTok My Apps")
	if e := fs.Parse(args[1:]); e != nil {
		if errors.Is(e, flag.ErrHelp) {
			return nil
		}
		return e
	}
	if fs.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	if *scopeName != "single_advertiser" && *scopeName != "portfolio" {
		return errors.New("scope must be single_advertiser or portfolio")
	}
	switch command {
	case "chat", "inspect", "report", "changes", "approve", "discard", "reconcile", "serve", "oauth-start", "oauth-callback":
	default:
		return errors.New("unknown command")
	}
	absolute, e := filepath.Abs(*root)
	if e != nil {
		return e
	}
	if command == "oauth-callback" {
		if *scopeName != "single_advertiser" {
			return errors.New("TikTok OAuth setup is account-bound and requires --scope single_advertiser")
		}
		if *addr != "127.0.0.1:3000" {
			return errors.New("oauth-callback requires --addr 127.0.0.1:3000")
		}
		appID, appSecret := os.Getenv("AD_AGENT_TIKTOK_APP_ID"), os.Getenv("AD_AGENT_TIKTOK_APP_SECRET")
		if appID == "" || appSecret == "" || *redirectURL == "" {
			return errors.New("oauth-callback requires redirect URL and TikTok app credentials in the documented environment variables")
		}
		s, err := store.Open(*data)
		if err != nil {
			return err
		}
		defer s.Close()
		vault, err := tiktokmapi.NewFileVault(filepath.Join(s.Dir, "credentials"))
		if err != nil {
			return err
		}
		oauth, err := tiktokmapi.NewOAuthClient(tiktokmapi.OAuthConfig{BaseURL: *tiktokBaseURL, AppID: appID, AppSecret: appSecret})
		if err != nil {
			return err
		}
		callback, err := oauthcallback.New(s, oauth, vault, *redirectURL)
		if err != nil {
			return err
		}
		server := &http.Server{Addr: *addr, Handler: callback.Handler(), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 15 * time.Second, MaxHeaderBytes: 8192}
		fmt.Fprintln(out, "TikTok callback-only listener: http://127.0.0.1:3000/\nThis endpoint accepts one-time OAuth callbacks only; it serves no management UI.")
		go func() {
			<-ctx.Done()
			stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_ = server.Shutdown(stopCtx)
		}()
		err = server.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
	if command == "oauth-start" {
		if *scopeName != "single_advertiser" {
			return errors.New("TikTok OAuth setup is account-bound and requires --scope single_advertiser")
		}
		if *redirectURL == "" || *authorizationURL == "" {
			return errors.New("oauth-start requires --redirect-url and the URL copied from TikTok My Apps in --authorization-url")
		}
		// Validate the portal URL before persisting a state row.
		if _, err := tiktokmapi.PrepareAuthorizationURL(*authorizationURL, *redirectURL, strings.Repeat("s", 43)); err != nil {
			return err
		}
		s, err := store.Open(*data)
		if err != nil {
			return err
		}
		defer s.Close()
		state, err := s.BeginOAuth(ctx, "primary", *redirectURL, 15*time.Minute)
		if err != nil {
			return err
		}
		prepared, err := tiktokmapi.PrepareAuthorizationURL(*authorizationURL, *redirectURL, state)
		if err != nil {
			return err
		}
		fmt.Fprintln(out, prepared)
		return nil
	}
	selection := ar.ModelSelection{Provider: *provider, Model: *model, Reasoning: "medium", AuthMode: *modelAuth}
	if *modelAuth == ar.APIKeyAuth {
		selection.API = *modelAPI
		selection.BaseURL = *modelBaseURL
		selection.APIKeyEnv = *modelAPIKeyEnv
		selection.ContextWindow = *modelContextWindow
		selection.MaxOutputTokens = *modelMaxOutput
	}
	selection, e = ar.NormalizeModel(selection)
	if e != nil {
		return e
	}
	var selectedRuntime ar.Runtime
	switch *runtimeName {
	case "pi":
		selectedRuntime = ar.Pi{Entry: filepath.Join(absolute, "runtime", "pi-bridge", "dist", "main.js")}
	case "j":
		selectedRuntime = ar.J{Entry: filepath.Join(absolute, "runtime", "j-model-bridge", "dist", "main.js")}
	case "claude":
		if selection.AuthMode != ar.APIKeyAuth || selection.Provider != "anthropic" || selection.API != ar.AnthropicMessages {
			return errors.New("claude runtime requires --model-auth api_key --provider anthropic --model-api anthropic-messages")
		}
		selectedRuntime = ar.Claude{Entry: filepath.Join(absolute, "runtime", "claude-bridge", "dist", "main.js")}
	default:
		return errors.New("runtime must be pi, j, or claude")
	}
	if *scopeName == "portfolio" {
		if *backendName != "sandbox" {
			return errors.New("portfolio scope currently requires --backend sandbox; live portfolios need explicit account bindings")
		}
		portfolioApp, err := app.OpenPortfolioSandboxRuntime(*data, *sandboxEnvironment, selectedRuntime)
		if err != nil {
			return err
		}
		defer portfolioApp.Store.Close()
		if err = portfolioApp.Host.ConfigureModel(selection); err != nil {
			return err
		}
		return runPortfolio(ctx, command, portfolioApp, absolute, *addr, *session, *message, *start, *end, *id, *asJSON, *events, selection, in, out)
	}
	var a *app.App
	if *backendName == "sandbox" {
		a, e = app.OpenSandboxRuntime(*data, *sandboxEnvironment, selectedRuntime)
	} else if *backendName == "tiktok" {
		if *sandboxEnvironment != "default" {
			return errors.New("sandbox-environment requires --backend sandbox")
		}
		if *tiktokAdvertiser == "" {
			return errors.New("tiktok backend requires --tiktok-advertiser")
		}
		vault, err := tiktokmapi.NewFileVault(filepath.Join(*data, "credentials"))
		if err != nil {
			return err
		}
		client, err := tiktokmapi.NewClient(tiktokmapi.Config{BaseURL: *tiktokBaseURL, AdvertiserID: *tiktokAdvertiser, Environment: *tiktokEnvironment, Tokens: vault, RevenueMetric: *tiktokRevenueMetric})
		if err != nil {
			return err
		}
		realBackend, err := tiktokmapi.NewBackend(client)
		if err != nil {
			return err
		}
		if *enableTikTokWrites {
			minBudget, minErr := decimal.NewFromString(*tiktokMinBudget)
			maxBudget, maxErr := decimal.NewFromString(*tiktokMaxBudget)
			maxDelta, deltaErr := decimal.NewFromString(*tiktokMaxDelta)
			if minErr != nil || maxErr != nil || deltaErr != nil || !minBudget.IsPositive() || !maxBudget.IsPositive() || !maxDelta.IsPositive() || minBudget.GreaterThan(maxBudget) {
				return errors.New("enabled TikTok writes require valid positive min budget, max budget, and max budget delta percent")
			}
			policy := ads.Policy{MinBudget: minBudget, MaxBudget: maxBudget, MaxDeltaPercent: maxDelta, LiveWrites: true}
			a, e = app.OpenAdBackendRuntime(*data, realBackend, policy, selectedRuntime)
		} else {
			a, e = app.OpenBackendRuntime(*data, realBackend, selectedRuntime)
		}
	} else {
		return errors.New("backend must be sandbox or tiktok")
	}
	if e != nil {
		return e
	}
	defer a.Store.Close()
	if err := a.Host.ConfigureModel(selection); err != nil {
		return err
	}
	a.Host.AutomaticMemoryCapture = *autoMemory
	encode := func(v any) error { enc := json.NewEncoder(out); enc.SetIndent("", "  "); return enc.Encode(v) }
	switch command {
	case "serve":
		host, port, err := net.SplitHostPort(*addr)
		if err != nil || port == "3000" || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
			return errors.New("serve requires a loopback IP and a port other than 3000")
		}
		origin := "http://" + *addr
		handler, err := httpapi.New(a, origin, filepath.Join(absolute, "web", "dist"))
		if err != nil {
			return err
		}
		server := &http.Server{Addr: *addr, Handler: handler.Handler(), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16384}
		fmt.Fprintf(out, "Ad Agent: %s\nThe login key is stored at %s (never send it to chat or commit it).\nSource: %s; runtime: %s; model: %s/%s; writes: %t; the main app is not connected to the port 3000 tunnel.\n", origin, filepath.Join(a.Store.Dir, "operator-key"), *backendName, *runtimeName, selection.Provider, selection.Model, a.Writable)
		go func() {
			<-ctx.Done()
			stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_ = server.Shutdown(stopCtx)
		}()
		err = server.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case "inspect":
		v, e := a.Backend.List(ctx, ads.EntityQuery{Level: ads.Level(*level), ParentID: *parent})
		if e != nil {
			return e
		}
		return encode(v)
	case "report":
		v, e := a.Backend.Report(ctx, ads.ReportQuery{Level: ads.Level(*level), Start: *start, End: *end, EntityID: *id})
		if e != nil {
			return e
		}
		return encode(v)
	case "changes":
		v, e := a.Store.Changes(ctx, *session)
		if e != nil {
			return e
		}
		return encode(v)
	case "approve":
		if *id == "" {
			return errors.New("approve requires --id for one exact change")
		}
		v, e := a.Changes.Apply(ctx, *session, *id, "local-cli-operator")
		if e != nil {
			return e
		}
		return encode(v)
	case "discard":
		if *id == "" {
			return errors.New("discard requires --id")
		}
		v, e := a.Changes.Discard(ctx, *session, *id)
		if e != nil {
			return e
		}
		return encode(v)
	case "reconcile":
		if *id == "" {
			return errors.New("reconcile requires --id")
		}
		v, e := a.Changes.Reconcile(ctx, *session, *id)
		if e != nil {
			return e
		}
		return encode(v)
	}
	chat := func(text string) error {
		var emit func(store.Event)
		if *events {
			enc := json.NewEncoder(out)
			emit = func(e store.Event) { _ = enc.Encode(e) }
		} else if !*asJSON {
			emit = func(e store.Event) {
				if e.Type == "text.delta" {
					var v struct {
						Text string `json:"text"`
					}
					if json.Unmarshal(e.Data, &v) == nil {
						fmt.Fprint(out, v.Text)
					}
				}
			}
		}
		result, err := a.Host.RunWithModel(ctx, *session, text, selection, emit)
		if *asJSON && !*events {
			if e := encode(result); e != nil {
				return e
			}
		} else if !*events {
			fmt.Fprintln(out)
			for _, c := range result.Cards {
				if c.Type == "change" && c.Change != nil {
					fmt.Fprintf(out, "Draft %s: %s (not applied)\n", c.Change.ID, c.Change.State)
				}
				if c.Type == "suggestions" {
					for _, s := range c.Suggestions {
						fmt.Fprintln(out, "·", s)
					}
				}
			}
			fmt.Fprintf(out, "[%s · %d ms · %s]\n", result.Status, result.ElapsedMS, *backendName)
		}
		return err
	}
	if *message != "" {
		return chat(*message)
	}
	if !*asJSON && !*events {
		fmt.Fprintf(out, "Ad Agent / %s / %s / %s\nEnter /exit to quit. Approval requires a separate approve --id command.\n", *backendName, *runtimeName, selection.Model)
	}
	scan := bufio.NewScanner(in)
	scan.Buffer(make([]byte, 4096), 16001)
	for {
		if !*asJSON && !*events {
			fmt.Fprint(out, "you > ")
		}
		if !scan.Scan() {
			break
		}
		text := strings.TrimSpace(scan.Text())
		if text == "/exit" {
			break
		}
		if text == "" {
			continue
		}
		if e := chat(text); e != nil {
			fmt.Fprintln(out, "turn failed:", e)
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
	return scan.Err()
}

func runPortfolio(ctx context.Context, command string, a *app.PortfolioApp, root, addr, session, message, start, end, id string, asJSON, events bool, selection ar.ModelSelection, in io.Reader, out io.Writer) error {
	encode := func(v any) error {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(v)
	}
	switch command {
	case "serve":
		host, port, err := net.SplitHostPort(addr)
		if err != nil || port == "3000" || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
			return errors.New("serve requires a loopback IP and a port other than 3000")
		}
		origin := "http://" + addr
		handler, err := httpapi.NewPortfolio(a, origin, filepath.Join(root, "web", "dist"))
		if err != nil {
			return err
		}
		server := &http.Server{Addr: addr, Handler: handler.Handler(), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16384}
		fmt.Fprintf(out, "Ad Agent: %s\nScope: portfolio; sandbox: %s; accounts: 3; runtime: %s; model: %s/%s.\nApproval remains a separate host action. The operator key is stored at %s.\n", origin, a.Scope.ID, a.Runtime, selection.Provider, selection.Model, filepath.Join(a.Store.Dir, "operator-key"))
		go func() {
			<-ctx.Done()
			stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_ = server.Shutdown(stopCtx)
		}()
		err = server.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case "inspect":
		value, err := a.Scope.Accounts(ctx)
		if err != nil {
			return err
		}
		return encode(value)
	case "report":
		value, err := a.Scope.Performance(ctx, start, end)
		if err != nil {
			return err
		}
		return encode(value)
	case "changes":
		value, err := a.Store.Changes(ctx, session)
		if err != nil {
			return err
		}
		return encode(value)
	case "approve", "discard", "reconcile":
		if id == "" {
			return fmt.Errorf("%s requires --id", command)
		}
		var value ads.Change
		var err error
		switch command {
		case "approve":
			value, err = a.Scope.Apply(ctx, session, id, "local-cli-operator")
		case "discard":
			value, err = a.Scope.Discard(ctx, session, id)
		case "reconcile":
			value, err = a.Scope.Reconcile(ctx, session, id)
		}
		if err != nil {
			return err
		}
		return encode(value)
	}
	chat := func(text string) error {
		var emit func(store.Event)
		if events {
			enc := json.NewEncoder(out)
			emit = func(event store.Event) { _ = enc.Encode(event) }
		} else if !asJSON {
			emit = func(event store.Event) {
				if event.Type != "text.delta" {
					return
				}
				var value struct {
					Text string `json:"text"`
				}
				if json.Unmarshal(event.Data, &value) == nil {
					fmt.Fprint(out, value.Text)
				}
			}
		}
		result, err := a.Host.RunWithModel(ctx, session, text, selection, emit)
		if asJSON && !events {
			if encodeErr := encode(result); encodeErr != nil {
				return encodeErr
			}
		} else if !events {
			fmt.Fprintf(out, "\n[%s · %d ms · portfolio sandbox]\n", result.Status, result.ElapsedMS)
		}
		return err
	}
	if message != "" {
		return chat(message)
	}
	if !asJSON && !events {
		fmt.Fprintf(out, "Ad Agent / portfolio / sandbox / %s\nEnter /exit to quit. Every advertiser change requires separate approval.\n", selection.Model)
	}
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 4096), 16001)
	for {
		if !asJSON && !events {
			fmt.Fprint(out, "you > ")
		}
		if !scanner.Scan() {
			break
		}
		text := strings.TrimSpace(scanner.Text())
		if text == "/exit" {
			break
		}
		if text == "" {
			continue
		}
		if err := chat(text); err != nil {
			fmt.Fprintln(out, "turn failed:", err)
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
	return scanner.Err()
}
