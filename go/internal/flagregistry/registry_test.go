package flagregistry

import (
	"regexp"
	"strings"
	"testing"
)

// L2.1 (concurrency-factory plan): the EVOLVE_* flag SSOT. The registry is
// metadata-only — it documents every flag on every surface (go + bash); it
// does NOT funnel env reads through config.Load (subprocess-reads-env is a
// deliberate architecture property).

func TestAll_NonEmptyAndWellFormed(t *testing.T) {
	if len(All) < 250 {
		t.Fatalf("registry has %d flags; the 2026-06 inventory counted 271 — registry is not complete", len(All))
	}
	// A real flag name never ends in '_' and never has '__' — those shapes
	// are grep artifacts (bash interpolations like EVOLVE_${PHASE}_MODEL and
	// doc wildcard prefixes like "EVOLVE_BUILDER_*").
	nameRE := regexp.MustCompile(`^EVOLVE(_[A-Z0-9]+)+$`)
	seen := map[string]bool{}
	for _, f := range All {
		if !nameRE.MatchString(f.Name) {
			t.Errorf("malformed flag name %q", f.Name)
		}
		if seen[f.Name] {
			t.Errorf("duplicate registry entry %q", f.Name)
		}
		seen[f.Name] = true
		switch f.Status {
		case StatusActive, StatusDeprecated, StatusDead, StatusInternal, StatusTestSeam:
		default:
			t.Errorf("%s: invalid status %q", f.Name, f.Status)
		}
		if f.Status == StatusDeprecated && f.ReplacedBy == "" && !strings.Contains(f.Doc, "remov") {
			// Deprecated flags should say what replaces them or when they go.
			t.Logf("note: deprecated %s has no ReplacedBy and no removal note", f.Name)
		}
	}
}

func TestAll_SortedByName(t *testing.T) {
	for i := 1; i < len(All); i++ {
		if All[i-1].Name >= All[i].Name {
			t.Fatalf("registry not sorted at %q >= %q — keep it sorted for stable generation", All[i-1].Name, All[i].Name)
		}
	}
}

// TestLookup_SpotChecks pins known flags against ground truth (the
// L2.1 acceptance: spot-check vs grep).
func TestLookup_SpotChecks(t *testing.T) {
	tests := []struct {
		name       string
		wantStatus Status
	}{
		{"EVOLVE_EVAL_GATE", StatusActive},
		{"EVOLVE_CONTRACT_GATE", StatusActive},
		{"EVOLVE_TRIAGE_CAP_GATE", StatusActive}, // added this wave (R9.2)
		{"EVOLVE_PHASE_RECOVERY", StatusActive},
		{"EVOLVE_CHANNEL", StatusDeprecated}, // ADR-0045 I6 fold
		{"EVOLVE_SANDBOX", StatusActive},
	}
	for _, tt := range tests {
		f, ok := Lookup(tt.name)
		if !ok {
			t.Errorf("Lookup(%q): missing from registry", tt.name)
			continue
		}
		if f.Status != tt.wantStatus {
			t.Errorf("%s status = %q, want %q", tt.name, f.Status, tt.wantStatus)
		}
	}
}

func TestLookup_Miss(t *testing.T) {
	if _, ok := Lookup("EVOLVE_NO_SUCH_FLAG_EVER"); ok {
		t.Error("Lookup must miss on unknown flags")
	}
}

// TestRenderIndex_StableAndComplete: the markdown index the `evolve flags
// generate` command projects into control-flags.md covers every flag and is
// deterministic (sorted input ⇒ byte-stable output).
func TestRenderIndex_StableAndComplete(t *testing.T) {
	out := RenderIndex()
	if RenderIndex() != out {
		t.Fatal("RenderIndex is not deterministic")
	}
	for _, f := range All {
		if !strings.Contains(out, "`"+f.Name+"`") {
			t.Errorf("rendered index missing %s", f.Name)
		}
	}
	if !strings.Contains(out, "| Flag | Status |") {
		t.Error("rendered index missing table header")
	}
}
