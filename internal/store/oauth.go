package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"net/url"
	"time"
)

type OAuthIntent struct {
	ConnectionID string    `json:"connection_id"`
	RedirectURL  string    `json:"redirect_url"`
	CreatedAt    time.Time `json:"created_at"`
	ExpiresAt    time.Time `json:"expires_at"`
}

var ErrOAuthState = errors.New("invalid, expired, or already used OAuth state")

// BeginOAuth returns the only copy of a high-entropy state. SQLite stores only its hash.
func (s *Store) BeginOAuth(ctx context.Context, connectionID, redirectURL string, ttl time.Duration) (string, error) {
	if connectionID == "" || ttl <= 0 || ttl > 15*time.Minute {
		return "", errors.New("invalid OAuth intent")
	}
	u, err := url.Parse(redirectURL)
	if err != nil || !validOAuthRedirect(u) || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("OAuth redirect URL must be an exact HTTPS or loopback HTTP URL without query or fragment")
	}
	var raw [32]byte
	if _, err = rand.Read(raw[:]); err != nil {
		return "", errors.New("generate OAuth state")
	}
	state := base64.RawURLEncoding.EncodeToString(raw[:])
	hash := sha256.Sum256([]byte(state))
	now := time.Now().UTC()
	intent := OAuthIntent{ConnectionID: connectionID, RedirectURL: redirectURL, CreatedAt: now, ExpiresAt: now.Add(ttl)}
	payload, err := json.Marshal(intent)
	if err != nil {
		return "", err
	}
	_, err = s.db.ExecContext(ctx, "INSERT INTO oauth_states(state_hash,payload,expires_unix) VALUES(?,?,?)", hash[:], payload, intent.ExpiresAt.Unix())
	if err != nil {
		return "", err
	}
	return state, nil
}

func validOAuthRedirect(u *url.URL) bool {
	if u == nil || u.Host == "" || u.Path != "/" {
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

// ConsumeOAuth atomically deletes a state before any token exchange, preventing replay.
func (s *Store) ConsumeOAuth(ctx context.Context, state string, now time.Time) (OAuthIntent, error) {
	if len(state) < 40 || len(state) > 128 {
		return OAuthIntent{}, ErrOAuthState
	}
	hash := sha256.Sum256([]byte(state))
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return OAuthIntent{}, err
	}
	defer tx.Rollback()
	var payload []byte
	var expires int64
	err = tx.QueryRowContext(ctx, "SELECT payload,expires_unix FROM oauth_states WHERE state_hash=?", hash[:]).Scan(&payload, &expires)
	if errors.Is(err, sql.ErrNoRows) {
		return OAuthIntent{}, ErrOAuthState
	}
	if err != nil {
		return OAuthIntent{}, err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM oauth_states WHERE state_hash=?", hash[:]); err != nil {
		return OAuthIntent{}, err
	}
	if err = tx.Commit(); err != nil {
		return OAuthIntent{}, err
	}
	if !now.UTC().Before(time.Unix(expires, 0)) {
		return OAuthIntent{}, ErrOAuthState
	}
	var intent OAuthIntent
	if json.Unmarshal(payload, &intent) != nil || intent.ConnectionID == "" {
		return OAuthIntent{}, ErrOAuthState
	}
	return intent, nil
}
