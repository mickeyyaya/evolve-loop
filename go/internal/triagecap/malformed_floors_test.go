package triagecap

// malformed_floors_test.go — RED tests for cycle-308 task
// `companion-malformed-must-surface` (inbox item 2026-06-12T16-13-51Z; cycle-305
// audit M1).
//
// Both CommittedFloorCount (floors.go) and DeferredFloorPackagesDecl
// (deferred.go) read the triage-decision.json companion with the pattern
// `if declared, ok, err := Read...; err == nil && ok { ... }`. When err != nil
// (the companion is present but its JSON is malformed) the count SILENTLY falls
// through to the prose scanner — indistinguishable from a missing file. A
// corrupt companion that the agent THINKS is governing the cycle therefore has
// zero effect with no signal.
//
// The fix separates three cases:
//
//	absent file            → silent fallback (backward compat)
//	present, field absent  → silent fallback (backward compat)
//	present-but-malformed  → SURFACE a non-empty correction string carrying the
//	                         JSON parse error
//
// New API this file pins (Builder implements):
//
//	MalformedCommittedFloorWarning(companionPath string) string   (floors.go)
//	MalformedDeferredFloorWarning(companionPath string) string    (deferred.go)
//
// — each returns a non-empty parse-error detail ONLY when the companion is
// present-but-malformed; "" for absent file, absent field, and well-formed.
// CommittedFloorCount / DeferredFloorPackagesDecl keep their fail-open prose
// fallback unchanged (no regression). writeCompanion lives in
// declarative_floors_test.go (same package).

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeMalformedCompanion writes a triage-decision.json whose JSON is corrupt.
func writeMalformedCompanion(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, triageDecisionFile)
	if err := os.WriteFile(path, []byte(`{ "committed_floors": [ this is not json`), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// ---------------------------------------------------------------------------
// committed_floors (floors.go)
// ---------------------------------------------------------------------------

// TestCommittedFloorCount_MalformedFieldSurfaces: a present-but-malformed
// companion must SURFACE via MalformedCommittedFloorWarning (non-empty, naming
// the parse failure), while a well-formed companion stays silent (""). The
// well-formed assertion is the anti-no-op guard: a function that always returned
// non-empty would fail it. CommittedFloorCount itself still fails open to prose.
func TestCommittedFloorCount_MalformedFieldSurfaces(t *testing.T) {
	wantProse := CountCommittedFloors(proseFloors3, knownPkgsFixture) // 3
	if wantProse != 3 {
		t.Fatalf("fixture precondition: prose count = %d, want 3", wantProse)
	}

	t.Run("malformed companion surfaces a non-empty warning", func(t *testing.T) {
		comp := writeMalformedCompanion(t, t.TempDir())
		warn := MalformedCommittedFloorWarning(comp)
		if warn == "" {
			t.Fatal("present-but-malformed companion must surface a non-empty warning (not silently fall through)")
		}
		// The parse error detail must be carried so the agent can fix the JSON.
		if !strings.Contains(strings.ToLower(warn), "committed_floors") {
			t.Errorf("warning must name the committed_floors companion; got %q", warn)
		}
	})

	t.Run("well-formed companion is silent (anti-no-op)", func(t *testing.T) {
		comp := writeCompanion(t, t.TempDir(), []string{"clihealth", "ledger"})
		if warn := MalformedCommittedFloorWarning(comp); warn != "" {
			t.Errorf("well-formed companion must NOT surface a warning; got %q", warn)
		}
	})

	t.Run("count still fails open to prose on malformed companion", func(t *testing.T) {
		comp := writeMalformedCompanion(t, t.TempDir())
		if got := CommittedFloorCount(proseFloors3, comp, knownPkgsFixture); got != wantProse {
			t.Errorf("CommittedFloorCount = %d, want %d (prose fallback preserved)", got, wantProse)
		}
	})
}

// TestCommittedFloorCount_AbsentCompanionFallsBackSilently: an absent companion
// (and a present companion without the committed_floors field) must NOT surface
// a warning and must fall back to the prose count — backward compatibility.
func TestCommittedFloorCount_AbsentCompanionFallsBackSilently(t *testing.T) {
	wantProse := CountCommittedFloors(proseFloors3, knownPkgsFixture) // 3

	t.Run("absent file is silent", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), triageDecisionFile) // not created
		if warn := MalformedCommittedFloorWarning(missing); warn != "" {
			t.Errorf("absent companion must be silent; got %q", warn)
		}
		if got := CommittedFloorCount(proseFloors3, missing, knownPkgsFixture); got != wantProse {
			t.Errorf("CommittedFloorCount = %d, want %d (prose fallback on absent companion)", got, wantProse)
		}
	})

	t.Run("present file without the field is silent", func(t *testing.T) {
		comp := writeCompanion(t, t.TempDir(), nil) // {"cycle":304,"top_n":[]}
		if warn := MalformedCommittedFloorWarning(comp); warn != "" {
			t.Errorf("companion without committed_floors must be silent (not malformed); got %q", warn)
		}
		if got := CommittedFloorCount(proseFloors3, comp, knownPkgsFixture); got != wantProse {
			t.Errorf("CommittedFloorCount = %d, want %d (prose fallback when field absent)", got, wantProse)
		}
	})
}

