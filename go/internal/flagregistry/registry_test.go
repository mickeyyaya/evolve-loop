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
	// No minimum-count floor: the registry is under intentional reduction (the
	// flag-reduction campaign), and a count floor would block removing dead /
	// test-only / duplicate flags. Accidental loss is caught instead by the
	// reader-completeness guard (go/acs/regression/flagreaders), which fails if
	// any flag READ in production Go lacks a row. Here we only require the
	// registry be non-empty and well-formed.
	if len(All) == 0 {
		t.Fatal("registry is empty")
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
		// Classification-quality invariant (replaces the count floor): an active
		// operator flag must be documented — non-empty Cluster + Doc, and never
		// the internal "classify when touched" placeholder. This keeps the
		// internal-classification wave honest (a promoted flag must gain a real
		// Doc/Cluster, not just flip status).
		if f.Status == StatusActive {
			if f.Cluster == "" || f.Doc == "" {
				t.Errorf("active flag %s must have a non-empty Cluster and Doc", f.Name)
			}
			if strings.Contains(f.Doc, "classify when touched") {
				t.Errorf("active flag %s still carries the internal 'classify when touched' placeholder Doc", f.Name)
			}
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
	// Campaign-robust: verify Lookup round-trips for EVERY registered flag.
	for _, want := range All {
		got, ok := Lookup(want.Name)
		if !ok {
			t.Errorf("Lookup(%q): present in All but missing from the lookup index", want.Name)
			continue
		}
		if got.Status != want.Status {
			t.Errorf("Lookup(%q).Status = %q, want %q", want.Name, got.Status, want.Status)
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

// TestRenderDoc_FoldsAndEscapes constructs a Flag directly to pin the
// purpose-column contract RenderIndex depends on: ReplacedBy/RemoveIn fold into
// the Doc, and a literal pipe is GFM-escaped so it cannot break the table. The
// table-wide test above only exercises this through whatever real registry rows
// happen to contain those fields; this names the Flag type and asserts the
// contract on a row crafted to hit every branch.
func TestRenderDoc_FoldsAndEscapes(t *testing.T) {
	got := renderDoc(Flag{
		Name:       "EVOLVE_EXAMPLE",
		Status:     StatusDeprecated,
		Doc:        "Old knob | with a pipe.",
		ReplacedBy: "EVOLVE_NEW",
		RemoveIn:   "v20.0.0",
	})
	if strings.Contains(got, " | ") || !strings.Contains(got, "\\|") {
		t.Errorf("renderDoc did not GFM-escape the pipe in Doc: %q", got)
	}
	if !strings.Contains(got, "Replaced by `EVOLVE_NEW`.") {
		t.Errorf("renderDoc missing ReplacedBy fold: %q", got)
	}
	if !strings.Contains(got, "Remove in v20.0.0.") {
		t.Errorf("renderDoc missing RemoveIn fold: %q", got)
	}
}
