//go:build acs

// Package cycle573 materialises the cycle-573 acceptance criteria for the three
// triage-committed top_n tasks (see scout-report.md / triage-report.md):
//
//   - Task 1 memo-tier-envelope-fix (inbox 0.95 critical) — align the memo
//     phase's model-tier pin with its profile's model_tier_envelope, config-only.
//   - Task 2 changedpkgs-from-git (inbox 0.96 critical) — replace the extinct
//     LLM handoff-build.json changed-package source with deterministic git, so
//     the apicover CI-parity gate stops failing open.
//   - Task 3 dossier-commit-rollback (inbox 0.84 medium) — on a permanent
//     commit failure, unstage the dossier pair so it can't pollute the next
//     cycle's tree-diff guard.
//
// Predicate strategy: behavioural-via-subprocess (the cycle-549…566 precedent).
// Each predicate shells `go test -run` over the RED unit tests authored this
// cycle in the internal packages. None is a source-grep — every one exercises
// the system under test (ValidatePin over shipped config, FromGit over a real
// git repo, changedPackagesForAudit over a real git repo, commitPairGit over a
// real failing git commit) and asserts on its result. RED now: the changedpkgs
// package fails to compile (FromGit undefined) and the policy/audit/dossier
// assertions fail against current behaviour. GREEN once all three land.
package cycle573

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	policyPkg      = "github.com/mickeyyaya/evolve-loop/go/internal/policy"
	changedpkgsPkg = "github.com/mickeyyaya/evolve-loop/go/internal/changedpkgs"
	auditPkg       = "github.com/mickeyyaya/evolve-loop/go/internal/phases/audit"
	dossierPkg     = "github.com/mickeyyaya/evolve-loop/go/internal/dossier"
)

// runGoTest shells `go test -run '^(<pattern>)$' -count=1 <pkg>` and reports
// whether it exited cleanly plus the combined output. -count=1 defeats the test
// cache so the predicate always exercises current source. A compile failure in
// the target package (e.g. undefined FromGit) surfaces as a non-zero exit — the
// intended RED signal before Builder implements.
func runGoTest(t *testing.T, pkg, pattern string) (ok bool, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", "^("+pattern+")$", "-count=1", pkg)
	out = stdout + stderr
	// code < 0 is a genuine launch failure (binary missing / killed by signal),
	// not a test verdict; SubprocessOutput returns non-nil err for ANY non-zero
	// exit, so a plain compile/assertion failure (code 1/2 — the RED signal) must
	// flow through as ok=false, NOT be misread as "failed to launch".
	if code < 0 {
		t.Fatalf("go test failed to launch for %s (%s): code=%d err=%v\n%s", pkg, pattern, code, err, out)
	}
	return code == 0, out
}

// TestC573_001_MemoPinWithinEnvelope — Task 1 AC-1a/1b: the shipped memo pin's
// model tier satisfies (and lands inside the rank band of) the memo profile's
// model_tier_envelope. Drives the shipped-config resolver tests in
// internal/policy.
func TestC573_001_MemoPinWithinEnvelope(t *testing.T) {
	ok, out := runGoTest(t, policyPkg, "TestMemoPin_WithinShippedEnvelope|TestMemoPin_TierRankMatchesEnvelope")
	if !ok {
		t.Errorf("memo pin/envelope drift not resolved in shipped config (config-only fix, no Go literals):\n%s", out)
	}
}

// TestC573_002_EnvelopeEnforcementIntact — Task 1 AC-1c (anti-no-op): the fix
// must not be achieved by gutting envelope enforcement; a fast pin under a
// balanced-only envelope must still be rejected. Drives the negative test in
// internal/policy.
func TestC573_002_EnvelopeEnforcementIntact(t *testing.T) {
	ok, out := runGoTest(t, policyPkg, "TestValidatePin_StillRejectsOutOfEnvelope")
	if !ok {
		t.Errorf("envelope enforcement gutted — ValidatePin no longer rejects an out-of-envelope pin:\n%s", out)
	}
}

// TestC573_003_ChangedPkgsFromGit — Task 2 AC-2a/2b/2c: changedpkgs.FromGit
// deterministically derives changed Go packages from git (detects a new
// package, ignores non-Go and no-op changes), with no handoff dependency. RED
// now: FromGit is undefined → the package does not compile. Drives the FromGit
// tests in internal/changedpkgs.
func TestC573_003_ChangedPkgsFromGit(t *testing.T) {
	ok, out := runGoTest(t, changedpkgsPkg, "TestFromGit_DetectsChangedGoPackage|TestFromGit_NoChangesEmpty|TestFromGit_IgnoresNonGoChanges")
	if !ok {
		t.Errorf("changedpkgs.FromGit missing or wrong — deterministic git-derived changed-package source not in place:\n%s", out)
	}
}

// TestC573_004_ApicoverGateNoLongerFailOpen — Task 2 AC-2d (integration): the
// audit phase's changedPackagesForAudit detects a git-changed package even with
// NO handoff file present, so the apicover CI-parity gate stops failing open.
// Drives the audit-package integration test.
func TestC573_004_ApicoverGateNoLongerFailOpen(t *testing.T) {
	ok, out := runGoTest(t, auditPkg, "TestChangedPackagesForAudit_GitDerivedNoHandoff")
	if !ok {
		t.Errorf("changedPackagesForAudit still fail-open on missing handoff (apicover gate silently no-ops):\n%s", out)
	}
}

// TestC573_005_DossierRollbackOnPermanentFailure — Task 3 AC-3a: a permanent
// dossier commit failure leaves the index empty (staged pair rolled back) and
// still returns the original error. Drives the rollback test in internal/dossier.
func TestC573_005_DossierRollbackOnPermanentFailure(t *testing.T) {
	ok, out := runGoTest(t, dossierPkg, "TestCommitPairGit_RollsBackStagedOnPermanentFailure")
	if !ok {
		t.Errorf("dossier commit does not roll back the staged pair on permanent failure (next-cycle tree-diff pollution):\n%s", out)
	}
}
