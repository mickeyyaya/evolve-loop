package phasestream

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestWriteEventsFile_InnerWriteErrorCleansTemp pins the writeEnvelope-error
// branch inside writeEventsFile's loop: an unmarshalable envelope makes the
// buffered write fail, and the function must surface the error AND remove the
// orphaned temp (no dot-prefixed leftover pollutes the *-events.ndjson glob).
func TestWriteEventsFile_InnerWriteErrorCleansTemp(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	bad := []Envelope{{
		SchemaVersion: SchemaVersion,
		Seq:           1,
		Data:          map[string]any{"ch": make(chan int)}, // unmarshalable
	}}
	err := writeEventsFile(ws, "build", bad)
	if err == nil {
		t.Fatal("expected a write error from an unmarshalable envelope")
	}
	if !strings.Contains(err.Error(), "marshal envelope") {
		t.Errorf("error should originate from the marshal step, got %q", err.Error())
	}
	// No events file and no orphaned temp.
	if _, statErr := os.Stat(filepath.Join(ws, "build-events.ndjson")); !os.IsNotExist(statErr) {
		t.Error("no events file should exist after a failed write")
	}
	entries, _ := os.ReadDir(ws)
	for _, e := range entries {
		if len(e.Name()) > 0 && e.Name()[0] == '.' {
			t.Errorf("orphaned temp left behind: %s", e.Name())
		}
	}
}

// TestWriteEventsFile_EmptyEnvelopesWritesEmptyFile pins the happy path with
// zero envelopes: the atomic rename still produces a (zero-length) events
// file, so consumers see a present-but-empty stream rather than a missing one.
func TestWriteEventsFile_EmptyEnvelopesWritesEmptyFile(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	if err := writeEventsFile(ws, "scout", nil); err != nil {
		t.Fatalf("writeEventsFile with no envelopes: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(ws, "scout-events.ndjson"))
	if err != nil {
		t.Fatalf("events file should exist: %v", err)
	}
	if len(body) != 0 {
		t.Errorf("empty envelope set should write an empty file, got %q", body)
	}
}

// TestProduce_StatNonNotExistError pins the Produce stat-error branch that is
// NOT os.ErrNotExist: when the would-be stdout log path has a non-directory
// parent component, os.Stat returns ENOTDIR (not NotExist), so Produce must
// return that error rather than treating it as "no output".
func TestProduce_StatNonNotExistError(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	// Create a regular file where Produce expects the workspace to contain a
	// child path: use that file as a parent dir component for the log path.
	blocker := filepath.Join(ws, "not-a-dir")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatalf("write blocker: %v", err)
	}
	// Workspace points THROUGH a regular file → stat of
	// <blocker>/build-stdout.log returns ENOTDIR, a non-NotExist error.
	err := Produce(ProduceConfig{Workspace: blocker, Phase: "build", CLI: "claude-p", Cycle: 1})
	if err == nil {
		t.Fatal("expected a non-NotExist stat error when the workspace path is a regular file")
	}
	if !strings.Contains(err.Error(), "stat") {
		t.Errorf("error should originate from the stat step, got %q", err.Error())
	}
}
