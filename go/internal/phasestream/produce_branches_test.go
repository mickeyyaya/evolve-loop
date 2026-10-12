package phasestream

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteEventsFile_InnerWriteErrorCleansTemp(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	bad := []Envelope{{
		SchemaVersion: SchemaVersion,
		Seq:           1,
		Data:          map[string]any{"ch": make(chan int)},
	}}
	err := writeEventsFile(ws, "build", bad)
	if err == nil {
		t.Fatal("expected a write error from an unmarshalable envelope")
	}
	if !strings.Contains(err.Error(), "marshal envelope") {
		t.Errorf("error should originate from the marshal step, got %q", err.Error())
	}
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

func TestProduce_StatNonNotExistError(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	blocker := filepath.Join(ws, "not-a-dir")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatalf("write blocker: %v", err)
	}
	err := Produce(ProduceConfig{Workspace: blocker, Phase: "build", CLI: "claude-p", Cycle: 1})
	if err == nil {
		t.Fatal("expected a non-NotExist stat error when the workspace path is a regular file")
	}
	if !strings.Contains(err.Error(), "stat") {
		t.Errorf("error should originate from the stat step, got %q", err.Error())
	}
}
