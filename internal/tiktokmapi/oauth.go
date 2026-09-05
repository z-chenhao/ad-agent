package tiktokmapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type OAuthConfig struct {
	BaseURL    string
	AppID      string
	AppSecret  string
	HTTPClient *http.Client
}

type OAuthClient struct {
	base   *url.URL
	appID  string
	secret string
	http   *http.Client
}

type OAuthToken struct {
	AccessToken   string
	AdvertiserIDs []string
	Scope         []int64
}

func NewOAuthClient(cfg OAuthConfig) (*OAuthClient, error) {
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultBaseURL
	}
	base, err := url.Parse(cfg.BaseURL)
	if err != nil || !validBaseURL(base) {
		return nil, errors.New("invalid TikTok OAuth base URL")
	}
	if cfg.AppID == "" || cfg.AppSecret == "" || strings.ContainsAny(cfg.AppID+cfg.AppSecret, "\r\n") {
		return nil, errors.New("TikTok app credentials are required")
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 20 * time.Second}
	}
	h := *cfg.HTTPClient
	h.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &OAuthClient{base: base, appID: cfg.AppID, secret: cfg.AppSecret, http: &h}, nil
}

func (c *OAuthClient) Exchange(ctx context.Context, authCode string) (OAuthToken, error) {
	if authCode == "" || len(authCode) > 4096 || strings.ContainsAny(authCode, "\r\n") {
		return OAuthToken{}, errors.New("invalid TikTok authorization code")
	}
	body, _ := json.Marshal(map[string]string{"app_id": c.appID, "auth_code": authCode, "secret": c.secret})
	u := *c.base
	u.Path = strings.TrimRight(c.base.Path, "/") + "/open_api/v1.3/oauth2/access_token/"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(body))
	if err != nil {
		return OAuthToken{}, errors.New("build TikTok OAuth request")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return OAuthToken{}, ctx.Err()
		}
		return OAuthToken{}, &Error{Kind: "transport"}
	}
	raw, readErr := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes+1))
	resp.Body.Close()
	if readErr != nil || len(raw) > maxBodyBytes {
		return OAuthToken{}, &Error{Kind: "invalid_response"}
	}
	var env struct {
		Code      int64  `json:"code"`
		Message   string `json:"message"`
		RequestID string `json:"request_id"`
		Data      struct {
			AccessToken   string   `json:"access_token"`
			AdvertiserIDs []string `json:"advertiser_ids"`
			Scope         []int64  `json:"scope"`
		} `json:"data"`
	}
	if json.Unmarshal(raw, &env) != nil {
		return OAuthToken{}, &Error{Kind: "invalid_response", StatusCode: resp.StatusCode}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return OAuthToken{}, &Error{Kind: "http", StatusCode: resp.StatusCode, RequestID: env.RequestID}
	}
	if env.Code != 0 {
		return OAuthToken{}, &Error{Kind: "business", Code: env.Code, RequestID: env.RequestID}
	}
	if env.Data.AccessToken == "" {
		return OAuthToken{}, &Error{Kind: "invalid_response", RequestID: env.RequestID}
	}
	return OAuthToken{AccessToken: env.Data.AccessToken, AdvertiserIDs: env.Data.AdvertiserIDs, Scope: env.Data.Scope}, nil
}
