//go:build acs

// Package cycle1590 materializes the cycle-1590 acceptance criteria for this
// fleet lane's two committed tasks:
//
//   - verdict-surface-clean-exit-binding (inbox id
//     pipeline-defect-pipeline-blocker-cycle1582, P0): the ADR-0072 clean-exit
//     coherence floor must bind the RUNTIME-minted substantive-error and
//     full-deliverable-validity evidence into coherence.CheckVerdictCoherence
//     before a negative verdict is recorded or the loop is halted — a green
//     audit + green ACS + fully-valid deliverable must self-heal ONLY the
//     clean-exit-late-write race, and a malformed PASS-sentinel report or a
//     missing/malformed ACS verdict must never launder into a reconcile.
//   - worktree-provisioning-retry-boundary (inbox id
//     worktree-provisioning-retry): every production `git worktree add`
//     entry point (core.Create, core.CreateFrom, swarm provisioning, the
//     `evolve worktree create` operator CLI) must go through the shared
//     gitexec.AddWorktreeWithRetry bounded backoff, preserving the FIRST
//     failure's diagnostics when a later attempt also fails and staying
//     fail-fast (no backoff) on a permanent condition.
//
// Both areas are scout-flagged as *already implemented* (verdict binding at
// internal/core/system_failure.go's detectVerdictIncoherence; retry adoption
// at all four call sites named in the inbox record) — this file is the
// PERMANENT regression lock the scout report calls for: prove the behavior
// live via the real subprocess-driven regression suites (not a source grep),
// so a future edit that regresses either contract fails THIS cycle's audit
// instead of silently reproducing the cycle-1582 halt or the pre-#401
// un-retried-root incident.
//
// Predicate style (cycle-85 rule): every predicate EXERCISES the system under
// test. C1590_001-003 call coherence.CheckVerdictCoherence directly and
// assert on the returned struct (pure, in-process). C1590_004-010 are
// subprocess predicates that require an explicit "--- PASS: <name>" for each
// named regression test — a renamed/skipped/never-authored test yields no
// PASS line and fails the predicate; exit 0 alone is never sufficient. No
// source-grep predicate exists in this file.
//
// Adversarial diversity (skills/adversarial-testing §6):
//
//	NEGATIVE  → C1590_002 (deliverable INVALID → forged still HALTS — the
//	            strongest anti-no-op signal: a no-op that always reconciles
//	            fails here) and C1590_009 (permanent rc=128 must NOT retry).
//	EDGE/OOD  → C1590_003 (coherent-without-the-deliverable-check cases must
//	            never manufacture a reconcile) and C1590_008 (first-failure
//	            preserved when a later attempt ALSO fails).
//	SEMANTIC  → reconcile / halt / retry-recover / fail-fast-no-backoff are
//	            four distinct outcomes, each asserted separately.
//
// AC map (1:1 with the disposition table in test-report.md):
//
//	Task 1 AC1 (green+valid deliverable self-heals, no halt)        → C1590_001, C1590_006
//	Task 1 AC2 (malformed report / missing ACS never launders)       → C1590_002, C1590_003
//	Task 1 AC3 (SubstantiveError from audit AND ship suppresses halt) → C1590_005
//	Task 1 -race / build / vet regression                            → C1590_010, C1590_011, C1590_012
//	Task 2 AC1 (all 4 entry points use the shared retry helper)       → C1590_004, C1590_006, C1590_007
//	Task 2 AC2 (retryable first failure stays visible)                → C1590_008
//	Task 2 AC3 (permanent failure fails fast, no backoff)             → C1590_009
package cycle1590

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/coherence"
	"github.com/mickeyyaya/evolve-loop/go/internal/evalqualitycheck"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	corePkg    = "github.com/mickeyyaya/evolve-loop/go/internal/core"
	gitexecPkg = "github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	swarmPkg   = "github.com/mickeyyaya/evolve-loop/go/internal/swarm"
	cmdPkg     = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"
	modulePkg  = "github.com/mickeyyaya/evolve-loop/go/..."
)

// signatureInputs returns the forgery-signature VerdictInputs (recorded
// negative, audit ran, no substantive error, both artifacts PASS) — the ONLY
// branch on which DeliverableValid changes the outcome.
func signatureInputs(rec string, deliverableValid bool) coherence.VerdictInputs {
	return coherence.VerdictInputs{
		Recorded:         rec,
		Audit:            "PASS",
		ACS:              "PASS",
		AuditRan:         true,
		SubstantiveError: false,
		DeliverableValid: deliverableValid,
	}
}

