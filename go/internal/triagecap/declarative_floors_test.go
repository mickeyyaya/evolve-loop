package triagecap

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/config"
	"github.com/mickeyyaya/evolveloop/go/internal/core"
)

// declarative_floors_test.go — RED tests for the cycle-304 declarative-floor-
// counter task (ADR-0046 Layer 1; inbox declarative-floor-commitment; phantom-
// floor incident 2026-06-12). The structural fix: floor counting becomes
// DECLARATION-primary (the triage-decision.json companion's committed_floors[])
// rather than prose-regex-primary, with the existing prose counter retained as
// fallback. Declarations are agent-owned ground truth — not constructable by the
// bullet contract's mandated metadata tokens — so they eliminate the phantom-
// floor class (cycle 301: an honest 2-bullet commitment counted 6 because
// evidence=/source=scout/"paths" matched real package basenames, making the
// correction directive unsatisfiable and burning the cycle).
//
// New API this file pins (Builder implements in floors.go):
//   - ReadDeclaredFloors(companionPath) ([]string, bool, error)
//   - CommittedFloorCount(artifact, companionPath, knownPkgs) int   (decl-primary)
//   - FloorDivergenceCorrective(artifact, companionPath, knownPkgs) string
// and the reviewer/recorder switch from CountCommittedFloors (prose) to
// CommittedFloorCount (declaration-primary) using the workspace companion at
// "<workspace>/triage-decision.json".

// triageDecisionFile is the companion filename the readers locate beside the
// triage artifact. Single literal here mirrors the path postship.go reads
// (".evolve/runs/cycle-N/triage-decision.json"); Builder should expose it as a
// package SSOT (e.g. TriageDecisionName()) so callers do not re-spell it.
const triageDecisionFile = "triage-decision.json"

// writeCompanion writes a triage-decision.json companion declaring the given
// committed_floors into dir. A nil slice writes NO committed_floors field (the
// "not declared" case); a non-nil (possibly empty) slice writes the field.
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

// proseFloors3 is a top_n artifact whose PROSE resolves to three committed
// floors (swarmrunner + swarmplan + bridge). Used to prove the declaration
// overrides prose in BOTH directions (decl<prose and decl>prose).
const proseFloors3 = "## top_n\n" +
	"- coverage-multi: add tests for swarmrunner, swarmplan, bridge coverage ≥98% — priority=H, source=scout\n"

// ---------------------------------------------------------------------------
// C1 — Declaration count is exact and PRIMARY (TestCountFromDeclaration)
// ---------------------------------------------------------------------------

// TestCountFromDeclaration pins the structural fix: when the companion declares
// committed_floors, the count is len(declared) — exactly, and regardless of what
// the prose would have counted. This is the property that kills the phantom-
// floor class: the agent's declaration is ground truth.
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
		// Prose would count 3 (swarmrunner+swarmplan+bridge); declaration says 1.
		comp := writeCompanion(t, dir, []string{"core"})
		got := CommittedFloorCount(proseFloors3, comp, knownPkgsFixture)
		if got != 1 {
			t.Errorf("CommittedFloorCount = %d, want 1 (declaration is primary, not the 3-floor prose)", got)
		}
	})

	t.Run("declared empty array counts zero even with prose floors", func(t *testing.T) {
		// An agent explicitly committing zero coverage floors is valid: prose may
		// mention packages in passing, but committed_floors:[] is authoritative.
		comp := writeCompanion(t, dir, []string{})
		got := CommittedFloorCount(proseFloors3, comp, knownPkgsFixture)
		if got != 0 {
			t.Errorf("CommittedFloorCount = %d, want 0 (committed_floors:[] declares zero floors)", got)
		}
	})

	t.Run("cycle-301 honest commitment counts 2 from declaration", func(t *testing.T) {
		// The incident replay: cycle-301 prose paired with its honest declaration.
		artifact := readFixture(t, "triage-cycle301.md")
		comp := writeCompanion(t, dir, []string{"clihealth", "ledger"})
		got := CommittedFloorCount(artifact, comp, knownPkgsFixture)
		if got != 2 {
			t.Errorf("cycle-301 declared count = %d, want 2 (no phantoms — declaration is ground truth)", got)
		}
	})
}

