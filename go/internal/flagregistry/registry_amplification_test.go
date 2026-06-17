package flagregistry

import (
	"strings"
	"testing"
)

// Cycle-353 adversarial amplification tests.
// These probe invariants orthogonal to the TDD spot-checks:
//   - Kind and Default metadata (not just Status)
//   - Cluster assignment on promoted flags
//   - Doc quality (no placeholder text, non-empty)
//   - Regression guard for the stale NUDGE_S "0" default
//   - Over-promotion guard for flags explicitly kept StatusInternal
//   - Global cluster invariant for all Active OBSERVER flags

// TestAmplify_ObserverFlagsHaveCorrectKind verifies that each promoted Observer
// flag carries the correct Kind ("int" or "string"). TestLookup_SpotChecks
// only asserts Status — Kind is an untested metadata dimension.
func TestAmplify_ObserverFlagsHaveCorrectKind(t *testing.T) {
	cases := []struct {
		name     string
		wantKind string
	}{
		{"EVOLVE_OBSERVER_STALL_S", "int"},
		{"EVOLVE_OBSERVER_POLL_S", "int"},
		{"EVOLVE_OBSERVER_NUDGE_S", "int"},
		{"EVOLVE_OBSERVER_NUDGE_BODY", "string"},
	}
	for _, tc := range cases {
		f, ok := Lookup(tc.name)
		if !ok {
			t.Errorf("Lookup(%q): missing from registry", tc.name)
			continue
		}
		if f.Kind != tc.wantKind {
			t.Errorf("%s Kind = %q, want %q", tc.name, f.Kind, tc.wantKind)
		}
	}
}

// TestAmplify_ObserverFlagsHaveCorrectDefaults pins the documented defaults for
// all 4 Observer flags. The NUDGE_S default changed 0→300 in cycle-353 (opt-in
// to opt-out), making this the highest-risk field to get wrong.
func TestAmplify_ObserverFlagsHaveCorrectDefaults(t *testing.T) {
	cases := []struct {
		name        string
		wantDefault string
	}{
		{"EVOLVE_OBSERVER_STALL_S", "600"},
		{"EVOLVE_OBSERVER_POLL_S", "5"},
		{"EVOLVE_OBSERVER_NUDGE_S", "300"},
		{"EVOLVE_OBSERVER_NUDGE_BODY", ""},
	}
	for _, tc := range cases {
		f, ok := Lookup(tc.name)
		if !ok {
			t.Errorf("Lookup(%q): missing from registry", tc.name)
			continue
		}
		if f.Default != tc.wantDefault {
			t.Errorf("%s Default = %q, want %q", tc.name, f.Default, tc.wantDefault)
		}
	}
}

// TestAmplify_ObserverFlagsAreInObserverCluster asserts that all 4 promoted
// flags carry Cluster = "Observer". Missing Cluster corrupts the hand-maintained
// control-flags.md table and breaks `evolve flags check` cluster reporting.
func TestAmplify_ObserverFlagsAreInObserverCluster(t *testing.T) {
	const wantCluster = "Observer"
	for _, name := range []string{
		"EVOLVE_OBSERVER_STALL_S",
		"EVOLVE_OBSERVER_POLL_S",
		"EVOLVE_OBSERVER_NUDGE_S",
		"EVOLVE_OBSERVER_NUDGE_BODY",
	} {
		f, ok := Lookup(name)
		if !ok {
			t.Errorf("Lookup(%q): missing from registry", name)
			continue
		}
		if f.Cluster != wantCluster {
			t.Errorf("%s Cluster = %q, want %q", name, f.Cluster, wantCluster)
		}
	}
}

