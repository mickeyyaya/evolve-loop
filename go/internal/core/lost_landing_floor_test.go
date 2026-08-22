package core

// lost_landing_floor_test.go — a cycle that lost its landing must not report PASS.
//
// Live incident (wave-20260822a-verify, 2026-08-22). Two lanes raced one main:
//
//	cycle-1536  ship-error GIT_FLEET_REBASE_NEEDED  +  ship-binding commit=adcbddb2  → LANDED
//	cycle-1535  ship-error GIT_FLEET_REBASE_NEEDED  +  no ship-binding at all        → LOST
//
// The recovery machinery behaved CORRECTLY for both: 1535's rebase hit a genuine
// non-derived conflict on .evolve/evals/pipeline-replay-contract-boundary.md and
// routed to the debugger, exactly as designed. What went wrong is downstream of
// that — cycle-1535 closed out with final_verdict PASS. Its audit passed, and
// nothing at cycle close ever asks whether the cycle's own ship actually landed:
// finalizeOutcome only reclassifies a SKIPPED verdict, so a PASS from audit
// stands whether or not ship succeeded.
//
// The cost is not cosmetic. A lost landing that reports PASS makes a zero-ship
// wave read as a builder-quality problem when it is really a landing race, which
// is the first thing the zero-ship halt protocol tells an operator to rule out.
//
// The fixtures are the REAL artifacts from both cycles, because the whole point
// is that the two are distinguishable ONLY by the presence of ship-binding.json.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// realCycleWorkspace copies a vendored cycle's ship artifacts into a temp
// workspace, so the detector reads the same bytes the live cycle wrote.
func realCycleWorkspace(t *testing.T, cycleDir string) string {
	t.Helper()
	ws := t.TempDir()
	src := filepath.Join("testdata", "lostlanding", cycleDir)
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatalf("read fixture dir %s: %v", src, err)
	}
	for _, e := range entries {
		b, rerr := os.ReadFile(filepath.Join(src, e.Name()))
		if rerr != nil {
			t.Fatalf("read fixture %s: %v", e.Name(), rerr)
		}
		if werr := os.WriteFile(filepath.Join(ws, e.Name()), b, 0o644); werr != nil {
			t.Fatalf("write fixture %s: %v", e.Name(), werr)
		}
	}
	return ws
}

// THE headline regression: cycle-1535's real artifacts must not read as a PASS.
func TestDetectLostLanding_RealCycle1535IsNotAPass(t *testing.T) {
	ws := realCycleWorkspace(t, "cycle-1535")
	sig := detectLostLanding(ws, VerdictPASS)
	if sig == nil {
		t.Fatalf("cycle-1535 shipped nothing (ship-error present, ship-binding absent) and must not close PASS")
	}
	if sig.Level != "system" {
		t.Fatalf("a landing destroyed by the pipeline is system-class, not a task failure; got level %q", sig.Level)
	}
	if !strings.Contains(sig.Evidence, "GIT_FLEET_REBASE_NEEDED") {
		t.Fatalf("the evidence must name the ship-error code an operator has to act on; got %q", sig.Evidence)
	}
}

// The other half of the SAME race must stay clean — otherwise the detector
// would convert every contended wave into a wall of false alarms.
func TestDetectLostLanding_RealCycle1536Landed(t *testing.T) {
	ws := realCycleWorkspace(t, "cycle-1536")
	if sig := detectLostLanding(ws, VerdictPASS); sig != nil {
		t.Fatalf("cycle-1536 hit the SAME error code and DID land (ship-binding commit adcbddb2); it must not be flagged: %+v", sig)
	}
}

// A cycle that never reached ship (no ship-error, no ship-binding) is not a lost
// landing — scout-only and audit-FAIL cycles must stay untouched.
func TestDetectLostLanding_CycleThatNeverShippedIsNotFlagged(t *testing.T) {
	if sig := detectLostLanding(t.TempDir(), VerdictPASS); sig != nil {
		t.Fatalf("a cycle with no ship artifacts at all did not lose a landing: %+v", sig)
	}
}

// Only a SHIPPING verdict can lose a landing. A cycle already reporting FAIL or
// WARN is telling the truth and must not be re-flagged.
func TestDetectLostLanding_OnlyShippingVerdictsAreFlagged(t *testing.T) {
	ws := realCycleWorkspace(t, "cycle-1535")
	for _, v := range []string{VerdictFAIL, VerdictWARN, VerdictSKIPPED} {
		if sig := detectLostLanding(ws, v); sig != nil {
			t.Fatalf("verdict %s already reports non-shipping; must not be flagged: %+v", v, sig)
		}
	}
}

