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
	ID                string            `json:"id"`
	Source            ads.Source        `json:"source"`
	Messages          []Message         `json:"messages"`
	Provenance        map[string]Seen   `json:"provenance"`
	Model             ar.ModelSelection `json:"model"`
	Runtime           string            `json:"runtime"`
	Checkpoint        string            `json:"-"`
	ExecutionContract string            `json:"-"`
}
type storedSession struct {
	Session
	PrivateCheckpoint        string `json:"private_checkpoint,omitempty"`
	PrivateExecutionContract string `json:"private_execution_contract,omitempty"`
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
 CREATE TABLE IF NOT EXISTS workspace_settings(id INTEGER PRIMARY KEY CHECK(id=1),payload BLOB NOT NULL);
 CREATE TABLE IF NOT EXISTS budget_policies(source TEXT PRIMARY KEY,payload BLOB NOT NULL);
 CREATE TABLE IF NOT EXISTS leases(id TEXT PRIMARY KEY,owner TEXT NOT NULL,until_unix INTEGER NOT NULL);
 CREATE TABLE IF NOT EXISTS changes(id TEXT PRIMARY KEY,session_id TEXT NOT NULL,state TEXT NOT NULL,payload BLOB NOT NULL);
 CREATE TABLE IF NOT EXISTS events(turn_id TEXT NOT NULL,seq INTEGER NOT NULL,payload BLOB NOT NULL,PRIMARY KEY(turn_id,seq));
 CREATE TABLE IF NOT EXISTS audit(seq INTEGER PRIMARY KEY AUTOINCREMENT,change_id TEXT NOT NULL,at TEXT NOT NULL,payload BLOB NOT NULL);
 CREATE TABLE IF NOT EXISTS sandbox_state(scenario TEXT NOT NULL,id TEXT NOT NULL,payload BLOB NOT NULL,PRIMARY KEY(scenario,id));
 CREATE TABLE IF NOT EXISTS sandbox_entities(environment TEXT NOT NULL,id TEXT NOT NULL,payload BLOB NOT NULL,PRIMARY KEY(environment,id));
 INSERT OR IGNORE INTO sandbox_entities SELECT scenario,id,payload FROM sandbox_state;
 CREATE TABLE IF NOT EXISTS sandbox_clock(environment TEXT PRIMARY KEY,current_time TEXT NOT NULL,payload BLOB NOT NULL);
 CREATE TABLE IF NOT EXISTS sandbox_hour_facts(environment TEXT NOT NULL,ad_id TEXT NOT NULL,hour TEXT NOT NULL,payload BLOB NOT NULL,PRIMARY KEY(environment,ad_id,hour));
 CREATE TABLE IF NOT EXISTS sandbox_operation_state(environment TEXT PRIMARY KEY,payload BLOB NOT NULL);
 CREATE TABLE IF NOT EXISTS oauth_states(state_hash BLOB PRIMARY KEY,payload BLOB NOT NULL,expires_unix INTEGER NOT NULL);
 CREATE TABLE IF NOT EXISTS memories(id TEXT PRIMARY KEY,source TEXT NOT NULL,text TEXT NOT NULL,created_at TEXT NOT NULL);`)
	if err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) SandboxOperationState(ctx context.Context, environment string) ([]byte, error) {
	var payload []byte
	err := s.db.QueryRowContext(ctx, "SELECT payload FROM sandbox_operation_state WHERE environment=?", environment).Scan(&payload)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return payload, err
}

func (s *Store) SaveSandboxOperationState(ctx context.Context, environment string, payload []byte) error {
	if len(payload) == 0 {
		return errors.New("empty sandbox operation state")
	}
	_, err := s.db.ExecContext(ctx, "INSERT INTO sandbox_operation_state(environment,payload) VALUES(?,?) ON CONFLICT(environment) DO UPDATE SET payload=excluded.payload", environment, payload)
	return err
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
	v.ExecutionContract = stored.PrivateExecutionContract
	if v.Source != source {
		return Session{}, errors.New("session_source_mismatch")
	}
	return v, nil
}
func (s *Store) SaveSession(ctx context.Context, v Session) error {
	b, err := json.Marshal(storedSession{Session: v, PrivateCheckpoint: v.Checkpoint, PrivateExecutionContract: v.ExecutionContract})
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
	if err == nil {
		// Event contents stay in authenticated state storage; the trace index is safe metadata.
		var fields struct {
			Tool       string `json:"name"`
			CallID     string `json:"id"`
			Success    *bool  `json:"ok"`
			Error      string `json:"error"`
			DurationMS int64  `json:"duration_ms"`
		}
		if e.Type == "tool.started" || e.Type == "tool.finished" {
			_ = json.Unmarshal(e.Data, &fields)
		}
		entry := Diagnostic{Type: e.Type, RequestID: RequestID(ctx), TurnID: e.TurnID, Sequence: e.Seq, Bytes: len(e.Data), Tool: fields.Tool, CallID: fields.CallID, Success: fields.Success}
		entry.DurationMS = fields.DurationMS
		if e.Type == "turn.started" {
			var started struct {
				Runtime string `json:"runtime"`
			}
			if json.Unmarshal(e.Data, &started) == nil {
				switch started.Runtime {
				case "builtin", "pi", "codex", "claude":
					entry.Runtime = started.Runtime
				}
			}
		}
		if fields.Success != nil && !*fields.Success {
			entry.ErrorCode = diagnosticErrorCode(fields.Error)
		}
		if e.Type == "turn.completed" {
			var result struct {
				Status    string `json:"status"`
				ElapsedMS int64  `json:"elapsed_ms"`
				ErrorCode string `json:"error_code"`
			}
			if json.Unmarshal(e.Data, &result) == nil {
				switch result.Status {
				case "completed", "failed", "cancelled", "budget_exhausted":
					entry.Outcome = result.Status
				}
				entry.DurationMS = result.ElapsedMS
				if result.Status != "completed" && result.ErrorCode != "" {
					entry.ErrorCode = ar.SafeFailureCode(result.ErrorCode)
				}
			}
		}
		if logErr := s.RecordDiagnostic("agent-trace", entry); logErr != nil {
			// Diagnostic failure must not transform a committed event into an unsent event.
			_, _ = os.Stderr.WriteString("agent diagnostic log unavailable\n")
		}
	}
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
func (s *Store) SandboxEntities(ctx context.Context, environment string) ([]ads.Entity, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT payload FROM sandbox_entities WHERE environment=? ORDER BY id", environment)
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
func (s *Store) SaveSandbox(ctx context.Context, environment string, e ads.Entity) error {
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, "INSERT INTO sandbox_entities VALUES(?,?,?) ON CONFLICT(environment,id) DO UPDATE SET payload=excluded.payload", environment, e.ID, b)
	return err
}

type SandboxFactPayload struct {
	AdID    string
	Hour    time.Time
	Payload []byte
}

func (s *Store) SandboxSimulation(ctx context.Context, environment string) ([]byte, [][]byte, error) {
	var raw []byte
	err := s.db.QueryRowContext(ctx, "SELECT payload FROM sandbox_clock WHERE environment=?", environment).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		raw = nil
	} else if err != nil {
		return nil, nil, err
	}
	rows, err := s.db.QueryContext(ctx, "SELECT payload FROM sandbox_hour_facts WHERE environment=? ORDER BY hour,ad_id", environment)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	facts := [][]byte{}
	for rows.Next() {
		var payload []byte
		if err = rows.Scan(&payload); err != nil {
			return nil, nil, err
		}
		facts = append(facts, payload)
	}
	return raw, facts, rows.Err()
}

// SaveSandboxAdvance atomically appends immutable facts and compare-and-swaps the clock.
type SandboxAdvanceResources struct {
	Environment string
	Entities    []ads.Entity
	Operations  []byte
}

func (s *Store) SaveSandboxAdvance(ctx context.Context, environment string, previous, current time.Time, statePayload []byte, facts []SandboxFactPayload, resources ...SandboxAdvanceResources) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var persisted string
	err = tx.QueryRowContext(ctx, `SELECT "current_time" FROM sandbox_clock WHERE environment=?`, environment).Scan(&persisted)
	if errors.Is(err, sql.ErrNoRows) {
		if _, err = tx.ExecContext(ctx, `INSERT INTO sandbox_clock(environment,"current_time",payload) VALUES(?,?,?)`, environment, current.UTC().Format(time.RFC3339), statePayload); err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		if persisted != previous.UTC().Format(time.RFC3339) {
			return errors.New("sandbox_clock_conflict")
		}
		if _, err = tx.ExecContext(ctx, `UPDATE sandbox_clock SET "current_time"=?,payload=? WHERE environment=? AND "current_time"=?`, current.UTC().Format(time.RFC3339), statePayload, environment, persisted); err != nil {
			return err
		}
	}
	for _, fact := range facts {
		if _, err = tx.ExecContext(ctx, "INSERT INTO sandbox_hour_facts(environment,ad_id,hour,payload) VALUES(?,?,?,?)", environment, fact.AdID, fact.Hour.UTC().Format(time.RFC3339), fact.Payload); err != nil {
			return err
		}
	}
	for _, snapshot := range resources {
		if err = saveSandboxResources(ctx, tx, snapshot); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) SaveSandboxResources(ctx context.Context, snapshot SandboxAdvanceResources) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := saveSandboxResources(ctx, tx, snapshot); err != nil {
		return err
	}
	return tx.Commit()
}

func saveSandboxResources(ctx context.Context, tx *sql.Tx, snapshot SandboxAdvanceResources) error {
	for _, entity := range snapshot.Entities {
		payload, err := json.Marshal(entity)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO sandbox_entities VALUES(?,?,?) ON CONFLICT(environment,id) DO UPDATE SET payload=excluded.payload", snapshot.Environment, entity.ID, payload); err != nil {
			return err
		}
	}
	_, err := tx.ExecContext(ctx, "INSERT INTO sandbox_operation_state(environment,payload) VALUES(?,?) ON CONFLICT(environment) DO UPDATE SET payload=excluded.payload", snapshot.Environment, snapshot.Operations)
	return err
}
