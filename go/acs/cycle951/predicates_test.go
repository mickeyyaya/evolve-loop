//go:build acs

// Package cycle951 materializes the cycle-951 acceptance criteria for this
// fleet lane's sole committed task, coherence-reconcile-selfheal (triage top_n
// id / inbox id: clean-exit-late-write-verdict-race-TRUE-root-cause).
//
// Root cause (LIVE-proven in the inbox item): the bridge declares a phase's
// clean exit before Claude Code finishes its post-turn async writes. The
// runner's ~3s reconcile-settle window is far shorter than the observed 60-90s
// deliverable dribble, so the runner records FAIL while the audit-report is
// still landing. Minutes later the file is valid and green on disk, and the
// ADR-0072 verdict-coherence floor (coherence.CheckVerdictCoherence, consulted
// from core.detectVerdictIncoherence) sees recorded=FAIL vs on-disk audit=PASS
// + acs=PASS and HALTS the loop as a "forged verdict" — when it was a benign
// timing race (cycles 930/931/932/cycle-3 family).
//
// Fix (this cycle): make the coherence floor SELF-HEAL instead of halt when the
// recorded-negative is contradicted by green artifacts AND the audit-report
// passes the FULL deliverable.Verify chain (challenge-token + required sections
// + ADR-0039 failure-context) — a benign clean-exit-late-write race reconciles
// to PASS. Halt is PRESERVED as the fallback for any case where the deliverable
// does NOT fully verify (genuine forgery / a malformed report merely tagged with
// a PASS sentinel) — the anti-gaming boundary the inbox explicitly must-preserve.
//
// SUT surface the Builder must implement (see test-report.md handoff), WITHOUT
// modifying this file:
//
//	coherence.VerdictInputs gains:  DeliverableValid bool
//	    // the on-disk audit-report passed the FULL deliverable.Verify chain
//	    // (challenge-token + required sections + ADR-0039 failure-context),
//	    // NOT just the cheap ParseVerdictSentinel read.
//	coherence.Coherence gains:      Reconciled bool
//	    // the recorded negative was a benign clean-exit-late-write race:
//	    // green artifacts AND a fully-valid deliverable → self-heal to PASS,
//	    // do NOT halt. Mutually exclusive with Incoherent.
//	coherence.CheckVerdictCoherence: within the existing forgery-signature branch
//	    (rec FAIL/WARN, AuditRan, !SubstantiveError, audit==PASS, acs==PASS):
//	        DeliverableValid==true  → Coherence{Reconciled:true}   (Incoherent=false, no halt)
//	        DeliverableValid==false → Coherence{Incoherent:true, Category:"verdict-incoherence"} (halt, as today)
//	    Every non-signature case (PASS recorded, SubstantiveError, !AuditRan,
//	    audit!=PASS, acs absent) returns the zero Coherence{} REGARDLESS of
//	    DeliverableValid — DeliverableValid only ever DOWNGRADES a would-be halt
//	    to a reconcile; it never manufactures a reconcile out of a coherent case.
//	core.detectVerdictIncoherence: compute DeliverableValid by running the FULL
//	    deliverable.Verify / VerifyCatalogAware on the on-disk audit-report
//	    artifact (NOT ReadCycleVerdicts's sentinel parse) and feed it into
//	    VerdictInputs; on Reconciled, the recorded final verdict is updated to
//	    PASS and NO halt signal is returned.
//
// Predicate style (cycle-85 rule): every predicate EXERCISES the system under
// test — the direct-call predicates (001-003) invoke coherence.CheckVerdictCoherence
// and assert on its returned struct; the subprocess predicates (004-005) require
// an explicit "--- PASS: <name>" for each named unit test (rename/skip/vacuous-run
// gaming is caught — exit 0 alone never satisfies a predicate); the build/vet/-race
// predicates (006-008) run the toolchain; 009 runs the SSOT eval quality checker.
// No source-grep predicate exists in this file.
//
// Adversarial diversity (skills/adversarial-testing §6):
//
//	NEGATIVE  → C951_002 (deliverable INVALID → forged still HALTS; the strongest
//	            anti-no-op signal: a no-op that always reconciles fails here).
//	EDGE/OOD  → C951_003 (coherent cases must NOT reconcile even with a valid
//	            deliverable: PASS recorded, SubstantiveError, !AuditRan, acs absent).
//	SEMANTIC  → reconcile, halt, and no-op-coherent are three DISTINCT outcomes,
//	            each asserted separately, not one behavior restated.
//
// AC map (1:1 with the disposition table in test-report.md):
//
//	AC1 valid deliverable → reconcile-to-PASS, no halt → C951_001 (pure) · C951_004 (unit) · C951_005 (call-site)
//	AC2 invalid deliverable → forged still halts       → C951_002 (pure/NEGATIVE) · C951_004 (unit) · C951_005 (call-site)
//	AC1/AC2 no over-reconcile of coherent cases        → C951_003 (pure/EDGE)
//	AC3 -race green, no regression                     → C951_006 (scoped -race) · C951_007 (build) · C951_008 (vet)
//	AC4 ship-guard/pane/teardown untouched             → manual+checklist (Auditor diff-scope) — see test-report.md
//	Step 6b eval file passes quality-check             → C951_009
package cycle951

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/coherence"
	"github.com/mickeyyaya/evolve-loop/go/internal/evalqualitycheck"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	coherencePkg = "github.com/mickeyyaya/evolve-loop/go/internal/coherence"
	corePkg      = "github.com/mickeyyaya/evolve-loop/go/internal/core"
	modulePkg    = "github.com/mickeyyaya/evolve-loop/go/..."

	taskSlug = "coherence-reconcile-selfheal"
)

