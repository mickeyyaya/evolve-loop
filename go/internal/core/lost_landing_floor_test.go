package core

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

func TestDetectLostLanding_RealCycle1536Landed(t *testing.T) {
	ws := realCycleWorkspace(t, "cycle-1536")
	if sig := detectLostLanding(ws, VerdictPASS); sig != nil {
		t.Fatalf("cycle-1536 hit the SAME error code and DID land (ship-binding commit adcbddb2); it must not be flagged: %+v", sig)
	}
}

func TestDetectLostLanding_CycleThatNeverShippedIsNotFlagged(t *testing.T) {
	if sig := detectLostLanding(t.TempDir(), VerdictPASS); sig != nil {
		t.Fatalf("a cycle with no ship artifacts at all did not lose a landing: %+v", sig)
	}
}

func TestDetectLostLanding_OnlyShippingVerdictsAreFlagged(t *testing.T) {
	ws := realCycleWorkspace(t, "cycle-1535")
	for _, v := range []string{VerdictFAIL, VerdictWARN, VerdictSKIPPED} {
		if sig := detectLostLanding(ws, v); sig != nil {
			t.Fatalf("verdict %s already reports non-shipping; must not be flagged: %+v", v, sig)
		}
	}
}

// filepath.Join("", "ship-error.json") is "ship-error.json", i.e. a read
// from the process CWD, so without the guard an empty workspace silently
// inspects the working directory. Deliberately not t.Parallel — it chdirs.
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

func TestDetectLostLanding_DoesNotHaltTheBatch(t *testing.T) {
	sig := detectLostLanding(realCycleWorkspace(t, "cycle-1535"), VerdictPASS)
	if sig == nil {
		t.Fatalf("expected a signal")
	}
	if sig.Halt {
		t.Fatalf("a landing race must not halt the batch; the verdict correction is the remedy")
	}
}

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
