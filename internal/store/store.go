// Package store persists public host state separately from private runtime checkpoints.
package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/z-chenhao/ad-agent/internal/ads"
	ar "github.com/z-chenhao/ad-agent/internal/runtime"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
	"time"
)

func ID(prefix string) string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return prefix + "_" + hex.EncodeToString(b[:])
}

type Message struct {
	Role   string `json:"role"`
	Text   string `json:"text"`
	TurnID string `json:"turn_id"`
	Status string `json:"status"`
}
type Seen struct {
	Entity ads.Entity `json:"entity"`
	At     time.Time  `json:"at"`
}
type Session struct {
	ID         string            `json:"id"`
	Source     ads.Source        `json:"source"`
	Messages   []Message         `json:"messages"`
	Provenance map[string]Seen   `json:"provenance"`
	Model      ar.ModelSelection `json:"model"`
	Checkpoint string            `json:"-"`
}
type storedSession struct {
	Session
	PrivateCheckpoint string `json:"private_checkpoint,omitempty"`
}
type Event struct {
	Version string          `json:"v"`
	Type    string          `json:"type"`
	TurnID  string          `json:"turnId"`
	Seq     int64           `json:"seq"`
	At      time.Time       `json:"at"`
	Data    json.RawMessage `json:"data"`
}
type Store struct {
	db  *sql.DB
	Dir string
}

