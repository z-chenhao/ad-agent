// Package tiktokmapi implements the private TikTok Marketing API wire adapter.
// Credentials and raw HTTP details do not cross this package boundary.
package tiktokmapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultBaseURL = "https://business-api.tiktok.com"
	maxBodyBytes   = 2 << 20
)

type TokenResolver interface {
	Resolve(context.Context, string) (string, error)
}

type TokenResolverFunc func(context.Context, string) (string, error)

func (f TokenResolverFunc) Resolve(ctx context.Context, advertiserID string) (string, error) {
	return f(ctx, advertiserID)
}

type Config struct {
	BaseURL          string
	AdvertiserID     string
	Environment      string
	HTTPClient       *http.Client
	Tokens           TokenResolver
	MaxPages         int
	RevenueMetric    string
	BusinessCenterID string
}

type Client struct {
	base             *url.URL
	advertiserID     string
	environment      string
	http             *http.Client
	tokens           TokenResolver
	maxPages         int
	revenueMetric    string
	businessCenterID string
}

func NewClient(cfg Config) (*Client, error) {
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultBaseURL
	}
	base, err := url.Parse(cfg.BaseURL)
	if err != nil || !validBaseURL(base) {
		return nil, errors.New("invalid TikTok MAPI base URL")
	}
	if cfg.AdvertiserID == "" || strings.ContainsAny(cfg.AdvertiserID, "\r\n") {
		return nil, errors.New("advertiser ID is required")
	}
	if cfg.Environment != "sandbox" && cfg.Environment != "live" {
		return nil, errors.New("TikTok environment must be sandbox or live")
	}
	if cfg.Tokens == nil {
		return nil, errors.New("TikTok token resolver is required")
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 20 * time.Second}
	}
	if cfg.MaxPages == 0 {
		cfg.MaxPages = 20
	}
	if cfg.MaxPages < 1 || cfg.MaxPages > 100 {
		return nil, errors.New("TikTok max pages must be between 1 and 100")
	}
	if cfg.RevenueMetric != "" && cfg.RevenueMetric != "total_purchase_value" && cfg.RevenueMetric != "total_complete_payment_rate" && cfg.RevenueMetric != "onsite_total_purchase_value" {
		return nil, errors.New("unsupported TikTok revenue metric")
	}
	copyClient := *cfg.HTTPClient
	copyClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	if strings.ContainsAny(cfg.BusinessCenterID, "\r\n") {
		return nil, errors.New("invalid TikTok Business Center ID")
	}
	return &Client{base: base, advertiserID: cfg.AdvertiserID, environment: cfg.Environment, http: &copyClient, tokens: cfg.Tokens, maxPages: cfg.MaxPages, revenueMetric: cfg.RevenueMetric, businessCenterID: cfg.BusinessCenterID}, nil
}

func validBaseURL(base *url.URL) bool {
	if base == nil || base.Host == "" || (base.Path != "" && base.Path != "/") || base.RawQuery != "" || base.Fragment != "" || base.User != nil {
		return false
	}
	host := base.Hostname()
	if base.Scheme == "https" && host == "business-api.tiktok.com" {
		return true
	}
	ip := net.ParseIP(host)
	return (host == "localhost" || (ip != nil && ip.IsLoopback())) && (base.Scheme == "http" || base.Scheme == "https")
}

type Error struct {
	Kind       string
	StatusCode int
	Code       int64
	RequestID  string
}

func (e *Error) Error() string {
	switch e.Kind {
	case "business":
		return fmt.Sprintf("TikTok MAPI business error (code=%d, request_id=%s)", e.Code, presence(e.RequestID))
	case "http":
		return fmt.Sprintf("TikTok MAPI HTTP error (status=%d, request_id=%s)", e.StatusCode, presence(e.RequestID))
	default:
		return "TikTok MAPI " + e.Kind + " error"
	}
}

func presence(s string) string {
	if s == "" {
		return "unavailable"
	}
	return "available"
}

