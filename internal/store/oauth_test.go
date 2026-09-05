package store

import (
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"testing"
	"time"
)

func TestOAuthStateIsHashedOneTimeAndBound(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	state, err := s.BeginOAuth(context.Background(), "tiktok-primary", "https://example.test/", 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	var stored []byte
	if err = s.db.QueryRow("SELECT state_hash FROM oauth_states").Scan(&stored); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256([]byte(state))
	if string(stored) != string(hash[:]) || string(stored) == state {
		t.Fatal("state must be stored only as SHA-256 hash")
	}
	intent, err := s.ConsumeOAuth(context.Background(), state, time.Now())
	if err != nil || intent.ConnectionID != "tiktok-primary" || intent.RedirectURL != "https://example.test/" {
		t.Fatalf("intent=%#v err=%v", intent, err)
	}
	if _, err = s.ConsumeOAuth(context.Background(), state, time.Now()); !errors.Is(err, ErrOAuthState) {
		t.Fatalf("replay err=%v", err)
	}
}

func TestOAuthStateRejectsExpiryAndUnsafeRedirect(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err = s.BeginOAuth(context.Background(), "x", "http://example.test/callback", time.Minute); err == nil {
		t.Fatal("expected HTTPS redirect rejection")
	}
	if _, err = s.BeginOAuth(context.Background(), "x", "http://localhost:3000/", time.Minute); err != nil {
		t.Fatalf("officially supported localhost redirect rejected: %v", err)
	}
	state, err := s.BeginOAuth(context.Background(), "x", "https://example.test/", time.Nanosecond)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.ConsumeOAuth(context.Background(), state, time.Now().Add(time.Second)); !errors.Is(err, ErrOAuthState) {
		t.Fatalf("expiry err=%v", err)
	}
	if _, err = s.ConsumeOAuth(context.Background(), state, time.Now()); !errors.Is(err, ErrOAuthState) {
		t.Fatalf("expired state must also be consumed: %v", err)
	}
}
