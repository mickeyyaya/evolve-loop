package selfheal

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	repairFile    = "go/internal/gc/sweep.go"
	landingSHA    = "4fc458846355c8e758bf56dd6c6770295d56b61c"
	repairLoopInc = "inc-2026-10-27-99f19b25"
	flakyInc      = "inc-2026-10-27-6f89f94c"
)

var testLimits = Limits{Window: 24 * time.Hour, Repairs: map[string]int{TierPipeline: 3, TierItem: 6}, BreakerFailures: 2, BreakerOpen: 24 * time.Hour}

func testValidator(t *testing.T) Validator {
	t.Helper()
	inv := Inventory{Items: map[string]bool{churnItem: true, "core-one-diagnostics-writer-lane": true}, Families: []string{"agy", "claude"},
		Landings: []Landing{{SHA: landingSHA, At: t0.Add(-time.Hour)}}, FileExists: func(path string) bool { return path == repairFile }}
	inv.Requests = map[string]bool{"dispatch-1": true}
	return Validator{Inventory: inv, Limits: testLimits, Stage: "shadow"}
}

func openRepair() phasecontract.DiagnosisAction {
	return phasecontract.DiagnosisAction{Kind: phasecontract.ActionOpenRepairItem, Params: phasecontract.DiagnosisParams{Files: []string{repairFile}, Repro: "a boundary run hangs"}}
}

func TestBudget_RefusesTheFourthT1RepairInAWindow(t *testing.T) {
	t.Parallel()
	v := testValidator(t)
	inc := liveIncident(t, ciHangIncident)
	for i := range 3 {
		d := v.Decide(inc, openRepair(), t0.Add(time.Duration(i)*time.Hour))
		if d.Verdict != VerdictAccepted {
			t.Fatalf("repair %d of the T1 budget of 3 must be accepted, got %+v", i+1, d)
		}
		v = v.Record(d)
	}

	fourth := v.Decide(inc, openRepair(), t0.Add(3*time.Hour))

	if fourth.Verdict != VerdictRefused || fourth.Precondition != PreBudget || !strings.Contains(fourth.Reason, "T1") {
		t.Fatalf("a fourth T1 repair in one window must be refused by the budget, got %+v", fourth)
	}
	if later := v.Decide(inc, openRepair(), t0.Add(24*time.Hour+time.Minute)); later.Verdict != VerdictAccepted {
		t.Errorf("a repair after the window of the first one has budget again, got %+v", later)
	}
	if t2 := v.Decide(liveIncident(t, churnIncident), openRepair(), t0.Add(3*time.Hour)); t2.Verdict != VerdictAccepted {
		t.Errorf("the T1 budget does not hold a T2 repair, got %+v", t2)
	}
	halt := phasecontract.DiagnosisAction{Kind: phasecontract.ActionHaltAndEscalate}
	if d := v.Decide(inc, halt, t0.Add(3*time.Hour)); d.Verdict != VerdictAccepted || !d.Escalated {
		t.Errorf("halt-and-escalate is no repair and always goes to the operator, got %+v", d)
	}
}

func TestBreaker_OpensAfterTwoFailedRepairs(t *testing.T) {
	t.Parallel()
	inc := liveIncident(t, stallIncident)
	one := testValidator(t)
	one.History.Repairs = []Repair{{IncidentID: "inc-other-1", Class: ClassStall, Failed: true, At: t0.Add(-2 * time.Hour)}}
	if d := one.Decide(inc, openRepair(), t0); d.Verdict != VerdictAccepted {
		t.Fatalf("one failed repair of the class leaves the breaker closed, got %+v", d)
	}

	two := one
	two.History.Repairs = append(slices.Clone(one.History.Repairs), Repair{IncidentID: "inc-other-2", Class: ClassStall, Failed: true, At: t0.Add(-time.Hour)})
	open := two.Decide(inc, openRepair(), t0)

	if open.Verdict != VerdictRefused || open.Precondition != PreBreaker || !open.Escalated {
		t.Fatalf("two failed repairs of one class must open the breaker and escalate, got %+v", open)
	}
	if other := two.Decide(liveIncident(t, ciHangIncident), openRepair(), t0); other.Verdict != VerdictAccepted {
		t.Errorf("the breaker of the class stall does not hold the class ci-hang, got %+v", other)
	}
	if d := two.Decide(inc, phasecontract.DiagnosisAction{Kind: phasecontract.ActionHaltAndEscalate}, t0); d.Verdict != VerdictAccepted {
		t.Errorf("an open breaker never refuses halt-and-escalate, got %+v", d)
	}
}