type envelope struct {
	Code      int64           `json:"code"`
	Message   string          `json:"message"`
	RequestID string          `json:"request_id"`
	Data      json.RawMessage `json:"data"`
}

func (c *Client) get(ctx context.Context, path string, query url.Values, out any) (string, error) {
	return c.call(ctx, http.MethodGet, path, query, nil, true, out)
}

func (c *Client) post(ctx context.Context, path string, body any, out any) (string, error) {
	return c.call(ctx, http.MethodPost, path, nil, body, false, out)
}

func (c *Client) call(ctx context.Context, method, path string, query url.Values, body any, retryRead bool, out any) (string, error) {
	if !strings.HasPrefix(path, "/open_api/v1.3/") || strings.Contains(path, "?") {
		return "", errors.New("invalid TikTok MAPI path")
	}
	token, err := c.tokens.Resolve(ctx, c.advertiserID)
	if err != nil || token == "" || strings.ContainsAny(token, "\r\n") {
		return "", &Error{Kind: "credential"}
	}
	var encoded []byte
	if body != nil {
		encoded, err = json.Marshal(body)
		if err != nil {
			return "", errors.New("encode TikTok MAPI request")
		}
	}
	attempts := 1
	if retryRead {
		attempts = 3
	}
	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 {
			d := time.Duration(50*(1<<uint(attempt-1))) * time.Millisecond
			t := time.NewTimer(d)
			select {
			case <-ctx.Done():
				t.Stop()
				return "", ctx.Err()
			case <-t.C:
			}
		}
		u := *c.base
		u.Path = strings.TrimRight(c.base.Path, "/") + path
		u.RawQuery = query.Encode()
		var reader io.Reader
		if encoded != nil {
			reader = bytes.NewReader(encoded)
		}
		req, reqErr := http.NewRequestWithContext(ctx, method, u.String(), reader)
		if reqErr != nil {
			return "", errors.New("build TikTok MAPI request")
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Access-Token", token)
		if encoded != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, doErr := c.http.Do(req)
		if doErr != nil {
			if ctx.Err() != nil {
				return "", ctx.Err()
			}
			if retryRead && attempt+1 < attempts {
				continue
			}
			return "", &Error{Kind: "transport"}
		}
		limited := io.LimitReader(resp.Body, maxBodyBytes+1)
		raw, readErr := io.ReadAll(limited)
		resp.Body.Close()
		if readErr != nil {
			return "", &Error{Kind: "response"}
		}
		if len(raw) > maxBodyBytes {
			return "", &Error{Kind: "response_too_large"}
		}
		if (resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500) && retryRead && attempt+1 < attempts {
			continue
		}
		var env envelope
		if err = json.Unmarshal(raw, &env); err != nil {
			return "", &Error{Kind: "invalid_response", StatusCode: resp.StatusCode}
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return "", &Error{Kind: "http", StatusCode: resp.StatusCode, RequestID: env.RequestID}
		}
		if env.Code != 0 {
			return "", &Error{Kind: "business", Code: env.Code, RequestID: env.RequestID}
		}
		if out != nil && len(env.Data) > 0 && string(env.Data) != "null" {
			if err = json.Unmarshal(env.Data, out); err != nil {
				return "", &Error{Kind: "invalid_response", RequestID: env.RequestID}
			}
		}
		return env.RequestID, nil
	}
	return "", &Error{Kind: "transport"}
}

func jsonQuery(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

type pageInfo struct {
	Page        int `json:"page"`
	PageSize    int `json:"page_size"`
	TotalNumber int `json:"total_number"`
	TotalPage   int `json:"total_page"`
}

func morePages(p pageInfo, current, count, pageSize int) bool {
	if p.TotalPage > 0 {
		return current < p.TotalPage
	}
	if p.TotalNumber > 0 {
		return current*pageSize < p.TotalNumber
	}
	return count == pageSize
}

func putInt(q url.Values, key string, value int) { q.Set(key, strconv.Itoa(value)) }
