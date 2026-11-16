package wave_test

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclespend"
	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
)

type hostClock struct{ at time.Time }

func (c *hostClock) now() time.Time {
	t := c.at
	c.at = c.at.Add(15 * time.Minute)
	return t
}

func (p plane) seedGateSpans(id int, gates ...string) {
	p.t.Helper()
	stepper := &hostClock{at: clock(0)}
	s := cyclespend.NewSampler(func() float64 { return 3.5 }, stepper.now)
	for _, gate := range gates {
		if err := s.Measure(gate, func() error { return nil }); err != nil {
			p.t.Fatal(err)
		}
	}
	if err := s.Flush(filepath.Join(p.root, ".evolve", "runs", "cycle-"+strconv.Itoa(id))); err != nil {
		p.t.Fatal(err)
	}
}

func TestReadCycles_TheGateSpansOfACycleGiveItsHostMinutesBesideItsLLMMinutes(t *testing.T) {
	t.Parallel()
	p := newPlane(t)
	p.seedFailedCycle(31, eventAt(30, "gate.rejected", "ORCHESTRATOR_BUILD_FLOOR", ""))
	p.seedGateSpans(31, "build-floor", "ship")

	got, warnings := wave.ReadCycles(p.root, 0, 0)

	if len(got) != 1 || len(warnings) != 0 {
		t.Fatalf("ReadCycles = %+v, warnings %q; want one cycle", got, warnings)
	}
	wantVerdictMinutes(t, got[0].LLMVerdict, 50, 30, 20, 0.6)
	if got[0].LLMVerdict.HostMinutesToVerdict != 30 {
		t.Errorf("HostMinutesToVerdict = %v, want 30 (two gate spans of 15 minutes)", got[0].LLMVerdict.HostMinutesToVerdict)
	}
}

func TestReadCycles_ACycleWithNoGateSpansHasZeroHostMinutes(t *testing.T) {
	t.Parallel()
	p := newPlane(t)
	p.seedFailedCycle(32, eventAt(30, "gate.rejected", "ORCHESTRATOR_BUILD_FLOOR", ""))

	got, warnings := wave.ReadCycles(p.root, 0, 0)

	if len(got) != 1 || len(warnings) != 0 || got[0].LLMVerdict.HostMinutesToVerdict != 0 {
		t.Errorf("ReadCycles = %+v, warnings %q; want one cycle with zero host minutes and no warning", got, warnings)
	}
}

func TestReadCycles_AMalformedGateSpanLedgerIsAWarningAndTheLLMVerdictStaysWhole(t *testing.T) {
	t.Parallel()
	p := newPlane(t)
	p.seedFailedCycle(33, eventAt(30, "gate.rejected", "ORCHESTRATOR_BUILD_FLOOR", ""))
	p.write(".evolve/runs/cycle-33/"+cyclespend.GateSpansFile, "{not json\n")

	got, warnings := wave.ReadCycles(p.root, 0, 0)

	if len(got) != 1 {
		t.Fatalf("ReadCycles = %+v, want one cycle", got)
	}
	wantVerdictMinutes(t, got[0].LLMVerdict, 50, 30, 20, 0.6)
	joined := strings.Join(warnings, "\n")
	if !strings.Contains(joined, cyclespend.GateSpansFile) {
		t.Errorf("warnings = %q, want one that names %s", warnings, cyclespend.GateSpansFile)
	}
}
