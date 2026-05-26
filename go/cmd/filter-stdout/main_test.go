// go/cmd/filter-stdout/main_test.go
// Package: package main (internal test, same package as main.go)
//
// These tests require Builder to refactor main.go to extract:
//   func run(args []string) int
// returning the intended exit code without calling os.Exit directly.
// Until that function exists, this file will FAIL TO COMPILE → RED.
//
// Copy verbatim to go/cmd/filter-stdout/main_test.go (package main).

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRun_NoArgs_ExitsWithCode2(t *testing.T) {
	rc := run(nil)
	if rc != 2 {
		t.Errorf("rc=%d, want 2 (usage error when wrong arg count)", rc)
	}
}

func TestRun_OneArg_ExitsWithCode2(t *testing.T) {
	rc := run([]string{"onlyone"})
	if rc != 2 {
		t.Errorf("rc=%d, want 2 (usage error when only 1 arg)", rc)
	}
}

func TestRun_HappyPath_WritesCleanFile(t *testing.T) {
	ws := t.TempDir()
	// Write a minimal stream-json log line (assistant text event)
	rawContent := `{"type":"assistant","content":[{"type":"text","text":"Hello world"}]}` + "\n"
	rawPath := filepath.Join(ws, "scout-stdout.log")
	if err := os.WriteFile(rawPath, []byte(rawContent), 0o644); err != nil {
		t.Fatalf("write raw log: %v", err)
	}

	rc := run([]string{ws, "scout"})
	if rc != 0 {
		t.Errorf("rc=%d, want 0 (happy path)", rc)
	}

	cleanPath := filepath.Join(ws, "scout-stdout.clean.txt")
	if _, err := os.Stat(cleanPath); err != nil {
		t.Errorf("clean file not created at %q: %v", cleanPath, err)
	}
}
