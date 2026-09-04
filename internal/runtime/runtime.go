// Package runtime defines the private host/runtime seam. Business tools belong to the host.
package runtime

import (
	"context"
	"encoding/json"
	"os"
	"strings"
)

type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}
type Call struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
	Round     int             `json:"round"`
}
type ToolResult struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data,omitempty"`
	Error string          `json:"error,omitempty"`
	Close bool            `json:"close,omitempty"`
}

func Value(v any) ToolResult {
	b, e := json.Marshal(v)
	if e != nil {
		return Failure("result_encoding_failed")
	}
	return ToolResult{OK: true, Data: b}
}
func Failure(code string) ToolResult { return ToolResult{Error: code} }

type Request struct {
	System     string `json:"system"`
	Prompt     string `json:"prompt"`
	Model      ModelSelection `json:"model"`
	Tools      []Tool `json:"tools"`
	MaxRounds  int    `json:"max_rounds"`
	Checkpoint string `json:"checkpoint,omitempty"`
	SessionDir string `json:"session_dir,omitempty"`
}
type Usage struct {
	Input      int64 `json:"input"`
	Output     int64 `json:"output"`
	CacheRead  int64 `json:"cache_read"`
	CacheWrite int64 `json:"cache_write"`
}
type Result struct {
	Text       string `json:"text"`
	Stop       string `json:"stop"`
	Checkpoint string `json:"checkpoint,omitempty"`
	Usage      Usage  `json:"usage"`
}
type Event struct {
	Type      string          `json:"type"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Text      string          `json:"text,omitempty"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}
type Hooks struct {
	Execute    func(context.Context, Call) ToolResult
	Emit       func(Event)
	CloseAfter func(Call, ToolResult) bool
}
type Runtime interface {
	Run(context.Context, Request, Hooks) (Result, error)
}

// Model processes inherit OAuth and proxy configuration, but never TikTok
// application credentials that may be present for a separate callback command.
func modelProcessEnv() []string {
	env := make([]string, 0, len(os.Environ()))
	for _, value := range os.Environ() {
		key, _, _ := strings.Cut(value, "=")
		if strings.HasPrefix(key, "AD_AGENT_TIKTOK_") || strings.HasPrefix(key, "TIKTOK_") {
			continue
		}
		env = append(env, value)
	}
	return env
}
