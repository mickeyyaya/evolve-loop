package core

// worktree_provision_cause_test.go — cycle-1474 RED contract for
// `worktree-provisioning-cause-fingerprint`.
//
// cyclerun.go:462-464 prints the provisioning error to stderr and deliberately
// continues with an empty ActiveWorktree; the role-gate then denies every
// source phase. That fail-fast is CORRECT and is not what this task touches.
// What is missing is the CAUSE: the git error never reaches the cycle's
// failure-reason / failure-digest surfaces, so the recorded identity is the
// downstream phase refusal ("scout|infra-error|…") and the real, recurring
// provisioning failure is invisible to the identical-fingerprint breaker and to
// anyone reading the digest afterwards.
//
// These tests drive the REAL RunCycle path (o.newCycleRun provisions through
// the injected WorktreeProvisioner) and read the REAL assembler
// (AssembleFailureDigest) over the cycle's own workspace — no reimplementation
// of either. They deliberately do NOT pin a field name or a file name: any
// existing fail-reason/digest seam that carries the cause satisfies them.

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// provisionCycle runs one real cycle with the injected provisioner outcome and
// returns the cycle number and the workspace the run actually used. createErr
// nil ⇒ provisioning succeeds (the control axis).
func provisionCycle(t *testing.T, createErr error) (cycle int, workspace string) {
	t.Helper()
	st := &fakeStorage{state: State{LastCycleNumber: 1473}} // cycle 1474
	wt := &fakeWorktree{path: t.TempDir(), createErr: createErr}
	o := NewOrchestrator(st, &fakeLedger{}, buildRunners(nil), WithWorktreeProvisioner(wt))

	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "g"})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if len(wt.createdCycles) == 0 {
		t.Fatal("precondition: the provisioner was never called — this test is not exercising the provisioning path")
	}
	ws := st.cycleState.WorkspacePath
	if ws == "" {
		t.Fatal("precondition: the cycle recorded no workspace path")
	}
	return res.Cycle, ws
}

// workspaceMentions reports whether ANY artifact the cycle wrote carries the
// token. Format-agnostic on purpose: the requirement is that the cause is
// PERSISTED where the failure surfaces read, not that it lands in one
// particular file.
func workspaceMentions(t *testing.T, workspace, token string) bool {
	t.Helper()
	found := false
	err := filepath.WalkDir(workspace, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || found {
			return nil //nolint:nilerr // unreadable entries are simply not evidence
		}
		b, rerr := os.ReadFile(path)
		if rerr == nil && strings.Contains(string(b), token) {
			found = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk workspace %s: %v", workspace, err)
	}
	return found
}

// TestWorktreeProvisionFailure_PersistsCause is the crux: today the git error
// exists only on the orchestrator's stderr, so nothing durable in the cycle
// record names why the lane had no worktree.
func TestWorktreeProvisionFailure_PersistsCause(t *testing.T) {
	const cause = "ZZ-PROVISION-CAUSE-A: git worktree add rc=255 (lock held)"
	_, ws := provisionCycle(t, errors.New(cause))

	if !workspaceMentions(t, ws, "ZZ-PROVISION-CAUSE-A") {
		t.Errorf("no cycle artifact in %s names the provisioning cause %q — the failure record keeps only the downstream source-phase refusal, which is what misdirects recurrence handling", ws, cause)
	}
}

// TestWorktreeProvisionFailure_DigestUsesProvisioningCause pins the identity
// consequence, through the real assembler: the digest must be content-bearing
// (never the "unexplained" class), must SEPARATE two different provisioning
// causes, and must be DETERMINISTIC for the same cause — otherwise the
// identical-fingerprint breaker cannot see genuine recurrence.
func TestWorktreeProvisionFailure_DigestUsesProvisioningCause(t *testing.T) {
	digestOf := func(cause string) FailureDigest {
		t.Helper()
		cycle, ws := provisionCycle(t, errors.New(cause))
		d, err := AssembleFailureDigest(cycle, ws, nil)
		if err != nil {
			t.Fatalf("AssembleFailureDigest(%s): %v", ws, err)
		}
		return d
	}

	const causeA = "ZZ-PROVISION-CAUSE-A: git worktree add rc=255 (lock held)"
	const causeB = "ZZ-PROVISION-CAUSE-B: git worktree add rc=128 (already checked out)"

	a := digestOf(causeA)
	if a.Unexplained {
		t.Errorf("digest for a failed provisioning is content-free (fingerprint=%q) — the cause never reached the identity, so this cycle fingerprints like any other unexplained infra death", a.Fingerprint)
	}
	if a.Fingerprint == "" {
		t.Fatal("digest fingerprint is empty — no identity to recur on")
	}
	if b := digestOf(causeB); a.Fingerprint == b.Fingerprint {
		t.Errorf("two DIFFERENT provisioning causes collapsed to one fingerprint %q — distinct failures must stay distinguishable", a.Fingerprint)
	}
	if again := digestOf(causeA); again.Fingerprint != a.Fingerprint {
		t.Errorf("the same provisioning cause minted two fingerprints (%q vs %q) — a non-deterministic identity can never trip the recurrence breaker", a.Fingerprint, again.Fingerprint)
	}
}

// TestWorktreeProvisionFailure_SuccessAddsNoFailureReason is the negative axis
// and the anti-gaming guard: a successful provision must add NO failure reason
// at all. An implementation that unconditionally stamps a provisioning reason
// would satisfy the two tests above while fabricating failures on every healthy
// cycle.
func TestWorktreeProvisionFailure_SuccessAddsNoFailureReason(t *testing.T) {
	cycle, ws := provisionCycle(t, nil)

	for _, token := range []string{"worktree provisioning failed", "provisioning cause", "rc=255"} {
		if workspaceMentions(t, ws, token) {
			t.Errorf("a SUCCESSFUL provision left %q in the cycle record — no failure reason may be fabricated on a healthy cycle", token)
		}
	}
	d, err := AssembleFailureDigest(cycle, ws, nil)
	if err != nil {
		t.Fatalf("AssembleFailureDigest: %v", err)
	}
	if !d.Unexplained {
		t.Errorf("a cycle that provisioned cleanly produced a content-bearing failure identity (%q) — the cause path is firing when there is no cause", d.Fingerprint)
	}
}

// TestWorktreeProvisionFailure_EmptyStderrCauseStillIdentifies is the edge
// axis. The live incident shape is rc=255 with NOTHING on stderr, so the cause
// text can be near-empty: the identity must still be non-empty, content-bearing
// and stable, never degrading back to the unexplained class the whole task
// exists to remove.
func TestWorktreeProvisionFailure_EmptyStderrCauseStillIdentifies(t *testing.T) {
	const terse = "rc=255"
	cycle, ws := provisionCycle(t, errors.New(terse))

	d, err := AssembleFailureDigest(cycle, ws, nil)
	if err != nil {
		t.Fatalf("AssembleFailureDigest: %v", err)
	}
	if d.Fingerprint == "" || d.Unexplained {
		t.Errorf("an empty-stderr provisioning failure yielded fingerprint=%q unexplained=%v — want a non-empty, content-bearing identity", d.Fingerprint, d.Unexplained)
	}

	cycle2, ws2 := provisionCycle(t, errors.New(terse))
	d2, err := AssembleFailureDigest(cycle2, ws2, nil)
	if err != nil {
		t.Fatalf("AssembleFailureDigest (second run): %v", err)
	}
	if d2.Fingerprint != d.Fingerprint {
		t.Errorf("terse cause is not stable across runs: %q vs %q", d.Fingerprint, d2.Fingerprint)
	}
}
