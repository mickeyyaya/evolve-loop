package handoff_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/handoff"
)

func TestLedger_JudgeRoundTripsUnderTheJudgeKey(t *testing.T) {
	ledger := handoff.Ledger{Judge: &handoff.Judgement{
		Seat:     handoff.Seat{CLI: "agy", Tier: "deep"},
		Verdicts: []handoff.Verdict{{Ask: "a1", Agrees: true, Note: "ok"}},
		Basis:    handoff.Basis{AuthorModel: "opus", SeatModel: "Gemini 4 Hydrogen", AuthorVendors: []string{"claude"}, SeatVendors: []string{"gemini"}, TierCeiling: "deep"},
	}}
	raw, err := json.Marshal(ledger)
	if err != nil {
		t.Fatal(err)
	}
	var generic map[string]json.RawMessage
	if err := json.Unmarshal(raw, &generic); err != nil {
		t.Fatal(err)
	}
	if _, ok := generic["judge"]; !ok {
		t.Fatalf("the judgement must sit under the key judge: %s", raw)
	}
	var back handoff.Ledger
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if back.Judge == nil || back.Judge.Seat.CLI != "agy" || back.Judge.Basis.SeatModel != "Gemini 4 Hydrogen" || back.Judge.Basis.TierCeiling != "deep" {
		t.Fatalf("round trip lost the record: %+v", back.Judge)
	}
	var bare map[string]json.RawMessage
	rawBare, _ := json.Marshal(handoff.Ledger{})
	_ = json.Unmarshal(rawBare, &bare)
	if _, ok := bare["judge"]; ok {
		t.Fatal("a ledger with no judgement must omit the judge key")
	}
}

func TestJudgement_RefusedAndErrorAreDistinctKeys(t *testing.T) {
	refused, _ := json.Marshal(handoff.Judgement{Refused: "no independent seat"})
	failed, _ := json.Marshal(handoff.Judgement{Error: "exit 2"})
	if !strings.Contains(string(refused), `"refused":"no independent seat"`) || strings.Contains(string(refused), `"error"`) {
		t.Fatalf("refused record = %s", refused)
	}
	if !strings.Contains(string(failed), `"error":"exit 2"`) || strings.Contains(string(failed), `"refused"`) {
		t.Fatalf("error record = %s", failed)
	}
}

func TestCheckVerdicts_RejectsMissingExtraAndDuplicateAskAndUnknownKey(t *testing.T) {
	asks := []string{"a1", "a2"}
	cases := []struct {
		name string
		raw  string
	}{
		{"missing", `{"verdicts":[{"ask":"a1","agrees":true,"note":""}]}`},
		{"extra", `{"verdicts":[{"ask":"a1","agrees":true,"note":""},{"ask":"a2","agrees":true,"note":""},{"ask":"a3","agrees":true,"note":""}]}`},
		{"duplicate", `{"verdicts":[{"ask":"a1","agrees":true,"note":""},{"ask":"a1","agrees":true,"note":""},{"ask":"a2","agrees":true,"note":""}]}`},
		{"unknown key", `{"verdicts":[{"ask":"a1","agrees":true,"note":"","extra":1},{"ask":"a2","agrees":true,"note":""}]}`},
		{"no verdicts key", `{}`},
		{"prose", `the judge agrees`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			verdicts, err := handoff.CheckVerdicts(asks, []byte(c.raw))
			if !errors.Is(err, handoff.ErrBadVerdicts) {
				t.Fatalf("err = %v, want ErrBadVerdicts", err)
			}
			if verdicts != nil {
				t.Fatalf("a refused reply yields no verdicts: %v", verdicts)
			}
		})
	}
	verdicts, err := handoff.CheckVerdicts(asks, []byte(`{"verdicts":[{"ask":"a2","agrees":false,"note":"n"},{"ask":"a1","agrees":true,"note":""}]}`))
	if err != nil || len(verdicts) != 2 || verdicts[0].Ask != "a2" || verdicts[0].Agrees {
		t.Fatalf("a full set in any order is valid: %v %v", verdicts, err)
	}
}

func TestJudgement_DisagreementsNameEachAskThatDoesNotAgree(t *testing.T) {
	j := handoff.Judgement{Verdicts: []handoff.Verdict{{Ask: "a1", Agrees: true}, {Ask: "a2", Agrees: false, Note: "no proof"}}}
	if j.Agrees([]string{"a1", "a2"}) {
		t.Fatal("a2 does not agree")
	}
	got := j.Disagreements([]string{"a1", "a2", "a3"})
	if len(got) != 2 || got[0] != "a2: no proof" || got[1] != "a3: no verdict" {
		t.Fatalf("Disagreements = %q", got)
	}
	if !j.Agrees([]string{"a1"}) {
		t.Fatal("a1 agrees")
	}
}

func TestJudgement_OutcomeNamesRefusedErrorAgreeAndDisagree(t *testing.T) {
	asks := []string{"a1"}
	cases := map[string]handoff.Judgement{
		"refused":  {Refused: "none"},
		"error":    {Error: "exit 2"},
		"agree":    {Verdicts: []handoff.Verdict{{Ask: "a1", Agrees: true}}},
		"disagree": {Verdicts: []handoff.Verdict{{Ask: "a1", Agrees: false}}},
	}
	for want, j := range cases {
		if got := j.Outcome(asks); got != want {
			t.Errorf("Outcome = %q, want %q", got, want)
		}
	}
}

func TestLedger_MetAsksListsOnlyTheMetEntriesInOrder(t *testing.T) {
	l := handoff.Ledger{Entries: []handoff.Entry{
		{Ask: "a1", Status: handoff.StatusMet}, {Ask: "a2", Status: handoff.StatusUnmet},
		{Ask: "a3", Status: handoff.StatusNotApplicable}, {Ask: "a4", Status: handoff.StatusMet},
	}}
	if got := l.MetAsks(); len(got) != 2 || got[0] != "a1" || got[1] != "a4" {
		t.Fatalf("MetAsks = %v", got)
	}
}
