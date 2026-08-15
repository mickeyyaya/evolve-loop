//go:build acs

// Package cycle1474 materialises the cycle-1474 acceptance criteria for the two
// fleet-scoped pipeline tasks pinned to this lane:
//
//   - worktree-retry-diagnostic-integrity   → the shared `git worktree add`
//     retry must keep BOTH the initiating and the terminal failure, and must
//     announce contention BEFORE it pays the backoff.
//   - worktree-provisioning-cause-fingerprint → a failed worktree provision must
//     put its git cause into the cycle's failure record, so the recorded
//     identity names the provisioning failure instead of the downstream
//     source-phase refusal it caused.
//
// Predicate strategy — every predicate DRIVES the production seam in-process and
// asserts on returned values or emitted artifacts; none greps source (the
// cycle-85 degenerate-predicate ban):
//
//   - 001/002 call gitexec.Git.AddWorktreeWithRetry directly with a scripted
//     runner and assert on what it returns / the order it calls back.
//   - 003/004 run the REAL core.Orchestrator.RunCycle with a provisioner that
//     fails, then read the cycle's own workspace and the REAL
//     core.AssembleFailureDigest — the same assembler production uses.
//   - 005 is the negative axis: a clean provision must fabricate no failure.
//
// No `go test` subprocess, no whole-package sweep, no wall-clock bound, no
// literal PID: all five are in-process and deterministic.
package cycle1474

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

// --- task 1: worktree-retry-diagnostic-integrity ---------------------------

// mixedFailRunner scripts the recorded two-failure sequence: attempt 1 is the
// live collision shape (rc=255, EMPTY stderr), attempt 2 is the permanent
// rc=128 that first failure caused.
func mixedFailRunner(attempts *int) sysexec.RunFunc {
	return func(_ context.Context, _, _ string, args, _ []string, _ io.Reader, _, stderr io.Writer) (int, error) {
		if len(args) >= 2 && args[0] == "worktree" && args[1] == "add" {
			*attempts++
			if *attempts == 1 {
				return 255, nil
			}
			if stderr != nil {
				io.WriteString(stderr, "fatal: '/base/cycle-9' already exists\n")
			}
			return 128, nil
		}
		return 0, nil
	}
}

// TestC1474_001_RetryPreservesInitiatingFailure — a transient rc=255 followed by
// a different terminal failure must leave BOTH recoverable from the returned
// diagnostic. The first attempt has no stderr, so its exit code is the only
// evidence it happened at all.
func TestC1474_001_RetryPreservesInitiatingFailure(t *testing.T) {
	attempts := 0
	g := gitexec.Git{Dir: t.TempDir(), Exec: mixedFailRunner(&attempts)}

	_, stderr, code, _ := g.AddWorktreeWithRetry(context.Background(),
		gitexec.WorktreeAddRetry{
			Sleep:     func(time.Duration) {},
			Retryable: gitexec.RetryableWorktreeAddFailure,
		},
		"-B", "lane", filepath.Join(t.TempDir(), "wt"), "HEAD")

	if attempts != 2 {
		t.Fatalf("attempts=%d, want 2 (transient rc=255, then the permanent rc=128)", attempts)
	}
	if code != 128 {
		t.Errorf("exit code=%d, want the FINAL 128 — the terminal failure is what the caller fails on", code)
	}
	if !strings.Contains(stderr, "already exists") {
		t.Errorf("terminal diagnostic lost git's own final stderr: %q", stderr)
	}
	if !strings.Contains(stderr, "255") {
		t.Errorf("terminal diagnostic does not preserve the INITIATING rc=255 failure: %q", stderr)
	}
}

// TestC1474_002_RetryAnnouncesBeforeBackoff — OnRetry exists so a caller can
// announce contention WHILE it is happening; a lane stuck inside the 2s/4s
// ladder must not be silent until the sleep completes.
func TestC1474_002_RetryAnnouncesBeforeBackoff(t *testing.T) {
	attempts := 0
	var events []string
	oneFail := func(_ context.Context, _, _ string, args, _ []string, _ io.Reader, _, stderr io.Writer) (int, error) {
		if len(args) >= 2 && args[0] == "worktree" && args[1] == "add" {
			attempts++
			if attempts == 1 {
				if stderr != nil {
					io.WriteString(stderr, "Preparing worktree (new branch 'lane')\n")
				}
				return 255, nil
			}
		}
		return 0, nil
	}
	g := gitexec.Git{Dir: t.TempDir(), Exec: oneFail}

	_, _, code, err := g.AddWorktreeWithRetry(context.Background(),
		gitexec.WorktreeAddRetry{
			Sleep:   func(time.Duration) { events = append(events, "sleep") },
			OnRetry: func(_, _, _ int, _ string) { events = append(events, "retry") },
		},
		"-B", "lane", filepath.Join(t.TempDir(), "wt"), "HEAD")

	if err != nil || code != 0 {
		t.Fatalf("one transient failure must still be absorbed, got code=%d err=%v", code, err)
	}
	if len(events) != 2 || events[0] != "retry" || events[1] != "sleep" {
		t.Errorf("callback order=%v, want [retry sleep] — announce, THEN pay the backoff", events)
	}
}

// --- task 2: worktree-provisioning-cause-fingerprint -----------------------

