package scopedelta

import (
	"slices"
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

var gateConfiguration = []string{"go/.apicover-enforce", "go/go.mod", "go/go.sum"}

func TestDefaultClosureRules_GateConfigurationIsNeverClosure(t *testing.T) {
	t.Parallel()
	goCycle := Scope{Cycle: 1450, Declared: []string{"go/internal/scopedelta/scopedelta.go"}}
	docsOnly := Scope{Cycle: 1450, Declared: []string{"docs/architecture/adr/0087-x.md"}}
	for _, scope := range []Scope{goCycle, docsOnly} {
		for _, p := range gateConfiguration {
			got := Classify(p, Declaration{}, scope, DefaultClosureRules())
			if got.Class == ClassClosure || got.Disposition == DispositionKeep {
				t.Errorf("%s classified %s/%s with declared %v, want an adjudicated non-closure — closure is exempt from evidence, and an enrollment edit that drops another package's line disables that package's gate",
					p, got.Class, got.Disposition, scope.Declared)
			}
		}
	}
}

func TestAccount_GateConfigurationEditNeedsCorroboration(t *testing.T) {
	t.Parallel()
	const work = "go/internal/scopedelta/scopedelta.go"
	scope := Scope{Cycle: 1450, Declared: []string{work}}
	for _, p := range gateConfiguration {
		changed := []string{work, p}

		res := Account(changed, scope, DefaultClosureRules(), nil)
		if res.OK() || !slices.Contains(res.Unaccounted, p) {
			t.Errorf("an unadjudicated %s edit accounted clean (unaccounted=%v) — it shipped with no decision and no evidence", p, res.Unaccounted)
		}

		droppedLineAsClosure := Entry{
			Path: p, Class: ClassClosure, Disposition: DispositionKeep, Effect: EffectLoosens,
			Reason: "drops the enrollment line of a package this change no longer needs gated",
		}
		if res := Account(changed, scope, DefaultClosureRules(), []Entry{droppedLineAsClosure}); res.OK() {
			t.Errorf("a %s edit that removes another package's line was kept as closure with no corroboration", p)
		}

		uncorroboratedEnrollment := Entry{
			Path: p, Class: ClassDiscovered, Disposition: DispositionKeep, Effect: EffectTightens,
			Reason: "enrolls the package whose coverage gate this change completes",
		}
		if res := Account(changed, scope, DefaultClosureRules(), []Entry{uncorroboratedEnrollment}); res.OK() {
			t.Errorf("a %s keep resting on the author's word alone accounted clean", p)
		}

		corroboratedEnrollment := uncorroboratedEnrollment
		corroboratedEnrollment.Corroboration = Corroboration{FailsWithout: true, Command: "go test -count=1 ./internal/apicover/"}
		if res := Account(changed, scope, DefaultClosureRules(), []Entry{corroboratedEnrollment}); !res.OK() {
			t.Errorf("a corroborated tightening of %s must account — refusing it strands every new package outside its gate: unaccounted=%v invalid=%v",
				p, res.Unaccounted, res.Invalid)
		}
	}
}

func TestAccount_ProtectedPathIsBoundaryWhateverTheLabel(t *testing.T) {
	t.Parallel()
	const work = "go/internal/salvage/extract.go"
	protectedSignal := "go/internal/phases/ship/gitops.go"
	protectedSubject := "go/internal/bridge/manifest.go"
	scope := Scope{
		Cycle:      1450,
		Declared:   []string{work},
		Protected:  []string{"go/internal/phases/ship", protectedSubject},
		LaneOthers: []string{"go/internal/phases/ship"},
	}
	corroborated := Corroboration{FailsWithout: true, Command: "go test -count=1 ./internal/phases/ship/ -run TestStage"}
	labels := []Class{ClassDiscovered, ClassOpportunistic, ClassMisunderstood, ClassCrossLane, ""}
	for _, p := range []string{protectedSignal, protectedSubject} {
		for _, label := range labels {
			for _, d := range []Disposition{DispositionKeep, DispositionCarve} {
				relabelled := Entry{
					Path: p, Class: label, Disposition: d, Effect: EffectTightens, Corroboration: corroborated,
					Reason: "the staging bug is real and the fix is one line", PatchRef: ".evolve/carved/cycle-1450/protected.patch",
				}
				res := Account([]string{work, p}, scope, DefaultClosureRules(), []Entry{relabelled})
				if res.OK() {
					t.Errorf("protected %s labelled %q with disposition %s accounted clean — the producer's label decided a boundary it may only refuse", p, label, d)
					continue
				}
				if !slices.ContainsFunc(res.Invalid, func(err error) bool { return strings.Contains(err.Error(), p) }) {
					t.Errorf("protected %s labelled %q/%s: Invalid = %v, want an error naming the path", p, label, d, res.Invalid)
				}
			}
		}

		refused := Entry{Path: p, Class: ClassBoundary, Disposition: DispositionRefuse,
			Reason: "operator-owned surface (ADR-0074); the finding is preserved for console review"}
		if res := Account([]string{work, p}, scope, DefaultClosureRules(), []Entry{refused}); !res.OK() {
			t.Errorf("refusing protected %s is the one legal shape: unaccounted=%v invalid=%v", p, res.Unaccounted, res.Invalid)
		}
	}

	declaredAndProtected := Scope{Cycle: 1450, Declared: []string{protectedSubject}, Protected: []string{protectedSubject}}
	licensed := Entry{Path: protectedSubject, Class: ClassInScope, Disposition: DispositionKeep, Reason: "the operator named this manifest edit as the task's deliverable"}
	if res := Account([]string{protectedSubject}, declaredAndProtected, DefaultClosureRules(), []Entry{licensed}); !res.OK() {
		t.Errorf("a protected path the cycle's own scope declares is in-scope, as Classify says: unaccounted=%v invalid=%v", res.Unaccounted, res.Invalid)
	}

	unprotected := "go/internal/router/pick.go"
	adjacentFix := Entry{Path: unprotected, Class: ClassDiscovered, Disposition: DispositionKeep, Corroboration: corroborated,
		Reason: "genuine nil-deref on the empty-candidate path"}
	if res := Account([]string{work, unprotected}, scope, DefaultClosureRules(), []Entry{adjacentFix}); !res.OK() {
		t.Errorf("a corroborated discovered keep off the protected surface must still account: unaccounted=%v invalid=%v", res.Unaccounted, res.Invalid)
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

func TestAccount_ProtectedPathAClosureRuleWouldCoverStillNeedsARefusal(t *testing.T) {
	t.Parallel()
	const declared = "go/internal/phases/ship/stage.go"
	const protectedTest = "go/internal/phases/ship/gitops_test.go"
	scope := Scope{Cycle: 1450, Declared: []string{declared}, Protected: []string{"go/internal/phases/ship"}}
	changed := []string{declared, protectedTest}

	if got := Classify(protectedTest, Declaration{}, scope, DefaultClosureRules()); got.Class != ClassBoundary {
		t.Fatalf("Classify(%s) = %s, want boundary — protection outranks closure", protectedTest, got.Class)
	}
	if res := Account(changed, scope, DefaultClosureRules(), nil); !slices.Contains(res.Unaccounted, protectedTest) {
		t.Errorf("a protected %s omitted from the record accounted clean as closure (unaccounted=%v) — omitting the entry bypassed the boundary a labelled entry is refused for", protectedTest, res.Unaccounted)
	}
	refused := Entry{Path: protectedTest, Class: ClassBoundary, Disposition: DispositionRefuse,
		Reason: "operator-owned ship gate test; the finding is preserved for console review"}
	if res := Account(changed, scope, DefaultClosureRules(), []Entry{refused}); !res.OK() {
		t.Errorf("refusing the protected test must account: unaccounted=%v invalid=%v", res.Unaccounted, res.Invalid)
	}
}
