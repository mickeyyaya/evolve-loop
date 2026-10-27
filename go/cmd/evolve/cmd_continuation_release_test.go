package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

func boundContinuationRoot(t *testing.T, scope string, cycle int) string {
	t.Helper()
	root := t.TempDir()
	if err := continuation.WriteRegistryEntry(root, scope, continuation.Continuation{SnapshotSHA: "s1", BaseSHA: "b1", Branch: "br", Cycle: cycle}); err != nil {
		t.Fatal(err)
	}
	return root
}

func releaseRun(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	code := runContinuationRelease(args, &out, &errb)
	return code, out.String(), errb.String()
}

func TestRunContinuationRelease_OnlyTheOperatorParameterReleasesABinding(t *testing.T) {
	if code, _, _ := releaseRun("--bogus"); code != 10 {
		t.Errorf("a bad flag: code = %d, want 10", code)
	}
	if code, _, se := releaseRun("--project-root", t.TempDir()); code != 10 || !strings.Contains(se, "exactly one <scope-id>") {
		t.Errorf("no scope: code = %d, stderr = %q, want 10", code, se)
	}
	t.Setenv("EVOLVE_OPERATOR_CONFIRM", "1")
	root := boundContinuationRoot(t, "item-a", 0)
	if code, _, se := releaseRun("--project-root", root, "item-a"); code != 10 || !strings.Contains(se, "-operator") {
		t.Errorf("no -operator: code = %d, stderr = %q, want a refusal that the environment cannot lift", code, se)
	}
	if code, _, se := releaseRun("--project-root", root, "-operator", "item-b"); code != 1 || !strings.Contains(se, "holds no continuation binding") {
		t.Errorf("an unbound scope: code = %d, stderr = %q, want 1", code, se)
	}
	code, out, se := releaseRun("--project-root", root, "-operator", "item-a")
	if code != 0 || !strings.Contains(out, "by operator via -operator") {
		t.Errorf("release: code = %d, stdout = %q, stderr = %q, want 0 by -operator", code, out, se)
	}

	broken := t.TempDir()
	if err := os.MkdirAll(filepath.Dir(continuation.RegistryPath(broken)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(continuation.RegistryPath(broken), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, _, se := releaseRun("--project-root", broken, "-operator", "item-a"); code != 1 || !strings.Contains(se, "registry unreadable") {
		t.Errorf("a broken registry: code = %d, stderr = %q, want 1", code, se)
	}
}

func TestRunContinuationRelease_ALiveLaneNeedsForce(t *testing.T) {
	t.Parallel()
	root := boundContinuationRoot(t, "item-a", 7)
	runDir := paths.RunWorkspace(root, 7)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runlease.Write(runDir, runlease.Lease{RunID: "r", OwnerPID: os.Getpid()}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if code, _, se := releaseRun("--project-root", root, "-operator", "item-a"); code != 10 || !strings.Contains(se, "still LIVE") {
		t.Errorf("a live lane: code = %d, stderr = %q, want 10", code, se)
	}
	if code, out, se := releaseRun("--project-root", root, "-operator", "-force", "item-a"); code != 0 || !strings.Contains(out, "released") {
		t.Errorf("-force: code = %d, stdout = %q, stderr = %q, want 0", code, out, se)
	}
}

func TestRunContinuationRelease_AFailedOrLostReleaseIsExit1(t *testing.T) {
	saved := releaseContinuationBinding
	t.Cleanup(func() { releaseContinuationBinding = saved })
	root := boundContinuationRoot(t, "item-a", 0)
	releaseContinuationBinding = func(inboxmover.Options, string, string, string) (continuation.Continuation, bool, error) {
		return continuation.Continuation{}, false, errors.New("disk full")
	}
	if code, _, se := releaseRun("--project-root", root, "-operator", "item-a"); code != 1 || !strings.Contains(se, "disk full") {
		t.Errorf("a failed release: code = %d, stderr = %q, want 1", code, se)
	}
	releaseContinuationBinding = func(inboxmover.Options, string, string, string) (continuation.Continuation, bool, error) {
		return continuation.Continuation{Cycle: 9}, false, nil
	}
	if code, _, se := releaseRun("--project-root", root, "-operator", "item-a"); code != 1 || !strings.Contains(se, "rebound by a live lane") {
		t.Errorf("a lost release: code = %d, stderr = %q, want 1", code, se)
	}
}