// ---------------------------------------------------------------------------
// C4 — Missing / undeclared / malformed companion falls back to prose
// ---------------------------------------------------------------------------

// TestCountFallbackToProse pins backward compatibility and fail-open behavior:
// with no usable declaration, counting reverts to the existing prose counter
// (CountCommittedFloors). This keeps every pre-existing reviewer/recorder test
// green and means a cycle whose agent has not yet learned to emit committed_floors
// is governed exactly as before.
func TestCountFallbackToProse(t *testing.T) {
	wantProse := CountCommittedFloors(proseFloors3, knownPkgsFixture) // 3
	if wantProse != 3 {
		t.Fatalf("fixture precondition: prose count = %d, want 3", wantProse)
	}

	t.Run("missing companion file falls back to prose", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), triageDecisionFile) // not created
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
		comp := writeCompanion(t, t.TempDir(), nil) // {"cycle":304,"top_n":[]}
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

	// Negative / adversarial: a corrupt companion must fail OPEN to prose, never
	// crash and never silently count zero (which would let an overpacked cycle
	// through by writing garbage).
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

// ---------------------------------------------------------------------------
// C3 — Prose/declaration divergence yields a SATISFIABLE correction (never reject)
// ---------------------------------------------------------------------------

// TestFloorDivergenceCorrective pins the cross-examiner. When prose and
// declaration name different floor packages, it returns a non-empty, satisfiable
// correction that tells the agent how to reconcile (add to committed_floors OR
// remove from prose) — naming the divergent package. Critically it is a
// CORRECTION STRING, never a reject: the prose-primary path's correction was
// unsatisfiable (it demanded removing contract-mandated tokens) and burned the
// cycle. When prose and declaration agree — or no declaration exists — it returns
// "" (no correction; nothing to reconcile).
func TestFloorDivergenceCorrective(t *testing.T) {
	t.Run("divergence returns satisfiable correction naming the package", func(t *testing.T) {
		// Prose floor: gc. Declaration: clihealth+ledger. gc is prose-only.
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
		// Prose floors: clihealth + ledger. Declaration: same two. No divergence.
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
		missing := filepath.Join(t.TempDir(), triageDecisionFile) // not created
		if msg := FloorDivergenceCorrective(artifact, missing, knownPkgsFixture); msg != "" {
			t.Errorf("no companion = nothing to cross-check, want no correction, got: %s", msg)
		}
	})
}

// ---------------------------------------------------------------------------
// C6 — Reviewer (capacity clamp) counts DECLARED floors, not prose
// ---------------------------------------------------------------------------

// TestReviewer_UsesDeclaredFloors pins that the R9.2 capacity clamp reads the
// companion declaration. The overpacked cycle-283 PROSE (12 floors) paired with
// an honest 2-floor declaration must now be APPROVED (declared 2 <= cap 7) — the
// phantom-floor false-reject the incident describes. The converse (lean prose,
// over-declared companion) must REJECT, proving the clamp truly uses the
// declaration in both directions (a no-op that ignored the companion would
// approve the 1-floor prose).
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
			over[i] = string(rune('a' + i)) // 12 distinct declared floors
		}
		writeCompanion(t, ws, over) // declared = 12 > cap 7
		r := newTestReviewer(config.StageEnforce, nil, nil)
		rr := r.Review(context.Background(), reviewIn(ws))
		if rr.Approve {
			t.Error("declared 12 floors > cap 7 must reject even though prose is 1 floor (clamp uses declaration)")
		}
	})
}

// ---------------------------------------------------------------------------
// C7 — Recorder calibrates the throughput window from DECLARED floors
// ---------------------------------------------------------------------------

// TestRecorder_DeclaredFloors pins H2: the rolling throughput window must record
// the DECLARED floor count, not the prose count. Recording phantom prose floors
// poisoned K (K=4 from a true-K=1 cycle, cycle 298). With a declaration of 1 the
// recorded entry must be 1, even though the prose would have counted 3.
func TestRecorder_DeclaredFloors(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "triage-report.md"), []byte(proseFloors3), 0o644); err != nil {
		t.Fatal(err) // prose = 3 floors
	}
	writeCompanion(t, ws, []string{"core"}) // declared = 1

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