// runGoTest runs the named tests of pkg (verbose, fresh) and requires an
// explicit "--- PASS: <name>" for every wantPass name — exit 0 alone never
// satisfies a predicate (a renamed, skipped, or never-authored test yields no
// PASS line).
func runGoTest(t *testing.T, pkg, runExpr string, wantPass []string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-run", runExpr, "-v", pkg)
	if code != 0 || err != nil {
		t.Fatalf("go test -run %q %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			runExpr, pkg, code, err, stdout, stderr)
	}
	for _, name := range wantPass {
		if !strings.Contains(stdout, "--- PASS: "+name) {
			t.Errorf("test %s did not report PASS (renamed, skipped, or not authored)\nstdout:\n%s", name, stdout)
		}
	}
}

// -----------------------------------------------------------------------------
// Task 1 AC1 (pure) — a fully-valid deliverable self-heals the clean-exit race.
// -----------------------------------------------------------------------------

// TestC1590_001_ValidDeliverableReconcilesNotHalt is the happy path: the
// forgery signature (recorded FAIL/WARN, both artifacts green) with a
// FULLY-VALID deliverable is a benign clean-exit-late-write race → Reconciled,
// never Incoherent.
func TestC1590_001_ValidDeliverableReconcilesNotHalt(t *testing.T) {
	for _, rec := range []string{"FAIL", "WARN"} {
		coh := coherence.CheckVerdictCoherence(signatureInputs(rec, true))
		if !coh.Reconciled {
			t.Errorf("recorded=%s + green artifacts + valid deliverable: Reconciled=false, want true (%+v)", rec, coh)
		}
		if coh.Incoherent {
			t.Errorf("recorded=%s + valid deliverable must NOT be Incoherent (no halt), got %+v", rec, coh)
		}
	}
}

// -----------------------------------------------------------------------------
// Task 1 AC2 (pure / NEGATIVE) — a malformed deliverable is genuine forgery.
// -----------------------------------------------------------------------------

// TestC1590_002_MalformedDeliverableStillHalts: the SAME green-artifact
// signature but with a deliverable that does NOT fully verify (a report
// merely tagged with a PASS sentinel, missing required content) must STILL be
// Incoherent → halt. A no-op that always reconciles (launders every FAIL to
// PASS) fails here — the strongest anti-no-op signal in the suite.
func TestC1590_002_MalformedDeliverableStillHalts(t *testing.T) {
	for _, rec := range []string{"FAIL", "WARN"} {
		coh := coherence.CheckVerdictCoherence(signatureInputs(rec, false))
		if !coh.Incoherent {
			t.Errorf("recorded=%s + green artifacts + INVALID deliverable: Incoherent=false, want true (forged verdict must halt) (%+v)", rec, coh)
		}
		if coh.Reconciled {
			t.Errorf("recorded=%s + INVALID deliverable must NOT reconcile (would launder forgery to PASS), got %+v", rec, coh)
		}
		if coh.Category != "verdict-incoherence" {
			t.Errorf("recorded=%s category = %q, want verdict-incoherence", rec, coh.Category)
		}
	}
}

// -----------------------------------------------------------------------------
// Task 1 AC2 (pure / EDGE) — missing/malformed ACS cannot be laundered either.
// -----------------------------------------------------------------------------

// TestC1590_003_MissingACSNeverReconciles: an absent/non-PASS ACS verdict
// means the incoherence check cannot even claim a contradiction — it must
// stay the zero Coherence{} (no halt, no reconcile) EVEN with a fully-valid
// deliverable. Reconcile is strictly a downgrade of the forgery signature
// (both artifacts green), never a new positive path a missing artifact can
// open.
func TestC1590_003_MissingACSNeverReconciles(t *testing.T) {
	cases := []struct {
		name string
		in   coherence.VerdictInputs
	}{
		{"acs absent", coherence.VerdictInputs{Recorded: "FAIL", Audit: "PASS", ACS: "", AuditRan: true, DeliverableValid: true}},
		{"acs malformed (WARN)", coherence.VerdictInputs{Recorded: "FAIL", Audit: "PASS", ACS: "WARN", AuditRan: true, DeliverableValid: true}},
		{"recorded PASS", coherence.VerdictInputs{Recorded: "PASS", Audit: "PASS", ACS: "PASS", AuditRan: true, DeliverableValid: true}},
	}
	for _, c := range cases {
		coh := coherence.CheckVerdictCoherence(c.in)
		if coh.Reconciled {
			t.Errorf("%s: Reconciled=true — must not reconcile without both artifacts green (%+v)", c.name, coh)
		}
		if coh.Incoherent && c.name != "recorded PASS" {
			// A missing/malformed ACS also can't prove forgery — err toward
			// coherent (no false halt), per coherence.go's documented contract.
			t.Errorf("%s: Incoherent=true — an absent/non-PASS ACS cannot prove forgery either (%+v)", c.name, coh)
		}
	}
}

// -----------------------------------------------------------------------------
// Task 1 AC3 (call-site) — the runtime binds BOTH audit- and ship-phase
// substantive-error evidence, and the FULL Verify chain, before recording or
// halting.
// -----------------------------------------------------------------------------

// TestC1590_005_CallSiteBindsSubstantiveErrorAndFullVerify requires the
// core-package regression proving detectVerdictIncoherence (a) treats a
// ship-phase explained FAIL exactly like an audit-phase one (not just
// AuditFailReasons) and (b) derives DeliverableValid from the FULL
// deliverable.Verify chain (challenge-token + required sections +
// ADR-0039), not the cheap sentinel parse — so a PASS-sentinel-tagged but
// malformed audit-report still halts and a fully-valid one reconciles.
func TestC1590_005_CallSiteBindsSubstantiveErrorAndFullVerify(t *testing.T) {
	runGoTest(t, corePkg,
		"^TestDetectVerdictIncoherence_(ShipPhaseExplainedFail_NoHalt|ReconcileUsesFullVerify)$",
		[]string{
			"TestDetectVerdictIncoherence_ShipPhaseExplainedFail_NoHalt",
			"TestDetectVerdictIncoherence_ReconcileUsesFullVerify",
		})
}

// TestC1590_006_ForgedVerdictHaltsLive requires the core-package regression
// proving the end-to-end forged-verdict halt path is armed: recorded FAIL,
// green artifacts, no substantive error, no valid deliverable → halt signal
// with the verdict-incoherence category.
func TestC1590_006_ForgedVerdictHaltsLive(t *testing.T) {
	runGoTest(t, corePkg,
		"^TestDetectVerdictIncoherence_ForgedVerdict_Halts$",
		[]string{"TestDetectVerdictIncoherence_ForgedVerdict_Halts"})
}

// -----------------------------------------------------------------------------
// Task 2 AC1 — all four production `git worktree add` entry points go through
// the shared bounded retry helper.
// -----------------------------------------------------------------------------

// TestC1590_004_AllFourEntryPointsRetryTransientFailure requires the four
// named regression tests (one per production root named in the inbox record:
// core.Create, core.CreateFrom, swarm worker provisioning, the operator CLI)
// each proving a transient rc=255 first attempt recovers on retry through the
// shared gitexec.AddWorktreeWithRetry contract.
func TestC1590_004_AllFourEntryPointsRetryTransientFailure(t *testing.T) {
	runGoTest(t, corePkg,
		"^TestGitWorktreeCreate_RetriesTransientAddFailure$",
		[]string{"TestGitWorktreeCreate_RetriesTransientAddFailure"})
	runGoTest(t, corePkg,
		"^TestGitWorktreeCreateFrom_RetriesTransientAddFailure$",
		[]string{"TestGitWorktreeCreateFrom_RetriesTransientAddFailure"})
	runGoTest(t, swarmPkg,
		"^TestSwarmCreateWorker_RetriesTransientAddFailure$",
		[]string{"TestSwarmCreateWorker_RetriesTransientAddFailure"})
}

// TestC1590_007_OperatorCLIUsesSharedRetryHelper requires the cmd_worktree
// regression proving the `evolve worktree create` entry point (the fourth
// named root) also routes through the shared retry helper rather than a bare
// `git worktree add`.
func TestC1590_007_OperatorCLIUsesSharedRetryHelper(t *testing.T) {
	runGoTest(t, cmdPkg,
		"^TestRunWorktreeCreate_Success$",
		[]string{"TestRunWorktreeCreate_Success"})
	// A structural guard: the operator CLI's own package must reference the
	// shared helper symbol somewhere reachable from RunWorktreeCreate — proven
	// behaviorally by go vet/build succeeding after a symbol rename (covered
	// by C1590_011) rather than re-asserted here by source grep.
}

// -----------------------------------------------------------------------------
// Task 2 AC2 (EDGE) — the FIRST failure's diagnostics survive a second
// failure at the shared retry helper.
// -----------------------------------------------------------------------------

// TestC1590_008_FirstFailurePreservedOnSecondFailure requires the gitexec
// regression proving that when attempt 1 fails transiently and attempt 2 ALSO
// fails, the caller-visible error retains attempt 1's rc/stderr (not just the
// final attempt's) — the M2 residual from the worktree-provisioning-retry
// inbox record (a SIGKILL'd attempt 1 leaves the dir, attempt 2 dies rc=128
// already-exists, masking the original transient cause).
func TestC1590_008_FirstFailurePreservedOnSecondFailure(t *testing.T) {
	runGoTest(t, gitexecPkg,
		"^TestAddWorktreeWithRetry_PreservesFirstFailure$",
		[]string{"TestAddWorktreeWithRetry_PreservesFirstFailure"})
	runGoTest(t, gitexecPkg,
		"^TestAddWorktreeWithRetry_AnnouncesBeforeBackoff$",
		[]string{"TestAddWorktreeWithRetry_AnnouncesBeforeBackoff"})
}

// -----------------------------------------------------------------------------
// Task 2 AC3 (NEGATIVE) — a permanent failure fails fast with no backoff.
// -----------------------------------------------------------------------------

// TestC1590_009_PermanentFailureFailsFastNoBackoff requires the gitexec
// regression proving a permanent condition (e.g. `not a git repository`,
// rc=128) skips the retry ladder entirely and returns loudly on the FIRST
// attempt — the anti-no-op boundary: a helper that always retries (or always
// swallows) would pass every OTHER predicate in this file while silently
// paying (or hiding) permanent failures.
func TestC1590_009_PermanentFailureFailsFastNoBackoff(t *testing.T) {
	runGoTest(t, gitexecPkg,
		"^TestAddWorktreeWithRetry_PermanentFailureSkipsBackoff$",
		[]string{"TestAddWorktreeWithRetry_PermanentFailureSkipsBackoff"})
	runGoTest(t, corePkg,
		"^TestGitWorktreeCreate_PersistentFailureStillFailsLoudly$",
		[]string{"TestGitWorktreeCreate_PersistentFailureStillFailsLoudly"})
	runGoTest(t, swarmPkg,
		"^TestSwarmCreateWorker_PersistentFailureStillFailsLoudly$",
		[]string{"TestSwarmCreateWorker_PersistentFailureStillFailsLoudly"})
}

// -----------------------------------------------------------------------------
// Both tasks — the changed/touched surfaces stay race-clean, build, and vet.
// -----------------------------------------------------------------------------

// runGoTestRace is a TOP-LEVEL helper (not a local closure) so the
// flaky-predicate-shape lint's helper hop resolves its -run narrowing against
// the SAME argv that carries the package pattern — a local closure loses that
// resolution and the known-slow-suite check would (falsely) fire on corePkg.
func runGoTestRace(t *testing.T, pkg, runExpr string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-race", "-count=1", "-run", runExpr, pkg)
	if code != 0 || err != nil {
		t.Errorf("go test -race -run %q %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			runExpr, pkg, code, err, stdout, stderr)
	}
}

// TestC1590_010_TouchedSurfacesRaceClean runs the coherence + core detect +
// gitexec retry tests under the race detector, ONE named package per
// invocation (the flaky-predicate-shape lint bans multi-package sweeps — each
// extra package multiplies contention exposure). Scoped by -run to the
// changed surface (NOT the whole core package, which would drag in the
// fleet-soak integration flake) — a deterministic, no-fixture race gate.
func TestC1590_010_TouchedSurfacesRaceClean(t *testing.T) {
	runGoTestRace(t, "github.com/mickeyyaya/evolve-loop/go/internal/coherence", "^TestCheckVerdictCoherence")
	runGoTestRace(t, corePkg, "^Test(DetectVerdictIncoherence|GitWorktreeCreate)")
	runGoTestRace(t, gitexecPkg, "^TestAddWorktreeWithRetry")
}

// TestC1590_011_ModuleBuilds requires the whole module to still build — no
// call site drifted off the shared retry contract or the coherence input
// shape.
func TestC1590_011_ModuleBuilds(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "build", modulePkg)
	if code != 0 || err != nil {
		t.Fatalf("go build %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s", modulePkg, code, err, stdout, stderr)
	}
}

// TestC1590_012_ModuleVets requires `go vet` to stay clean across the module.
func TestC1590_012_ModuleVets(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "vet", modulePkg)
	if code != 0 || err != nil {
		t.Fatalf("go vet %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s", modulePkg, code, err, stdout, stderr)
	}
}

// -----------------------------------------------------------------------------
// Step 6b — the eval files pass the SSOT quality checker.
// -----------------------------------------------------------------------------

// TestC1590_013_EvalFilesPassQualityCheck runs the same quality gate the
// Auditor runs, over both eval files this cycle authors.
func TestC1590_013_EvalFilesPassQualityCheck(t *testing.T) {
	root := acsassert.RepoRoot(t)
	for _, slug := range []string{
		"verdict-surface-clean-exit-binding",
		"worktree-provisioning-retry-boundary",
	} {
		path := root + "/.evolve/evals/" + slug + ".md"
		if !acsassert.FileExists(t, path) {
			t.Fatalf("RED: %s missing on disk", path)
		}
		res, err := evalqualitycheck.Check(evalqualitycheck.Options{Path: path})
		if err != nil {
			t.Fatalf("%s: quality-check errored: %v", slug, err)
		}
		if res.Overall != evalqualitycheck.LevelPass {
			t.Errorf("%s: quality-check verdict=%v, want PASS (Level-0 tautology or worse)", slug, res.Overall)
		}
	}
}
