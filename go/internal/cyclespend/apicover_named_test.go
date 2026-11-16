package cyclespend

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSignalNames_AreTheSpellingThatTheReadersParse(t *testing.T) {
	t.Parallel()
	for got, want := range map[string]string{SignalCode: "CYCLE_SPEND_LIMIT", FieldDecision: "decision", ActionOver: "over"} {
		if got != want {
			t.Errorf("name = %q, want %q", got, want)
		}
	}
}

func TestDecision_ZeroIsNoLimit(t *testing.T) {
	t.Parallel()
	var d Decision = Judge(Spend{}, defaultLimits, ActionStop)
	if d != (Decision{Action: ActionNone}) {
		t.Errorf("Judge of no spend = %+v, want none and no limit", d)
	}
}

func TestVerdictOf_IsNamedForTheAPICoverGate(t *testing.T) {
	t.Parallel()
	var v Verdict = VerdictOf(nil, time.Time{}, true).WithHost([]GateSpan{{Gate: GateSpansFile, Start: cycleStart, End: cycleStart.Add(2 * time.Minute), Load: 1}})
	if v.LLMMinutesToVerdict != 0 || v.FailPointMinutes != nil || v.HostMinutesToVerdict != 2 {
		t.Errorf("VerdictOf of no calls = %+v, want zero LLM minutes, 2 host minutes, and no fail point", v)
	}
}

func TestHostMinutes_ExportsAndEdgeCasesAreNamedAndCovered(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	var s *Sampler = NewSampler(func() float64 {
		return ReadLoadAverage(func(string) ([]byte, error) { return []byte("1.25 0.50 0.25 1/100 1234\n"), nil }, nil)
	}, func() time.Time { return cycleStart })
	if err := s.Measure("gate", func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if spans := s.Spans(); len(spans) != 1 || spans[0].Minutes() != 0 || spans[0].Load != 1.25 {
		t.Fatalf("Spans = %+v, want 1 span with load 1.25", spans)
	}
	if err := s.Flush(ws); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(ws, "dir-as-file", GateSpansFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadGateSpans(filepath.Join(ws, "dir-as-file")); !errors.Is(err, ErrGateSpans) {
		t.Errorf("ReadGateSpans on dir = %v, want ErrGateSpans", err)
	}
	fileAsDir := filepath.Join(ws, "not-a-dir")
	if err := os.WriteFile(fileAsDir, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = s.Measure("fail-flush", func() error { return nil })
	if err := s.Flush(fileAsDir); err == nil {
		t.Errorf("Flush to file path = nil, want error")
	}
	nanSampler := NewSampler(func() float64 { return math.NaN() }, func() time.Time { return cycleStart })
	_ = nanSampler.Measure("nan", func() error { return nil })
	if err := nanSampler.Flush(ws); err == nil {
		t.Errorf("Flush with NaN load = nil, want error")
	}
	f, err := os.OpenFile(filepath.Join(ws, GateSpansFile), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString("\n")
	_ = f.Close()
	if spans, err := ReadGateSpans(ws); err != nil || len(spans) != 1 {
		t.Errorf("ReadGateSpans with blank line = %+v, %v; want 1 span and nil", spans, err)
	}
	if got := ReadLoadAverage(func(string) ([]byte, error) { return nil, os.ErrNotExist }, func() (string, error) { return "", os.ErrInvalid }); got != 0 {
		t.Errorf("ReadLoadAverage error = %v, want 0", got)
	}
	if got := ReadLoadAverage(func(string) ([]byte, error) { return nil, os.ErrNotExist }, func() (string, error) { return "{ 2.5 1.0 0.5 }", nil }); got != 2.5 {
		t.Errorf("ReadLoadAverage sysctl = %v, want 2.5", got)
	}
	if got := ParseLoadAverage(""); got != 0 {
		t.Errorf("ParseLoadAverage empty = %v, want 0", got)
	}
	if got := ParseLoadAverage("abc"); got != 0 {
		t.Errorf("ParseLoadAverage invalid = %v, want 0", got)
	}
}
