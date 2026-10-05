package scopedelta

import (
	"strings"
	"testing"
)

func TestSurfaceOf_SeparatesTheJudgedFromTheJudging(t *testing.T) {
	t.Parallel()
	const productCodeBesideItsTest = "go/internal/deliverable/reviewer.go"
	signal := []string{
		"go/internal/deliverable/reviewer_test.go",
		"go/acs/cycle1441/predicates_test.go",
		productCodeBesideItsTest,
		"agents/evolve-auditor.md",
		".evolve/policy.json",
		".evolve/bad-verdict-baseline.jsonl",
		"go/internal/phases/ship/repocontract.go",
		".evolve/runs/cycle-1450/audit-report.md",
	}
	subject := []string{
		productCodeBesideItsTest,
		"go/internal/salvage/extract.go",
		"docs/architecture/adr/0087-x.md",
	}
	for _, p := range signal {
		if p == productCodeBesideItsTest {
			continue
		}
		if got := SurfaceOf(p); got != SurfaceSignal {
			t.Errorf("SurfaceOf(%q) = %q, want signal — an out-of-scope edit to the judging apparatus is categorically different from one to the code being judged", p, got)
		}
	}
	for _, p := range subject {
		if got := SurfaceOf(p); got != SurfaceSubject {
			t.Errorf("SurfaceOf(%q) = %q, want subject", p, got)
		}
	}
}

func TestAdmissible_LooseningTheJudgingApparatusIsNeverKeptOnTheProducersWord(t *testing.T) {
	t.Parallel()
	e := Entry{
		Path: "go/internal/deliverable/reviewer_test.go", Class: ClassDiscovered,
		Disposition: DispositionKeep,
		Reason:      "the assertion was over-strict and unrelated to my change; relaxing it unblocks a real fix",
		Effect:      EffectLoosens,
		Corroboration: Corroboration{
			FailsWithout: true, Command: "go test ./internal/deliverable/",
		},
	}
	if err := Admissible(e); err == nil {
		t.Error("a KEEP that LOOSENS the apparatus judging this very change was admitted on the producer's adjudication — that is the shape of every test-weakening")
	}
	e.Disposition = DispositionCarve
	e.PatchRef = "p"
	if err := Admissible(e); err != nil {
		t.Errorf("carving a loosening change is the right disposition, got %v", err)
	}

	e = Entry{
		Path: "go/acs/cycle1441/predicates_test.go", Class: ClassDiscovered,
		Disposition: DispositionKeep, Effect: EffectTightens,
		Reason:        "the predicate passed vacuously; this asserts the behaviour it claimed to",
		Corroboration: Corroboration{FailsWithout: true, Command: "go test -tags acs ./acs/cycle1441/"},
	}
	if err := Admissible(e); err != nil {
		t.Errorf("a corroborated TIGHTENING must be admissible — refusing it is how a pipeline teaches agents not to improve their own scrutiny: %v", err)
	}
}

func TestAdmissible_KeepRequiresCorroborationFromOutsideTheAuthor(t *testing.T) {
	t.Parallel()
	base := Entry{
		Path: "go/internal/router/pick.go", Class: ClassDiscovered,
		Disposition: DispositionKeep,
		Reason:      "genuine nil-deref on the empty-candidate path; I hit it while working",
	}
	if err := Admissible(base); err == nil {
		t.Error("an uncorroborated KEEP shipped on narrative alone — the one thing an agent can always produce")
	}

	withProof := base
	withProof.Corroboration = Corroboration{FailsWithout: true, Command: "go test ./internal/router/ -run TestPick_EmptyCandidates"}
	if err := Admissible(withProof); err != nil {
		t.Errorf("a counterfactual-backed keep is exactly what meaningful work looks like: %v", err)
	}

	withItem := base
	withItem.Corroboration = Corroboration{QueuedItemID: "router-empty-candidate-nil-deref"}
	if err := Admissible(withItem); err != nil {
		t.Errorf("a keep corroborated by a pre-existing item must be admissible: %v", err)
	}

	hollow := base
	hollow.Corroboration = Corroboration{FailsWithout: true}
	if err := Admissible(hollow); err == nil {
		t.Error("FailsWithout with no command is an assertion wearing the costume of evidence")
	}
}

func TestAdmissible_ComputedClosureNeedsNoFurtherProof(t *testing.T) {
	t.Parallel()
	e := Entry{
		Path: "go/internal/salvage/extract_test.go", Class: ClassClosure,
		Disposition: DispositionKeep, Reason: "necessary closure (same-package-test)",
	}
	if err := Admissible(e); err != nil {
		t.Errorf("closure is established by a rule the producer cannot influence; got %v", err)
	}
}

func TestGamingSignals_SurfaceThePatternNoSingleEntryShows(t *testing.T) {
	t.Parallel()
	entries := []Entry{
		{Path: "a_test.go", Class: ClassDiscovered, Disposition: DispositionKeep, Reason: "r", Effect: EffectLoosens},
		{Path: "b_test.go", Class: ClassDiscovered, Disposition: DispositionKeep, Reason: "r", Effect: EffectLoosens},
		{Path: "c_test.go", Class: ClassOpportunistic, Disposition: DispositionKeep, Reason: "r", Effect: EffectLoosens},
		{Path: "d.go", Class: ClassDiscovered, Disposition: DispositionKeep, Reason: "r"},
	}
	got := GamingSignals(entries)
	joined := strings.Join(got, " | ")
	if !strings.Contains(joined, "loosen") {
		t.Errorf("a delta that is mostly loosening the judging apparatus must be named as such; got %q", joined)
	}
	if !strings.Contains(joined, "signal") {
		t.Errorf("a signal-heavy delta must be named; got %q", joined)
	}

	if s := GamingSignals([]Entry{{Path: "x.go", Class: ClassDiscovered, Disposition: DispositionCarve, Reason: "r", PatchRef: "p"}}); len(s) != 0 {
		t.Errorf("an ordinary delta must raise nothing, got %v", s)
	}
}

