package triagecap

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

const triageDecisionFile = "triage-decision.json"

// writeCompanion omits committed_floors for a nil slice and writes it for a non-nil, possibly empty, one.
func writeCompanion(t *testing.T, dir string, committedFloors []string) string {
	t.Helper()
	var body string
	if committedFloors == nil {
		body = `{"cycle":304,"top_n":[]}`
	} else {
		quoted := make([]string, len(committedFloors))
		for i, f := range committedFloors {
			quoted[i] = `"` + f + `"`
		}
		body = `{"cycle":304,"committed_floors":[` + strings.Join(quoted, ",") + `]}`
	}
	path := filepath.Join(dir, triageDecisionFile)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// proseFloors3's prose counts three floors, so a declaration can be shown to win in either direction.
const proseFloors3 = "## top_n\n" +
	"- coverage-multi: add tests for swarmrunner, swarmplan, bridge coverage ≥98% — priority=H, source=scout\n"

func TestCountFromDeclaration(t *testing.T) {
	dir := t.TempDir()

	t.Run("exact count from companion", func(t *testing.T) {
		comp := writeCompanion(t, dir, []string{"clihealth", "ledger"})
		declared, ok, err := ReadDeclaredFloors(comp)
		if err != nil {
			t.Fatalf("ReadDeclaredFloors error: %v", err)
		}
		if !ok {
			t.Fatal("companion declares committed_floors — ReadDeclaredFloors must report present=true")
		}
		if len(declared) != 2 {
			t.Errorf("declared floor count = %d, want 2 (clihealth + ledger)", len(declared))
		}
		want := map[string]bool{"clihealth": true, "ledger": true}
		for _, f := range declared {
			if !want[f] {
				t.Errorf("unexpected declared floor %q", f)
			}
		}
	})

	t.Run("declaration overrides higher prose count", func(t *testing.T) {
		comp := writeCompanion(t, dir, []string{"core"})
		got := CommittedFloorCount(proseFloors3, comp, knownPkgsFixture)
		if got != 1 {
			t.Errorf("CommittedFloorCount = %d, want 1 (declaration is primary, not the 3-floor prose)", got)
		}
	})

	t.Run("declared empty array counts zero even with prose floors", func(t *testing.T) {
		comp := writeCompanion(t, dir, []string{})
		got := CommittedFloorCount(proseFloors3, comp, knownPkgsFixture)
		if got != 0 {
			t.Errorf("CommittedFloorCount = %d, want 0 (committed_floors:[] declares zero floors)", got)
		}
	})

	t.Run("cycle-301 honest commitment counts 2 from declaration", func(t *testing.T) {
		artifact := readFixture(t, "triage-cycle301.md")
		comp := writeCompanion(t, dir, []string{"clihealth", "ledger"})
		got := CommittedFloorCount(artifact, comp, knownPkgsFixture)
		if got != 2 {
			t.Errorf("cycle-301 declared count = %d, want 2 (no phantoms — declaration is ground truth)", got)
		}
	})
}

func TestCountFallbackToProse(t *testing.T) {
	wantProse := CountCommittedFloors(proseFloors3, knownPkgsFixture)
	if wantProse != 3 {
		t.Fatalf("fixture precondition: prose count = %d, want 3", wantProse)
	}

	t.Run("missing companion file falls back to prose", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), triageDecisionFile)
		declared, ok, err := ReadDeclaredFloors(missing)
		if err != nil {
			t.Errorf("missing companion must NOT error (fail-open), got: %v", err)
		}
		if ok || declared != nil {
			t.Errorf("missing companion must report present=false, got present=%v declared=%v", ok, declared)
		}
		if got := CommittedFloorCount(proseFloors3, missing, knownPkgsFixture); got != wantProse {
			t.Errorf("CommittedFloorCount = %d, want %d (prose fallback on missing companion)", got, wantProse)
		}
	})

	t.Run("companion without committed_floors field falls back to prose", func(t *testing.T) {
		comp := writeCompanion(t, t.TempDir(), nil)
		_, ok, err := ReadDeclaredFloors(comp)
		if err != nil {
			t.Errorf("companion without the field must NOT error, got: %v", err)
		}
		if ok {
			t.Error("companion without committed_floors must report present=false (not declared)")
		}
		if got := CommittedFloorCount(proseFloors3, comp, knownPkgsFixture); got != wantProse {
			t.Errorf("CommittedFloorCount = %d, want %d (prose fallback when field absent)", got, wantProse)
		}
	})

	t.Run("malformed companion JSON fails open to prose", func(t *testing.T) {
		dir := t.TempDir()
		comp := filepath.Join(dir, triageDecisionFile)
		if err := os.WriteFile(comp, []byte("{ this is not json"), 0o644); err != nil {
			t.Fatal(err)
		}
		_, ok, err := ReadDeclaredFloors(comp)
		if err == nil {
			t.Error("malformed companion must surface an error from ReadDeclaredFloors")
		}
		if ok {
			t.Error("malformed companion must report present=false")
		}
		if got := CommittedFloorCount(proseFloors3, comp, knownPkgsFixture); got != wantProse {
			t.Errorf("CommittedFloorCount = %d, want %d (prose fallback on malformed companion)", got, wantProse)
		}
	})
}