func Open(dir string) (*Store, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	// A single local operator owns these files. Fail closed on an externally writable state directory.
	info, err := os.Lstat(dir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0077 != 0 {
		return nil, errors.New("state directory must have mode 0700")
	}
	path := filepath.Join(dir, "state.db")
	if existing, statErr := os.Lstat(path); statErr == nil {
		if !existing.Mode().IsRegular() || existing.Mode()&os.ModeSymlink != 0 {
			return nil, errors.New("state database must be a regular file")
		}
	} else if !os.IsNotExist(statErr) {
		return nil, statErr
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	opened, statErr := f.Stat()
	closeErr := f.Close()
	if statErr != nil || !opened.Mode().IsRegular() {
		return nil, errors.New("state database changed during open")
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if err = os.Chmod(path, 0600); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db, Dir: dir}
	_, err = db.Exec(`PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000;
 CREATE TABLE IF NOT EXISTS sessions(id TEXT PRIMARY KEY,payload BLOB NOT NULL);
 CREATE TABLE IF NOT EXISTS leases(id TEXT PRIMARY KEY,owner TEXT NOT NULL,until_unix INTEGER NOT NULL);
 CREATE TABLE IF NOT EXISTS changes(id TEXT PRIMARY KEY,session_id TEXT NOT NULL,state TEXT NOT NULL,payload BLOB NOT NULL);
 CREATE TABLE IF NOT EXISTS events(turn_id TEXT NOT NULL,seq INTEGER NOT NULL,payload BLOB NOT NULL,PRIMARY KEY(turn_id,seq));
 CREATE TABLE IF NOT EXISTS audit(seq INTEGER PRIMARY KEY AUTOINCREMENT,change_id TEXT NOT NULL,at TEXT NOT NULL,payload BLOB NOT NULL);
 CREATE TABLE IF NOT EXISTS fixture_state(id TEXT PRIMARY KEY,payload BLOB NOT NULL);
 CREATE TABLE IF NOT EXISTS oauth_states(state_hash BLOB PRIMARY KEY,payload BLOB NOT NULL,expires_unix INTEGER NOT NULL);
 CREATE TABLE IF NOT EXISTS memories(id TEXT PRIMARY KEY,source TEXT NOT NULL,text TEXT NOT NULL,created_at TEXT NOT NULL);`)
	if err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}
func (s *Store) Close() error { return s.db.Close() }
func (s *Store) Session(ctx context.Context, id string, source ads.Source) (Session, error) {
	var raw []byte
	err := s.db.QueryRowContext(ctx, "SELECT payload FROM sessions WHERE id=?", id).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{ID: id, Source: source, Messages: []Message{}, Provenance: map[string]Seen{}}, nil
	}
	if err != nil {
		return Session{}, err
	}
	var stored storedSession
	if err = json.Unmarshal(raw, &stored); err != nil {
		return Session{}, err
	}
	v := stored.Session
	v.Checkpoint = stored.PrivateCheckpoint
	if v.Source != source {
		return Session{}, errors.New("session_source_mismatch")
	}
	return v, nil
}
func (s *Store) SaveSession(ctx context.Context, v Session) error {
	b, err := json.Marshal(storedSession{Session: v, PrivateCheckpoint: v.Checkpoint})
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, "INSERT INTO sessions(id,payload) VALUES(?,?) ON CONFLICT(id) DO UPDATE SET payload=excluded.payload", v.ID, b)
	return err
}
func (s *Store) Lease(ctx context.Context, id, owner string, until time.Time) error {
	r, err := s.db.ExecContext(ctx, `INSERT INTO leases VALUES(?,?,?) ON CONFLICT(id) DO UPDATE SET owner=excluded.owner,until_unix=excluded.until_unix WHERE leases.until_unix < ?`, id, owner, until.Unix(), time.Now().Unix())
	if err != nil {
		return err
	}
	n, _ := r.RowsAffected()
	if n != 1 {
		return errors.New("session_busy")
	}
	return nil
}
func (s *Store) Release(id, owner string) {
	_, _ = s.db.Exec("DELETE FROM leases WHERE id=? AND owner=?", id, owner)
}
func (s *Store) AddEvent(ctx context.Context, e Event) error {
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, "INSERT INTO events VALUES(?,?,?)", e.TurnID, e.Seq, b)
	return err
}
func (s *Store) Events(ctx context.Context, turn string, after int64) ([]Event, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT payload FROM events WHERE turn_id=? AND seq>? ORDER BY seq", turn, after)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Event{}
	for rows.Next() {
		var b []byte
		var e Event
		if err = rows.Scan(&b); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(b, &e); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
func (s *Store) InsertChange(ctx context.Context, c ads.Change) error {
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "INSERT INTO changes VALUES(?,?,?,?)", c.ID, c.SessionID, c.State, b); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO audit(change_id,at,payload) VALUES(?,?,?)", c.ID, time.Now().UTC().Format(time.RFC3339Nano), b); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) Change(ctx context.Context, id string) (ads.Change, error) {
	var b []byte
	err := s.db.QueryRowContext(ctx, "SELECT payload FROM changes WHERE id=?", id).Scan(&b)
	if err != nil {
		return ads.Change{}, err
	}
	var c ads.Change
	err = json.Unmarshal(b, &c)
	return c, err
}
func (s *Store) Changes(ctx context.Context, sessionID string) ([]ads.Change, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT payload FROM changes WHERE session_id=? ORDER BY rowid DESC", sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ads.Change{}
	for rows.Next() {
		var b []byte
		var c ads.Change
		if err = rows.Scan(&b); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(b, &c); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Transition atomically changes one state and appends its audit record. Approvals cannot be reused.
func (s *Store) Transition(ctx context.Context, from ads.ChangeState, c ads.Change) error {
	if !ads.CanTransition(from, c.State) {
		return errors.New("illegal_change_transition")
	}
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	r, err := tx.ExecContext(ctx, "UPDATE changes SET state=?,payload=? WHERE id=? AND state=?", c.State, b, c.ID, from)
	if err != nil {
		return err
	}
	n, _ := r.RowsAffected()
	if n != 1 {
		return errors.New("change_state_conflict")
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO audit(change_id,at,payload) VALUES(?,?,?)", c.ID, time.Now().UTC().Format(time.RFC3339Nano), b); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) FixtureEntities(ctx context.Context) ([]ads.Entity, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT payload FROM fixture_state ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ads.Entity{}
	for rows.Next() {
		var b []byte
		var e ads.Entity
		if err = rows.Scan(&b); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(b, &e); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
func (s *Store) SaveFixture(ctx context.Context, e ads.Entity) error {
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, "INSERT INTO fixture_state VALUES(?,?) ON CONFLICT(id) DO UPDATE SET payload=excluded.payload", e.ID, b)
	return err
}
