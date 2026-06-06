package phasespec

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverUserSpecsFromRoots_ConcatPreservesRootOrder(t *testing.T) {
	rootA := t.TempDir()
	rootB := t.TempDir()
	writeUserPhase(t, rootA, "alpha-check", `{"name":"alpha-check","optional":true}`)
	writeUserPhase(t, rootB, "beta-check", `{"name":"beta-check","optional":true}`)

	specs, sources, warnings := DiscoverUserSpecsFromRoots([]string{rootA, rootB})
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	if len(specs) != 2 || specs[0].Name != "alpha-check" || specs[1].Name != "beta-check" {
		t.Fatalf("specs = %+v, want alpha-check then beta-check", specs)
	}
	if sources["alpha-check"] != rootA || sources["beta-check"] != rootB {
		t.Errorf("sources = %v", sources)
	}
}

func TestDiscoverUserSpecsFromRoots_CollisionLeftmostWins(t *testing.T) {
	rootA := t.TempDir()
	rootB := t.TempDir()
	writeUserPhase(t, rootA, "dup-check", `{"name":"dup-check","optional":true,"description":"from A"}`)
	writeUserPhase(t, rootB, "dup-check", `{"name":"dup-check","optional":true,"description":"from B"}`)

	specs, sources, warnings := DiscoverUserSpecsFromRoots([]string{rootA, rootB})
	if len(specs) != 1 {
		t.Fatalf("want 1 spec after dedupe, got %d", len(specs))
	}
	if specs[0].Description != "from A" {
		t.Errorf("left-most root must win; got %q", specs[0].Description)
	}
	if sources["dup-check"] != rootA {
		t.Errorf("provenance must point at the winning root; got %q", sources["dup-check"])
	}
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "dup-check") && strings.Contains(w, rootB) {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a shadowing warning naming dup-check and the losing root; got %v", warnings)
	}
}

func TestDiscoverUserSpecsFromRoots_MissingRootSkipped(t *testing.T) {
	rootA := t.TempDir()
	writeUserPhase(t, rootA, "only-check", `{"name":"only-check","optional":true}`)
	ghost := filepath.Join(t.TempDir(), "does-not-exist")

	specs, _, warnings := DiscoverUserSpecsFromRoots([]string{ghost, rootA})
	if len(warnings) != 0 {
		t.Fatalf("missing root must be fail-open (no warnings): %v", warnings)
	}
	if len(specs) != 1 || specs[0].Name != "only-check" {
		t.Fatalf("specs = %+v", specs)
	}
}

func TestDiscoverUserSpecsFromRoots_Empty(t *testing.T) {
	specs, sources, warnings := DiscoverUserSpecsFromRoots(nil)
	if specs != nil || len(sources) != 0 || warnings != nil {
		t.Errorf("nil roots → nothing; got %v / %v / %v", specs, sources, warnings)
	}
}

// Per-root malformed-JSON warnings must surface through the multi-root path.
func TestDiscoverUserSpecsFromRoots_PropagatesPerRootWarnings(t *testing.T) {
	rootA := t.TempDir()
	writeUserPhase(t, rootA, "broken", `{not json`)

	_, _, warnings := DiscoverUserSpecsFromRoots([]string{rootA})
	if len(warnings) != 1 || !strings.Contains(warnings[0], "malformed JSON") {
		t.Errorf("warnings = %v", warnings)
	}
}