func TestAccount_InadmissibleKeepsBlockTheShip(t *testing.T) {
	t.Parallel()
	scope := Scope{Cycle: 1450, Declared: []string{"go/internal/salvage/extract.go"}}
	res := Account([]string{"go/internal/salvage/extract.go", "go/internal/deliverable/reviewer_test.go"},
		scope, DefaultClosureRules(),
		[]Entry{{
			Path: "go/internal/deliverable/reviewer_test.go", Class: ClassDiscovered,
			Disposition: DispositionKeep, Effect: EffectLoosens,
			Reason: "the assertion was too strict for my change",
		}})
	if res.OK() {
		t.Error("Account approved a delta whose only decision was to loosen a test on the producer's word")
	}
}

func TestAccount_DeclaredClosureIsReDerived(t *testing.T) {
	t.Parallel()
	scope := Scope{Cycle: 1450, Declared: []string{"go/internal/salvage/extract.go"}}
	res := Account([]string{"go/internal/salvage/extract.go", "go/internal/policy/defaults.go"},
		scope, DefaultClosureRules(),
		[]Entry{{
			Path: "go/internal/policy/defaults.go", Class: ClassClosure, Disposition: DispositionKeep,
			Reason: "the gate default was inconsistent with the shipped contract",
		}})
	if res.OK() {
		t.Error("a self-declared closure that NO rule covers was admitted — that is the anti-gaming hinge bypassed by one string field")
	}
	res = Account([]string{"go/internal/salvage/extract.go", "go/internal/salvage/extract_test.go"},
		scope, DefaultClosureRules(),
		[]Entry{{Path: "go/internal/salvage/extract_test.go", Class: ClassClosure,
			Disposition: DispositionKeep, Reason: "necessary closure (same-package-test)"}})
	if !res.OK() {
		t.Errorf("rule-confirmed closure must still be admitted: %v %v", res.Unaccounted, res.Invalid)
	}
}

func TestAdmissible_InScopeWorkIsNeverAskedToCorroborate(t *testing.T) {
	t.Parallel()
	e := Entry{Path: "go/internal/salvage/extract.go", Class: ClassInScope,
		Disposition: DispositionKeep, Reason: "declared in this cycle's scope"}
	if err := Admissible(e); err != nil {
		t.Errorf("the cycle's own licensed work must not need corroboration: %v", err)
	}
}

func TestAdmissible_UnknownDirectionOnASignalSurfaceIsNotSafe(t *testing.T) {
	t.Parallel()
	e := Entry{Path: "go/acs/cycle1450/predicates_test.go", Class: ClassDiscovered,
		Disposition: DispositionKeep, Reason: "the predicate was wrong",
		Corroboration: Corroboration{FailsWithout: true, Command: "go test -tags acs ./acs/cycle1450/"}}
	if err := Admissible(e); err == nil {
		t.Error("a KEEP on the judging apparatus with an UNDECLARED direction was admitted — omission is cheaper than a lie and must not be the safe default")
	}
}

func TestGamingSignals_DoNotFireOnTheModalHonestDelta(t *testing.T) {
	t.Parallel()
	only := []Entry{{Path: "go/internal/salvage/extract_test.go", Class: ClassClosure,
		Disposition: DispositionKeep, Reason: "necessary closure"}}
	if s := GamingSignals(only); len(s) != 0 {
		t.Errorf("a cycle whose one out-of-scope path is its own covering test must raise nothing, got %v", s)
	}
}

func TestAccount_RefusesContradictoryDuplicateDecisions(t *testing.T) {
	t.Parallel()
	res := Account([]string{"x.go"}, Scope{Cycle: 1}, DefaultClosureRules(), []Entry{
		{Path: "x.go", Class: ClassDiscovered, Disposition: DispositionCarve, Reason: "adjacent defect", PatchRef: "p"},
		{Path: "x.go", Class: ClassClosure, Disposition: DispositionKeep, Reason: "actually necessary"},
	})
	if res.OK() {
		t.Error("one path carried two contradictory decisions and the delta accounted clean")
	}
}

func TestSurfaceOf_TheCommentProofAndTheCommitGateJudgeWhetherReviewRuns(t *testing.T) {
	t.Parallel()
	for _, p := range []string{"go/internal/commentaudit/equivalence.go", "go/internal/commitgate/comment_only.go"} {
		if got := SurfaceOf(p); got != SurfaceSignal {
			t.Errorf("SurfaceOf(%q) = %q, want signal: a wrong comment-only proof skips review, so this code judges other changes", p, got)
		}
		loosening := Entry{
			Path: p, Class: ClassDiscovered, Disposition: DispositionKeep, Effect: EffectLoosens,
			Reason:        "the directive pattern was too strict for my change, so I widened it",
			Corroboration: Corroboration{FailsWithout: true, Command: "go test ./internal/commentaudit/"},
		}
		if err := Admissible(loosening); err == nil {
			t.Errorf("a lane KEEP that loosens %s was admitted on the producer's word", p)
		}
	}
}
