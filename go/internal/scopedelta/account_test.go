package scopedelta

import (
	"strings"
	"testing"
)

func TestEntry_Validate_BoundaryRefusalIsNotNegotiable(t *testing.T) {
	t.Parallel()
	e := Entry{
		Path: "go/internal/phases/ship/gitops.go", Class: ClassBoundary,
		Disposition: DispositionKeep, Reason: "the staging bug is real and the fix is one line",
	}
	if err := e.Validate(); err == nil {
		t.Error("a boundary path was KEPT by the record — protected surfaces are policy, not merit, and must not be reversible downstream of Classify")
	}
	for _, d := range []Disposition{DispositionCarve, DispositionKeep} {
		e.Disposition = d
		if err := e.Validate(); err == nil {
			t.Errorf("boundary + %s must be rejected", d)
		}
	}
	e.Disposition = DispositionRefuse
	e.Reason = "operator-owned ship gate (ADR-0074); the finding is preserved for console review"
	if err := e.Validate(); err != nil {
		t.Errorf("boundary + refuse is the one legal shape, got %v", err)
	}
}

func TestAccount_UnvalidatedEntriesDoNotCountAsAccounted(t *testing.T) {
	t.Parallel()
	scope := Scope{Cycle: 1450, Declared: []string{"go/internal/salvage/extract.go"}}
	changed := []string{"go/internal/salvage/extract.go", "go/internal/router/pick.go"}

	res := Account(changed, scope, DefaultClosureRules(), []Entry{
		{Path: "go/internal/router/pick.go", Class: ClassDiscovered, Disposition: DispositionCarve, Reason: "real adjacent defect"},
	})
	if res.OK() {
		t.Error("Account approved a carve that names no patch — the work it claims to preserve is gone")
	}
	if len(res.Invalid) != 1 {
		t.Fatalf("Invalid = %v, want the malformed carve named", res.Invalid)
	}

	res = Account(changed, scope, DefaultClosureRules(), []Entry{
		{Path: "go/internal/router/pick.go", Class: ClassDiscovered, Disposition: DispositionCarve,
			Reason: "real adjacent defect, unreviewed in this cycle", PatchRef: ".evolve/carved/cycle-1450/router-pick.patch"},
	})
	if !res.OK() {
		t.Errorf("a well-formed carve must account; unaccounted=%v invalid=%v", res.Unaccounted, res.Invalid)
	}
}

func TestAccount_ReportsUnaccountedAndInvalidTogether(t *testing.T) {
	t.Parallel()
	scope := Scope{Cycle: 1450, Declared: []string{"a.go"}}
	res := Account(
		[]string{"a.go", "b.go", "c.go"},
		scope, DefaultClosureRules(),
		[]Entry{{Path: "c.go", Class: ClassOpportunistic, Disposition: DispositionRefuse, Reason: "out of scope"}},
	)
	if len(res.Unaccounted) != 1 || res.Unaccounted[0] != "b.go" {
		t.Errorf("Unaccounted = %v, want [b.go]", res.Unaccounted)
	}
	if len(res.Invalid) != 1 {
		t.Errorf("Invalid = %v, want the category-as-reason refusal named", res.Invalid)
	}
	if res.OK() {
		t.Error("OK() must be false while either list is non-empty — both are ship blockers")
	}
}

func TestEntry_Validate_ReasonMustSayMoreThanTheCategory(t *testing.T) {
	t.Parallel()
	bare := []string{
		"out of scope", "not in scope", "outside scope", "out-of-scope for this cycle",
		"not this cycle's scope", "different scope", "wrong scope", "scope violation",
		"out of the declared scope", "not in scope for this batch",
		"Out of scope.", "OOS", "n/a", "unrelated",
	}
	for _, r := range bare {
		e := Entry{Path: "p.go", Class: ClassOpportunistic, Disposition: DispositionRefuse, Reason: r}
		if err := e.Validate(); err == nil {
			t.Errorf("reason %q restates the category and decides nothing — must be rejected", r)
		}
	}
	substantive := []string{
		"touches the ship gate, an operator-owned surface (ADR-0074)",
		"belongs to the router lane's scope and would collide with its in-flight edit",
		"unreviewed rewrite of the retry loop; no test covers the new branch",
	}
	for _, r := range substantive {
		e := Entry{Path: "p.go", Class: ClassOpportunistic, Disposition: DispositionRefuse, Reason: r}
		if err := e.Validate(); err != nil {
			t.Errorf("reason %q names a risk and must be admitted, got %v", r, err)
		}
	}
	e := Entry{Path: "p.go", Class: ClassOpportunistic, Disposition: DispositionCarve, Reason: "out of scope", PatchRef: "p"}
	if err := e.Validate(); err == nil {
		t.Error("the category-as-reason floor must apply to every disposition, not only refuse")
	}
}