func TestBreaker_HalfOpenLetsOneRepairRunAndAFailOpensItAgain(t *testing.T) {
	t.Parallel()
	inc := liveIncident(t, stallIncident)
	v := testValidator(t)
	v.History.Repairs = []Repair{
		{IncidentID: "inc-other-1", Class: ClassStall, Failed: true, At: t0.Add(-2 * time.Hour)},
		{IncidentID: "inc-other-2", Class: ClassStall, Failed: true, At: t0.Add(-time.Hour)},
	}
	halfOpen := t0.Add(23 * time.Hour)

	probe := v.Decide(inc, openRepair(), halfOpen)
	if probe.Verdict != VerdictAccepted {
		t.Fatalf("after the open time the half-open breaker lets one repair run, got %+v", probe)
	}
	v = v.Record(probe)
	if d := v.Decide(inc, openRepair(), halfOpen.Add(time.Minute)); d.Verdict != VerdictRefused || d.Precondition != PreBreaker {
		t.Fatalf("the half-open breaker lets only one repair run, got %+v", d)
	}

	failed := v
	failed.History.Repairs = append(slices.Clone(v.History.Repairs), Repair{IncidentID: "inc-other-3", Class: ClassStall, Failed: true, At: halfOpen.Add(time.Hour)})
	if d := failed.Decide(inc, openRepair(), halfOpen.Add(2*time.Hour)); d.Verdict != VerdictRefused || d.Precondition != PreBreaker {
		t.Errorf("a FAIL in the half-open state opens the breaker again, got %+v", d)
	}
	early := v
	early.History.Repairs = append(slices.Clone(v.History.Repairs), Repair{IncidentID: "inc-other-4", Class: ClassStall, Failed: false, At: t0})
	early.History.Decisions = nil
	if d := early.Decide(inc, openRepair(), t0.Add(time.Hour)); d.Verdict != VerdictRefused || d.Precondition != PreBreaker {
		t.Errorf("an outcome that ends while the breaker is open does not close it, got %+v", d)
	}
	passed := v
	passed.History.Repairs = append(slices.Clone(v.History.Repairs), Repair{IncidentID: "inc-other-3", Class: ClassStall, Failed: false, At: halfOpen.Add(time.Hour)})
	if d := passed.Decide(inc, openRepair(), halfOpen.Add(2*time.Hour)); d.Verdict != VerdictAccepted {
		t.Errorf("a PASS in the half-open state closes the breaker, got %+v", d)
	}
	reset := testValidator(t)
	reset.History.Repairs = []Repair{
		{IncidentID: "inc-other-1", Class: ClassStall, Failed: true, At: t0.Add(-3 * time.Hour)},
		{IncidentID: "inc-other-2", Class: ClassStall, Failed: false, At: t0.Add(-2 * time.Hour)},
		{IncidentID: "inc-other-3", Class: ClassStall, Failed: true, At: t0.Add(-time.Hour)},
	}
	if d := reset.Decide(inc, openRepair(), t0); d.Verdict != VerdictAccepted {
		t.Errorf("a PASS between two failures resets the count of the breaker, got %+v", d)
	}
}

func TestConverge_ARepairThatFailsTwiceEscalates(t *testing.T) {
	t.Parallel()
	inc := liveIncident(t, ciHangIncident)
	v := testValidator(t)
	v.Limits.BreakerFailures = 5
	v.History.Repairs = []Repair{
		{IncidentID: inc.ID, Class: inc.Class, Failed: true, At: t0.Add(-2 * time.Hour)},
		{IncidentID: inc.ID, Class: inc.Class, Failed: true, At: t0.Add(-time.Hour)},
	}
	if d := v.Decide(inc, openRepair(), t0); d.Verdict != VerdictRefused || d.Precondition != PreConverge || !d.Escalated {
		t.Fatalf("an incident with two failed repairs must escalate to the operator, got %+v", d)
	}
}

