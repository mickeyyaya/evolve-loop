package selfheal

import (
	"slices"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

var t0 = time.Date(2026, 10, 27, 1, 0, 0, 0, time.UTC)

type fakeVerifier struct {
	repro, waveEnded, retro bool
}

func (f fakeVerifier) ReproGreen(Incident) bool                 { return f.repro }
func (f fakeVerifier) WaveEndedAfter(time.Time) bool            { return f.waveEnded }
func (f fakeVerifier) RetrospectiveExists(string) bool          { return f.retro }
func allClear() fakeVerifier                                    { return fakeVerifier{true, true, true} }
func breach(seq uint64, at time.Time) signalcenter.Event        { return pq1Breach(seq, at) }
func observationsOf(events ...signalcenter.Event) []Observation { return FromSignals(events) }

func pq1Breach(seq uint64, at time.Time) signalcenter.Event {
	return signalcenter.Event{
		PID: 7, Seq: seq, TS: at.Format(time.RFC3339Nano), Module: signalcenter.ModuleQuality,
		Kind: signalcenter.KindQualityBreach, Severity: signalcenter.SeverityWarn, Code: "QUALITY_PQ1_BREACH",
		Fields: map[string]string{"index": "PQ1"},
	}
}

func redMain(id int64, at time.Time) CIRun {
	return CIRun{ID: id, Branch: "main", Status: "completed", Conclusion: "failure", Attempt: 1, Created: at, Updated: at.Add(20 * time.Minute)}
}

func TestDetect_ARedMainAndAPQ1BreachEachOpenOneIncidentRecord(t *testing.T) {
	t.Parallel()
	obs := append(FromSignals([]signalcenter.Event{pq1Breach(1, t0)}), FromCIRuns([]CIRun{redMain(40, t0)}, t0)...)
	got, transitions := Detect(nil, obs, fakeVerifier{}, t0)
	if len(got) != 2 || len(transitions) != 2 {
		t.Fatalf("a red main and a PQ1 breach must open two records, got %d records and %d transitions: %+v", len(got), len(transitions), got)
	}
	classes := []Class{got[0].Class, got[1].Class}
	slices.Sort(classes)
	if !slices.Equal(classes, []Class{ClassMainRed, ClassQuality}) {
		t.Fatalf("classes = %v, want main-red and quality", classes)
	}
	for _, inc := range got {
		if inc.State != StateOpen || inc.ID == "" || inc.OpenedAt.IsZero() || inc.ClosedAt != nil || len(inc.Sources) != 1 || inc.Schema != RecordSchema {
			t.Errorf("an opened record needs an id, the open state, its source and an open time: %+v", inc)
		}
	}
	for _, tr := range transitions {
		if tr.Kind != TransitionOpened {
			t.Errorf("transition %q, want opened", tr.Kind)
		}
	}
}

func TestDetect_ASecondPassOverTheSameEvidenceChangesNothing(t *testing.T) {
	t.Parallel()
	obs := observationsOf(breach(1, t0))
	first, _ := Detect(nil, obs, fakeVerifier{}, t0)
	second, transitions := Detect(first, obs, fakeVerifier{}, t0.Add(time.Hour))
	if len(transitions) != 0 || len(second) != 1 || second[0].Recurrence != 1 {
		t.Fatalf("the same evidence must not open, count or announce twice: %+v %+v", second, transitions)
	}
}

func TestDetect_NewEvidenceOfOneFingerprintRaisesTheRecurrenceAndThenTheSeverity(t *testing.T) {
	t.Parallel()
	first, _ := Detect(nil, observationsOf(breach(1, t0)), fakeVerifier{}, t0)
	if first[0].Severity != signalcenter.SeverityWarn {
		t.Fatalf("one breach is a WARN: %+v", first[0])
	}
	later := observationsOf(breach(1, t0), breach(2, t0.Add(time.Minute)), breach(3, t0.Add(2*time.Minute)))
	got, transitions := Detect(first, later, fakeVerifier{}, t0.Add(time.Hour))
	if len(got) != 1 || got[0].Recurrence != 3 || got[0].ID != first[0].ID || len(transitions) != 0 {
		t.Fatalf("three breaches of one fingerprint are one incident with recurrence 3: %+v %+v", got, transitions)
	}
	if got[0].Severity != signalcenter.SeverityIncident {
		t.Fatalf("a recurrence of %d rates INCIDENT: %+v", EscalateAt, got[0])
	}
	if !got[0].LastSeenAt.Equal(t0.Add(2 * time.Minute)) {
		t.Fatalf("last_seen_at follows the newest evidence: %v", got[0].LastSeenAt)
	}
}

func TestDetect_ALevelTriggeredPassReopensAnIncidentWhoseSignalStillFiresAfterAMissedEvent(t *testing.T) {
	t.Parallel()
	opened, _ := Detect(nil, observationsOf(breach(1, t0)), fakeVerifier{}, t0)
	closed, transitions := Detect(opened, nil, allClear(), t0.Add(time.Hour))
	if closed[0].State != StateClosed || len(transitions) != 1 || transitions[0].Kind != TransitionClosed {
		t.Fatalf("with the four conditions and no new evidence the incident closes: %+v %+v", closed, transitions)
	}
	stillFiring := observationsOf(breach(1, t0), breach(9, t0.Add(2*time.Hour)))
	got, transitions := Detect(closed, stillFiring, allClear(), t0.Add(3*time.Hour))
	if got[0].State != StateOpen || got[0].Reopened != 1 || got[0].ClosedAt != nil || got[0].ID != opened[0].ID {
		t.Fatalf("the channel still holds a breach after the close, so the pass reopens the same record: %+v", got[0])
	}
	if len(transitions) != 1 || transitions[0].Kind != TransitionReopened {
		t.Fatalf("a reopen is announced once: %+v", transitions)
	}
}

func TestDetect_AClosedIncidentStaysClosedWhileOnlyOtherFingerprintsFire(t *testing.T) {
	t.Parallel()
	opened, _ := Detect(nil, observationsOf(breach(1, t0)), fakeVerifier{}, t0)
	closed, _ := Detect(opened, nil, allClear(), t0.Add(time.Hour))
	got, transitions := Detect(closed, FromCIRuns([]CIRun{redMain(40, t0.Add(2*time.Hour))}, t0), allClear(), t0.Add(2*time.Hour))
	for _, inc := range got {
		if inc.Class == ClassQuality && inc.State != StateClosed {
			t.Fatalf("a red main does not reopen the quality incident: %+v", inc)
		}
	}
	if len(transitions) != 1 || transitions[0].Kind != TransitionOpened {
		t.Fatalf("only the red main opens: %+v", transitions)
	}
}

func TestDetect_APersistentConditionWithAFixedEvidenceTimeNeverClosesAndReopensAClosedRecord(t *testing.T) {
	t.Parallel()
	stale := FromUsage([]UsageReading{{Family: "claude", Declared: true, ObservedAt: t0}}, t0.Add(48*time.Hour))
	opened, _ := Detect(nil, stale, fakeVerifier{}, t0.Add(48*time.Hour))
	still, transitions := Detect(opened, stale, allClear(), t0.Add(49*time.Hour))
	if still[0].State != StateOpen || len(transitions) != 0 {
		t.Fatalf("the stale reading is still observed, so the signal is not clear: %+v %+v", still, transitions)
	}
	closedAt := t0.Add(50 * time.Hour)
	closed := still[0]
	closed.State, closed.ClosedAt = StateClosed, &closedAt
	got, transitions := Detect([]Incident{closed}, stale, allClear(), t0.Add(51*time.Hour))
	if got[0].State != StateOpen || got[0].Reopened != 1 || len(transitions) != 1 || transitions[0].Kind != TransitionReopened || got[0].Recurrence != 1 {
		t.Fatalf("a closed record whose condition is still observed reopens, and the old evidence does not count twice: %+v %+v", got, transitions)
	}
}

func TestDetect_AnOpenIncidentWithNewEvidenceDoesNotClose(t *testing.T) {
	t.Parallel()
	opened, _ := Detect(nil, observationsOf(breach(1, t0)), fakeVerifier{}, t0)
	got, transitions := Detect(opened, observationsOf(breach(2, t0.Add(time.Minute))), allClear(), t0.Add(time.Hour))
	if got[0].State != StateOpen || len(transitions) != 0 {
		t.Fatalf("a signal that fires in this pass is not clear: %+v %+v", got, transitions)
	}
}

func TestDetect_AQualityBreachAfterTheOpeningHoldsEveryOtherIncidentOpen(t *testing.T) {
	t.Parallel()
	opened, _ := Detect(nil, FromCIRuns([]CIRun{redMain(40, t0)}, t0), fakeVerifier{}, t0)
	got, _ := Detect(opened, observationsOf(breach(1, t0.Add(time.Minute))), allClear(), t0.Add(time.Hour))
	for _, inc := range got {
		if inc.Class == ClassMainRed && inc.State != StateOpen {
			t.Fatalf("a quality index regressed after the opening, so condition 3 fails: %+v", inc)
		}
	}
}

func TestDetect_ProtectedEvidenceRatesTierT0AndIncident(t *testing.T) {
	t.Parallel()
	obs := FromCycles([]Cycle{{ID: 2035, Item: "retro-minting", Started: t0, RepairAttempts: 2, ProtectedFindings: []string{"go/internal/guards/role.go"}}})
	got, _ := Detect(nil, obs, fakeVerifier{}, t0)
	if len(got) != 1 || got[0].Tier != TierKernel || got[0].Severity != signalcenter.SeverityIncident {
		t.Fatalf("a blocking finding on a protected surface is T0 and INCIDENT: %+v", got)
	}
}

func TestDetect_TheRecordKeepsOnlyTheNewestSources(t *testing.T) {
	t.Parallel()
	var events []signalcenter.Event
	for i := range MaxSources + 5 {
		events = append(events, breach(uint64(i+1), t0.Add(time.Duration(i)*time.Minute)))
	}
	got, _ := Detect(nil, observationsOf(events...), fakeVerifier{}, t0)
	if len(got[0].Sources) != MaxSources || got[0].Recurrence != MaxSources+5 || got[0].Sources[MaxSources-1] != "signal:7.25" {
		t.Fatalf("the record keeps the %d newest sources and counts each: %+v", MaxSources, got[0])
	}
}

func TestDetect_ReturnsTheRecordsInIDOrder(t *testing.T) {
	t.Parallel()
	obs := append(observationsOf(breach(1, t0)), FromCIRuns([]CIRun{redMain(40, t0)}, t0)...)
	got, _ := Detect(nil, obs, fakeVerifier{}, t0)
	if got[0].ID > got[1].ID {
		t.Fatalf("records sort by id: %s, %s", got[0].ID, got[1].ID)
	}
}

func TestDetect_TwoObservationsOfOneFingerprintKeepTheRecordFresh(t *testing.T) {
	t.Parallel()
	opened, _ := Detect(nil, observationsOf(breach(1, t0)), fakeVerifier{}, t0)
	newer, older := observationsOf(breach(2, t0.Add(time.Minute)))[0], observationsOf(breach(1, t0))[0]
	got, transitions := Detect(opened, []Observation{newer, older}, allClear(), t0.Add(time.Hour))
	if got[0].State != StateOpen || len(transitions) != 0 {
		t.Fatalf("new evidence in one observation keeps the record open, whatever the order: %+v %+v", got, transitions)
	}
}

func TestDetect_AClosedRecordWithNoCloseTimeReopensOnNewEvidence(t *testing.T) {
	t.Parallel()
	opened, _ := Detect(nil, observationsOf(breach(1, t0)), fakeVerifier{}, t0)
	broken := opened[0]
	broken.State = StateClosed
	got, transitions := Detect([]Incident{broken}, observationsOf(breach(2, t0.Add(time.Minute))), fakeVerifier{}, t0.Add(time.Hour))
	if got[0].State != StateOpen || len(transitions) != 1 || transitions[0].Kind != TransitionReopened {
		t.Fatalf("a closed record with closed_at null reopens and does not panic: %+v %+v", got, transitions)
	}
}

func TestDetect_AQualityBreachBeforeTheOpeningDoesNotHoldAnotherIncidentOpen(t *testing.T) {
	t.Parallel()
	opened, _ := Detect(nil, FromCIRuns([]CIRun{redMain(40, t0)}, t0), fakeVerifier{}, t0.Add(time.Hour))
	got, _ := Detect(opened, observationsOf(breach(1, t0)), allClear(), t0.Add(2*time.Hour))
	for _, inc := range got {
		if inc.Class == ClassMainRed && inc.State != StateClosed {
			t.Fatalf("a breach older than the opening is no regression, so the red main closes: %+v", inc)
		}
	}
}

func TestDetect_AnotherClassFiringDoesNotHoldAQualityIncidentOpen(t *testing.T) {
	t.Parallel()
	opened, _ := Detect(nil, observationsOf(breach(1, t0)), fakeVerifier{}, t0)
	got, _ := Detect(opened, FromCIRuns([]CIRun{redMain(40, t0.Add(time.Minute))}, t0), allClear(), t0.Add(time.Hour))
	for _, inc := range got {
		if inc.Class == ClassQuality && inc.State != StateClosed {
			t.Fatalf("a red main is not a quality regression: %+v", inc)
		}
	}
}
