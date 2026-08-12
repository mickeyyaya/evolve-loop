package scopedelta

// account_test.go — the review findings that showed the never-drop invariant
// held only by CALLER DISCIPLINE, plus the closure gaps that would have made
// the gate produce the broken tree it exists to prevent. Written RED first.
//
// The two that mattered:
//   HIGH — Validate ignored class/disposition coherence, so the boundary
//          refusal the ADR calls non-negotiable was reversible in the record.
//   HIGH — Unaccounted counted UNVALIDATED entries as accounted, so a carve
//          with no patch satisfied the invariant it was meant to violate.

import (
	"strings"
	"testing"
)

// --- HIGH 1: the record cannot overturn policy --------------------------

func TestEntry_Validate_BoundaryRefusalIsNotNegotiable(t *testing.T) {
	t.Parallel()
	// The exploit the reviewer wrote out: the adjudicator agrees the finding is
	// real and simply keeps an operator-owned edit.
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

// --- HIGH 2: one seam that both accounts AND validates -------------------

func TestAccount_UnvalidatedEntriesDoNotCountAsAccounted(t *testing.T) {
	t.Parallel()
	scope := Scope{Cycle: 1450, Declared: []string{"go/internal/salvage/extract.go"}}
	changed := []string{"go/internal/salvage/extract.go", "go/internal/router/pick.go"}

	// A carve with no PatchRef: the silent drop wearing a better word.
	res := Account(changed, scope, DefaultClosureRules(), []Entry{
		{Path: "go/internal/router/pick.go", Class: ClassDiscovered, Disposition: DispositionCarve, Reason: "real adjacent defect"},
	})
	if res.OK() {
		t.Error("Account approved a carve that names no patch — the work it claims to preserve is gone")
	}
	if len(res.Invalid) != 1 {
		t.Fatalf("Invalid = %v, want the malformed carve named", res.Invalid)
	}

	// The same entry, made whole, accounts cleanly.
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

// --- Reason floor: a denylist of four phrases was bypassed by paraphrase --

func TestEntry_Validate_ReasonMustSayMoreThanTheCategory(t *testing.T) {
	t.Parallel()
	// Every one of these passed the original denylist. They are the normal
	// case, not the adversarial one: the reason is LLM-authored, so paraphrase
	// is what actually arrives.
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
	// A real risk statement is admitted, including one that mentions scope.
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
	// The floor applies to CARVE too — carve is the most numerous disposition,
	// and "out of scope" explains it no better than it explains a refusal.
	e := Entry{Path: "p.go", Class: ClassOpportunistic, Disposition: DispositionCarve, Reason: "out of scope", PatchRef: "p"}
	if err := e.Validate(); err == nil {
		t.Error("the category-as-reason floor must apply to every disposition, not only refuse")
	}
}

// --- Closure gaps that would produce the broken tree ---------------------

// Demonstrated by this very change: a new package must be enrolled in
// .apicover-enforce or the coverage gate reds. Carving that line ships the
// package with its own gate disabled — precisely the outcome ClassClosure
// exists to prevent, and it is decidable from the path alone.
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
	// Still scoped to Go changes: build metadata is not closure of a docs-only
	// cycle, or every cycle would silently license it.
	docsOnly := Scope{Cycle: 1450, Declared: []string{"docs/architecture/adr/0087-x.md"}}
	if got := Classify("go/go.sum", Declaration{}, docsOnly, DefaultClosureRules()); got.Class == ClassClosure {
		t.Error("build metadata must not be closure of a cycle that touched no Go file")
	}
}

// --- Total function: Classify must be safe on an in-scope path -----------

func TestClassify_InScopePathIsNotADelta(t *testing.T) {
	t.Parallel()
	scope := Scope{Cycle: 1450, Declared: []string{"go/internal/salvage/extract.go"}}
	got := Classify("go/internal/salvage/extract.go", Declaration{}, scope, DefaultClosureRules())
	if got.Class != ClassInScope || got.Disposition != DispositionKeep {
		t.Errorf("a declared path classified %s/%s — a wiring author passing the FULL changed set would have the cycle's own work carved away",
			got.Class, got.Disposition)
	}
}

// --- Path-set matching must fail CLOSED on a missing trailing slash ------

func TestScope_ProtectedWithoutTrailingSlashStillProtects(t *testing.T) {
	t.Parallel()
	// The convention is a trailing "/" for directories; omitting it used to
	// degrade to an exact-path rule that no file ever matches, so the whole
	// protected surface fell through to carve — fail-OPEN on the one class
	// that is policy rather than merit.
	scope := Scope{Cycle: 1450, Protected: []string{"go/internal/phases/ship"}}
	got := Classify("go/internal/phases/ship/gitops.go", Declaration{}, scope, DefaultClosureRules())
	if got.Class != ClassBoundary {
		t.Errorf("Class = %q, want boundary — a missing trailing slash must not silently disable protection", got.Class)
	}
	// And a sibling directory sharing a prefix is still NOT protected.
	if got := Classify("go/internal/phases/shipping/x.go", Declaration{}, scope, DefaultClosureRules()); got.Class == ClassBoundary {
		t.Error("prefix matching must respect the path separator")
	}
}

// --- Dominance rule pinned at its boundary -------------------------------

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

// The invalid-entry errors must NAME the path, or an operator holding a
// blocked ship cannot tell which of forty entries to fix.
func TestAccount_ErrorsNameTheOffendingPath(t *testing.T) {
	t.Parallel()
	var res AccountResult = Account([]string{"x.go"}, Scope{Cycle: 1}, DefaultClosureRules(),
		[]Entry{{Path: "x.go", Class: ClassOpportunistic, Disposition: DispositionCarve, Reason: "tidy"}})
	if len(res.Invalid) != 1 || !strings.Contains(res.Invalid[0].Error(), "x.go") {
		t.Errorf("Invalid = %v, want an error naming x.go", res.Invalid)
	}
	// A clean cycle is the common case and must need no special handling: the
	// zero value of AccountResult is itself the "nothing to decide" answer.
	if !(AccountResult{}).OK() {
		t.Error("the zero AccountResult must read as OK — a cycle with no delta has nothing to block on")
	}
}