// signatureInputs returns the forgery-signature VerdictInputs (recorded negative,
// audit ran, no substantive error, both artifacts PASS) — the ONLY branch on
// which DeliverableValid changes the outcome. rec is FAIL or WARN.
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
// explicit "--- PASS: <name>" for every wantPass — exit 0 alone never satisfies
// a predicate (a renamed, skipped, or never-authored test yields no PASS line).
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
// AC1 (pure) — a valid deliverable reconciles the late-write race to PASS.
// -----------------------------------------------------------------------------

// TestC951_001_ValidDeliverableReconciles is the happy path: the forgery
// signature (recorded FAIL/WARN, both artifacts green) with a FULLY-VALID
// deliverable is a benign clean-exit-late-write race → Reconciled, NOT halted.
// Exercises coherence.CheckVerdictCoherence directly for both negative verdicts.
func TestC951_001_ValidDeliverableReconciles(t *testing.T) {
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
// AC2 (pure / NEGATIVE) — an invalid deliverable is genuine forgery: still HALT.
// -----------------------------------------------------------------------------

// TestC951_002_InvalidDeliverableStillHalts is the anti-gaming boundary: the
// SAME green-artifact signature but with a deliverable that does NOT fully
// verify (missing challenge-token / required section / ADR-0039 failure-context,
// or a malformed report merely tagged with a PASS sentinel) must STILL be
// Incoherent → halt, exactly as before the fix. A no-op that always reconciles
// (launders every FAIL to PASS) fails here — this is the strongest anti-no-op
// signal in the suite.
func TestC951_002_InvalidDeliverableStillHalts(t *testing.T) {
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
// AC1/AC2 (pure / EDGE) — DeliverableValid never manufactures a reconcile out of
// a genuinely-coherent case; it only ever downgrades a would-be halt.
// -----------------------------------------------------------------------------

// TestC951_003_ValidDeliverableNeverManufacturesReconcile pins the boundary from
// the other side: every case that is coherent WITHOUT the deliverable check (a
// recorded PASS, a substantive-error-explained negative, an audit that never
// ran, an absent ACS artifact) must remain the zero Coherence{} — neither
// Incoherent NOR Reconciled — even when DeliverableValid=true. Reconcile is
// strictly a downgrade of the forgery signature, never a new positive path.
func TestC951_003_ValidDeliverableNeverManufacturesReconcile(t *testing.T) {
	cases := []struct {
		name string
		in   coherence.VerdictInputs
	}{
		{"recorded PASS", coherence.VerdictInputs{Recorded: "PASS", Audit: "PASS", ACS: "PASS", AuditRan: true, DeliverableValid: true}},
		{"substantive error explains negative", coherence.VerdictInputs{Recorded: "FAIL", Audit: "PASS", ACS: "PASS", AuditRan: true, SubstantiveError: true, DeliverableValid: true}},
		{"audit never ran", coherence.VerdictInputs{Recorded: "FAIL", Audit: "", ACS: "PASS", AuditRan: false, DeliverableValid: true}},
		{"acs artifact absent", coherence.VerdictInputs{Recorded: "FAIL", Audit: "PASS", ACS: "", AuditRan: true, DeliverableValid: true}},
	}
	for _, c := range cases {
		coh := coherence.CheckVerdictCoherence(c.in)
		if coh.Reconciled {
			t.Errorf("%s: Reconciled=true — a coherent case must not reconcile just because the deliverable is valid (%+v)", c.name, coh)
		}
		if coh.Incoherent {
			t.Errorf("%s: Incoherent=true — coherent case must stay coherent (%+v)", c.name, coh)
		}
	}
}

// -----------------------------------------------------------------------------
// AC1/AC2 (coherence unit tests) — the Builder's table-tests for reconcile and
// forged-still-halts must be present and GREEN (durable regression twins).
// -----------------------------------------------------------------------------

// TestC951_004_CoherenceUnitTestsGreen requires the two named coherence unit
// tests the scout's verifiableBy calls for to exist and PASS by name.
func TestC951_004_CoherenceUnitTestsGreen(t *testing.T) {
	runGoTest(t, coherencePkg,
		"^TestCheckVerdictCoherence_(Reconcile|ForgedStillHalts)$",
		[]string{
			"TestCheckVerdictCoherence_Reconcile",
			"TestCheckVerdictCoherence_ForgedStillHalts",
		})
}

// -----------------------------------------------------------------------------
// AC1/AC2 (call-site) — the core floor computes DeliverableValid from the FULL
// deliverable.Verify chain, not the cheap sentinel, and reconciles/halts on it.
// -----------------------------------------------------------------------------

// TestC951_005_CallSiteUsesFullVerify requires the core-package unit test proving
// detectVerdictIncoherence derives DeliverableValid from the real
// deliverable.Verify / VerifyCatalogAware (challenge-token + required sections +
// ADR-0039) — so a PASS-sentinel-tagged but malformed audit-report still halts,
// and a fully-valid one reconciles to PASS with no halt signal.
func TestC951_005_CallSiteUsesFullVerify(t *testing.T) {
	runGoTest(t, corePkg,
		"^TestDetectVerdictIncoherence_ReconcileUsesFullVerify$",
		[]string{"TestDetectVerdictIncoherence_ReconcileUsesFullVerify"})
}

// -----------------------------------------------------------------------------
// AC3 — the changed logic is race-clean (scoped to the touched tests to avoid
// the whole-integration-suite flake trap, cycles 858/859/862).
// -----------------------------------------------------------------------------

// TestC951_006_ChangedLogicRaceClean runs the coherence + detect tests under the
// race detector. Scoped by -run to the changed surface (NOT the whole core
// package, which would drag in the fleet-soak integration flake) — a
// deterministic, no-fixture race gate over exactly the modified logic.
func TestC951_006_ChangedLogicRaceClean(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-race", "-count=1",
		"-run", "^Test(CheckVerdictCoherence|DetectVerdictIncoherence)",
		coherencePkg, corePkg)
	if code != 0 || err != nil {
		t.Fatalf("go test -race (coherence+detect) exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			code, err, stdout, stderr)
	}
}

// -----------------------------------------------------------------------------
// AC3/AC4 — the whole module still builds (new field + input broke no caller).
// -----------------------------------------------------------------------------

// TestC951_007_ModuleBuilds asserts the new DeliverableValid input, Reconciled
// output, and the rewired call site broke no caller across the module.
func TestC951_007_ModuleBuilds(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "build", modulePkg)
	if code != 0 || err != nil {
		t.Fatalf("go build %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			modulePkg, code, err, stdout, stderr)
	}
}

// TestC951_008_VetTouchedPackagesClean vets the two touched packages.
func TestC951_008_VetTouchedPackagesClean(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "vet", coherencePkg, corePkg)
	if code != 0 || err != nil {
		t.Fatalf("go vet %s %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			coherencePkg, corePkg, code, err, stdout, stderr)
	}
}

// -----------------------------------------------------------------------------
// Step 6b — the durable eval file exists and passes the SSOT quality checker
// with a NON-EMPTY command set (a missing/empty command block PASSes vacuously,
// the existence-check gaming the eval gate forbids).
// -----------------------------------------------------------------------------

// TestC951_009_EvalFilePassesQualityCheck runs evalqualitycheck.Check on the
// task's durable eval and requires overall PASS with >=2 classified commands.
func TestC951_009_EvalFilePassesQualityCheck(t *testing.T) {
	root := acsassert.RepoRoot(t)
	evalPath := filepath.Join(root, ".evolve", "evals", taskSlug+".md")
	res, err := evalqualitycheck.Check(evalqualitycheck.Options{Path: evalPath})
	if err != nil {
		t.Fatalf("eval quality-check %s: %v", evalPath, err)
	}
	if res.Overall != evalqualitycheck.LevelPass {
		for _, c := range res.Commands {
			if c.Level != evalqualitycheck.LevelPass {
				t.Errorf("eval command %q classified level %d: %s", c.Line, c.Level, c.Reason)
			}
		}
		t.Fatalf("eval %s overall level %d, want PASS(0)", taskSlug, res.Overall)
	}
	if len(res.Commands) < 2 {
		t.Errorf("eval %s classified only %d command(s) — a vacuous eval is not a PASS", taskSlug, len(res.Commands))
	}
}
