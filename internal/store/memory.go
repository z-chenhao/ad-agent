package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/z-chenhao/ad-agent/internal/ads"
)

type MemoryKind string

const (
	MemoryPreference MemoryKind = "preference"
	MemoryConstraint MemoryKind = "constraint"
	MemoryGoal       MemoryKind = "goal"
)

type Memory struct {
	ID        string     `json:"id"`
	Key       string     `json:"key,omitempty"`
	Kind      MemoryKind `json:"kind"`
	Text      string     `json:"text"`
	CreatedAt time.Time  `json:"created_at"`
}

func validMemory(key string, kind MemoryKind, text string) bool {
	return kind.valid() && text != "" && len(text) <= 500 && !strings.ContainsAny(text, "\r\n") &&
		len(key) <= 64 && !strings.ContainsAny(key, "\r\n")
}

func (k MemoryKind) valid() bool {
	return k == MemoryPreference || k == MemoryConstraint || k == MemoryGoal
}

func memorySourceKey(source ads.Source) (string, error) {
	if source.Backend == "" || source.Environment == "" || source.AccountID == "" {
		return "", errors.New("invalid_memory_source")
	}
	b, err := json.Marshal(source)
	return string(b), err
}

// SaveMemory stores only an explicitly requested, account-scoped stable fact.
// Product policy decides when to call this method; storage does not infer memories.
func (s *Store) SaveMemory(ctx context.Context, source ads.Source, kind MemoryKind, text string) (Memory, error) {
	text = strings.TrimSpace(text)
	if !validMemory("", kind, text) {
		return Memory{}, errors.New("invalid_memory")
	}
	key, err := memorySourceKey(source)
	if err != nil {
		return Memory{}, err
	}
	m := Memory{ID: ID("memory"), Kind: kind, Text: text, CreatedAt: time.Now().UTC()}
	payload, err := json.Marshal(m)
	if err != nil {
		return Memory{}, err
	}
	_, err = s.db.ExecContext(ctx,
		"INSERT INTO memories(id,source,text,created_at) VALUES(?,?,?,?)",
		m.ID, key, payload, m.CreatedAt.Format(time.RFC3339Nano))
	return m, err
}

// UpsertMemory stores an extracted durable fact under a stable, account-scoped key.
// Explicit memories remain append-only through SaveMemory; only the post-turn extractor
// uses keyed replacement so an updated standing rule does not accumulate stale copies.
func (s *Store) UpsertMemory(ctx context.Context, source ads.Source, factKey string, kind MemoryKind, text string) (Memory, error) {
	factKey = strings.ToLower(strings.TrimSpace(factKey))
	factKey = strings.Join(strings.Fields(factKey), "_")
	text = strings.TrimSpace(text)
	if factKey == "" || !validMemory(factKey, kind, text) {
		return Memory{}, errors.New("invalid_memory")
	}
	sourceKey, err := memorySourceKey(source)
	if err != nil {
		return Memory{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Memory{}, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, "SELECT id,text FROM memories WHERE source=? ORDER BY created_at DESC,id DESC", sourceKey)
	if err != nil {
		return Memory{}, err
	}
	var existingID string
	for rows.Next() {
		var id string
		var payload []byte
		var memory Memory
		if err = rows.Scan(&id, &payload); err != nil {
			rows.Close()
			return Memory{}, err
		}
		if json.Unmarshal(payload, &memory) == nil && memory.Key == factKey {
			existingID = id
			break
		}
	}
	if closeErr := rows.Close(); err == nil && closeErr != nil {
		return Memory{}, closeErr
	}
	if err = rows.Err(); err != nil {
		return Memory{}, err
	}
	memory := Memory{ID: existingID, Key: factKey, Kind: kind, Text: text, CreatedAt: time.Now().UTC()}
	if memory.ID == "" {
		memory.ID = ID("memory")
	}
	payload, err := json.Marshal(memory)
	if err != nil {
		return Memory{}, err
	}
	if existingID == "" {
		_, err = tx.ExecContext(ctx, "INSERT INTO memories(id,source,text,created_at) VALUES(?,?,?,?)", memory.ID, sourceKey, payload, memory.CreatedAt.Format(time.RFC3339Nano))
	} else {
		_, err = tx.ExecContext(ctx, "UPDATE memories SET text=?,created_at=? WHERE id=? AND source=?", payload, memory.CreatedAt.Format(time.RFC3339Nano), memory.ID, sourceKey)
	}
	if err != nil {
		return Memory{}, err
	}
	return memory, tx.Commit()
}

func (s *Store) Memories(ctx context.Context, source ads.Source, limit int) ([]Memory, error) {
	if limit < 1 || limit > 50 {
		return nil, errors.New("invalid_memory_limit")
	}
	key, err := memorySourceKey(source)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx,
		"SELECT text FROM memories WHERE source=? ORDER BY created_at DESC,id DESC LIMIT ?", key, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Memory{}
	for rows.Next() {
		var payload []byte
		var m Memory
		if err = rows.Scan(&payload); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(payload, &m); err != nil {
			return nil, errors.New("invalid_stored_memory")
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) DeleteMemory(ctx context.Context, source ads.Source, id string) (Memory, error) {
	key, err := memorySourceKey(source)
	if err != nil {
		return Memory{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Memory{}, err
	}
	defer tx.Rollback()
	var payload []byte
	if err = tx.QueryRowContext(ctx, "SELECT text FROM memories WHERE id=? AND source=?", id, key).Scan(&payload); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Memory{}, errors.New("memory_not_found")
		}
		return Memory{}, err
	}
	var m Memory
	if err = json.Unmarshal(payload, &m); err != nil {
		return Memory{}, errors.New("invalid_stored_memory")
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM memories WHERE id=? AND source=?", id, key); err != nil {
		return Memory{}, err
	}
	return m, tx.Commit()
}
