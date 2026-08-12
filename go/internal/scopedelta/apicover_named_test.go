package scopedelta

// apicover_named_test.go — names AND exercises the exports the behavioural
// suite reaches only indirectly (export-naming floor, ADR-0069). Not ceremony:
// each test below pins a contract a caller depends on, and the repo has burned
// four CI reds this month on new exports that had behavioural coverage from a
// neighbouring package but no test naming them here.

import "testing"

// TestScope_InScope_DirectoryPrefixesAndExactPaths names Scope.InScope and pins
// the matching rule the whole package rests on: a trailing "/" is a directory
// prefix, anything else is an exact path. Getting this wrong in either
// direction is silent — too loose admits a sibling's file as "declared", too
// strict makes a declared file look like a delta.
func TestScope_InScope_DirectoryPrefixesAndExactPaths(t *testing.T) {
	t.Parallel()
	s := Scope{Declared: []string{"go/internal/salvage/extract.go", "docs/research/"}}

	for _, tc := range []struct {
		path string
		want bool
	}{
		{"go/internal/salvage/extract.go", true},    // exact
		{"docs/research/README.md", true},           // under the declared prefix
		{"docs/research/deep/notes.md", true},       // nested under it
		{"go/internal/salvage/extract_x.go", false}, // exact entries never prefix-match
		{"docs/researchers/other.md", false},        // prefix must respect the separator
	} {
		if got := s.InScope(tc.path); got != tc.want {
			t.Errorf("InScope(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

// TestClosureRule_DefaultsImplementTheInterface names ClosureRule and pins that
// the shipped rule set actually satisfies it — the Strategy seam is what keeps
// "is this closure?" a mechanical question, so a rule that silently stopped
// being one would return the decision to persuasion.
func TestClosureRule_DefaultsImplementTheInterface(t *testing.T) {
	t.Parallel()
	rules := DefaultClosureRules()
	if len(rules) == 0 {
		t.Fatal("no closure rules: every out-of-scope path would need adjudication, including a change's own covering test")
	}
	seen := map[string]bool{}
	for _, r := range rules {
		var _ ClosureRule = r
		if r.Name() == "" {
			t.Error("a rule with no name cannot be recorded against the KEEP it licensed")
		}
		if seen[r.Name()] {
			t.Errorf("duplicate rule name %q — the record could not say which rule fired", r.Name())
		}
		seen[r.Name()] = true
	}
}

// TestSurface_AndEffectUnknown_AreTheHonestDefaults names Surface and
// EffectUnknown, and pins the posture behind both: when the direction of a
// change is not mechanically decidable, the record says so rather than
// guessing. A fabricated "tightens" would be worse than silence — it would put
// a confident label on the exact judgement an adjudicator needs to make.
func TestSurface_AndEffectUnknown_AreTheHonestDefaults(t *testing.T) {
	t.Parallel()
	var s Surface = SurfaceOf("go/internal/salvage/extract.go")
	if s != SurfaceSubject {
		t.Errorf("SurfaceOf(product code) = %q, want subject", s)
	}
	// The zero Effect is Unknown, so an entry that never set it cannot be
	// mistaken for a claim about direction.
	if (Entry{}).Effect != EffectUnknown {
		t.Error("the zero Effect must be Unknown — an unset field must not read as a claim")
	}
	// Unknown never buys admission on its own: an unset direction on a signal
	// path still needs corroboration like any other keep.
	e := Entry{Path: "go/acs/cycle1/predicates_test.go", Class: ClassDiscovered,
		Disposition: DispositionKeep, Reason: "r", Effect: EffectUnknown}
	if err := Admissible(e); err == nil {
		t.Error("an unknown direction must not be treated as a safe one")
	}
}

// TestSummary_ZeroValueIsUsable names Summary and pins that a cycle with NO
// out-of-scope work summarises cleanly: the common case must not need a
// special case at the call site, and TaskStatementSuspect must not fire on an
// empty delta (0 misunderstood paths are not evidence about the item).
func TestSummary_ZeroValueIsUsable(t *testing.T) {
	t.Parallel()
	var s Summary = Summarize(nil)
	if s.Kept != 0 || s.Carved != 0 || s.Refused != 0 {
		t.Errorf("empty delta must summarise to zeros, got %+v", s)
	}
	if s.TaskStatementSuspect {
		t.Error("an empty delta is not evidence that the task statement was ambiguous")
	}
	if s.ByClass == nil {
		t.Error("ByClass must be usable without a nil check at every call site")
	}
}
