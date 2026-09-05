package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/z-chenhao/ad-agent/internal/ads"
)

// Workspace settings contain configuration and operator skill text, never keys.
func (s *Store) WorkspaceSettings(ctx context.Context) (json.RawMessage, error) {
	var data []byte
	err := s.db.QueryRowContext(ctx, "SELECT payload FROM workspace_settings WHERE id=1").Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return data, err
}

func (s *Store) BudgetPolicy(ctx context.Context, source ads.Source, fallback ads.Policy) (ads.Policy, error) {
	identity, _ := json.Marshal(source)
	var payload []byte
	err := s.db.QueryRowContext(ctx, "SELECT payload FROM budget_policies WHERE source=?", string(identity)).Scan(&payload)
	if errors.Is(err, sql.ErrNoRows) {
		return fallback, nil
	}
	if err != nil {
		return fallback, err
	}
	var saved ads.Policy
	if err = json.Unmarshal(payload, &saved); err != nil {
		return fallback, err
	}
	saved.LiveWrites = fallback.LiveWrites
	return saved, nil
}

func (s *Store) SaveWorkspaceConfiguration(ctx context.Context, data json.RawMessage, source ads.Source, policy ads.Policy) error {
	if !json.Valid(data) || len(data) > 1000000 {
		return errors.New("invalid_workspace_settings")
	}
	identity, _ := json.Marshal(source)
	policy.LiveWrites = false
	budget, _ := json.Marshal(policy)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "INSERT INTO workspace_settings(id,payload) VALUES(1,?) ON CONFLICT(id) DO UPDATE SET payload=excluded.payload", []byte(data)); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO budget_policies(source,payload) VALUES(?,?) ON CONFLICT(source) DO UPDATE SET payload=excluded.payload", string(identity), budget); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) SaveWorkspaceSettings(ctx context.Context, data json.RawMessage) error {
	if !json.Valid(data) || len(data) > 1000000 {
		return errors.New("invalid_workspace_settings")
	}
	_, err := s.db.ExecContext(ctx, "INSERT INTO workspace_settings(id,payload) VALUES(1,?) ON CONFLICT(id) DO UPDATE SET payload=excluded.payload", []byte(data))
	return err
}