func TestValidator_RefusesAnActionOutsideTheCatalog(t *testing.T) {
	t.Parallel()
	v := testValidator(t)
	d := v.Decide(liveIncident(t, ciHangIncident), phasecontract.DiagnosisAction{Kind: "run-shell"}, t0)
	if d.Verdict != VerdictRefused || d.Precondition != PreCatalog || !strings.Contains(d.Reason, `"run-shell"`) {
		t.Fatalf("an action outside the catalog must be refused, got %+v", d)
	}
}

func TestCatalog_HasEachKindOfTheDiagnosisSchema(t *testing.T) {
	t.Parallel()
	if got := CatalogActions(); !slices.Equal(got, phasecontract.DiagnosisActions()) {
		t.Fatalf("the catalog %v and the diagnosis schema %v must name the same actions", got, phasecontract.DiagnosisActions())
	}
}

func TestValidator_RefusesEachFailedPrecondition(t *testing.T) {
	t.Parallel()
	retry := func(item, variation string) phasecontract.DiagnosisAction {
		return phasecontract.DiagnosisAction{Kind: phasecontract.ActionRetryWithVariation, Params: phasecontract.DiagnosisParams{Item: item, Variation: variation}}
	}
	reroute := func(item, family string) phasecontract.DiagnosisAction {
		return phasecontract.DiagnosisAction{Kind: phasecontract.ActionRerouteFamily, Params: phasecontract.DiagnosisParams{Item: item, Family: family}}
	}
	quarantine := func(item string) phasecontract.DiagnosisAction {
		return phasecontract.DiagnosisAction{Kind: phasecontract.ActionQuarantineItem, Params: phasecontract.DiagnosisParams{Item: item}}
	}
	revert := func(sha string) phasecontract.DiagnosisAction {
		return phasecontract.DiagnosisAction{Kind: phasecontract.ActionRevertLastLanding, Params: phasecontract.DiagnosisParams{SHA: sha}}
	}
	repairOf := func(files ...string) phasecontract.DiagnosisAction {
		return phasecontract.DiagnosisAction{Kind: phasecontract.ActionOpenRepairItem, Params: phasecontract.DiagnosisParams{Files: files, Repro: "x"}}
	}
	dispatch := func(request string) phasecontract.DiagnosisAction {
		return phasecontract.DiagnosisAction{Kind: phasecontract.ActionDispatchPhaseAgent, Params: phasecontract.DiagnosisParams{Request: request}}
	}
	mainRed := func(v Validator) Validator { v.Inventory.MainRed = true; return v }
	triedModel := func(v Validator) Validator {
		return v.Record(Decision{IncidentID: churnIncident, Verdict: VerdictAccepted, Action: &phasecontract.DiagnosisAction{Kind: phasecontract.ActionRetryWithVariation, Params: phasecontract.DiagnosisParams{Item: churnItem, Variation: "model"}}, At: t0.Add(-time.Hour)})
	}
	same := func(v Validator) Validator { return v }
	cases := []struct {
		name     string
		incident string
		action   phasecontract.DiagnosisAction
		setup    func(Validator) Validator
		want     string
		reason   string
	}{
		{"a T0 incident takes no loop repair", repairLoopInc, retry(churnItem, "model"), same, PreTierLimit, "T0"},
		{"retry names an item that is not in the inbox", churnIncident, retry("no-such-item", "model"), same, PreItemKnown, `"no-such-item"`},
		{"retry repeats an earlier variation", churnIncident, retry(churnItem, "model"), triedModel, PreVariationNew, `"model"`},
		{"reroute names an item that is not in the inbox", churnIncident, reroute("no-such-item", "agy"), same, PreItemKnown, `"no-such-item"`},
		{"reroute names a family that is not installed", churnIncident, reroute(churnItem, "gpt-9"), same, PreFamilyKnown, `"gpt-9"`},
		{"quarantine names an item that is not in the inbox", churnIncident, quarantine("no-such-item"), same, PreItemKnown, `"no-such-item"`},
		{"quarantine names an item of another incident", churnIncident, quarantine("core-one-diagnostics-writer-lane"), same, PreItemOfIncident, churnItem},
		{"revert while main is green", ciHangIncident, revert(landingSHA[:7]), same, PreMainRed, "main"},
		{"revert of a commit that is no landing of the loop", ciHangIncident, revert("abc1234"), mainRed, PreLandingInWindow, `"abc1234"`},
		{"a dispatch of a request that the pipeline did not validate", churnIncident, dispatch("dispatch-2"), same, PreRequestValid, `"dispatch-2"`},
		{"a dispatch runs only at T2", ciHangIncident, dispatch("dispatch-1"), same, PreTierLimit, "T1"},
		{"a repair item for a file that does not exist", ciHangIncident, repairOf(repairFile, "go/internal/gc/nope.go"), same, PreFilesExist, `"go/internal/gc/nope.go"`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			v := c.setup(testValidator(t))
			d := v.Decide(liveIncident(t, c.incident), c.action, t0)
			if d.Verdict != VerdictRefused || d.Precondition != c.want || !strings.Contains(d.Reason, c.reason) {
				t.Fatalf("want a refusal of %s that names %s, got %+v", c.want, c.reason, d)
			}
		})
	}
}

