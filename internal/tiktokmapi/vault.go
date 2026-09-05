package tiktokmapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileVault is the initial single-user credential store. Protection is the local
// OS account plus directory mode 0700 and file mode 0600; it is not a multi-user vault.
type FileVault struct{ dir string }

type tokenRecord struct {
	ConnectionID string  `json:"connection_id"`
	AdvertiserID string  `json:"advertiser_id"`
	AccessToken  string  `json:"access_token"`
	Scope        []int64 `json:"scope,omitempty"`
}

func NewFileVault(dir string) (*FileVault, error) {
	absolute, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(absolute, 0700); err != nil {
		return nil, err
	}
	info, err := os.Lstat(absolute)
	if err != nil || !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return nil, errors.New("TikTok credential directory must have mode 0700")
	}
	return &FileVault{dir: absolute}, nil
}

func (v *FileVault) Store(ctx context.Context, connectionID string, token OAuthToken) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if connectionID == "" || token.AccessToken == "" || len(token.AdvertiserIDs) == 0 || strings.ContainsAny(token.AccessToken, "\r\n") {
		return errors.New("invalid TikTok OAuth token record")
	}
	seen := map[string]bool{}
	for _, advertiserID := range token.AdvertiserIDs {
		if advertiserID == "" || strings.ContainsAny(advertiserID, "\r\n") || seen[advertiserID] {
			return errors.New("invalid TikTok advertiser authorization")
		}
		seen[advertiserID] = true
	}
	for _, advertiserID := range token.AdvertiserIDs {
		record := tokenRecord{ConnectionID: connectionID, AdvertiserID: advertiserID, AccessToken: token.AccessToken, Scope: token.Scope}
		encoded, err := json.Marshal(record)
		if err != nil {
			return err
		}
		if err = v.atomicWrite(v.path(advertiserID), encoded); err != nil {
			return err
		}
	}
	return nil
}

func (v *FileVault) Resolve(ctx context.Context, advertiserID string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	path := v.path(advertiserID)
	info, err := os.Lstat(path)
	if err != nil {
		return "", errors.New("TikTok credential is not available")
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		return "", errors.New("TikTok credential file must be a regular file with mode 0600")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", errors.New("read TikTok credential")
	}
	var record tokenRecord
	if json.Unmarshal(raw, &record) != nil || record.AdvertiserID != advertiserID || record.AccessToken == "" {
		return "", errors.New("invalid TikTok credential record")
	}
	return record.AccessToken, nil
}

func (v *FileVault) path(advertiserID string) string {
	hash := sha256.Sum256([]byte(advertiserID))
	return filepath.Join(v.dir, "tiktok-"+hex.EncodeToString(hash[:16])+".json")
}

func (v *FileVault) atomicWrite(path string, data []byte) error {
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return errors.New("create credential temp name")
	}
	tmp := filepath.Join(v.dir, fmt.Sprintf(".token-%x.tmp", random[:]))
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return errors.New("create TikTok credential")
	}
	remove := true
	defer func() {
		if remove {
			_ = os.Remove(tmp)
		}
	}()
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return errors.New("write TikTok credential")
	}
	if closeErr != nil {
		return errors.New("close TikTok credential")
	}
	if err = os.Rename(tmp, path); err != nil {
		return errors.New("replace TikTok credential")
	}
	remove = false
	return nil
}
