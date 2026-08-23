package core

// worktree_base_test.go — cycle-1544, the reuse → base-capture seam.
//
// A reused worktree's HEAD can be an ADR-0076 salvage snapshot. Recording that
// SHA as WorktreeBaseSHA makes base and preserved work the SAME commit, so
// normalizeWorktreeToBase soft-resets onto the snapshot and the salvaged diff
// reads as empty — the work salvage exists to protect normalizes away to
// nothing. The guard walks back to the first non-snapshot ancestor, and ONLY
// for a snapshot HEAD: an unconditional walk-back would re-base every ordinary
// lane, which is the wider defect the second test bounds.
//
// These run against a REAL git repo, because the whole seam is a git ancestry
// question and a faked runner would prove only that the fake agrees with itself.

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitRepoWithCommits initialises a repo and returns a helper that commits an
// empty commit with the given subject, returning its SHA. Empty commits are
// enough: the guard reads subjects and ancestry, never trees.
func gitRepoWithCommits(t *testing.T) (dir string, commit func(subject string, body ...string) string) {
	t.Helper()
	dir = t.TempDir()
	for _, args := range [][]string{
		{"init"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "test"},
		{"config", "commit.gpgsign", "false"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	commit = func(subject string, body ...string) string {
		t.Helper()
		args := []string{"commit", "--allow-empty", "--no-verify", "-m", subject}
		for _, paragraph := range body {
			args = append(args, "-m", paragraph)
		}
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git commit %q: %v\n%s", subject, err, out)
		}
		cmd = exec.Command("git", "rev-parse", "HEAD")
		cmd.Dir = dir
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("rev-parse: %v", err)
		}
		return strings.TrimSpace(string(out))
	}
	return dir, commit
}

// AC7. A reused worktree sitting on a salvage snapshot (two of them, because a
// lane that re-failed stacks one per attempt) must resolve to the real work
// underneath — never to the snapshot itself, which is what made normalization
// discard the preserved diff.
func TestWorktreeReuseBase_SalvageSnapshotHEADResolvesToFirstNonSalvageAncestor(t *testing.T) {
	wt, commit := gitRepoWithCommits(t)
	commit("initial")
	wantBase := commit("real work from the prior attempt")
	commit(salvageSnapshotSubject)
	snapshotHead := commit(salvageSnapshotSubject)

	got, err := resolveWorktreeBaseSHA(context.Background(), wt)
	if err != nil {
		t.Fatalf("resolveWorktreeBaseSHA: %v", err)
	}
	if got == snapshotHead {
		t.Fatalf("the base must not be the salvage snapshot itself — normalizing onto it discards the preserved work")
	}
	if got != wantBase {
		t.Fatalf("base = %s, want the first non-snapshot ancestor %s", got, wantBase)
	}
}

func TestWorktreeReuseBase_SalvagedWorkIsPendingInTheReviewDiffAfterNormalize(t *testing.T) {
	wt, commit := gitRepoWithCommits(t)
	wantBase := commit("initial")
	if err := os.WriteFile(filepath.Join(wt, "salvaged.txt"), []byte("preserved work\n"), 0o644); err != nil {
		t.Fatalf("write salvaged work: %v", err)
	}
	cmd := exec.Command("git", "add", "salvaged.txt")
	cmd.Dir = wt
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add salvaged work: %v\n%s", err, out)
	}
	commit(salvageSnapshotSubject)

	base, err := resolveWorktreeBaseSHA(context.Background(), wt)
	if err != nil {
		t.Fatalf("resolveWorktreeBaseSHA: %v", err)
	}
	if base != wantBase {
		t.Fatalf("base = %s, want pre-snapshot base %s", base, wantBase)
	}
	normalizeWorktreeToBase(context.Background(), wt, base)
	cmd = exec.Command("git", "diff", "HEAD", "--name-only")
	cmd.Dir = wt
	diff, err := cmd.Output()
	if err != nil {
		t.Fatalf("git diff salvaged work: %v", err)
	}
	if !strings.Contains(string(diff), "salvaged.txt") {
		t.Fatalf("salvaged work must be pending for audit after normalize; diff=%q", diff)
	}
}

func TestWorktreeReuseBase_StackedSalvageSnapshotsResolveToTheCommitBeneathAll(t *testing.T) {
	wt, commit := gitRepoWithCommits(t)
	wantBase := commit("initial")
	for range 3 {
		commit(salvageSnapshotSubject)
	}

	got, err := resolveWorktreeBaseSHA(context.Background(), wt)
	if err != nil {
		t.Fatalf("resolveWorktreeBaseSHA: %v", err)
	}
	if got != wantBase {
		t.Fatalf("base = %s, want ancestor beneath all snapshots %s", got, wantBase)
	}
}

// AC8, the blast-radius bound. Ordinary reuse — the overwhelmingly common case
// — must capture HEAD verbatim. A guard that walked back an ancestor
// unconditionally would pass AC7 and silently re-base every normal lane.
func TestWorktreeReuseBase_OrdinaryHEADIsRecordedVerbatim(t *testing.T) {
	wt, commit := gitRepoWithCommits(t)
	commit("initial")
	head := commit("ordinary cycle work")

	got, err := resolveWorktreeBaseSHA(context.Background(), wt)
	if err != nil {
		t.Fatalf("resolveWorktreeBaseSHA: %v", err)
	}
	if got != head {
		t.Fatalf("ordinary reuse must capture HEAD verbatim: got %s, want %s", got, head)
	}
}