// ---------------------------------------------------------------------------
// deferred_floors (deferred.go)
// ---------------------------------------------------------------------------

// (writeDeferredCompanion — a well-formed deferred_floors companion writer —
// already lives in deferred_test.go and is reused here.)

// TestDeferredFloorPackagesDecl_MalformedFieldSurfaces: same three-case
// separation for the deferred_floors companion read.
func TestDeferredFloorPackagesDecl_MalformedFieldSurfaces(t *testing.T) {
	candidates := []string{"clihealth", "ledger"}
	artifact := "## deferred\n- coverage-clihealth: raise clihealth coverage ≥95% — priority=L, source=scout\n"
	wantProse := DeferredFloorPackages(artifact, candidates) // ["clihealth"]
	if len(wantProse) != 1 || wantProse[0] != "clihealth" {
		t.Fatalf("fixture precondition: prose deferred = %v, want [clihealth]", wantProse)
	}

	t.Run("malformed companion surfaces a non-empty warning", func(t *testing.T) {
		comp := writeMalformedCompanion(t, t.TempDir())
		warn := MalformedDeferredFloorWarning(comp)
		if warn == "" {
			t.Fatal("present-but-malformed companion must surface a non-empty deferred warning")
		}
		if !strings.Contains(strings.ToLower(warn), "deferred_floors") {
			t.Errorf("warning must name the deferred_floors companion; got %q", warn)
		}
	})

	t.Run("well-formed companion is silent (anti-no-op)", func(t *testing.T) {
		comp := writeDeferredCompanion(t, t.TempDir(), []string{"clihealth"})
		if warn := MalformedDeferredFloorWarning(comp); warn != "" {
			t.Errorf("well-formed deferred companion must NOT surface a warning; got %q", warn)
		}
	})

	t.Run("decl still fails open to prose on malformed companion", func(t *testing.T) {
		comp := writeMalformedCompanion(t, t.TempDir())
		got := DeferredFloorPackagesDecl(artifact, comp, candidates)
		if len(got) != 1 || got[0] != "clihealth" {
			t.Errorf("DeferredFloorPackagesDecl = %v, want [clihealth] (prose fallback preserved)", got)
		}
	})
}

// TestDeferredFloorPackagesDecl_AbsentFieldFallsBackSilently: absent file and
// present-without-field must be silent and fall back to the prose scanner.
func TestDeferredFloorPackagesDecl_AbsentFieldFallsBackSilently(t *testing.T) {
	candidates := []string{"clihealth", "ledger"}
	artifact := "## deferred\n- coverage-clihealth: raise clihealth coverage ≥95% — priority=L, source=scout\n"

	t.Run("absent file is silent", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), triageDecisionFile) // not created
		if warn := MalformedDeferredFloorWarning(missing); warn != "" {
			t.Errorf("absent companion must be silent; got %q", warn)
		}
		got := DeferredFloorPackagesDecl(artifact, missing, candidates)
		if len(got) != 1 || got[0] != "clihealth" {
			t.Errorf("DeferredFloorPackagesDecl = %v, want [clihealth] (prose fallback on absent)", got)
		}
	})

	t.Run("present file without the field is silent", func(t *testing.T) {
		comp := writeCompanion(t, t.TempDir(), nil) // no deferred_floors field
		if warn := MalformedDeferredFloorWarning(comp); warn != "" {
			t.Errorf("companion without deferred_floors must be silent (not malformed); got %q", warn)
		}
		got := DeferredFloorPackagesDecl(artifact, comp, candidates)
		if len(got) != 1 || got[0] != "clihealth" {
			t.Errorf("DeferredFloorPackagesDecl = %v, want [clihealth] (prose fallback when field absent)", got)
		}
	})
}
