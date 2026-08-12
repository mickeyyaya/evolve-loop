package scopedelta

// gaming_test.go — how the adjudication tells MEANINGFUL out-of-scope work
// apart from work that is gaming its own evaluation.
//
// The premise the rest of this package rests on: a justification is authored by
// the party that benefits from it, so no criterion built on reading the
// justification survives a better-written one. The discriminators here are
// therefore all relationships between the change and something the producing
// agent does NOT control.
//
//	D1 counterfactual necessity — does anything fail WITHOUT it?
//	D2 signal vs subject       — does it edit the thing being judged, or the
//	                             apparatus doing the judging?
//	D3 direction of effect     — does it LOOSEN the bar or tighten it?
//	D4 corroboration           — does anything the agent did not author agree?
//
// The honest limit is stated in the code: none of this decides whether an
// uncorroborated claim is TRUE. It decides what may ship on the strength of it,
// and the default for the undecidable case is CARVE — preserve, do not ship —
// so a missed gaming attempt gains nothing and a missed meaningful change loses
// nothing but time.

import (
	"strings"
	"testing"
)

// --- D2: what a path DOES in the pipeline -------------------------------

func TestSurfaceOf_SeparatesTheJudgedFromTheJudging(t *testing.T) {
	t.Parallel()
	signal := []string{
		"go/internal/deliverable/reviewer_test.go", // a test
		"go/acs/cycle1441/predicates_test.go",      // an acceptance predicate
		"go/internal/deliverable/reviewer.go",      // wait: product code, see subject
		"agents/evolve-auditor.md",                 // the persona that grades
		".evolve/policy.json",                      // the dials the gates read
		".evolve/bad-verdict-baseline.jsonl",       // evidence a report cites
		"go/internal/phases/ship/repocontract.go",  // a gate
		".evolve/runs/cycle-1450/audit-report.md",  // the graded artifact itself
	}
	// reviewer.go is product code — it is judged, not judging. Kept in the list
	// above deliberately to force the distinction to be about ROLE, not about
	// which package a file lives in.
	subject := []string{
		"go/internal/deliverable/reviewer.go",
		"go/internal/salvage/extract.go",
		"docs/architecture/adr/0087-x.md",
	}
	for _, p := range signal {
		if p == "go/internal/deliverable/reviewer.go" {
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

// --- D3: the direction that matters -------------------------------------

// Nobody games by raising their own bar. A change that LOOSENS the apparatus
// judging the change is the strongest single signal available, and it is the
// one case where the justification is irrelevant: an agent that needs a weaker
// test to pass has told us what it could not do.
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
	// Even fully corroborated, it may be preserved and escalated, never kept.
	e.Disposition = DispositionCarve
	e.PatchRef = "p"
	if err := Admissible(e); err != nil {
		t.Errorf("carving a loosening change is the right disposition, got %v", err)
	}

	// Tightening is the mirror image: it raises the bar the agent is held to,
	// so it is admissible on ordinary corroboration.
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

// --- D1 + D4: the claim must cost something to make ----------------------

// "I found a real bug" is unfalsifiable from the text. It becomes checkable the
// moment the agent has to name what fails without the change — a counterfactual
// it can only produce by actually having done the work.
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

	// A named, executed counterfactual is corroboration: the command and its
	// direction are checkable by someone else.
	withProof := base
	withProof.Corroboration = Corroboration{FailsWithout: true, Command: "go test ./internal/router/ -run TestPick_EmptyCandidates"}
	if err := Admissible(withProof); err != nil {
		t.Errorf("a counterfactual-backed keep is exactly what meaningful work looks like: %v", err)
	}

	// So is a pre-existing queued item: the finding was known BEFORE this agent
	// claimed it, so the claim is not self-minted.
	withItem := base
	withItem.Corroboration = Corroboration{QueuedItemID: "router-empty-candidate-nil-deref"}
	if err := Admissible(withItem); err != nil {
		t.Errorf("a keep corroborated by a pre-existing item must be admissible: %v", err)
	}

	// A claimed counterfactual with no command is not a counterfactual.
	hollow := base
	hollow.Corroboration = Corroboration{FailsWithout: true}
	if err := Admissible(hollow); err == nil {
		t.Error("FailsWithout with no command is an assertion wearing the costume of evidence")
	}
}

// Computed closure needs no corroboration: the closure RULE is the thing the
// agent does not control, so demanding a second proof would only tax the one
// class that is already mechanically established.
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

// --- The population view: gaming is rarely a single instance -------------

// One "I fixed an adjacent bug" is unfalsifiable. Twenty of them, all editing
// the apparatus, all loosening, is a pattern no single adjudication can see —
// which is why the shape of the whole delta is reported, not just each entry.
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

	// A clean delta says nothing — no zero-noise, or the signal stops meaning
	// anything.
	if s := GamingSignals([]Entry{{Path: "x.go", Class: ClassDiscovered, Disposition: DispositionCarve, Reason: "r", PatchRef: "p"}}); len(s) != 0 {
		t.Errorf("an ordinary delta must raise nothing, got %v", s)
	}
}

// --- Wiring the discriminators into the one blocking seam ---------------

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

// --- Second-review BLOCK: the hinge was not enforced at the admitting seam --

// The cheapest bypass in the package was one string field: declare
// Class:"closure" and the whole evidence layer is skipped. Classify enforces
// "closure is computed, never claimed" — but nothing forced an adjudication
// record through Classify, and Admissible read the declared field.
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
	// The genuine article still passes: a rule actually covers it.
	res = Account([]string{"go/internal/salvage/extract.go", "go/internal/salvage/extract_test.go"},
		scope, DefaultClosureRules(),
		[]Entry{{Path: "go/internal/salvage/extract_test.go", Class: ClassClosure,
			Disposition: DispositionKeep, Reason: "necessary closure (same-package-test)"}})
	if !res.OK() {
		t.Errorf("rule-confirmed closure must still be admitted: %v %v", res.Unaccounted, res.Invalid)
	}
}

