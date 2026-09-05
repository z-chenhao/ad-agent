package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseVersionMatchesSourceAndWorkspaces(t *testing.T) {
	root := filepath.Join("..", "..")
	data, err := os.ReadFile(filepath.Join(root, "VERSION"))
	if err != nil || strings.TrimSpace(string(data)) != version {
		t.Fatalf("VERSION differs from binary: %v", err)
	}
	for _, file := range []string{"package.json", "web/package.json", "runtime/pi-bridge/package.json", "runtime/builtin-model-bridge/package.json", "runtime/codex-bridge/package.json", "runtime/claude-bridge/package.json"} {
		data, err := os.ReadFile(filepath.Join(root, file))
		if err != nil {
			t.Fatal(err)
		}
		var manifest struct {
			Version string `json:"version"`
		}
		if err := json.Unmarshal(data, &manifest); err != nil || manifest.Version != version {
			t.Fatalf("%s differs from binary: %v", file, err)
		}
	}
	var out bytes.Buffer
	if err := run(context.Background(), []string{"version"}, strings.NewReader(""), &out); err != nil || out.String() != "Ad Agent "+version+"\n" {
		t.Fatalf("version command: %q %v", out.String(), err)
	}
}
