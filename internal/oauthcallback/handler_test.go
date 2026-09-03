package oauthcallback

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/z-chenhao/ad-agent/internal/store"
	"github.com/z-chenhao/ad-agent/internal/tiktokmapi"
)

type fakeStates struct {
	intent store.OAuthIntent
	err    error
	calls  int
}

func (f *fakeStates) ConsumeOAuth(context.Context, string, time.Time) (store.OAuthIntent, error) {
	f.calls++
	return f.intent, f.err
}

type fakeOAuth struct {
	token tiktokmapi.OAuthToken
	err   error
	code  string
	calls int
}

func (f *fakeOAuth) Exchange(_ context.Context, code string) (tiktokmapi.OAuthToken, error) {
	f.calls++
	f.code = code
	return f.token, f.err
}

type fakeSink struct {
	connection string
	token      tiktokmapi.OAuthToken
	err        error
	calls      int
}

func (f *fakeSink) Store(_ context.Context, connection string, token tiktokmapi.OAuthToken) error {
	f.calls++
	f.connection, f.token = connection, token
	return f.err
}

func TestCallbackConsumesStateExchangesAndStoresWithoutEcho(t *testing.T) {
	states := &fakeStates{intent: store.OAuthIntent{ConnectionID: "primary", RedirectURL: "https://example.test/"}}
	oauth := &fakeOAuth{token: tiktokmapi.OAuthToken{AccessToken: "private-token", AdvertiserIDs: []string{"adv-1"}}}
	sink := &fakeSink{}
	h, err := New(states, oauth, sink, "https://example.test/")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/?state="+strings.Repeat("s", 43)+"&auth_code=private-code", nil)
	w := httptest.NewRecorder()
	h.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusOK || states.calls != 1 || oauth.calls != 1 || sink.calls != 1 || oauth.code != "private-code" || sink.connection != "primary" {
		t.Fatalf("status=%d state=%d oauth=%d sink=%d", w.Code, states.calls, oauth.calls, sink.calls)
	}
	body, _ := io.ReadAll(w.Result().Body)
	if strings.Contains(string(body), "private-code") || strings.Contains(string(body), "private-token") || w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Fatalf("unsafe response headers=%v body=%s", w.Header(), body)
	}
}

func TestCallbackRejectsWrongPathInvalidStateAndProviderError(t *testing.T) {
	tests := []struct {
		name, target string
		states       *fakeStates
		want         int
		oauthCalls   int
	}{
		{"wrong path", "/debug?state=" + strings.Repeat("s", 43) + "&auth_code=x", &fakeStates{intent: store.OAuthIntent{ConnectionID: "x", RedirectURL: "https://example.test/"}}, 404, 0},
		{"missing state", "/?auth_code=x", &fakeStates{intent: store.OAuthIntent{ConnectionID: "x", RedirectURL: "https://example.test/"}}, 400, 0},
		{"invalid state", "/?state=" + strings.Repeat("s", 43) + "&auth_code=x", &fakeStates{err: store.ErrOAuthState}, 400, 0},
		{"provider denied", "/?state=" + strings.Repeat("s", 43) + "&error=access_denied", &fakeStates{intent: store.OAuthIntent{ConnectionID: "x", RedirectURL: "https://example.test/"}}, 400, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			oauth, sink := &fakeOAuth{}, &fakeSink{}
			h, _ := New(tc.states, oauth, sink, "https://example.test/")
			w := httptest.NewRecorder()
			h.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, tc.target, nil))
			if w.Code != tc.want || oauth.calls != tc.oauthCalls || sink.calls != 0 {
				t.Fatalf("status=%d oauth=%d sink=%d", w.Code, oauth.calls, sink.calls)
			}
		})
	}
}

func TestCallbackAcceptsOfficiallySupportedLocalhostRedirect(t *testing.T) {
	states := &fakeStates{intent: store.OAuthIntent{ConnectionID: "primary", RedirectURL: "http://localhost:3000/"}}
	h, err := New(states, &fakeOAuth{token: tiktokmapi.OAuthToken{AccessToken: "x", AdvertiserIDs: []string{"1"}}}, &fakeSink{}, "http://localhost:3000/")
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	h.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/?state="+strings.Repeat("s", 43)+"&auth_code=x", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
}