func TestDefaultClosureRules_CoverBuildMetadataAChangeMechanicallyRequires(t *testing.T) {
	t.Parallel()
	scope := Scope{Cycle: 1450, Declared: []string{"go/internal/scopedelta/scopedelta.go"}}
	for _, p := range []string{"go/.apicover-enforce", "go/go.sum", "go/go.mod"} {
		got := Classify(p, Declaration{}, scope, DefaultClosureRules())
		if got.Class != ClassClosure || got.Disposition != DispositionKeep {
			t.Errorf("%s classified %s/%s, want closure/keep — carving it lands the change with its own gate unenforced",
				p, got.Class, got.Disposition)
		}
	}
	docsOnly := Scope{Cycle: 1450, Declared: []string{"docs/architecture/adr/0087-x.md"}}
	if got := Classify("go/go.sum", Declaration{}, docsOnly, DefaultClosureRules()); got.Class == ClassClosure {
		t.Error("build metadata must not be closure of a cycle that touched no Go file")
	}
}

func TestClassify_InScopePathIsNotADelta(t *testing.T) {
	t.Parallel()
	scope := Scope{Cycle: 1450, Declared: []string{"go/internal/salvage/extract.go"}}
	got := Classify("go/internal/salvage/extract.go", Declaration{}, scope, DefaultClosureRules())
	if got.Class != ClassInScope || got.Disposition != DispositionKeep {
		t.Errorf("a declared path classified %s/%s — a wiring author passing the FULL changed set would have the cycle's own work carved away",
			got.Class, got.Disposition)
	}
}

func TestScope_ProtectedWithoutTrailingSlashStillProtects(t *testing.T) {
	t.Parallel()
	scope := Scope{Cycle: 1450, Protected: []string{"go/internal/phases/ship"}}
	got := Classify("go/internal/phases/ship/gitops.go", Declaration{}, scope, DefaultClosureRules())
	if got.Class != ClassBoundary {
		t.Errorf("Class = %q, want boundary — a missing trailing slash must not silently disable protection", got.Class)
	}
	if got := Classify("go/internal/phases/shipping/x.go", Declaration{}, scope, DefaultClosureRules()); got.Class == ClassBoundary {
		t.Error("prefix matching must respect the path separator")
	}
}

func TestSummarize_DominanceIsStrictMajority(t *testing.T) {
	t.Parallel()
	mk := func(n int, c Class) []Entry {
		var out []Entry
		for i := 0; i < n; i++ {
			out = append(out, Entry{Path: "p", Class: c, Disposition: DispositionKeep, Reason: "r"})
		}
		return out
	}
	cases := []struct {
		name    string
		entries []Entry
		want    bool
	}{
		{"one of three is noise", append(mk(1, ClassMisunderstood), mk(2, ClassOpportunistic)...), false},
		{"exactly half is not dominance", append(mk(2, ClassMisunderstood), mk(2, ClassOpportunistic)...), false},
		{"a strict majority is a statement about the item", append(mk(3, ClassMisunderstood), mk(2, ClassOpportunistic)...), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := Summarize(tc.entries).TaskStatementSuspect; got != tc.want {
				t.Errorf("TaskStatementSuspect = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestAccount_ErrorsNameTheOffendingPath(t *testing.T) {
	t.Parallel()
	var res AccountResult = Account([]string{"x.go"}, Scope{Cycle: 1}, DefaultClosureRules(),
		[]Entry{{Path: "x.go", Class: ClassOpportunistic, Disposition: DispositionCarve, Reason: "tidy"}})
	if len(res.Invalid) != 1 || !strings.Contains(res.Invalid[0].Error(), "x.go") {
		t.Errorf("Invalid = %v, want an error naming x.go", res.Invalid)
	}
	if !(AccountResult{}).OK() {
		t.Error("the zero AccountResult must read as OK — a cycle with no delta has nothing to block on")
	}
}