// An empty workspace path (unit paths, dry runs) must be inert — and inert for
// the RIGHT reason. filepath.Join("", "ship-error.json") is "ship-error.json",
// i.e. a read from the process CWD, so without the guard an empty workspace
// silently inspects whatever happens to be lying in the working directory. That
// is not hypothetical: a mutation run earlier the same day left a stray record
// file in a package directory and broke an unrelated suite. Deliberately NOT
// t.Parallel — it chdirs.
func TestDetectLostLanding_NoWorkspaceDoesNotReadTheProcessCWD(t *testing.T) {
	decoy := t.TempDir()
	if err := os.WriteFile(filepath.Join(decoy, "ship-error.json"),
		[]byte(`{"code":"GIT_FLEET_REBASE_NEEDED","class":"transient","message":"decoy in CWD"}`), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if cerr := os.Chdir(decoy); cerr != nil {
		t.Fatalf("chdir: %v", cerr)
	}
	defer func() {
		if rerr := os.Chdir(prev); rerr != nil {
			t.Fatalf("restore cwd: %v", rerr)
		}
	}()

	if sig := detectLostLanding("", VerdictPASS); sig != nil {
		t.Fatalf("an empty workspace must not fall back to reading the process CWD; got %+v", sig)
	}
}

// The signal must not HALT the batch. The recovery machinery handled the
// conflict correctly and siblings are still working; halting a whole wave every
// time two lanes contend would be a worse failure than the one being reported.
// Correcting the verdict is the fix; stopping the batch is a policy call this
// floor deliberately does not make.
func TestDetectLostLanding_DoesNotHaltTheBatch(t *testing.T) {
	sig := detectLostLanding(realCycleWorkspace(t, "cycle-1535"), VerdictPASS)
	if sig == nil {
		t.Fatalf("expected a signal")
	}
	if sig.Halt {
		t.Fatalf("a landing race must not halt the batch; the verdict correction is the remedy")
	}
}

// The corrected verdict must be one the dossier accepts (PASS|WARN|FAIL) and
// must NOT count as shipped throughput.
func TestLostLandingVerdict_IsLegalAndNotShipping(t *testing.T) {
	got := lostLandingVerdict()
	switch got {
	case VerdictPASS, VerdictWARN, VerdictFAIL:
	default:
		t.Fatalf("dossier validation accepts only PASS|WARN|FAIL; got %q", got)
	}
	if IsShippingVerdict(got) {
		t.Fatalf("a cycle whose landing was lost must not count as shipped throughput; got %q", got)
	}
}

// THE WIRING TEST. The detector above is correct in isolation; this proves it
// FIRES from the real cycle-close path. A floor nothing calls is the same defect
// class it was written to catch.
func TestFinalizeCycle_LostLandingDowngradesTheVerdictAndRecordsTheSignal(t *testing.T) {
	ws := realCycleWorkspace(t, "cycle-1535")
	o := &Orchestrator{storage: &fakeUpdaterStorage{}, gitHEAD: func() (string, error) { return "same-head", nil }}
	result := &CycleResult{FinalVerdict: VerdictPASS}

	if _, err := o.finalizeCycle(context.Background(), CycleState{WorkspacePath: ws}, 1535, "same-head", "", result, &State{}, nil); err != nil {
		t.Fatalf("finalizeCycle: %v", err)
	}

	if result.FinalVerdict == VerdictPASS {
		t.Fatalf("a cycle that shipped nothing must not close PASS through the real finalize path")
	}
	if result.SystemFailure == nil || result.SystemFailure.Category != "landing-lost" {
		t.Fatalf("the lost landing must be recorded as a system signal; got %+v", result.SystemFailure)
	}
	if IsShippingVerdict(result.FinalVerdict) {
		t.Fatalf("the corrected verdict must not count as throughput; got %q", result.FinalVerdict)
	}
}

// The sibling that DID land must pass through finalizeCycle untouched — the
// no-regression half, through the same real path.
func TestFinalizeCycle_LandedCycleIsUnchanged(t *testing.T) {
	ws := realCycleWorkspace(t, "cycle-1536")
	o := &Orchestrator{storage: &fakeUpdaterStorage{}, gitHEAD: func() (string, error) { return "same-head", nil }}
	result := &CycleResult{FinalVerdict: VerdictPASS}

	if _, err := o.finalizeCycle(context.Background(), CycleState{WorkspacePath: ws}, 1536, "same-head", "", result, &State{}, nil); err != nil {
		t.Fatalf("finalizeCycle: %v", err)
	}
	if result.FinalVerdict != VerdictPASS || result.SystemFailure != nil {
		t.Fatalf("a landed cycle must be untouched; got verdict=%q sysfail=%+v", result.FinalVerdict, result.SystemFailure)
	}
}
