package cyclespend

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/llmcalls"
)

func spanOfMinutes(gate string, startMinute, minutes int, load float64) GateSpan {
	return GateSpan{Gate: gate, Start: minuteMark(startMinute), End: minuteMark(startMinute + minutes), Load: load}
}

type steppedClock struct {
	at   time.Time
	step time.Duration
}

func (c *steppedClock) now() time.Time {
	t := c.at
	c.at = c.at.Add(c.step)
	return t
}

type scriptedLoad struct {
	values []float64
	reads  int
}

func (l *scriptedLoad) read() float64 {
	v := l.values[l.reads%len(l.values)]
	l.reads++
	return v
}

func TestVerdict_CountsHostMinutes(t *testing.T) {
	t.Parallel()
	calls := []llmcalls.Record{callEndingAt(10, 10), callEndingAt(30, 20), callEndingAt(50, 20)}
	spans := []GateSpan{spanOfMinutes("build-floor", 10, 12, 2.5), spanOfMinutes("ship", 50, 18, 6)}

	got := VerdictOf(calls, minuteMark(30), false).WithHost(spans)

	if got.HostMinutesToVerdict != 30 {
		t.Errorf("HostMinutesToVerdict = %v, want 30 (12 + 18)", got.HostMinutesToVerdict)
	}
	if got.LLMMinutesToVerdict != 50 {
		t.Errorf("LLMMinutesToVerdict = %v, want 50: host minutes sit beside the LLM minutes, never inside", got.LLMMinutesToVerdict)
	}
	wantMinutes(t, "FailPointMinutes", got.FailPointMinutes, 30)
	wantMinutes(t, "WasteMinutes", got.WasteMinutes, 20)
	wantMinutes(t, "FailFastRatio", got.FailFastRatio, 0.6)
}

func TestVerdict_HostMinutesOfAShippedCycleAreCountedAndLeaveNoFailPoint(t *testing.T) {
	t.Parallel()
	calls := []llmcalls.Record{callEndingAt(10, 10), callEndingAt(30, 20)}

	got := VerdictOf(calls, minuteMark(10), true).WithHost([]GateSpan{spanOfMinutes("ship", 30, 7, 1)})

	if got.HostMinutesToVerdict != 7 || got.LLMMinutesToVerdict != 30 {
		t.Errorf("verdict = %+v, want 7 host minutes beside 30 LLM minutes", got)
	}
	if got.FailPointMinutes != nil || got.WasteMinutes != nil || got.FailFastRatio != nil {
		t.Errorf("verdict = %+v, want no fail point for a cycle that shipped", got)
	}
}

func TestVerdict_NoGateSpansLeaveTheHostMinutesAtZeroAndTheVerdictUnchanged(t *testing.T) {
	t.Parallel()
	base := VerdictOf([]llmcalls.Record{callEndingAt(10, 10)}, time.Time{}, false)

	got := base.WithHost(nil)

	if got != base || got.HostMinutesToVerdict != 0 {
		t.Errorf("WithHost(nil) = %+v, want the verdict %+v with zero host minutes", got, base)
	}
}

func TestVerdict_ASpanThatEndsBeforeItStartsCountsZeroHostMinutes(t *testing.T) {
	t.Parallel()
	inverted := GateSpan{Gate: "ship", Start: minuteMark(20), End: minuteMark(5)}

	got := Verdict{}.WithHost([]GateSpan{inverted, spanOfMinutes("ship", 30, 4, 0)})

	if got.HostMinutesToVerdict != 4 {
		t.Errorf("HostMinutesToVerdict = %v, want 4: an inverted span must not subtract", got.HostMinutesToVerdict)
	}
}

func TestGateSpan_MinutesIsTheSpanLengthAndNeverNegative(t *testing.T) {
	t.Parallel()
	if got := spanOfMinutes("g", 0, 90, 0).Minutes(); got != 90 {
		t.Errorf("Minutes = %v, want 90", got)
	}
	if got := (GateSpan{Start: minuteMark(9), End: minuteMark(3)}).Minutes(); got != 0 {
		t.Errorf("Minutes of an inverted span = %v, want 0", got)
	}
}

