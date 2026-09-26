package triagecap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeMalformedCompanion(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, triageDecisionFile)
	if err := os.WriteFile(path, []byte(`{ "committed_floors": [ this is not json`), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCommittedFloorCount_MalformedFieldSurfaces(t *testing.T) {
	wantProse := CountCommittedFloors(proseFloors3, knownPkgsFixture)
	if wantProse != 3 {
		t.Fatalf("fixture precondition: prose count = %d, want 3", wantProse)
	}

	t.Run("malformed companion surfaces a non-empty warning", func(t *testing.T) {
		comp := writeMalformedCompanion(t, t.TempDir())
		warn := MalformedCommittedFloorWarning(comp)
		if warn == "" {
			t.Fatal("present-but-malformed companion must surface a non-empty warning (not silently fall through)")
		}
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

func TestCommittedFloorCount_AbsentCompanionFallsBackSilently(t *testing.T) {
	wantProse := CountCommittedFloors(proseFloors3, knownPkgsFixture)

	t.Run("absent file is silent", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), triageDecisionFile)
		if warn := MalformedCommittedFloorWarning(missing); warn != "" {
			t.Errorf("absent companion must be silent; got %q", warn)
		}
		if got := CommittedFloorCount(proseFloors3, missing, knownPkgsFixture); got != wantProse {
			t.Errorf("CommittedFloorCount = %d, want %d (prose fallback on absent companion)", got, wantProse)
		}
	})

	t.Run("present file without the field is silent", func(t *testing.T) {
		comp := writeCompanion(t, t.TempDir(), nil)
		if warn := MalformedCommittedFloorWarning(comp); warn != "" {
			t.Errorf("companion without committed_floors must be silent (not malformed); got %q", warn)
		}
		if got := CommittedFloorCount(proseFloors3, comp, knownPkgsFixture); got != wantProse {
			t.Errorf("CommittedFloorCount = %d, want %d (prose fallback when field absent)", got, wantProse)
		}
	})
}

func TestDeferredFloorPackagesDecl_MalformedFieldSurfaces(t *testing.T) {
	candidates := []string{"clihealth", "ledger"}
	artifact := "## deferred\n- coverage-clihealth: raise clihealth coverage ≥95% — priority=L, source=scout\n"
	wantProse := DeferredFloorPackages(artifact, candidates)
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

func TestDeferredFloorPackagesDecl_AbsentFieldFallsBackSilently(t *testing.T) {
	candidates := []string{"clihealth", "ledger"}
	artifact := "## deferred\n- coverage-clihealth: raise clihealth coverage ≥95% — priority=L, source=scout\n"

	t.Run("absent file is silent", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), triageDecisionFile)
		if warn := MalformedDeferredFloorWarning(missing); warn != "" {
			t.Errorf("absent companion must be silent; got %q", warn)
		}
		got := DeferredFloorPackagesDecl(artifact, missing, candidates)
		if len(got) != 1 || got[0] != "clihealth" {
			t.Errorf("DeferredFloorPackagesDecl = %v, want [clihealth] (prose fallback on absent)", got)
		}
	})

	t.Run("present file without the field is silent", func(t *testing.T) {
		comp := writeCompanion(t, t.TempDir(), nil)
		if warn := MalformedDeferredFloorWarning(comp); warn != "" {
			t.Errorf("companion without deferred_floors must be silent (not malformed); got %q", warn)
		}
		got := DeferredFloorPackagesDecl(artifact, comp, candidates)
		if len(got) != 1 || got[0] != "clihealth" {
			t.Errorf("DeferredFloorPackagesDecl = %v, want [clihealth] (prose fallback when field absent)", got)
		}
	})
}
