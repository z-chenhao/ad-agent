package store

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type requestIDKey struct{}

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}
func RequestID(ctx context.Context) string {
	value, _ := ctx.Value(requestIDKey{}).(string)
	return value
}

// Diagnostic is deliberately metadata-only. Do not add credentials, arbitrary
// tool arguments/results, request bodies, URLs, or private provider transcripts.
type Diagnostic struct {
	At         time.Time `json:"at"`
	Type       string    `json:"type"`
	RequestID  string    `json:"request_id,omitempty"`
	TurnID     string    `json:"turn_id,omitempty"`
	Sequence   int64     `json:"sequence,omitempty"`
	Method     string    `json:"method,omitempty"`
	Route      string    `json:"route,omitempty"`
	Status     int       `json:"status,omitempty"`
	Bytes      int       `json:"bytes,omitempty"`
	DurationMS int64     `json:"duration_ms,omitempty"`
	Tool       string    `json:"tool,omitempty"`
	CallID     string    `json:"call_id,omitempty"`
	Success    *bool     `json:"success,omitempty"`
	Outcome    string    `json:"outcome,omitempty"`
	Runtime    string    `json:"runtime,omitempty"`
	ErrorCode  string    `json:"error_code,omitempty"`
}

// Never copy arbitrary backend/provider errors into metadata logs, even if they
// resemble identifiers. Unknown failures remain available in authenticated events.
func diagnosticErrorCode(message string) string {
	switch message {
	case "analysis_timeout", "analysis_cancelled", "analysis_runtime_failed",
		"analysis_budget_exhausted", "analysis_interrupted", "analysis_missing_submission",
		"analysis_incomplete", "analysis_delegate_limit", "budget_delta_exceeded",
		"budget_outside_limits", "budget_policy_not_configured", "report_budget_exceeded", "cancelled", "event_persistence_failed":
		return message
	case "":
		return ""
	default:
		return "tool_failed"
	}
}

var diagnosticMu sync.Mutex

// Two bounded files per stream, private to this local operator. Each write closes
// its handle so CLI and server diagnostics survive orderly process restarts.
func (s *Store) RecordDiagnostic(stream string, entry Diagnostic) error {
	if stream != "server" && stream != "agent-trace" {
		return os.ErrInvalid
	}
	diagnosticMu.Lock()
	defer diagnosticMu.Unlock()
	dir := filepath.Join(s.Dir, "logs")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return os.ErrPermission
	}
	path := filepath.Join(dir, stream+".jsonl")
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
			return os.ErrPermission
		}
		if info.Size() >= 10<<20 {
			if err := os.Rename(path, path+".1"); err != nil {
				return err
			}
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	entry.At = time.Now().UTC()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	err = json.NewEncoder(f).Encode(entry)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	return closeErr
}