func TestFloorDivergenceCorrective(t *testing.T) {
	t.Run("divergence returns satisfiable correction naming the package", func(t *testing.T) {
		artifact := "## top_n\n" +
			"- coverage-gc: cover internal/gc to ≥95% coverage — priority=H, source=scout\n"
		comp := writeCompanion(t, t.TempDir(), []string{"clihealth", "ledger"})
		msg := FloorDivergenceCorrective(artifact, comp, knownPkgsFixture)
		if msg == "" {
			t.Fatal("divergent prose/declaration must yield a non-empty correction")
		}
		if !strings.Contains(msg, "gc") {
			t.Errorf("correction must name the divergent package gc; got: %s", msg)
		}
		if !strings.Contains(msg, "committed_floors") {
			t.Errorf("correction must reference committed_floors so it is satisfiable; got: %s", msg)
		}
	})

	t.Run("agreement returns no correction", func(t *testing.T) {
		artifact := "## top_n\n" +
			"- a: raise clihealth coverage to ≥95% — priority=H, source=scout\n" +
			"- b: raise ledger coverage to ≥95% — priority=H, source=scout\n"
		comp := writeCompanion(t, t.TempDir(), []string{"clihealth", "ledger"})
		if msg := FloorDivergenceCorrective(artifact, comp, knownPkgsFixture); msg != "" {
			t.Errorf("prose and declaration agree — want no correction, got: %s", msg)
		}
	})

	t.Run("no declaration returns no correction", func(t *testing.T) {
		artifact := "## top_n\n- coverage-gc: cover internal/gc to ≥95% coverage\n"
		missing := filepath.Join(t.TempDir(), triageDecisionFile)
		if msg := FloorDivergenceCorrective(artifact, missing, knownPkgsFixture); msg != "" {
			t.Errorf("no companion = nothing to cross-check, want no correction, got: %s", msg)
		}
	})
}

func TestReviewer_UsesDeclaredFloors(t *testing.T) {
	t.Run("honest declaration overrides overpacked prose -> approve", func(t *testing.T) {
		ws := writeTriageWorkspace(t, readFixture(t, "triage-cycle283.md")) // prose = 12 floors
		writeCompanion(t, ws, []string{"clihealth", "ledger"})              // declared = 2 <= cap 7
		r := newTestReviewer(config.StageEnforce, nil, nil)                 // empty window -> K=5, cap=7
		if rr := r.Review(context.Background(), reviewIn(ws)); !rr.Approve {
			t.Errorf("declared 2 floors <= cap 7 must approve despite 12-floor prose; reason: %s", rr.Reason)
		}
	})

	t.Run("over-declared companion overrides lean prose -> reject", func(t *testing.T) {
		ws := writeTriageWorkspace(t, readFixture(t, "triage-cycle281.md")) // prose = 1 floor
		over := make([]string, 12)
		for i := range over {
			over[i] = string(rune('a' + i))
		}
		writeCompanion(t, ws, over)
		r := newTestReviewer(config.StageEnforce, nil, nil)
		rr := r.Review(context.Background(), reviewIn(ws))
		if rr.Approve {
			t.Error("declared 12 floors > cap 7 must reject even though prose is 1 floor (clamp uses declaration)")
		}
	})
}

func TestRecorder_DeclaredFloors(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "triage-report.md"), []byte(proseFloors3), 0o644); err != nil {
		t.Fatal(err)
	}
	writeCompanion(t, ws, []string{"core"})

	rec := Recorder(repoRoot(t))
	var st core.State
	rec(&st, 304, ws)

	if len(st.TriageThroughput) != 1 {
		t.Fatalf("window = %+v, want 1 entry", st.TriageThroughput)
	}
	if e := st.TriageThroughput[0]; e.Cycle != 304 || e.Floors != 1 {
		t.Errorf("recorded entry = %+v, want {Cycle:304 Floors:1} (declared count, not 3-floor prose)", e)
	}
}