// TestAmplify_ObserverFlagsHaveNonPlaceholderDoc guards against the "promote
// status only" anti-pattern where the StatusInternal inventory placeholder doc
// ("Undocumented production reader (inventory 2026-06-11); classify when
// touched.") is accidentally retained after promotion.
func TestAmplify_ObserverFlagsHaveNonPlaceholderDoc(t *testing.T) {
	const inventoryPlaceholder = "Undocumented production reader"
	for _, name := range []string{
		"EVOLVE_OBSERVER_STALL_S",
		"EVOLVE_OBSERVER_POLL_S",
		"EVOLVE_OBSERVER_NUDGE_S",
		"EVOLVE_OBSERVER_NUDGE_BODY",
	} {
		f, ok := Lookup(name)
		if !ok {
			t.Errorf("Lookup(%q): missing from registry", name)
			continue
		}
		if f.Doc == "" {
			t.Errorf("%s: Doc must not be empty for a StatusActive flag", name)
			continue
		}
		if strings.Contains(f.Doc, inventoryPlaceholder) {
			t.Errorf("%s: Doc still contains StatusInternal inventory placeholder text; real operator-facing doc required", name)
		}
	}
}

// TestAmplify_NudgeSDefaultIsNotLegacyZero is a targeted regression guard for
// the specific bug fixed in cycle-353: EVOLVE_OBSERVER_NUDGE_S previously had
// Default = "0" (opt-in) in both the registry and runtime-reference.md. The
// actual code default is 300 (opt-out; set =0 to disable). Default = "0" must
// never reappear.
func TestAmplify_NudgeSDefaultIsNotLegacyZero(t *testing.T) {
	f, ok := Lookup("EVOLVE_OBSERVER_NUDGE_S")
	if !ok {
		t.Fatal("EVOLVE_OBSERVER_NUDGE_S missing from registry")
	}
	if f.Default == "0" {
		t.Error(`EVOLVE_OBSERVER_NUDGE_S Default = "0" — regression: the correct default is "300" (opt-out semantics); "0" was the stale opt-in default fixed in cycle-353`)
	}
}

// TestAmplify_NudgeSDocMentionsOptOutSemantics verifies that the NUDGE_S doc
// captures the opt-out semantics. Operators relying only on the registry (not
// runtime-reference) must be able to infer that =0 disables the feature.
func TestAmplify_NudgeSDocMentionsOptOutSemantics(t *testing.T) {
	f, ok := Lookup("EVOLVE_OBSERVER_NUDGE_S")
	if !ok {
		t.Fatal("EVOLVE_OBSERVER_NUDGE_S missing from registry")
	}
	doc := strings.ToLower(f.Doc)
	// Doc must mention either the =0 disable mechanism OR the opt-out framing.
	if !strings.Contains(doc, "=0") && !strings.Contains(doc, "opt-out") && !strings.Contains(doc, "disable") {
		t.Errorf("EVOLVE_OBSERVER_NUDGE_S Doc should document =0 disables the nudge (opt-out semantics), got: %q", f.Doc)
	}
}

// TestAmplify_OtherObserverFlagsRemainInternal guards against over-promotion.
// The cycle-353 Scout explicitly kept EVOLVE_OBSERVER_ENABLED,
// EVOLVE_OBSERVER_ENFORCE, and EVOLVE_OBSERVER_EOF_GRACE_S as StatusInternal
// (deferred to a future cycle). Accidental promotion would silently expose
// internal-only flags as operator-facing.
func TestAmplify_OtherObserverFlagsRemainInternal(t *testing.T) {
	for _, name := range []string{
		"EVOLVE_OBSERVER_ENABLED",
		"EVOLVE_OBSERVER_ENFORCE",
		"EVOLVE_OBSERVER_EOF_GRACE_S",
	} {
		f, ok := Lookup(name)
		if !ok {
			// Missing entirely is also acceptable — the flag might not exist yet.
			continue
		}
		if f.Status == StatusActive {
			t.Errorf("%s was promoted to StatusActive but cycle-353 scout explicitly deferred it; revert or open a dedicated cycle", name)
		}
	}
}

// TestAmplify_AllActiveObserverFlagsHaveCluster is a global invariant: any flag
// with "OBSERVER" in its name that is StatusActive must have a non-empty Cluster.
// This catches future Observer flags that are promoted without setting Cluster.
func TestAmplify_AllActiveObserverFlagsHaveCluster(t *testing.T) {
	for _, f := range All {
		if !strings.Contains(f.Name, "OBSERVER") {
			continue
		}
		if f.Status != StatusActive {
			continue
		}
		if f.Cluster == "" {
			t.Errorf("%s: StatusActive Observer flag has empty Cluster; all Observer flags must carry Cluster = \"Observer\"", f.Name)
		}
	}
}
