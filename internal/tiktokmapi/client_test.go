package tiktokmapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const testToken = "private-token-never-print"

func testClient(t *testing.T, server *httptest.Server, maxPages int) *Client {
	t.Helper()
	c, err := NewClient(Config{
		BaseURL: server.URL, AdvertiserID: "adv-1", Environment: "sandbox",
		HTTPClient: server.Client(), MaxPages: maxPages,
		Tokens: TokenResolverFunc(func(context.Context, string) (string, error) { return testToken, nil }),
	})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func writeEnvelope(t *testing.T, w http.ResponseWriter, status int, code int64, requestID string, data any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]any{"code": code, "message": "upstream message", "request_id": requestID, "data": data}); err != nil {
		t.Fatal(err)
	}
}

func TestClientEncodesOfficialJSONQueryAndHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/open_api/v1.3/campaign/get/" {
			t.Errorf("path=%s", r.URL.Path)
		}
		if got := r.Header.Get("Access-Token"); got != testToken {
			t.Errorf("token=%q", got)
		}
		if got := r.URL.Query().Get("fields"); got != `["campaign_id","campaign_name"]` {
			t.Errorf("fields=%q", got)
		}
		writeEnvelope(t, w, 200, 0, "req-1", map[string]any{"list": []any{}})
	}))
	defer server.Close()
	c := testClient(t, server, 1)
	fields, _ := jsonQuery([]string{"campaign_id", "campaign_name"})
	var out map[string]any
	rid, err := c.get(context.Background(), "/open_api/v1.3/campaign/get/", url.Values{"fields": {fields}}, &out)
	if err != nil || rid != "req-1" {
		t.Fatalf("rid=%q err=%v", rid, err)
	}
}

func TestClientRetriesReadsButNeverWrites(t *testing.T) {
	var gets, posts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			n := gets.Add(1)
			if n < 3 {
				writeEnvelope(t, w, 429, 0, "rate", nil)
				return
			}
			writeEnvelope(t, w, 200, 0, "ok", map[string]any{})
			return
		}
		posts.Add(1)
		writeEnvelope(t, w, 503, 0, "post-failed", nil)
	}))
	defer server.Close()
	c := testClient(t, server, 1)
	if _, err := c.get(context.Background(), "/open_api/v1.3/campaign/get/", url.Values{}, &map[string]any{}); err != nil {
		t.Fatal(err)
	}
	if gets.Load() != 3 {
		t.Fatalf("GET attempts=%d", gets.Load())
	}
	if _, err := c.post(context.Background(), "/open_api/v1.3/campaign/update/", map[string]string{"campaign_id": "1"}, nil); err == nil {
		t.Fatal("expected POST error")
	}
	if posts.Load() != 1 {
		t.Fatalf("POST attempts=%d", posts.Load())
	}
}

func TestClientBusinessErrorIsTypedAndRedacted(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeEnvelope(t, w, 200, 40100, testToken, nil)
	}))
	defer server.Close()
	c := testClient(t, server, 1)
	_, err := c.get(context.Background(), "/open_api/v1.3/campaign/get/", url.Values{}, nil)
	var apiErr *Error
	if !errors.As(err, &apiErr) || apiErr.Code != 40100 {
		t.Fatalf("err=%#v", err)
	}
	if strings.Contains(err.Error(), testToken) || strings.Contains(err.Error(), "upstream message") {
		t.Fatalf("secret or upstream message leaked: %v", err)
	}
}

func TestClientHonorsCancellationAndBodyLimit(t *testing.T) {
	t.Run("cancellation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
		defer server.Close()
		c := testClient(t, server, 1)
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		_, err := c.get(ctx, "/open_api/v1.3/campaign/get/", url.Values{}, nil)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("body limit", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, strings.Repeat("x", maxBodyBytes+1))
		}))
		defer server.Close()
		c := testClient(t, server, 1)
		_, err := c.get(context.Background(), "/open_api/v1.3/campaign/get/", url.Values{}, nil)
		if err == nil || !strings.Contains(err.Error(), "response_too_large") {
			t.Fatalf("err=%v", err)
		}
	})
}

func TestClientRejectsUnsafeConfiguration(t *testing.T) {
	_, err := NewClient(Config{BaseURL: "http://example.com", AdvertiserID: "1", Environment: "live", Tokens: TokenResolverFunc(func(context.Context, string) (string, error) { return "x", nil })})
	if err == nil {
		t.Fatal("expected HTTPS validation error")
	}
	_, err = NewClient(Config{BaseURL: "https://example.com", AdvertiserID: "1", Environment: "live", Tokens: TokenResolverFunc(func(context.Context, string) (string, error) { return "x", nil })})
	if err == nil {
		t.Fatal("expected non-TikTok host rejection")
	}
	_, err = NewClient(Config{AdvertiserID: "1", Environment: "live", Tokens: TokenResolverFunc(func(context.Context, string) (string, error) { return "", errors.New("missing") })})
	if err != nil {
		t.Fatalf("resolver failure should occur at request time: %v", err)
	}
}
