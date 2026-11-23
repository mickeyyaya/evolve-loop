package auditchain

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/handoff"
)

func ask(id string) handoff.Ask { return handoff.Ask{ID: id, Required: true} }

func metEntry(id string, refs ...string) handoff.Entry {
	e := handoff.Entry{Ask: id, Status: handoff.StatusMet}
	for _, r := range refs {
		e.Evidence = append(e.Evidence, handoff.Evidence{Kind: "predicate", Ref: r})
	}
	return e
}

func cleanPairs() []PairEval {
	return []PairEval{
		{
			Pair: "triage-tdd", Outcome: "pass",
			Request: handoff.Request{Pair: "triage-tdd", Asks: []handoff.Ask{ask("item#1")}},
			Ledger:  handoff.Ledger{Entries: []handoff.Entry{metEntry("item#1", "go/acs/cycle1/p_test.go:TestX")}},
		},
		{
			Pair: "tdd-build", Outcome: "pass",
			Request: handoff.Request{Pair: "tdd-build", Asks: []handoff.Ask{ask("TestX")}},
			Ledger:  handoff.Ledger{Entries: []handoff.Entry{metEntry("TestX", "go/acs/cycle1/p_test.go:TestX")}},
		},
	}
}

func TestWalkLedgers_ACleanChainHasNoFinding(t *testing.T) {
	t.Parallel()
	if got := WalkLedgers(LedgerInputs{Pairs: cleanPairs()}); len(got) != 0 {
		t.Fatalf("findings = %+v, want none", got)
	}
}

func TestWalkLedgers_AnUnmetRequiredAskIsAFinding(t *testing.T) {
	t.Parallel()
	pairs := cleanPairs()
	pairs[1].Outcome = "retry"
	pairs[1].Unmet = []string{"TestX"}
	pairs[1].Errors = []string{"digest differs"}
	pairs[1].Blocked = []string{"TestY"}

	got := WalkLedgers(LedgerInputs{Pairs: pairs})

	if len(got) != 1 || got[0].Pair != "tdd-build" || got[0].Link != LinkImplementation || got[0].Ask != "TestX" {
		t.Fatalf("findings = %+v, want one tdd-build implementation finding for TestX", got)
	}
	for _, want := range []string{"retry", "TestX", "TestY", "digest differs"} {
		if !strings.Contains(got[0].Message, want) {
			t.Errorf("message %q lacks %q", got[0].Message, want)
		}
	}
}

func TestWalkLedgers_AMissingLedgerIsAFindingForItsPair(t *testing.T) {
	t.Parallel()
	pairs := cleanPairs()
	pairs[0].LedgerAbsent = true
	pairs[0].Outcome = "error"

	got := WalkLedgers(LedgerInputs{Pairs: pairs})

	if len(got) != 1 || got[0].Pair != "triage-tdd" || got[0].Link != LinkSelection || !strings.Contains(got[0].Message, "ledger is missing") {
		t.Fatalf("findings = %+v, want one missing-ledger finding on triage-tdd", got)
	}
}

func TestWalkLedgers_APairThatNeverArrivedFailsClosed(t *testing.T) {
	t.Parallel()
	got := WalkLedgers(LedgerInputs{})
	if len(got) != 2 || got[0].Pair != "triage-tdd" || got[1].Pair != "tdd-build" {
		t.Fatalf("findings = %+v, want one per pair in pair order", got)
	}
}

func TestWalkLedgers_AnInboxAcceptanceWithNoMetBuildEntryIsAFinding(t *testing.T) {
	t.Parallel()
	tests := map[string]func(p []PairEval){
		"the build entry is unmet": func(p []PairEval) {
			p[1].Ledger.Entries[0].Status = handoff.StatusUnmet
		},
		"the build ledger has no entry": func(p []PairEval) {
			p[1].Ledger.Entries = nil
		},
		"the pointer names a predicate outside the build request": func(p []PairEval) {
			p[0].Ledger.Entries[0] = metEntry("item#1", "p_test.go:TestElsewhere")
		},
		"the triage entry is unmet": func(p []PairEval) {
			p[0].Ledger.Entries[0].Status = handoff.StatusUnmet
		},
		"the triage entry has no entry": func(p []PairEval) {
			p[0].Ledger.Entries = nil
		},
		"the triage entry points to no predicate": func(p []PairEval) {
			p[0].Ledger.Entries[0].Evidence = []handoff.Evidence{{Kind: "path", Ref: "TestX"}}
		},
	}
	for name, mutate := range tests {
		pairs := cleanPairs()
		mutate(pairs)
		got := WalkLedgers(LedgerInputs{Pairs: pairs})
		if len(got) != 1 || got[0].Link != LinkSpecification || got[0].Ask != "item#1" {
			t.Errorf("%s: findings = %+v, want one specification finding naming item#1", name, got)
		}
	}
}