// scriptedWorktree is a core.WorktreeProvisioner whose Create outcome is
// scripted. Cleanup is a no-op: nothing is ever really provisioned.
type scriptedWorktree struct {
	path      string
	createErr error
	created   int
}

func (w *scriptedWorktree) Create(_ string, _ int) (string, error) {
	w.created++
	if w.createErr != nil {
		return "", w.createErr
	}
	return w.path, nil
}

func (w *scriptedWorktree) Cleanup(_, _ string) error { return nil }

// runProvisionCycle runs one REAL cycle through the production RunCycle path
// with the scripted provisioning outcome, and returns the cycle number and the
// workspace the run actually used.
func runProvisionCycle(t *testing.T, createErr error) (int, string) {
	t.Helper()
	st := &fixtures.FakeStorage{State: core.State{LastCycleNumber: 1473}}
	wt := &scriptedWorktree{path: t.TempDir(), createErr: createErr}
	o := core.NewOrchestrator(st, &fixtures.FakeLedger{}, fixtures.BuildRunners(nil),
		core.WithWorktreeProvisioner(wt))

	res, err := o.RunCycle(context.Background(), core.CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "g"})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if wt.created == 0 {
		t.Fatal("the provisioner was never called — this predicate is not exercising the provisioning path")
	}
	ws := st.CycleState.WorkspacePath
	if ws == "" {
		t.Fatal("the cycle recorded no workspace path")
	}
	return res.Cycle, ws
}

// workspaceMentions reports whether ANY artifact the cycle emitted carries the
// token. Format-agnostic: the requirement is that the cause is persisted where
// the failure surfaces read, not that it lands in one named file.
func workspaceMentions(t *testing.T, workspace, token string) bool {
	t.Helper()
	found := false
	if err := filepath.WalkDir(workspace, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || found {
			return nil //nolint:nilerr // unreadable entries are simply not evidence
		}
		if b, rerr := os.ReadFile(path); rerr == nil && strings.Contains(string(b), token) {
			found = true
		}
		return nil
	}); err != nil {
		t.Fatalf("walk workspace %s: %v", workspace, err)
	}
	return found
}

// TestC1474_003_ProvisioningCausePersistedInCycleRecord — the git cause must
// outlive the orchestrator's stderr and land in the cycle's own record.
func TestC1474_003_ProvisioningCausePersistedInCycleRecord(t *testing.T) {
	const cause = "ACS1474-CAUSE-A: git worktree add rc=255 (lock held)"
	_, ws := runProvisionCycle(t, errors.New(cause))

	if !workspaceMentions(t, ws, "ACS1474-CAUSE-A") {
		t.Errorf("no cycle artifact in %s names the provisioning cause %q — the record keeps only the downstream source-phase refusal", ws, cause)
	}
}

// TestC1474_004_ProvisioningCauseDrivesFailureDigestIdentity — read through the
// REAL assembler: the identity must be content-bearing, must separate two
// different causes, and must be deterministic for the same cause (otherwise the
// identical-fingerprint breaker cannot see recurrence).
func TestC1474_004_ProvisioningCauseDrivesFailureDigestIdentity(t *testing.T) {
	digestOf := func(cause string) core.FailureDigest {
		t.Helper()
		cycle, ws := runProvisionCycle(t, errors.New(cause))
		d, err := core.AssembleFailureDigest(cycle, ws, nil)
		if err != nil {
			t.Fatalf("AssembleFailureDigest(%s): %v", ws, err)
		}
		return d
	}

	a := digestOf("ACS1474-CAUSE-A: git worktree add rc=255 (lock held)")
	if a.Fingerprint == "" {
		t.Fatal("digest fingerprint is empty — no identity to recur on")
	}
	if a.Unexplained {
		t.Errorf("digest for a failed provisioning is content-free (%q) — the cause never reached the identity", a.Fingerprint)
	}
	if b := digestOf("ACS1474-CAUSE-B: git worktree add rc=128 (already checked out)"); a.Fingerprint == b.Fingerprint {
		t.Errorf("two DIFFERENT provisioning causes collapsed to one fingerprint %q", a.Fingerprint)
	}
	if again := digestOf("ACS1474-CAUSE-A: git worktree add rc=255 (lock held)"); again.Fingerprint != a.Fingerprint {
		t.Errorf("the same cause minted two fingerprints (%q vs %q) — a non-deterministic identity never trips the breaker", a.Fingerprint, again.Fingerprint)
	}
}

// TestC1474_005_CleanProvisioningAddsNoFabricatedFailure is the negative axis
// and the anti-gaming guard: an implementation that unconditionally stamps a
// provisioning reason would satisfy 003/004 while inventing a failure on every
// healthy cycle.
func TestC1474_005_CleanProvisioningAddsNoFabricatedFailure(t *testing.T) {
	cycle, ws := runProvisionCycle(t, nil)

	for _, token := range []string{"worktree provisioning failed", "provisioning cause", "rc=255"} {
		if workspaceMentions(t, ws, token) {
			t.Errorf("a SUCCESSFUL provision left %q in the cycle record — no failure may be fabricated on a healthy cycle", token)
		}
	}
	d, err := core.AssembleFailureDigest(cycle, ws, nil)
	if err != nil {
		t.Fatalf("AssembleFailureDigest: %v", err)
	}
	if !d.Unexplained {
		t.Errorf("a cleanly-provisioned cycle produced a content-bearing failure identity (%q) — the cause path fires with no cause", d.Fingerprint)
	}
}
