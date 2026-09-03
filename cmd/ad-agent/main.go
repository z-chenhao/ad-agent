package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/z-chenhao/ad-agent/internal/ads"
	"github.com/z-chenhao/ad-agent/internal/app"
	"github.com/z-chenhao/ad-agent/internal/httpapi"
	"github.com/z-chenhao/ad-agent/internal/oauthcallback"
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
		fmt.Fprintln(os.Stderr, "错误:", err)
		os.Exit(1)
	}
}
func run(ctx context.Context, args []string, in io.Reader, out io.Writer) error {
	if len(args) == 0 {
		fmt.Fprintln(out, "Ad Agent — 本地广告分析与审批\n命令: chat, inspect, report, changes, approve, discard\n默认数据源: fixture（虚构数据）；不会连接或修改真实 TikTok 广告。\n示例: ad-agent chat --message '过去 7 天哪个 campaign 拉低 ROAS？与前 7 天比较。'\n各命令使用 --help 查看选项。")
		return nil
	}
	command := args[0]
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(out)
	root := fs.String("root", ".", "项目根目录，用于定位已构建的 Pi bridge")
	data := fs.String("data-dir", ".data", "本机私有状态目录（0700）")
	session := fs.String("session", "local", "会话 ID")
	message := fs.String("message", "", "单轮消息；省略时进入交互模式")
	asJSON := fs.Bool("json", false, "输出结构化 JSON")
	events := fs.Bool("events", false, "输出公开事件 NDJSON")
	level := fs.String("level", "campaign", "campaign / ad_group / ad / advertiser（仅报表）")
	parent := fs.String("parent", "", "父对象 ID")
	start := fs.String("start", "2022-07-11", "起始日期，包含当天")
	end := fs.String("end", "2022-07-17", "结束日期，包含当天")
	id := fs.String("id", "", "准确的对象或草案 ID")
	addr := fs.String("addr", "127.0.0.1:8080", "Web 仅允许 loopback 地址；3000 保留给隔离回调")
	backendName := fs.String("backend", "fixture", "fixture 或 tiktok；tiktok 仅只读")
	tiktokAdvertiser := fs.String("tiktok-advertiser", "", "绑定的 TikTok advertiser ID")
	tiktokEnvironment := fs.String("tiktok-environment", "sandbox", "TikTok sandbox 或 live")
	tiktokBaseURL := fs.String("tiktok-base-url", tiktokmapi.DefaultBaseURL, "TikTok MAPI HTTPS base URL")
	tiktokRevenueMetric := fs.String("tiktok-revenue-metric", "", "已核对的 revenue metric；默认留空并禁用真实 ROAS")
	redirectURL := fs.String("redirect-url", "", "已登记的准确 HTTPS 或 localhost HTTP 根路径回调 URL")
	authorizationURL := fs.String("authorization-url", "", "从 TikTok My Apps 复制的 Advertiser authorization URL")
	if e := fs.Parse(args[1:]); e != nil {
		if errors.Is(e, flag.ErrHelp) {
			return nil
		}
		return e
	}
	if fs.NArg() != 0 {
		return errors.New("unexpected positional arguments")
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
		fmt.Fprintln(out, "TikTok callback-only listener: http://127.0.0.1:3000/\n这里只接收一次性 OAuth 回调；不会提供管理页面。")
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
	var a *app.App
	if *backendName == "fixture" {
		a, e = app.Open(absolute, *data)
	} else if *backendName == "tiktok" {
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
		a, e = app.OpenBackend(absolute, *data, realBackend)
	} else {
		return errors.New("backend must be fixture or tiktok")
	}
	if e != nil {
		return e
	}
	defer a.Store.Close()
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
		fmt.Fprintf(out, "Ad Agent: %s\n登录密钥保存在本机文件 %s（不要发到聊天或提交 Git）\n数据源：%s；主应用未连接 3000 隧道。\n", origin, filepath.Join(a.Store.Dir, "operator-key"), *backendName)
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
		result, err := a.Host.Run(ctx, *session, text, emit)
		if *asJSON && !*events {
			if e := encode(result); e != nil {
				return e
			}
		} else if !*events {
			fmt.Fprintln(out)
			for _, c := range result.Cards {
				if c.Type == "change" && c.Change != nil {
					fmt.Fprintf(out, "草案 %s：%s（尚未执行）\n", c.Change.ID, c.Change.State)
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
		fmt.Fprintf(out, "Ad Agent / %s / Pi + Luna\n输入 /exit 退出；审批需独立 approve --id 命令。\n", *backendName)
	}
	scan := bufio.NewScanner(in)
	scan.Buffer(make([]byte, 4096), 16001)
	for {
		if !*asJSON && !*events {
			fmt.Fprint(out, "你 > ")
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
			fmt.Fprintln(out, "本轮失败:", e)
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
	return scan.Err()
}
