//go:build acs

// Package cycle561 materialises the cycle-561 acceptance criteria for the THREE
// `## top_n` tasks triage-report.md committed THIS cycle (NOT scout-report's
// original three — scout picked auditor-egps/memo/sandbox; triage re-ranked the
// live inbox and committed workspace-hygiene-s1, auditor-egps, workspace-hygiene-s3,
// deferring memo + sandbox). Per the AC-Materialization Contract (R9.3 —
// "predicates bind ONLY to triage-committed work") this package predicates ONLY
// those three; the deferred memo/sandbox items get NO predicate here.
//
// READ-FIRST FINDING (AGENTS.md rule 8, surfaced loudly in test-report.md):
// two of the three committed tasks are ALREADY IMPLEMENTED in the worktree base
// (they landed in prior same-goal cycles, commits 5ee210dc / 9e72ac4e):
//
//   - workspace-hygiene-s1: runlease.OwnerLive(l, now, ttl, alive) exists and is
//     consumed at reset.go:149 (SealCycle fence), cmd_loop.go:320 (unfinished
//     guard) and cmd_cycle.go:123 — with a full passing unit suite.
//   - workspace-hygiene-s3: core.deleteCycleBranch (worktree.go:163) and the swarm
//     provisioner mirror already run `git branch -d <leaf>` post-remove, WARN-only,
//     never `-D`, gated on the `cycle-` prefix — with a full passing unit suite.
//
// Their predicates below are therefore behavioural REGRESSION LOCKS (pre-existing
// GREEN, documented as such in test-report.md), driving the REAL unit tests the
// prior cycles authored — not a source grep (the cycle-85 degenerate-predicate
// ban). The only GENUINE RED this cycle is auditor-egps: audit's normal-completion
// path reads red_count but never the authoritative acs-verdict ship_eligible flag,
// so a narrative PASS with ship_eligible:false slips through. TestC561_003 drives
// the RED unit test authored this cycle (audit_normalcompletion_reconcile_test.go).
//
// Predicate strategy: behavioural-via-subprocess (the cycle-549/553/555/557
// precedent) — each predicate shells `go test -run '^Name$' <fullModulePkg>` over
// the compiled SUT and asserts a clean (exit-0) run. Full module import paths so
// the subprocess resolves from the acs/cycle561 test cwd.
package cycle561

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	runleasePkg = "github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	corePkg     = "github.com/mickeyyaya/evolve-loop/go/internal/core"
	swarmPkg    = "github.com/mickeyyaya/evolve-loop/go/internal/swarm"
	auditPkg    = "github.com/mickeyyaya/evolve-loop/go/internal/phases/audit"
)

// runGoTest shells `go test -run '^<pattern>$' -count=1 <pkg>` and returns
// whether it exited cleanly plus the combined output for diagnostics. -count=1
// defeats the test cache so the predicate always exercises the current source.
func runGoTest(t *testing.T, pkg, pattern string) (ok bool, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", "^("+pattern+")$", "-count=1", pkg)
	if err != nil {
		t.Fatalf("go test failed to launch for %s (%s): %v\nstderr:\n%s", pkg, pattern, err, stderr)
	}
	return code == 0, stdout + stderr
}

// TestC561_001_S1_PidAwareLeaseStaleness — workspace-hygiene-s1.
// A dead owner with a still-fresh heartbeat must NOT read as live (that is the
// exact false-live that forced `evolve cycle reset --force` before every batch),
// while a running owner with a fresh heartbeat still does, and legacy callers
// (nil probe / pid 0) fall back to freshness-only. Drives the real runlease
// OwnerLive suite. Pre-existing GREEN (regression lock).
func TestC561_001_S1_PidAwareLeaseStaleness(t *testing.T) {
	ok, out := runGoTest(t, runleasePkg,
		"TestOwnerLive_DeadPidFreshHeartbeatNotLive|TestOwnerLive_FreshAliveOwnerIsLive|TestOwnerLive_StaleAliveOwnerNotLive|TestOwnerLive_NilAlive|TestOwnerLive_Pid0")
	if !ok {
		t.Errorf("runlease.OwnerLive pid-aware-staleness suite is not green — S1 lease liveness regressed:\n%s", out)
	}
}

// TestC561_002_S1_SealFenceConsumesPidAlive — workspace-hygiene-s1.
// The SealCycle fence and the loop's unfinished-cycle guard must consult pid
// liveness (not freshness alone): a dead owner + fresh lease seals WITHOUT
// --force, a live owner + fresh lease still refuses. Drives the real core +
// cmd/evolve pid-fence tests. Pre-existing GREEN (regression lock).
func TestC561_002_S1_SealFenceConsumesPidAlive(t *testing.T) {
	ok, out := runGoTest(t, corePkg,
		"TestSealCycle_DeadOwnerFreshLease.*|TestSealCycle_LiveOwnerFreshLease.*")
	if !ok {
		t.Errorf("SealCycle pid-fence suite is not green — S1 seal fence regressed:\n%s", out)
	}
}

// TestC561_003_AuditorEGPS_NormalCompletionShipEligibleGate — auditor-egps.
// THE GENUINE RED this cycle: on the normal-completion path a narrative PASS with
// acs-verdict ship_eligible:false must be rejected. RED until Builder wires audit
// to read the authoritative ship_eligible field; GREEN thereafter. Also drives
// the genuine-agreement (ship_eligible:true) and legacy-absent (back-compat)
// companions so the fix cannot game the gate by blanket-downgrading.
func TestC561_003_AuditorEGPS_NormalCompletionShipEligibleGate(t *testing.T) {
	ok, out := runGoTest(t, auditPkg,
		"TestRun_NormalCompletion_PassReport_ShipEligibleFalse_RejectsAsUnreconciled|TestRun_NormalCompletion_PassReport_ShipEligibleTrue_StaysPass|TestRun_NormalCompletion_PassReport_ShipEligibleAbsent_StaysPass|TestRun_NormalCompletion_PassReport_RedCountPositive_StaysFail")
	if !ok {
		t.Errorf("audit normal-completion ship_eligible reconciliation gate is not green — a narrative PASS with ship_eligible:false still ships (or the fix broke genuine-agreement/back-compat):\n%s", out)
	}
}

// TestC561_004_S3_InlineBranchDeleteNonForce — workspace-hygiene-s3.
// Cleanup must delete the merged cycle branch via non-force `branch -d` after
// worktree removal, leave an unmerged branch in place (WARN-only), and NEVER
// escalate to `-D` — in both the core provisioner and the swarm mirror. Drives
// the real branch-delete unit suites. Pre-existing GREEN (regression lock).
func TestC561_004_S3_InlineBranchDeleteNonForce(t *testing.T) {
	ok, out := runGoTest(t, corePkg,
		"TestCleanup_DeletesMergedCycleBranch|TestCleanup_UnmergedBranchSurvives_WarnsOnly")
	if !ok {
		t.Errorf("core Cleanup branch-delete suite is not green — S3 in-cycle branch deletion regressed:\n%s", out)
	}
	ok2, out2 := runGoTest(t, swarmPkg,
		"TestCleanup_RoutesGitBranchDeleteThroughSeam|TestCleanup_UnmergedBranch_NeverForceDeletes")
	if !ok2 {
		t.Errorf("swarm Cleanup branch-delete mirror is not green — S3 swarm branch deletion regressed:\n%s", out2)
	}
}
