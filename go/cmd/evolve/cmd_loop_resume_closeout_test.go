//go:build integration

package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

func TestRunResumeBatch_AFailureOutcomeTheInboxCannotTakeIsAWarningAndTheFailStands(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir, ws := seedFailedCycleInbox(t, root, "worked", 7)
	if _, err := inboxmover.Claim(inboxmover.Options{ProjectRoot: root, Stderr: io.Discard}, "worked", "7"); err != nil {
		t.Fatalf("claim: %v", err)
	}
	writeResumeCheckpoint(t, evolveDir, ws)
	processing := filepath.Join(evolveDir, "inbox", "processing")
	if err := os.Chmod(processing, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(processing, 0o755) })
	var stdout, stderr bytes.Buffer
	lr := loopResult{}

	rc := runResumeBatch(context.Background(), loopConfig{ProjectRoot: root, EvolveDir: evolveDir, Resume: true},
		resumedFailRunner{result: core.CycleResult{Cycle: 7, FinalVerdict: core.VerdictFAIL}}, newFakeLedger(), nil,
		map[string]string{}, map[string]string{}, &lr, &stdout, &stderr)

	if rc != 2 || !strings.Contains(stderr.String(), "WARN: could not apply cycle 7 failure outcome to the inbox") {
		t.Fatalf("rc=%d, want rc 2 and the inbox WARN\nstderr:\n%s", rc, stderr.String())
	}
}