// The mirror failure: in-scope work is not a delta and must never be asked to
// corroborate itself, or the fix for false-REDs becomes a false-RED generator
// on every honest cycle.
func TestAdmissible_InScopeWorkIsNeverAskedToCorroborate(t *testing.T) {
	t.Parallel()
	e := Entry{Path: "go/internal/salvage/extract.go", Class: ClassInScope,
		Disposition: DispositionKeep, Reason: "declared in this cycle's scope"}
	if err := Admissible(e); err != nil {
		t.Errorf("the cycle's own licensed work must not need corroboration: %v", err)
	}
}

// An unset direction on a signal-surface KEEP must not read as a safe one:
// omission is the cheapest possible claim, and EffectUnknown is the zero value.
func TestAdmissible_UnknownDirectionOnASignalSurfaceIsNotSafe(t *testing.T) {
	t.Parallel()
	e := Entry{Path: "go/acs/cycle1450/predicates_test.go", Class: ClassDiscovered,
		Disposition: DispositionKeep, Reason: "the predicate was wrong",
		Corroboration: Corroboration{FailsWithout: true, Command: "go test -tags acs ./acs/cycle1450/"}}
	if err := Admissible(e); err == nil {
		t.Error("a KEEP on the judging apparatus with an UNDECLARED direction was admitted — omission is cheaper than a lie and must not be the safe default")
	}
}

// One test edit is ordinary work. The majority rule degenerated to a presence
// rule at len==1, so the commonest honest delta tripped the alarm.
func TestGamingSignals_DoNotFireOnTheModalHonestDelta(t *testing.T) {
	t.Parallel()
	only := []Entry{{Path: "go/internal/salvage/extract_test.go", Class: ClassClosure,
		Disposition: DispositionKeep, Reason: "necessary closure"}}
	if s := GamingSignals(only); len(s) != 0 {
		t.Errorf("a cycle whose one out-of-scope path is its own covering test must raise nothing, got %v", s)
	}
}

// Two entries for one path with contradictory dispositions both validated, and
// whichever the consumer read first decided what shipped.
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
