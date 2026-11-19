package core

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScopeDeltaFloorChecks_CorruptArtifactIsReplacedAndEmitsOneSummary(t *testing.T) {
	t.Parallel()

	_, _, in, events := scopeDeltaThreePathsRound(t)
	if err := os.WriteFile(filepath.Join(in.Workspace, scopeDeltaFileName), []byte("{corrupt"), 0o644); err != nil {
		t.Fatal(err)
	}

	ScopeDeltaFloorChecks(scopeDeltaProtected)(context.Background(), in)

	if len(*events) != 1 {
		t.Fatalf("a corrupt artifact does not suppress the summary signal; got %d: %+v", len(*events), *events)
	}
	if got := readScopeDeltaArtifact(t, in.Workspace).verdicts(); len(got) != 4 {
		t.Fatalf("the floor overwrites the corrupt artifact with all 4 entries; got %v", got)
	}
}

func TestScopeDeltaFloorChecks_UnwritableWorkspaceWarnsAndStillEmitsSummary(t *testing.T) {
	t.Parallel()

	_, _, in, events := scopeDeltaThreePathsRound(t)
	notADir := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(notADir, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	in.Workspace = notADir
	in.Stderr = &stderr

	ScopeDeltaFloorChecks(scopeDeltaProtected)(context.Background(), in)

	if len(*events) != 1 {
		t.Fatalf("an unwritable workspace still emits the summary signal; got %d: %+v", len(*events), *events)
	}
	if !strings.Contains(stderr.String(), "[build-floor] WARN scope-delta: write ") {
		t.Fatalf("an unwritable workspace logs a write warning on stderr; got %q", stderr.String())
	}
}