func TestValidator_AcceptsEachActionWhosePreconditionsHold(t *testing.T) {
	t.Parallel()
	v := testValidator(t)
	v.Inventory.MainRed = true
	cases := map[string]struct {
		incident string
		action   phasecontract.DiagnosisAction
	}{
		"retry":      {churnIncident, phasecontract.DiagnosisAction{Kind: phasecontract.ActionRetryWithVariation, Params: phasecontract.DiagnosisParams{Item: churnItem, Variation: "tier"}}},
		"reroute":    {churnIncident, phasecontract.DiagnosisAction{Kind: phasecontract.ActionRerouteFamily, Params: phasecontract.DiagnosisParams{Item: churnItem, Family: "agy"}}},
		"quarantine": {churnIncident, phasecontract.DiagnosisAction{Kind: phasecontract.ActionQuarantineItem, Params: phasecontract.DiagnosisParams{Item: churnItem}}},
		"revert":     {ciHangIncident, phasecontract.DiagnosisAction{Kind: phasecontract.ActionRevertLastLanding, Params: phasecontract.DiagnosisParams{SHA: landingSHA[:9]}}},
		"T0 repair":  {repairLoopInc, openRepair()},
		"dispatch":   {churnIncident, phasecontract.DiagnosisAction{Kind: phasecontract.ActionDispatchPhaseAgent, Params: phasecontract.DiagnosisParams{Request: "dispatch-1"}}},
	}
	for name, c := range cases {
		inc := liveIncident(t, c.incident)
		d := v.Decide(inc, c.action, t0)
		if d.Verdict != VerdictAccepted || d.Escalated || d.Precondition != "" || d.Action.Kind != c.action.Kind || d.IncidentID != inc.ID || d.Tier != inc.Tier || d.Stage != "shadow" || !d.At.Equal(t0) {
			t.Errorf("%s: %+v", name, d)
		}
	}
}

func TestValidator_TheSecondRefusalOfAnIncidentEscalates(t *testing.T) {
	t.Parallel()
	v := testValidator(t)
	inc := liveIncident(t, ciHangIncident)
	first := v.Refuse(inc, errors.New("diagnosis.json: the hypothesis is empty"), t0)
	if first.Verdict != VerdictRefused || first.Precondition != PreDiagnosis || first.Escalated || first.Action != nil || !strings.Contains(first.Reason, "hypothesis") {
		t.Fatalf("a diagnosis that fails its schema is a refusal, got %+v", first)
	}
	v = v.Record(first)
	second := v.Decide(inc, phasecontract.DiagnosisAction{Kind: "run-shell"}, t0.Add(time.Hour))
	if second.Verdict != VerdictRefused || !second.Escalated {
		t.Fatalf("the second refusal of one incident escalates, got %+v", second)
	}
	if other := v.Decide(liveIncident(t, stallIncident), phasecontract.DiagnosisAction{Kind: "run-shell"}, t0); other.Escalated {
		t.Errorf("a refusal of another incident is a first refusal, got %+v", other)
	}
}

