package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gcpolicy"
)

func TestArchivePollutedWorkspace_AnOccupiedArchiveNameKeepsTheWorkspace(t *testing.T) {
	t.Parallel()
	ws := filepath.Join(t.TempDir(), "ws")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "stray.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	archived := ws + gcpolicy.PollutedArchiveName(coverNow())
	if err := os.MkdirAll(filepath.Join(archived, "earlier"), 0o755); err != nil {
		t.Fatal(err)
	}

	err := archivePollutedWorkspace(ws, coverNow)

	if err == nil || !strings.HasPrefix(err.Error(), "rename to "+archived+": ") {
		t.Fatalf("archivePollutedWorkspace = %v, want the rename error naming %s", err, archived)
	}
	if _, serr := os.Stat(filepath.Join(ws, "stray.md")); serr != nil {
		t.Errorf("the workspace lost its file after a failed archive: %v", serr)
	}
}
