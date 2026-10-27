package selfheal

import (
	"slices"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func signal(seq uint64, kind signalcenter.Kind, code signalcenter.Code, at time.Time, fields map[string]string) signalcenter.Event {
	return signalcenter.Event{PID: 9, Seq: seq, TS: at.Format(time.RFC3339Nano), Kind: kind, Code: code, Severity: signalcenter.SeverityWarn, Fields: fields}
}

func onlyObservation(t *testing.T, obs []Observation) Observation {
	t.Helper()
	if len(obs) != 1 {
		t.Fatalf("want one observation, got %d: %+v", len(obs), obs)
	}
	return obs[0]
}

func TestFromSignals_EachDetectionSourceOfTheSpecMapsToItsClass(t *testing.T) {
	t.Parallel()
	cases := []struct {
		event       signalcenter.Event
		fingerprint string
	}{
		{signal(1, signalcenter.KindQualityBreach, "QUALITY_PQ1_BREACH", t0, nil), "quality:QUALITY_PQ1_BREACH"},
		{signal(2, signalcenter.KindIsolationViolation, "ISOLATION_I5_VIOLATION", t0, map[string]string{"stage": "enforce", "rule": "read.runs.unrelated"}), "isolation:ISOLATION_I5_VIOLATION:read.runs.unrelated"},
		{signal(3, signalcenter.KindPaneLiveness, "LIVENESS_PHASE_STALLED", t0, nil), "stall:LIVENESS_PHASE_STALLED:unknown"},
		{signal(4, signalcenter.KindSystemFailure, "ORCHESTRATOR_SYSTEM_FAILURE", t0, nil), "system:ORCHESTRATOR_SYSTEM_FAILURE"},
		{signal(5, signalcenter.KindLoopHalt, "LOOP_ZERO_SHIP_HALT", t0, nil), "zero-ship:loop"},
		{signal(6, signalcenter.KindLoopWave, "LOOP_ORCH_SUPPLY_SHORT", t0, map[string]string{"admissible": "2", "window": "4"}), "lane-starvation:supply"},
	}
	for _, c := range cases {
		o := onlyObservation(t, FromSignals([]signalcenter.Event{c.event}))
		if o.Fingerprint() != c.fingerprint || len(o.Evidence) != 1 || !o.Evidence[0].At.Equal(t0) || o.Summary == "" {
			t.Errorf("%s: got %q %+v, want %q", c.event.Code, o.Fingerprint(), o, c.fingerprint)
		}
	}
}

func TestFromSignals_AStallNamesItsPhaseAndASystemFailureKeepsItsSeverity(t *testing.T) {
	t.Parallel()
	stall := signal(1, signalcenter.KindPaneLiveness, "LIVENESS_PHASE_STALLED", t0, nil)
	stall.Phase = "build"
	if got := onlyObservation(t, FromSignals([]signalcenter.Event{stall})).Fingerprint(); got != "stall:LIVENESS_PHASE_STALLED:build" {
		t.Fatalf("fingerprint %q", got)
	}
	failure := signal(2, signalcenter.KindSystemFailure, "ORCHESTRATOR_SYSTEM_FAILURE", t0, nil)
	failure.Severity = signalcenter.SeverityIncident
	if got := onlyObservation(t, FromSignals([]signalcenter.Event{failure})).Severity; got != signalcenter.SeverityIncident {
		t.Fatalf("severity %s", got)
	}
}

func TestFromSignals_IgnoresAShadowViolationOtherCodesAndABadTime(t *testing.T) {
	t.Parallel()
	events := []signalcenter.Event{
		signal(1, signalcenter.KindIsolationViolation, "ISOLATION_I2_VIOLATION", t0, map[string]string{"stage": "shadow"}),
		signal(2, signalcenter.KindGatePassed, "ORCHESTRATOR_SCOPE_DELTA", t0, nil),
		signal(3, signalcenter.KindLoopWave, "LOOP_ORCH_POOL_UNDERFILLED", t0, nil),
		{PID: 9, Seq: 4, TS: "yesterday", Kind: signalcenter.KindQualityBreach, Code: "QUALITY_PQ1_BREACH"},
		signal(5, signalcenter.KindQualityWarning, "QUALITY_VACUOUS_DENOMINATOR", t0, nil),
	}
	if got := FromSignals(events); len(got) != 0 {
		t.Fatalf("a shadow violation, a passed gate, an underfilled pool, an unreadable time and a quality warning open nothing: %+v", got)
	}
}

func TestFromSignals_GroupsOneFingerprintAndDropsTheSameEventSeenOnTwoChannels(t *testing.T) {
	t.Parallel()
	a, b := pq1Breach(1, t0), pq1Breach(2, t0.Add(time.Minute))
	o := onlyObservation(t, FromSignals([]signalcenter.Event{a, b, a}))
	if len(o.Evidence) != 2 || o.Evidence[0].ID != "signal:7.1" || o.Evidence[1].ID != "signal:7.2" {
		t.Fatalf("evidence %+v", o.Evidence)
	}
}

func TestFromCIRuns_TheLatestCompletedMainRunDecidesARedMain(t *testing.T) {
	t.Parallel()
	green := CIRun{ID: 41, Branch: "main", Status: "completed", Conclusion: "success", Attempt: 1, Created: t0.Add(time.Hour), Updated: t0.Add(80 * time.Minute)}
	running := CIRun{ID: 42, Branch: "main", Status: "in_progress", Attempt: 1, Created: t0.Add(2 * time.Hour)}
	if got := FromCIRuns([]CIRun{redMain(40, t0), green, running}, t0.Add(2*time.Hour)); len(got) != 0 {
		t.Fatalf("a green run after the red one clears main: %+v", got)
	}
	o := onlyObservation(t, FromCIRuns([]CIRun{green, redMain(43, t0.Add(3*time.Hour))}, t0.Add(4*time.Hour)))
	if o.Fingerprint() != "main-red:main" || o.Evidence[0].ID != "ci:43" || o.Severity != signalcenter.SeverityIncident {
		t.Fatalf("observation %+v", o)
	}
}

func TestFromCIRuns_AMainRunThatRanPastTheHangLimitIsACIHang(t *testing.T) {
	t.Parallel()
	hung := CIRun{ID: 40265789886, Branch: "main", Status: "completed", Conclusion: "cancelled", Attempt: 1, Created: t0, Updated: t0.Add(5*time.Hour + 8*time.Minute)}
	superseded := CIRun{ID: 2, Branch: "main", Status: "completed", Conclusion: "cancelled", Attempt: 1, Created: t0, Updated: t0.Add(3 * time.Minute)}
	stuck := CIRun{ID: 3, Branch: "main", Status: "in_progress", Attempt: 1, Created: t0}
	young := CIRun{ID: 4, Branch: "main", Status: "queued", Attempt: 1, Created: t0.Add(5 * time.Hour)}
	pr := CIRun{ID: 5, Branch: "feat/x", Status: "completed", Conclusion: "cancelled", Attempt: 1, Created: t0, Updated: t0.Add(6 * time.Hour)}
	o := onlyObservation(t, FromCIRuns([]CIRun{hung, superseded, stuck, young, pr}, t0.Add(5*time.Hour+30*time.Minute)))
	ids := []string{o.Evidence[0].ID, o.Evidence[1].ID}
	slices.Sort(ids)
	if o.Fingerprint() != "ci-hang:main" || !slices.Equal(ids, []string{"ci:3", "ci:40265789886"}) {
		t.Fatalf("a five-hour cancelled run and a run still open past %v hang; a superseded run, a young run and a branch run do not: %+v", HangAfter, o)
	}
}

func TestFromCIRuns_ARerunThatTurnedGreenIsAFlake(t *testing.T) {
	t.Parallel()
	flake := CIRun{ID: 40269836920, Branch: "docs/cell-context-index", Status: "completed", Conclusion: "success", Attempt: 2, Created: t0, Updated: t0.Add(time.Hour)}
	firstTry := CIRun{ID: 7, Branch: "docs/x", Status: "completed", Conclusion: "success", Attempt: 1, Created: t0, Updated: t0.Add(time.Hour)}
	rerunRed := CIRun{ID: 8, Branch: "docs/y", Status: "completed", Conclusion: "failure", Attempt: 2, Created: t0, Updated: t0.Add(time.Hour)}
	o := onlyObservation(t, FromCIRuns([]CIRun{flake, firstTry, rerunRed}, t0.Add(2*time.Hour)))
	if o.Fingerprint() != "flaky-test:rerun-green" || len(o.Evidence) != 1 || o.Evidence[0].ID != "ci:40269836920" || !o.Evidence[0].At.Equal(t0.Add(time.Hour)) {
		t.Fatalf("observation %+v", o)
	}
}

func TestFromCycles_ARepairLoopThatDidNotShipAndAnItemThatManyCyclesClaimed(t *testing.T) {
	t.Parallel()
	var cycles []Cycle
	for i, id := range []int{1983, 2001, 2004, 2008, 2010, 2015, 2020, 2026, 2030, 2035} {
		cycles = append(cycles, Cycle{ID: id, Item: "retro-minting-files-through-inbox-add-lane", Started: t0.Add(time.Duration(i) * time.Hour)})
	}
	cycles[9].RepairAttempts = 2
	cycles = append(cycles, Cycle{ID: 2036, Item: "practice-agy", Shipped: true, RepairAttempts: 3, Started: t0})
	obs := FromCycles(cycles)
	byPrint := map[string]Observation{}
	for _, o := range obs {
		byPrint[o.Fingerprint()] = o
	}
	if len(obs) != 2 {
		t.Fatalf("want a repair loop and a claim churn, got %+v", obs)
	}
	loop := byPrint["repair-loop:retro-minting-files-through-inbox-add-lane"]
	if len(loop.Evidence) != 1 || loop.Evidence[0].ID != "cycle:2035" {
		t.Fatalf("repair loop %+v", loop)
	}
	churn := byPrint["claim-churn:retro-minting-files-through-inbox-add-lane"]
	if len(churn.Evidence) != 10 || churn.Evidence[0].ID != "cycle:1983" || churn.Evidence[9].ID != "cycle:2035" {
		t.Fatalf("ten claims with no ship are one churn with ten sources: %+v", churn)
	}
}

func TestFromCycles_AShipResetsTheClaimCountAndAShortRunIsNoChurn(t *testing.T) {
	t.Parallel()
	cycles := []Cycle{
		{ID: 1, Item: "a", Started: t0}, {ID: 2, Item: "a", Started: t0}, {ID: 3, Item: "a", Started: t0},
		{ID: 4, Item: "a", Shipped: true, Started: t0}, {ID: 5, Item: "a", Started: t0}, {ID: 6, Item: "a", Started: t0},
		{ID: 7, Started: t0}, {ID: 8, Started: t0}, {ID: 9, Started: t0},
	}
	if got := FromCycles(cycles); len(got) != 0 {
		t.Fatalf("two claims after a ship, and cycles with no item, are no churn: %+v", got)
	}
}

func TestFromCycles_TheCyclesNeedNotComeInOrder(t *testing.T) {
	t.Parallel()
	cycles := []Cycle{{ID: 9, Item: "a", Started: t0}, {ID: 3, Item: "a", Shipped: true, Started: t0}, {ID: 7, Item: "a", Started: t0}, {ID: 8, Item: "a", Started: t0}}
	o := onlyObservation(t, FromCycles(cycles))
	if o.Evidence[0].ID != "cycle:7" || len(o.Evidence) != ChurnAfter {
		t.Fatalf("the claims after the ship of cycle 3 count: %+v", o)
	}
}

func TestFromUsage_ADeclaredFamilyThatReadsNoWindowOrAStaleOneIsBlind(t *testing.T) {
	t.Parallel()
	now := t0.Add(48 * time.Hour)
	readings := []UsageReading{
		{Family: "claude", Declared: true, ObservedAt: now.Add(-time.Hour), Windows: 0},
		{Family: "agy", Declared: true, ObservedAt: now.Add(-time.Hour), Windows: 5},
		{Family: "codex", Declared: true, ObservedAt: now.Add(-UsageStaleAfter - time.Minute), Windows: 3},
		{Family: "ollama", Declared: false, ObservedAt: now.Add(-time.Hour)},
		{Family: "gemini", Declared: true, ObservedAt: now.Add(-time.Hour), Windows: 2, Error: "capture stopped"},
	}
	var got []string
	for _, o := range FromUsage(readings, now) {
		got = append(got, o.Fingerprint())
		if o.Evidence[0].ID != "usage:"+o.Subject+"@"+o.Evidence[0].At.UTC().Format(time.RFC3339) {
			t.Errorf("evidence id %q", o.Evidence[0].ID)
		}
	}
	slices.Sort(got)
	if !slices.Equal(got, []string{"usage-blind:claude", "usage-blind:codex", "usage-blind:gemini"}) {
		t.Fatalf("blind families %v", got)
	}
}

func TestFromSignals_TheHighestSeverityOfOneFingerprintWins(t *testing.T) {
	t.Parallel()
	warn := signal(1, signalcenter.KindSystemFailure, "ORCHESTRATOR_SYSTEM_FAILURE", t0, nil)
	incident := signal(2, signalcenter.KindSystemFailure, "ORCHESTRATOR_SYSTEM_FAILURE", t0.Add(time.Minute), nil)
	incident.Severity = signalcenter.SeverityIncident
	later := signal(3, signalcenter.KindSystemFailure, "ORCHESTRATOR_SYSTEM_FAILURE", t0.Add(2*time.Minute), nil)
	o := onlyObservation(t, FromSignals([]signalcenter.Event{warn, incident, later}))
	if o.Severity != signalcenter.SeverityIncident || len(o.Evidence) != 3 {
		t.Fatalf("one INCIDENT event rates the whole observation INCIDENT: %+v", o)
	}
}

func TestFromCycles_TheProtectedFindingsOfEachRepairLoopCycleOfOneItemAreMerged(t *testing.T) {
	t.Parallel()
	first := make([]string, 1, 2)
	first[0] = "go/internal/guards/role.go"
	cycles := []Cycle{
		{ID: 1, Item: "a", RepairAttempts: 2, Started: t0, ProtectedFindings: first},
		{ID: 2, Item: "a", RepairAttempts: 2, Started: t0.Add(time.Hour), ProtectedFindings: []string{"go/internal/guards/ship.go", "go/internal/guards/role.go"}},
	}
	var loop Observation
	for _, o := range FromCycles(cycles) {
		if o.Class == ClassRepairLoop {
			loop = o
		}
	}
	if !slices.Equal(loop.Protected, []string{"go/internal/guards/role.go", "go/internal/guards/ship.go"}) || len(loop.Evidence) != 2 {
		t.Fatalf("protected %v", loop.Protected)
	}
	if first[:2][1] != "" {
		t.Fatal("the merge must not change the findings of the caller")
	}
}

func TestFromCycles_ASystemLevelFailureDecisionIsASystemIncident(t *testing.T) {
	t.Parallel()
	cycles := []Cycle{{ID: 5, FailureLevel: "system", Started: t0}, {ID: 6, Item: "a", FailureLevel: "task", Started: t0}}
	o := onlyObservation(t, FromCycles(cycles))
	if o.Fingerprint() != "system:failure-decision" || o.Evidence[0].ID != "cycle:5" || o.Severity != signalcenter.SeverityIncident {
		t.Fatalf("observation %+v", o)
	}
}

func TestFromSignals_ALauncherDropAtEnforceIsTheControlAndACellAttemptAtEnforceIsAnIncident(t *testing.T) {
	t.Parallel()
	drop := signal(1, signalcenter.KindIsolationViolation, "ISOLATION_I2_VIOLATION", t0, map[string]string{"stage": "enforce", "action": "refused", "source": "launcher", "rule": "env.allowlist"})
	read := signal(2, signalcenter.KindIsolationViolation, "ISOLATION_I5_VIOLATION", t0, map[string]string{"stage": "enforce", "action": "refused", "source": "sbpl-report", "rule": "read.runs.unrelated"})
	shadowRead := signal(3, signalcenter.KindIsolationViolation, "ISOLATION_I5_VIOLATION", t0, map[string]string{"stage": "shadow", "source": "sbpl-report", "rule": "read.runs.unrelated"})
	got, _ := Detect(nil, FromSignals([]signalcenter.Event{drop, read, shadowRead}), fakeVerifier{}, t0)
	if len(got) != 1 || got[0].Fingerprint != "isolation:ISOLATION_I5_VIOLATION:read.runs.unrelated" || got[0].Recurrence != 1 {
		t.Fatalf("only the read that the cell tried at enforce opens a record: %+v", got)
	}
}