func TestDecision_EachRefusalEmitsProposalRefusedWithThePrecondition(t *testing.T) {
	t.Parallel()
	v := testValidator(t)
	inc := liveIncident(t, ciHangIncident)
	refused := v.Decide(inc, phasecontract.DiagnosisAction{Kind: "run-shell"}, t0)
	events := refused.Events()
	if len(events) != 1 || events[0].Code != CodeProposalRefused || events[0].Severity != signalcenter.SeverityWarn || events[0].Fields["precondition"] != PreCatalog ||
		events[0].Kind != signalcenter.KindSelfhealDecision || events[0].Module != signalcenter.ModuleHealing || events[0].Fields["incident"] != inc.ID || events[0].Fields["stage"] != "shadow" {
		t.Fatalf("a refusal emits one HEALING_PROPOSAL_REFUSED event with the precondition, got %+v", events)
	}
	escalated := v.Record(refused).Decide(inc, phasecontract.DiagnosisAction{Kind: "run-shell"}, t0)
	codes := []signalcenter.Code{}
	for _, e := range escalated.Events() {
		codes = append(codes, e.Code)
	}
	if !slices.Equal(codes, []signalcenter.Code{CodeProposalRefused, CodeEscalated}) || escalated.Events()[1].Severity != signalcenter.SeverityIncident {
		t.Errorf("an escalated refusal emits the refusal and the escalation, got %v", codes)
	}
	accepted := v.Decide(inc, openRepair(), t0).Events()
	if len(accepted) != 1 || accepted[0].Code != "" || accepted[0].Severity != signalcenter.SeverityInfo || accepted[0].Fields["verdict"] != VerdictAccepted || accepted[0].Fields["action"] != phasecontract.ActionOpenRepairItem {
		t.Errorf("an accepted proposal is one INFO event with no code, got %+v", accepted)
	}
}

func TestRepairDispatch_NeedsAnAcceptedProposal(t *testing.T) {
	t.Parallel()
	v := testValidator(t)
	inc := liveIncident(t, ciHangIncident)
	diagnosed := NewProposal(inc, "shadow", "", t0).WithDiagnosis(phasecontract.Diagnosis{Confidence: phasecontract.ConfidenceHigh, Action: openRepair()})
	for name, p := range map[string]Proposal{
		"a proposal with no decision": diagnosed,
		"a refused proposal":          diagnosed.WithDecision(v.Decide(inc, phasecontract.DiagnosisAction{Kind: "run-shell"}, t0)),
		"a failed run":                NewProposal(inc, "shadow", "", t0).Failed(errors.New("x")),
	} {
		if _, err := p.Dispatchable(); !errors.Is(err, ErrNoAcceptedProposal) {
			t.Errorf("%s: no repair may dispatch, got %v", name, err)
		}
	}
	accepted := diagnosed.WithDecision(v.Decide(inc, openRepair(), t0))
	got, err := accepted.Dispatchable()
	if err != nil || got.Kind != phasecontract.ActionOpenRepairItem {
		t.Fatalf("an accepted proposal dispatches its decided action, got %+v %v", got, err)
	}
}

