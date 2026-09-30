package scopedelta

import "testing"

func TestScope_InScope_DirectoryPrefixesAndExactPaths(t *testing.T) {
	t.Parallel()
	s := Scope{Declared: []string{"go/internal/salvage/extract.go", "docs/research/"}}

	for _, tc := range []struct {
		path string
		want bool
	}{
		{"go/internal/salvage/extract.go", true},
		{"docs/research/README.md", true},
		{"docs/research/deep/notes.md", true},
		{"go/internal/salvage/extract_x.go", false},
		{"docs/researchers/other.md", false},
	} {
		if got := s.InScope(tc.path); got != tc.want {
			t.Errorf("InScope(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

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

func TestSurface_AndEffectUnknown_AreTheHonestDefaults(t *testing.T) {
	t.Parallel()
	var s Surface = SurfaceOf("go/internal/salvage/extract.go")
	if s != SurfaceSubject {
		t.Errorf("SurfaceOf(product code) = %q, want subject", s)
	}
	if (Entry{}).Effect != EffectUnknown {
		t.Error("the zero Effect must be Unknown — an unset field must not read as a claim")
	}
	e := Entry{Path: "go/acs/cycle1/predicates_test.go", Class: ClassDiscovered,
		Disposition: DispositionKeep, Reason: "r", Effect: EffectUnknown}
	if err := Admissible(e); err == nil {
		t.Error("an unknown direction must not be treated as a safe one")
	}
}

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
