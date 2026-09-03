package tiktokmapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOAuthExchangeUsesJSONAndReturnsBoundCandidates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/open_api/v1.3/oauth2/access_token/" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Access-Token") != "" {
			t.Fatal("OAuth exchange must not use Access-Token")
		}
		var body map[string]string
		if json.NewDecoder(r.Body).Decode(&body) != nil {
			t.Fatal("invalid request JSON")
		}
		if body["app_id"] != "app-1" || body["secret"] != "secret-1" || body["auth_code"] != "code-1" {
			t.Fatalf("body=%v", body)
		}
		writeEnvelope(t, w, 200, 0, "oauth-1", map[string]any{"access_token": testToken, "advertiser_ids": []string{"adv-1"}, "scope": []int64{4}})
	}))
	defer server.Close()
	c, err := NewOAuthClient(OAuthConfig{BaseURL: server.URL, AppID: "app-1", AppSecret: "secret-1", HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	token, err := c.Exchange(context.Background(), "code-1")
	if err != nil || token.AccessToken != testToken || len(token.AdvertiserIDs) != 1 {
		t.Fatalf("token=%#v err=%v", token, err)
	}
}

func TestOAuthErrorDoesNotLeakCredentialsOrCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { writeEnvelope(t, w, 200, 40001, "secret-1-code-1", nil) }))
	defer server.Close()
	c, err := NewOAuthClient(OAuthConfig{BaseURL: server.URL, AppID: "app-1", AppSecret: "secret-1", HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.Exchange(context.Background(), "code-1")
	if err == nil || strings.Contains(err.Error(), "secret-1") || strings.Contains(err.Error(), "code-1") {
		t.Fatalf("unsafe error: %v", err)
	}
}