func TestWalkLedgers_TheValidationRunsOnlyWhenBothPairsPass(t *testing.T) {
	t.Parallel()
	pairs := cleanPairs()
	pairs[1].Ledger.Entries = nil
	pairs[1].Outcome = "retry"
	got := WalkLedgers(LedgerInputs{Pairs: pairs})
	if len(got) != 1 || got[0].Link != LinkImplementation {
		t.Fatalf("findings = %+v, want only the build pair finding, no validation noise", got)
	}
}

func TestWalkLedgers_ACycleThatSkippedTDDHasNoFinding(t *testing.T) {
	t.Parallel()
	pairs := cleanPairs()
	pairs[1].RequestAbsent = true
	pairs[1].Outcome = "error"
	pairs[0].LedgerAbsent = true
	pairs[0].Outcome = "error"
	in := LedgerInputs{Pairs: pairs}
	if got := WalkLedgers(in); len(got) != 0 {
		t.Errorf("findings = %+v, want none", got)
	}
	if !TDDSkipped(in) {
		t.Error("TDDSkipped = false")
	}
	if TDDSkipped(LedgerInputs{Pairs: cleanPairs()}) {
		t.Error("TDDSkipped = true for a cycle that ran TDD")
	}
}

func TestWalkLedgers_AnAbsentTriageRequestStillJudgesTheBuildPair(t *testing.T) {
	t.Parallel()
	pairs := cleanPairs()
	pairs[0].RequestAbsent = true
	pairs[0].Outcome = "error"
	pairs[1].LedgerAbsent = true
	pairs[1].Outcome = "error"
	in := LedgerInputs{Pairs: pairs}
	got := WalkLedgers(in)
	if len(got) != 1 || got[0].Pair != "tdd-build" || got[0].Link != LinkImplementation {
		t.Fatalf("findings = %+v, want exactly the tdd-build missing-ledger finding", got)
	}
	if TDDSkipped(in) || !TriageSkipped(in) {
		t.Errorf("TDDSkipped = %v, TriageSkipped = %v, want false and true", TDDSkipped(in), TriageSkipped(in))
	}
	if TriageSkipped(LedgerInputs{Pairs: cleanPairs()}) {
		t.Error("TriageSkipped = true for a cycle with a committed set")
	}
}

func TestWalkLedgers_AnAbsentTriageRequestRunsNoValidation(t *testing.T) {
	t.Parallel()
	pairs := cleanPairs()
	pairs[0].RequestAbsent = true
	pairs[0].Outcome = "error"
	pairs[0].Request = handoff.Request{Pair: "triage-tdd", Asks: []handoff.Ask{ask("item#1")}}
	if got := WalkLedgers(LedgerInputs{Pairs: pairs}); len(got) != 0 {
		t.Errorf("findings = %+v, want none for a clean build pair", got)
	}
}

func TestWalkLedgers_AMissingTriageLedgerWithPredicatesIsOneFinding(t *testing.T) {
	t.Parallel()
	pairs := cleanPairs()
	pairs[0].LedgerAbsent = true
	pairs[0].Outcome = "error"
	if got := WalkLedgers(LedgerInputs{Pairs: pairs}); len(got) != 1 {
		t.Fatalf("findings = %+v, want exactly one", got)
	}
}

func TestWalkLedgers_JudgedLinksGetNoFinding(t *testing.T) {
	t.Parallel()
	pairs := cleanPairs()
	pairs[0].Outcome = "invalid"
	pairs[1].Outcome = "retry"
	pairs[1].Unmet = []string{"TestX"}
	for _, f := range WalkLedgers(LedgerInputs{Pairs: pairs}) {
		if f.Link == LinkNarrative || f.Link == LinkDelivery {
			t.Errorf("finding %+v lands on a judged link", f)
		}
	}
}

func TestShadowRecord_CarriesTheLedgerFindingsUnderTheirJSONNames(t *testing.T) {
	t.Parallel()
	rec := ShadowRecord{
		LedgerFindings: []LedgerFinding{{Link: LinkSpecification, Pair: "triage-tdd", Ask: "item#1", Message: "m"}},
		LedgerNote:     "tdd did not run",
	}
	raw, err := json.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"ledger_findings":[{"link":"specification-fidelity","pair":"triage-tdd","ask":"item#1","message":"m"}]`, `"ledger_note":"tdd did not run"`} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("record %s lacks %s", raw, want)
		}
	}
	empty, _ := json.Marshal(ShadowRecord{})
	if strings.Contains(string(empty), "ledger") {
		t.Errorf("an empty record must omit the ledger fields: %s", empty)
	}
}
