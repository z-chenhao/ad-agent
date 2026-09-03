// Package oauthcallback exposes the isolated public callback mux used by the
// HTTPS tunnel. It intentionally does not serve the local app or static files.
package oauthcallback

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/z-chenhao/ad-agent/internal/store"
	"github.com/z-chenhao/ad-agent/internal/tiktokmapi"
)

type StateConsumer interface {
	ConsumeOAuth(context.Context, string, time.Time) (store.OAuthIntent, error)
}

type Exchanger interface {
	Exchange(context.Context, string) (tiktokmapi.OAuthToken, error)
}

type TokenSink interface {
	Store(context.Context, string, tiktokmapi.OAuthToken) error
}

type Handler struct {
	states      StateConsumer
	oauth       Exchanger
	sink        TokenSink
	redirectURL string
	now         func() time.Time
}

func New(states StateConsumer, oauth Exchanger, sink TokenSink, redirectURL string) (*Handler, error) {
	u, err := url.Parse(redirectURL)
	if states == nil || oauth == nil || sink == nil || err != nil || !validRedirect(u) || u.Path != "/" || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("OAuth callback dependencies are required")
	}
	return &Handler{states: states, oauth: oauth, sink: sink, redirectURL: redirectURL, now: time.Now}, nil
}

func validRedirect(u *url.URL) bool {
	if u == nil || u.Host == "" {
		return false
	}
	if u.Scheme == "https" {
		return true
	}
	if u.Scheme != "http" {
		return false
	}
	host := u.Hostname()
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (h *Handler) Handler() http.Handler {
	return http.HandlerFunc(h.serveHTTP)
}

func (h *Handler) serveHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; base-uri 'none'; frame-ancestors 'none'")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if r.Method != http.MethodGet || r.URL.Path != "/" {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(failurePage))
		return
	}
	query := r.URL.Query()
	stateValues, codeValues := query["state"], query["auth_code"]
	if len(stateValues) != 1 || len(codeValues) > 1 || len(stateValues[0]) > 128 {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(failurePage))
		return
	}
	intent, err := h.states.ConsumeOAuth(r.Context(), stateValues[0], h.now())
	if err != nil || intent.RedirectURL != h.redirectURL {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(failurePage))
		return
	}
	if query.Get("error") != "" || len(codeValues) != 1 || codeValues[0] == "" {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(failurePage))
		return
	}
	token, err := h.oauth.Exchange(r.Context(), codeValues[0])
	if err != nil || h.sink.Store(r.Context(), intent.ConnectionID, token) != nil {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(failurePage))
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(successPage))
}

const successPage = `<!doctype html><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>授权完成</title><style>body{font:16px system-ui;margin:3rem;max-width:36rem}h1{font-size:1.5rem}</style><h1>TikTok 授权已保存</h1><p>请关闭此页面，回到本机 Ad Agent 继续。</p>`
const failurePage = `<!doctype html><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>授权未完成</title><style>body{font:16px system-ui;margin:3rem;max-width:36rem}h1{font-size:1.5rem}</style><h1>TikTok 授权未完成</h1><p>请关闭此页面，并从本机 Ad Agent 重新开始连接。</p>`