func TestWorktreeReuseBase_CommitMentioningSalvageInBodyIsNotTreatedAsASnapshot(t *testing.T) {
	wt, commit := gitRepoWithCommits(t)
	commit("initial")
	head := commit("ordinary subject", "mentions "+salvageSnapshotSubject)

	got, err := resolveWorktreeBaseSHA(context.Background(), wt)
	if err != nil {
		t.Fatalf("resolveWorktreeBaseSHA: %v", err)
	}
	if got != head {
		t.Fatalf("a body-only marker mention must not classify as a snapshot: got %s, want HEAD %s", got, head)
	}
}

// AC9, fail loudly. A chain that is snapshots all the way down has no honest
// base. Falling back to the snapshot reproduces the defect; falling back to an
// empty base disables normalization entirely (cyclerun.go:501-505). Both are
// silent, so the guard must error instead.
func TestWorktreeReuseBase_UnresolvableSnapshotAncestorFailsLoudly(t *testing.T) {
	wt, commit := gitRepoWithCommits(t)
	commit(salvageSnapshotSubject)
	commit(salvageSnapshotSubject)

	got, err := resolveWorktreeBaseSHA(context.Background(), wt)
	if err == nil {
		t.Fatalf("an unresolvable snapshot ancestry must surface an error; got base %q and nil error", got)
	}
	if got != "" {
		t.Fatalf("a failed resolve must return no base rather than a snapshot fallback; got %q", got)
	}
	if !strings.Contains(err.Error(), "salvage snapshot") {
		t.Errorf("the error must name WHY the base is unresolvable; got %v", err)
	}
}

// An unresolvable snapshot is unsafe at the production seam too: continuing
// with an empty WorktreeBaseSHA disables normalization and makes the preserved
// work invisible to the next audit. newCycleRun must surface that provisioning
// failure instead of merely logging it and dispatching source-writing phases.
func TestNewCycleRun_UnresolvableSnapshotBaseFailsProvisioning(t *testing.T) {
	wt, commit := gitRepoWithCommits(t)
	commit(salvageSnapshotSubject)

	st := &fakeStorage{state: State{LastCycleNumber: 1543}}
	o := NewOrchestrator(st, &fakeLedger{}, buildRunners(nil),
		WithWorktreeProvisioner(&fakeWorktree{path: wt}))

	_, cleanup, err := o.newCycleRun(context.Background(), CycleRequest{
		ProjectRoot:           t.TempDir(),
		GoalHash:              "g",
		DisableWorkspaceGuard: true,
	})
	if cleanup != nil {
		defer cleanup(false, false)
	}
	if err == nil {
		t.Fatal("a snapshot-only reused worktree must fail provisioning; continuing disables normalization of preserved work")
	}
	if !strings.Contains(err.Error(), "salvage snapshot") {
		t.Fatalf("provisioning error must identify the unresolvable salvage snapshot; got %v", err)
	}
}

func TestWorktreeReuseBase_UnresolvableAncestorWarnsLoudlyAndDoesNotDegradeSilently(t *testing.T) {
	wt, commit := gitRepoWithCommits(t)
	commit(salvageSnapshotSubject)
	o := NewOrchestrator(&fakeStorage{state: State{LastCycleNumber: 1543}}, &fakeLedger{}, buildRunners(nil),
		WithWorktreeProvisioner(&fakeWorktree{path: wt}))

	var runErr error
	stderr := captureStderr(t, func() {
		_, _, runErr = o.newCycleRun(context.Background(), CycleRequest{
			ProjectRoot: t.TempDir(), GoalHash: "g", DisableWorkspaceGuard: true,
		})
	})
	if runErr == nil {
		t.Fatal("unresolvable snapshot ancestry must abort provisioning")
	}
	if !strings.Contains(stderr, "WARN") || !strings.Contains(stderr, "salvage snapshot") {
		t.Fatalf("stderr must WARN and name the salvage snapshot; got:\n%s", stderr)
	}
}

func TestWorktreeReuseBase_ProvisioningPathRecordsGuardedBaseInCycleState(t *testing.T) {
	wt, commit := gitRepoWithCommits(t)
	wantBase := commit("ordinary base")
	commit(salvageSnapshotSubject)
	st := &fakeStorage{state: State{LastCycleNumber: 1543}}
	o := NewOrchestrator(st, &fakeLedger{}, buildRunners(nil),
		WithWorktreeProvisioner(&fakeWorktree{path: wt}))

	_, cleanup, err := o.newCycleRun(context.Background(), CycleRequest{
		ProjectRoot: t.TempDir(), GoalHash: "g", DisableWorkspaceGuard: true,
	})
	if cleanup != nil {
		defer cleanup(false, false)
	}
	if err != nil {
		t.Fatalf("newCycleRun: %v", err)
	}
	if got := st.cycleState.WorktreeBaseSHA; got != wantBase {
		t.Fatalf("production provisioning recorded base %s, want guarded ancestor %s", got, wantBase)
	}
}
