package tiktokmapi

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFileVaultStoresPerAdvertiserWithStrictModes(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "tokens")
	vault, err := NewFileVault(dir)
	if err != nil {
		t.Fatal(err)
	}
	err = vault.Store(context.Background(), "primary", OAuthToken{AccessToken: testToken, AdvertiserIDs: []string{"adv-1", "adv-2"}, Scope: []int64{4}})
	if err != nil {
		t.Fatal(err)
	}
	for _, advertiser := range []string{"adv-1", "adv-2"} {
		got, resolveErr := vault.Resolve(context.Background(), advertiser)
		if resolveErr != nil || got != testToken {
			t.Fatalf("advertiser=%s token=%q err=%v", advertiser, got, resolveErr)
		}
		info, statErr := os.Stat(vault.path(advertiser))
		if statErr != nil || info.Mode().Perm() != 0600 {
			t.Fatalf("mode=%v err=%v", info.Mode(), statErr)
		}
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 2 {
		t.Fatalf("entries=%d", len(entries))
	}
}

func TestFileVaultRejectsLooseDirectoryAndSymlinkCredential(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "loose")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := NewFileVault(dir); err == nil {
		t.Fatal("expected loose mode rejection")
	}
	targetDir := filepath.Join(t.TempDir(), "target-dir")
	if err := os.Mkdir(targetDir, 0700); err != nil {
		t.Fatal(err)
	}
	symlinkDir := filepath.Join(t.TempDir(), "linked-dir")
	if err := os.Symlink(targetDir, symlinkDir); err != nil {
		t.Fatal(err)
	}
	if _, err := NewFileVault(symlinkDir); err == nil {
		t.Fatal("expected credential directory symlink rejection")
	}
	secure := filepath.Join(t.TempDir(), "secure")
	vault, err := NewFileVault(secure)
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "target")
	if err = os.WriteFile(target, []byte(`{"advertiser_id":"adv-1","access_token":"bad"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink(target, vault.path("adv-1")); err != nil {
		t.Fatal(err)
	}
	if _, err = vault.Resolve(context.Background(), "adv-1"); err == nil {
		t.Fatal("expected symlink rejection")
	}
}