func TestHistory_RoundTripsTheDecisionLedgerAndReadsTheRepairOutcomes(t *testing.T) {
	t.Parallel()
	evolveDir := t.TempDir()
	v := testValidator(t)
	first := v.Decide(liveIncident(t, ciHangIncident), openRepair(), t0)
	second := v.Refuse(liveIncident(t, stallIncident), errors.New("bad"), t0)
	if err := AppendDecisions(DecisionsPath(evolveDir), []Decision{first}); err != nil {
		t.Fatal(err)
	}
	if err := AppendDecisions(DecisionsPath(evolveDir), []Decision{second}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(RepairsPath(evolveDir), []byte(`{"incident_id":"inc-a","class":"stall","failed":true,"at":"2026-10-27T00:00:00Z"}`+"\n\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	h, err := LoadHistory(evolveDir)
	if err != nil || len(h.Decisions) != 2 || h.Decisions[0].Verdict != VerdictAccepted || h.Decisions[1].Precondition != PreDiagnosis || h.Decisions[0].Schema != DecisionSchema {
		t.Fatalf("decisions %+v %v", h.Decisions, err)
	}
	if len(h.Repairs) != 1 || !h.Repairs[0].Failed || h.Repairs[0].Class != ClassStall {
		t.Fatalf("repairs %+v", h.Repairs)
	}
	if empty, err := LoadHistory(t.TempDir()); err != nil || len(empty.Decisions)+len(empty.Repairs) != 0 {
		t.Fatalf("no ledger is an empty history: %+v %v", empty, err)
	}
}

func TestHistory_ACorruptOrUnreadableLedgerFails(t *testing.T) {
	t.Parallel()
	cases := map[string]func(dir string) error{
		"a corrupt decision": func(dir string) error {
			return os.WriteFile(DecisionsPath(dir), []byte("{\n"), 0o644)
		},
		"a corrupt repair": func(dir string) error {
			return os.WriteFile(RepairsPath(dir), []byte("[]\n"), 0o644)
		},
		"a ledger that is a directory": func(dir string) error {
			return os.MkdirAll(DecisionsPath(dir), 0o755)
		},
	}
	for name, setup := range cases {
		dir := t.TempDir()
		if err := os.MkdirAll(Dir(dir), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := setup(dir); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadHistory(dir); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}

func TestAppendDecisions_Faults(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	blocker := filepath.Join(dir, "file")
	if err := os.WriteFile(blocker, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	ok := []Decision{{IncidentID: "inc-a", At: t0}}
	cases := map[string]struct {
		path string
		ds   []Decision
	}{
		"a parent that is a file":  {filepath.Join(blocker, "decisions.jsonl"), ok},
		"a path that is a dir":     {dir, ok},
		"a time JSON cannot write": {filepath.Join(dir, "x.jsonl"), []Decision{{At: time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)}}},
	}
	for name, c := range cases {
		if err := AppendDecisions(c.path, c.ds); err == nil || !strings.Contains(err.Error(), "decisions") {
			t.Errorf("%s: want a decisions error, got %v", name, err)
		}
	}
}

func TestLoadLandings_ReadsTheLoopLandingsOfTheWindow(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "ship-journal.jsonl")
	body := `{"sha":"old","class":"cycle","ts":"2026-10-20T00:00:00Z","cycle":1}
{"sha":"manual","class":"manual","ts":"2026-10-27T00:00:00Z"}
{"sha":"nots","class":"cycle","cycle":3}
{"sha":"` + landingSHA + `","class":"cycle","ts":"2026-10-27T00:30:00Z","cycle":2038}
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := LoadLandings(path, t0.Add(-24*time.Hour))
	if err != nil || len(got) != 1 || got[0].SHA != landingSHA || !got[0].At.Equal(t0.Add(-30*time.Minute)) {
		t.Fatalf("only the loop landings of the window: %+v %v", got, err)
	}
	if err := os.WriteFile(path, []byte("{\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadLandings(path, t0); err == nil || !strings.Contains(err.Error(), "ship journal") {
		t.Fatalf("a corrupt journal fails: %v", err)
	}
}

func TestLoadLandings_ALandingAtTheWindowStartIsInTheWindow(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "ship-journal.jsonl")
	body := `{"sha":"` + landingSHA + `","class":"cycle","ts":"2026-10-27T00:30:00Z","cycle":2038}
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := LoadLandings(path, t0.Add(-30*time.Minute))
	if err != nil || len(got) != 1 || got[0].SHA != landingSHA {
		t.Fatalf("a landing at exactly the window start counts: %+v %v", got, err)
	}
}

func TestValidator_AnEarlierVariationCountsOnlyWhenItWasAnAcceptedRetryOfThisIncident(t *testing.T) {
	t.Parallel()
	retry := phasecontract.DiagnosisAction{Kind: phasecontract.ActionRetryWithVariation, Params: phasecontract.DiagnosisParams{Item: churnItem, Variation: "model"}}
	inc := liveIncident(t, churnIncident)
	for name, earlier := range map[string]Decision{
		"a retry of another incident": {IncidentID: "inc-other", Verdict: VerdictAccepted, Action: &retry, At: t0.Add(-time.Hour)},
		"a refused retry":             {IncidentID: inc.ID, Verdict: VerdictRefused, Action: &retry, At: t0.Add(-time.Hour)},
	} {
		v := testValidator(t).Record(earlier)
		if d := v.Decide(inc, retry, t0); d.Verdict != VerdictAccepted {
			t.Errorf("%s does not use the variation: %+v", name, d)
		}
	}
}

func TestBudget_AStopOrAQuarantineSpendsNoRepair(t *testing.T) {
	t.Parallel()
	v := testValidator(t)
	inc := liveIncident(t, ciHangIncident)
	halt := phasecontract.DiagnosisAction{Kind: phasecontract.ActionHaltAndEscalate}
	quarantine := phasecontract.DiagnosisAction{Kind: phasecontract.ActionQuarantineItem, Params: phasecontract.DiagnosisParams{Item: churnItem}}
	for i := range 3 {
		v = v.Record(v.Decide(inc, halt, t0.Add(time.Duration(i)*time.Minute)))
		v = v.Record(v.Decide(liveIncident(t, churnIncident), quarantine, t0.Add(time.Duration(i)*time.Minute)))
	}
	if d := v.Decide(inc, openRepair(), t0.Add(time.Hour)); d.Verdict != VerdictAccepted {
		t.Fatalf("three stops and three quarantines leave the T1 budget whole, got %+v", d)
	}
}

func TestBreaker_ARepairAcceptedBeforeTheBreakerOpenedDoesNotUseTheHalfOpenSlot(t *testing.T) {
	t.Parallel()
	inc := liveIncident(t, stallIncident)
	v := testValidator(t)
	action := openRepair()
	v = v.Record(Decision{IncidentID: "inc-other-0", Class: ClassStall, Tier: TierPipeline, Verdict: VerdictAccepted, Action: &action, At: t0.Add(-3 * time.Hour)})
	v.History.Repairs = []Repair{
		{IncidentID: "inc-other-1", Class: ClassStall, Failed: true, At: t0.Add(-2 * time.Hour)},
		{IncidentID: "inc-other-2", Class: ClassStall, Failed: true, At: t0.Add(-time.Hour)},
	}
	if d := v.Decide(inc, openRepair(), t0.Add(23*time.Hour)); d.Verdict != VerdictAccepted {
		t.Fatalf("the half-open slot is free when the only accepted repair is older than the open time, got %+v", d)
	}
}

func TestDecision_AnAcceptedHaltEmitsOnlyTheEscalation(t *testing.T) {
	t.Parallel()
	d := testValidator(t).Decide(liveIncident(t, ciHangIncident), phasecontract.DiagnosisAction{Kind: phasecontract.ActionHaltAndEscalate}, t0)
	events := d.Events()
	if len(events) != 1 || events[0].Code != CodeEscalated || events[0].Fields["verdict"] != VerdictAccepted {
		t.Fatalf("an accepted halt-and-escalate is one HEALING_ESCALATED event, got %+v", events)
	}
}

func TestBudget_ALoopTierWithNoBudgetRefusesEachRepair(t *testing.T) {
	t.Parallel()
	v := testValidator(t)
	v.Limits.Repairs = map[string]int{TierPipeline: 3}
	if d := v.Decide(liveIncident(t, churnIncident), openRepair(), t0); d.Verdict != VerdictRefused || d.Precondition != PreBudget {
		t.Fatalf("a T2 repair with no T2 budget must be refused, got %+v", d)
	}
}

func TestBreaker_AFailInTheHalfOpenStateOpensItForAFullOpenTime(t *testing.T) {
	t.Parallel()
	inc := liveIncident(t, stallIncident)
	halfOpen := t0.Add(23 * time.Hour)
	v := testValidator(t)
	v.History.Repairs = []Repair{
		{IncidentID: "inc-other-1", Class: ClassStall, Failed: true, At: t0.Add(-2 * time.Hour)},
		{IncidentID: "inc-other-2", Class: ClassStall, Failed: true, At: t0.Add(-time.Hour)},
		{IncidentID: "inc-other-3", Class: ClassStall, Failed: true, At: halfOpen.Add(time.Hour)},
	}
	if d := v.Decide(inc, openRepair(), halfOpen.Add(2*time.Hour)); d.Verdict != VerdictRefused || d.Precondition != PreBreaker {
		t.Fatalf("a FAIL in the half-open state opens the breaker again from that FAIL, got %+v", d)
	}
	if d := v.Decide(inc, openRepair(), halfOpen.Add(25*time.Hour)); d.Verdict != VerdictAccepted {
		t.Errorf("after the new open time the breaker is half-open again, got %+v", d)
	}
}

func TestBreaker_TheHalfOpenSlotIsForTheClassOfTheBreaker(t *testing.T) {
	t.Parallel()
	inc := liveIncident(t, stallIncident)
	halfOpen := t0.Add(23 * time.Hour)
	v := testValidator(t)
	v.History.Repairs = []Repair{
		{IncidentID: "inc-other-1", Class: ClassStall, Failed: true, At: t0.Add(-2 * time.Hour)},
		{IncidentID: "inc-other-2", Class: ClassStall, Failed: true, At: t0.Add(-time.Hour)},
	}
	v = v.Record(v.Decide(liveIncident(t, ciHangIncident), openRepair(), halfOpen))
	if d := v.Decide(inc, openRepair(), halfOpen.Add(time.Minute)); d.Verdict != VerdictAccepted {
		t.Fatalf("a ci-hang repair does not use the half-open slot of the stall breaker, got %+v", d)
	}
}

func TestBudget_ADecisionExactlyOneWindowOldIsOutOfTheWindow(t *testing.T) {
	t.Parallel()
	v := testValidator(t)
	inc := liveIncident(t, ciHangIncident)
	for i := range 3 {
		v = v.Record(v.Decide(inc, openRepair(), t0.Add(time.Duration(i)*time.Minute)))
	}
	if d := v.Decide(inc, openRepair(), t0.Add(24*time.Hour)); d.Verdict != VerdictAccepted {
		t.Fatalf("the repair at the start of the window is out of it, got %+v", d)
	}
}

func TestValidator_AFirstRefusalAfterAnAcceptedDecisionDoesNotEscalate(t *testing.T) {
	t.Parallel()
	v := testValidator(t)
	inc := liveIncident(t, ciHangIncident)
	v = v.Record(v.Decide(inc, openRepair(), t0))
	if d := v.Decide(inc, phasecontract.DiagnosisAction{Kind: "run-shell"}, t0.Add(time.Hour)); d.Verdict != VerdictRefused || d.Escalated {
		t.Fatalf("only an earlier refusal makes a refusal the second one, got %+v", d)
	}
}

var personaRefusalWords = map[string]string{
	PreItemKnown: "pending inbox item", PreVariationNew: "same variation", PreFamilyKnown: "not installed", PreItemOfIncident: "subject of the incident",
	PreMainRed: "main is not red", PreLandingInWindow: "landing of the loop", PreFilesExist: "does not exist",
	PreRequestValid: "did not validate",
}

func TestCatalog_ThePersonaStatesTheRefusalsOfEachAction(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile("../../../agents/evolve-selfheal-orchestrator.md")
	if err != nil {
		t.Fatal(err)
	}
	rows := map[string]string{}
	for _, line := range strings.Split(string(raw), "\n") {
		if kind, ok := strings.CutPrefix(line, "| `"); ok && strings.Contains(kind, "` |") {
			rows[kind[:strings.Index(kind, "`")]] = line
		}
	}
	for _, a := range catalog {
		row, ok := rows[a.kind]
		if !ok {
			t.Errorf("the persona has no row for %s", a.kind)
			continue
		}
		if loopOnly := !slices.Contains(a.tiers, TierKernel); loopOnly != strings.Contains(row, "T0") {
			t.Errorf("the row of %s must name T0 exactly when the action does not run at T0: %s", a.kind, row)
		}
		for _, p := range a.pre {
			words, ok := personaRefusalWords[p.name]
			if !ok || !strings.Contains(row, words) {
				t.Errorf("the row of %s must state the refusal %s (%q): %s", a.kind, p.name, words, row)
			}
		}
	}
}
