package tiktokmapi

import (
	"net/url"
	"strings"
	"testing"
)

func TestPrepareAuthorizationURLPreservesPortalParameters(t *testing.T) {
	state := strings.Repeat("s", 43)
	raw := "https://business-api.tiktok.com/open_api/v1.3/oauth2/authorize/?app_id=public-app&redirect_uri=http%3A%2F%2Flocalhost%3A3000%2F&state=portal-placeholder"
	got, err := PrepareAuthorizationURL(raw, "http://localhost:3000/", state)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(got)
	if u.Query().Get("state") != state || u.Query().Get("app_id") != "public-app" || u.Query().Get("redirect_uri") != "http://localhost:3000/" {
		t.Fatalf("unexpected URL %s", got)
	}
}

func TestPrepareAuthorizationURLRejectsUntrustedOrMismatchedURL(t *testing.T) {
	state := strings.Repeat("s", 43)
	for _, raw := range []string{
		"https://example.com/oauth?state=x",
		"http://business-api.tiktok.com/oauth?state=x",
		"https://business-api.tiktok.com/?state=x",
		"https://business-api.tiktok.com/oauth?state=one&state=two",
		"https://business-api.tiktok.com/oauth?access_token=private",
		"https://business-api.tiktok.com/oauth?redirect_uri=https%3A%2F%2Fwrong.example%2F",
	} {
		if _, err := PrepareAuthorizationURL(raw, "http://localhost:3000/", state); err == nil {
			t.Fatalf("unsafe authorization URL accepted: %s", raw)
		}
	}
}
