package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func agedBindingsRoot(t *testing.T) (*gittest.Repo, string) {
	t.Helper()
	r := gittest.Fixture(t)
	r.Git("commit", "-q", "--allow-empty", "-m", "base")
	oldWT := filepath.Join(r.Dir, ".evolve", "worktrees", staleLeaf(1127))
	r.Git("worktree", "add", "-q", "-b", staleLeaf(1127), oldWT, "main")
	staleSeal(t, r.Dir, 1127)
	for _, w := range []struct {
		scope string
		c     continuation.Continuation
	}{
		{"old-item", continuation.Continuation{Worktree: oldWT, Branch: staleLeaf(1127), SnapshotSHA: "0ld", Cycle: 1127}},
		{"new-item", continuation.Continuation{Worktree: "/elsewhere/cycle-2040", SnapshotSHA: "n3w", Cycle: 2040}},
	} {
		if err := continuation.WriteRegistryEntry(r.Dir, w.scope, w.c); err != nil {
			t.Fatal(err)
		}
	}
	inbox := filepath.Join(r.Dir, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inbox, "old-item.json"), []byte(`{"id":"old-item","continuation":{"snapshot_sha":"0ld","cycle":1127}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(r.Dir, ".evolve", "state.json"), []byte(`{"lastCycleNumber":2052}`), 0o644); err != nil {
		t.Fatal(err)
	}
	return r, oldWT
}

func continuationSweep(t *testing.T, root string, dryRun bool, maxAge int) (bool, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	r := newGCRun(context.Background(), dryRun, &out, &errb)
	r.kill = func(int) error { t.Fatal("the continuation sweep stops no process"); return nil }
	failed := r.continuations(root, maxAge, filepath.Join(root, ".evolve", "worktrees"))
	return failed, out.String(), errb.String()
}

func TestGCContinuations_ReleasesAnEntryOlderThanThePolicyAgeAndKeepsANewerOne(t *testing.T) {
	t.Parallel()
	r, oldWT := agedBindingsRoot(t)

	failed, out, se := continuationSweep(t, r.Dir, false, 100)

	if failed {
		t.Fatalf("sweep failed:\n%s\n%s", out, se)
	}
	if _, ok, _ := continuation.ReadRegistryEntry(r.Dir, "old-item"); ok {
		t.Error("the binding of cycle 1127 is older than 100 cycles and must be released")
	}
	if c, ok, _ := continuation.ReadRegistryEntry(r.Dir, "new-item"); !ok || c.Cycle != 2040 {
		t.Errorf("the binding of cycle 2040 is newer than the age and must stay: ok=%v %+v", ok, c)
	}
	if _, err := os.Stat(oldWT); !os.IsNotExist(err) {
		t.Errorf("the worktree of the released binding must be removed: %v\n%s", err, out)
	}
	if body, _ := os.ReadFile(filepath.Join(r.Dir, ".evolve", "inbox", "old-item.json")); !strings.Contains(string(body), `"released_continuations"`) || strings.Contains(string(body), `"continuation":`) {
		t.Errorf("the released pointer is preserved on its item and no longer live there: %s", body)
	}
	for _, want := range []string{"released 1 continuation binding(s) older than 100 cycles", "old-item cycle=1127", staleLeaf(1127) + " removed"} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout must name %q:\n%s", want, out)
		}
	}
}

func TestGCContinuations_ADryRunReleasesNothing(t *testing.T) {
	t.Parallel()
	r, oldWT := agedBindingsRoot(t)

	failed, out, _ := continuationSweep(t, r.Dir, true, 100)

	if failed || !strings.Contains(out, "1 continuation binding(s) older than 100 cycles would be released") {
		t.Errorf("failed=%v stdout:\n%s", failed, out)
	}
	if _, ok, _ := continuation.ReadRegistryEntry(r.Dir, "old-item"); !ok {
		t.Error("a dry run must keep the binding")
	}
	if _, err := os.Stat(oldWT); err != nil {
		t.Errorf("a dry run must keep the worktree: %v", err)
	}
}

func TestGCContinuations_AnUnreadableStateOrRegistryFailsTheStep(t *testing.T) {
	t.Parallel()
	r, _ := agedBindingsRoot(t)
	if err := os.WriteFile(continuation.RegistryPath(r.Dir), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if failed, _, se := continuationSweep(t, r.Dir, false, 100); !failed || !strings.Contains(se, "continuation registry") {
		t.Errorf("an unreadable registry: failed=%v stderr=%q", failed, se)
	}
	if err := os.WriteFile(filepath.Join(r.Dir, ".evolve", "state.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if failed, _, se := continuationSweep(t, r.Dir, false, 100); !failed || !strings.Contains(se, "last cycle") {
		t.Errorf("an unreadable state: failed=%v stderr=%q", failed, se)
	}
}

func TestGCContinuations_KeepsADirtyWorktreeAndNamesTheReason(t *testing.T) {
	t.Parallel()
	r, oldWT := agedBindingsRoot(t)
	if err := os.WriteFile(filepath.Join(oldWT, "scratch.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	failed, out, se := continuationSweep(t, r.Dir, false, 100)

	if failed || !strings.Contains(out, staleLeaf(1127)+" kept: dirty worktree") {
		t.Errorf("failed=%v; a dirty worktree is kept by name:\n%s\n%s", failed, out, se)
	}
	if _, err := os.Stat(oldWT); err != nil {
		t.Errorf("the dirty worktree must stay: %v", err)
	}
}

func TestGCContinuations_AReleaseOrWorktreeListFaultFailsTheStep(t *testing.T) {
	t.Parallel()
	plain := t.TempDir()
	if err := continuation.WriteRegistryEntry(plain, "old-item", continuation.Continuation{Worktree: "/gone/cycle-feed01-1", Cycle: 1}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plain, ".evolve", "state.json"), []byte(`{"lastCycleNumber":500}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if failed, _, se := continuationSweep(t, plain, false, 100); !failed || !strings.Contains(se, "list worktrees") {
		t.Errorf("a root that is no git repository: failed=%v stderr=%q", failed, se)
	}

	linked := t.TempDir()
	if err := continuation.WriteRegistryEntry(linked, "old-item", continuation.Continuation{Cycle: 1}); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(target, []byte(`{"lastCycleNumber":500}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(linked, ".evolve", "state.json")); err != nil {
		t.Fatal(err)
	}
	if failed, _, se := continuationSweep(t, linked, false, 100); !failed || !strings.Contains(se, "release old-item") {
		t.Errorf("a release the inbox refuses: failed=%v stderr=%q", failed, se)
	}
}

func TestGCContinuations_TheAgeBoundaryAndAnUnboundLaneStay(t *testing.T) {
	t.Parallel()
	r, _ := agedBindingsRoot(t)
	for scope, cycle := range map[string]int{"boundary-item": 1952, "unknown-age-item": 0} {
		if err := continuation.WriteRegistryEntry(r.Dir, scope, continuation.Continuation{SnapshotSHA: scope, Cycle: cycle}); err != nil {
			t.Fatal(err)
		}
	}
	freeWT := filepath.Join(r.Dir, ".evolve", "worktrees", staleLeaf(1500))
	r.Git("worktree", "add", "-q", "-b", staleLeaf(1500), freeWT, "main")
	staleSeal(t, r.Dir, 1500)

	if failed, out, se := continuationSweep(t, r.Dir, false, 100); failed {
		t.Fatalf("sweep failed:\n%s\n%s", out, se)
	}
	if _, ok, _ := continuation.ReadRegistryEntry(r.Dir, "boundary-item"); !ok {
		t.Error("a binding exactly max_age_cycles old is not older than the age and must stay")
	}
	if _, ok, _ := continuation.ReadRegistryEntry(r.Dir, "unknown-age-item"); !ok {
		t.Error("a binding with no cycle has an unknown age and must stay")
	}
	if _, err := os.Stat(freeWT); err != nil {
		t.Errorf("a stale lane that no released binding names must stay: %v", err)
	}
}

func TestRunGC_TheContinuationStepReadsThePolicyAge(t *testing.T) {
	t.Parallel()
	r, _ := agedBindingsRoot(t)
	for age, want := range map[string]string{
		"5000": "0 continuation binding(s) older than 5000 cycles would be released",
		"100":  "1 continuation binding(s) older than 100 cycles would be released",
	} {
		if err := os.WriteFile(filepath.Join(r.Dir, ".evolve", "policy.json"), []byte(`{"continuation":{"max_age_cycles":`+age+`}}`), 0o644); err != nil {
			t.Fatal(err)
		}
		var out, errb bytes.Buffer
		runGC([]string{"--dry-run", "--project-root", r.Dir}, nil, &out, &errb)
		if !strings.Contains(out.String(), want) {
			t.Errorf("max_age_cycles %s: stdout must say %q:\n%s", age, want, out.String())
		}
	}
}

func TestRunGC_APolicyThatDoesNotLoadReleasesNoBinding(t *testing.T) {
	t.Parallel()
	r, oldWT := agedBindingsRoot(t)
	if err := os.WriteFile(filepath.Join(r.Dir, ".evolve", "policy.json"), []byte(`{"continuation":{"max_age_cycles":5000},"typo":`), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	gcr := newGCRun(context.Background(), false, &out, &errb)

	failed := gcr.agedContinuations(r.Dir, continuationAge{maxAge: 100, loaded: false}, filepath.Join(r.Dir, ".evolve", "worktrees"))

	if failed || !strings.Contains(errb.String(), "continuation sweep skipped") {
		t.Errorf("failed=%v stderr=%q; a policy that does not load must skip the sweep loudly", failed, errb.String())
	}
	if _, ok, _ := continuation.ReadRegistryEntry(r.Dir, "old-item"); !ok {
		t.Error("no binding may be released when the policy does not load")
	}
	if _, err := os.Stat(oldWT); err != nil {
		t.Errorf("the worktree must stay: %v", err)
	}
	runGC([]string{"--dry-run", "--project-root", r.Dir}, nil, &out, &errb)
	if !strings.Contains(errb.String(), "continuation sweep skipped") {
		t.Errorf("evolve gc must skip the sweep on a policy that does not load:\n%s", errb.String())
	}
}
