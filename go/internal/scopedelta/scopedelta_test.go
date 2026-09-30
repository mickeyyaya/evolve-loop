package scopedelta

import (
	"strings"
	"testing"
)

func TestUnaccounted_IsTheWholePoint(t *testing.T) {
	t.Parallel()
	scope := Scope{
		Cycle:      1450,
		Declared:   []string{"go/internal/salvage/extract.go"},
		Protected:  []string{"go/internal/phases/ship/"},
		LaneOthers: []string{"go/internal/router/"},
	}
	cases := []struct {
		name        string
		changed     []string
		adjudicated []Entry
		want        []string
	}{
		{
			name:    "declared paths are not deltas",
			changed: []string{"go/internal/salvage/extract.go"},
			want:    nil,
		},
		{
			name:    "closure is computed, so it needs no adjudication",
			changed: []string{"go/internal/salvage/extract.go", "go/internal/salvage/extract_test.go"},
			want:    nil,
		},
		{
			name:    "an undeclared, unadjudicated path is UNACCOUNTED — the defect this package exists to make impossible",
			changed: []string{"go/internal/salvage/extract.go", "go/internal/router/pick.go"},
			want:    []string{"go/internal/router/pick.go"},
		},
		{
			name:        "an adjudicated path is accounted for, whatever the disposition",
			changed:     []string{"go/internal/salvage/extract.go", "go/internal/router/pick.go"},
			adjudicated: []Entry{{Path: "go/internal/router/pick.go", Class: ClassCrossLane, Disposition: DispositionCarve, Reason: "belongs to the router lane; patch preserved"}},
			want:        nil,
		},
		{
			name:        "a REFUSED path is still accounted for — refusal is a decision, not a disposal",
			changed:     []string{"go/internal/phases/ship/gitops.go"},
			adjudicated: []Entry{{Path: "go/internal/phases/ship/gitops.go", Class: ClassBoundary, Disposition: DispositionRefuse, Reason: "protected surface: ship gate is operator-owned (ADR-0074)"}},
			want:        nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := Unaccounted(tc.changed, scope, DefaultClosureRules(), tc.adjudicated)
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Errorf("Unaccounted = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestClassify_ClosureIsComputed_UnconfirmedClaimsAreDowngraded(t *testing.T) {
	t.Parallel()
	scope := Scope{Cycle: 1450, Declared: []string{"go/internal/salvage/extract.go"}}

	got := Classify("go/internal/salvage/extract_test.go", Declaration{Class: ClassOpportunistic}, scope, DefaultClosureRules())
	if got.Class != ClassClosure {
		t.Errorf("a test file for an in-scope package is closure regardless of how it was declared; got %q", got.Class)
	}
	if got.Disposition != DispositionKeep {
		t.Errorf("closure keeps by construction (rejecting it yields a broken tree); got %q", got.Disposition)
	}

	got = Classify("go/internal/router/pick.go", Declaration{Class: ClassClosure, Justification: "needed for my change"}, scope, DefaultClosureRules())
	if got.Class == ClassClosure {
		t.Error("an unconfirmed closure CLAIM was honoured — that is the laundering label for any change")
	}
	if got.Disposition == DispositionKeep {
		t.Errorf("a downgraded claim must not keep on the agent's say-so; got %q", got.Disposition)
	}
}

func TestClassify_ProtectedSurfaceIsPolicyNotMerit(t *testing.T) {
	t.Parallel()
	scope := Scope{Cycle: 1450, Protected: []string{"go/internal/phases/ship/"}}
	got := Classify("go/internal/phases/ship/gitops.go",
		Declaration{Class: ClassDiscovered, Justification: "found a real bug in ship staging"}, scope, DefaultClosureRules())

	if got.Class != ClassBoundary {
		t.Errorf("Class = %q, want %q — a protected surface is not adjudicable by the agent that touched it", got.Class, ClassBoundary)
	}
	if got.Disposition != DispositionRefuse {
		t.Errorf("Disposition = %q, want %q", got.Disposition, DispositionRefuse)
	}
	if !got.Preserve {
		t.Error("a refused boundary change must still be PRESERVED — the finding may be real even when the edit is not admissible")
	}
}

func TestEntry_Validate_RejectsScopeAsItsOwnJustification(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		reason  string
		wantErr bool
	}{
		{"bare category", "out of scope", true},
		{"category with padding", "This change is out-of-scope for this cycle.", true},
		{"category as the whole reason, different wording", "not in scope", true},
		{"a named risk", "touches the ship gate, an operator-owned surface (ADR-0074)", false},
		{"a named risk mentioning scope in passing", "belongs to the router lane's scope and would collide with its in-flight edit", false},
		{"empty", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e := Entry{Path: "p.go", Class: ClassOpportunistic, Disposition: DispositionRefuse, Reason: tc.reason}
			err := e.Validate()
			if tc.wantErr && err == nil {
				t.Errorf("Reason %q must be rejected: a refusal has to name the risk, not restate the category", tc.reason)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("Reason %q must be accepted, got %v", tc.reason, err)
			}
		})
	}
}

func TestEntry_Validate_KeepAlsoNeedsAReason(t *testing.T) {
	t.Parallel()
	e := Entry{Path: "p.go", Class: ClassDiscovered, Disposition: DispositionKeep}
	if err := e.Validate(); err == nil {
		t.Error("a KEEP with no reason admits unreviewed surface on nobody's authority")
	}
}

func TestEntry_Validate_CarveMustNameItsDestination(t *testing.T) {
	t.Parallel()
	e := Entry{Path: "p.go", Class: ClassDiscovered, Disposition: DispositionCarve, Reason: "real adjacent defect, unreviewed here"}
	if err := e.Validate(); err == nil {
		t.Error("a CARVE without a patch reference loses the work it claims to preserve")
	}
	e.PatchRef = ".evolve/carved/cycle-1450/router-pick.patch"
	if err := e.Validate(); err != nil {
		t.Errorf("a carve naming its patch is well-formed, got %v", err)
	}
}

func TestSummarize_SurfacesScopeMisunderstandingAsATaskDefect(t *testing.T) {
	t.Parallel()
	entries := []Entry{
		{Path: "a.go", Class: ClassMisunderstood, Disposition: DispositionKeep, Reason: "the item's wording named this file"},
		{Path: "b.go", Class: ClassMisunderstood, Disposition: DispositionKeep, Reason: "same"},
		{Path: "c.go", Class: ClassOpportunistic, Disposition: DispositionCarve, Reason: "tidy-up", PatchRef: "p"},
	}
	s := Summarize(entries)
	if s.ByClass[ClassMisunderstood] != 2 {
		t.Errorf("ByClass[misunderstood] = %d, want 2", s.ByClass[ClassMisunderstood])
	}
	if !s.TaskStatementSuspect {
		t.Error("misunderstanding dominating the delta must flag the ITEM, not the agent — otherwise the retry reproduces it")
	}
	if s.Carved != 1 || s.Kept != 2 {
		t.Errorf("counts = carved %d kept %d, want 1 and 2", s.Carved, s.Kept)
	}
}