func TestSampler_RecordsLoadForEachGateSpan(t *testing.T) {
	t.Parallel()
	clock := &steppedClock{at: cycleStart, step: 6 * time.Minute}
	load := &scriptedLoad{values: []float64{1.5, 7.25}}
	s := NewSampler(load.read, clock.now)

	for _, gate := range []string{"build-floor", "ship"} {
		if err := s.Measure(gate, func() error { return nil }); err != nil {
			t.Fatalf("Measure(%s) = %v, want nil", gate, err)
		}
	}

	got := s.Spans()
	if len(got) != 2 || load.reads != 2 {
		t.Fatalf("spans = %+v after %d load reads, want 2 spans and exactly one load read for each", got, load.reads)
	}
	want := []GateSpan{
		{Gate: "build-floor", Start: cycleStart, End: cycleStart.Add(6 * time.Minute), Load: 1.5},
		{Gate: "ship", Start: cycleStart.Add(12 * time.Minute), End: cycleStart.Add(18 * time.Minute), Load: 7.25},
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("span %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestSampler_AGateThatFailsStillRecordsItsSpanAndLoadAndReturnsItsError(t *testing.T) {
	t.Parallel()
	boom := errors.New("gate refused")
	s := NewSampler((&scriptedLoad{values: []float64{9}}).read, (&steppedClock{at: cycleStart, step: time.Minute}).now)

	err := s.Measure("ship", func() error { return boom })

	got := s.Spans()
	if !errors.Is(err, boom) {
		t.Errorf("Measure = %v, want the error of the gate", err)
	}
	if len(got) != 1 || got[0].Gate != "ship" || got[0].Load != 9 || got[0].Minutes() != 1 {
		t.Errorf("spans = %+v, want one span of 1 minute at load 9", got)
	}
}

func TestSampler_FlushAppendsTheSpansToTheWorkspaceLedgerAndEmptiesTheBuffer(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	clock := &steppedClock{at: cycleStart, step: 3 * time.Minute}
	s := NewSampler((&scriptedLoad{values: []float64{2, 4, 8}}).read, clock.now)
	_ = s.Measure("one", func() error { return nil })
	_ = s.Measure("two", func() error { return nil })

	if err := s.Flush(ws); err != nil {
		t.Fatalf("Flush = %v, want nil", err)
	}
	if left := s.Spans(); len(left) != 0 {
		t.Errorf("spans after Flush = %+v, want none", left)
	}
	_ = s.Measure("three", func() error { return nil })
	if err := s.Flush(ws); err != nil {
		t.Fatalf("second Flush = %v, want nil", err)
	}

	got, err := ReadGateSpans(ws)
	if err != nil || len(got) != 3 {
		t.Fatalf("ReadGateSpans = %+v, %v; want the 3 spans in order", got, err)
	}
	for i, want := range []struct {
		gate string
		load float64
	}{{"one", 2}, {"two", 4}, {"three", 8}} {
		if got[i].Gate != want.gate || got[i].Load != want.load || got[i].Minutes() != 3 {
			t.Errorf("span %d = %+v, want gate %s, load %v, 3 minutes", i, got[i], want.gate, want.load)
		}
	}
}

func TestSampler_FlushOfNoSpansWritesNoLedger(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()

	if err := NewSampler(func() float64 { return 0 }, time.Now).Flush(ws); err != nil {
		t.Fatalf("Flush = %v, want nil", err)
	}

	if _, err := os.Stat(filepath.Join(ws, GateSpansFile)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("stat %s = %v, want not exist", GateSpansFile, err)
	}
}

func TestReadGateSpans_AMissingLedgerIsNoSpansAndNoError(t *testing.T) {
	t.Parallel()
	got, err := ReadGateSpans(t.TempDir())
	if err != nil || len(got) != 0 {
		t.Errorf("ReadGateSpans = %+v, %v; want none and nil", got, err)
	}
}

func TestReadGateSpans_AMalformedLineKeepsTheGoodSpansAndNamesTheLedgerInTheError(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	s := NewSampler((&scriptedLoad{values: []float64{1}}).read, (&steppedClock{at: cycleStart, step: time.Minute}).now)
	_ = s.Measure("good", func() error { return nil })
	if err := s.Flush(ws); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(filepath.Join(ws, GateSpansFile), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString("{not json\n")
	_ = f.Close()

	got, err := ReadGateSpans(ws)

	if !errors.Is(err, ErrGateSpans) {
		t.Errorf("error = %v, want ErrGateSpans", err)
	}
	if len(got) != 1 || got[0].Gate != "good" {
		t.Errorf("spans = %+v, want the one good span", got)
	}
}
