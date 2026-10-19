package usageprobe

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/quotastate"
)

func modelWeek(model string, usedPct float64, resetsIn time.Duration) quotastate.UsageWindow {
	return quotastate.UsageWindow{
		Scope: model, Kind: quotastate.KindWeek, PercentUsed: usedPct, ResetsText: "in " + resetsIn.String(),
		ResetsAt: resetIn(resetsIn), Family: "claude", Model: model, Exhausted: usedPct >= 100,
	}
}

func claudeWithModelWeeks(models ...quotastate.UsageWindow) []quotastate.UsageWindow {
	windows := claudeWindows(7, 0)[:2]
	return append(windows, models...)
}

func modelBenchOf(h *windowHarness, family, model string) (clihealth.Entry, bool) {
	e, ok := h.store.ActiveModels()[clihealth.Entry{Family: family, Model: model}.Key()]
	return e, ok
}

func TestProber_AnExhaustedPerModelWindowBenchesThatModelOnly(t *testing.T) {
	h := newWindowHarness(t, "claude", claudeWithModelWeeks(modelWeek("Opus", 100, 5*time.Hour)))

	h.prober.Run(context.Background())

	bench, ok := modelBenchOf(h, "claude", "Opus")
	if !ok {
		t.Fatalf("the drained Opus week did not bench claude/Opus: %v\n%s", h.store.ActiveModels(), h.log)
	}
	if lo, hi := probeNow.Add(5*time.Hour), probeNow.Add(5*time.Hour+10*time.Minute); !bench.BenchedUntil.After(lo) || bench.BenchedUntil.After(hi) {
		t.Errorf("benched until %v; want the Opus week's 5h reset", bench.BenchedUntil)
	}
	if !strings.Contains(bench.Evidence, "Opus") || bench.Reason != benchReason {
		t.Errorf("bench = %+v", bench)
	}
	if len(h.store.Active()) != 0 {
		t.Errorf("Active() = %v; the claude family must stay unbenched", h.store.Active())
	}
}

func TestProber_AModelBenchKeepsTheWarnAndTheRecordedWindow(t *testing.T) {
	h := newWindowHarness(t, "claude", claudeWithModelWeeks(modelWeek("Opus", 100, 5*time.Hour)))

	h.prober.Run(context.Background())

	for _, want := range []string{"WARN", "claude", "Opus", "not benched"} {
		if !strings.Contains(h.log.String(), want) {
			t.Errorf("the log lacks %q:\n%s", want, h.log)
		}
	}
	if recorded := h.recorded["claude"]; len(recorded) != 3 || recorded[2].Model != "Opus" || !recorded[2].Exhausted {
		t.Errorf("recorded = %+v; want all three windows with the exhausted Opus week", recorded)
	}
}

func TestProber_AHealthyPerModelWindowBenchesNoModel(t *testing.T) {
	h := newWindowHarness(t, "claude", claudeWithModelWeeks(modelWeek("Opus", 64, 5*time.Hour)))

	h.prober.Run(context.Background())

	if len(h.store.ActiveModels()) != 0 || len(h.store.Active()) != 0 {
		t.Fatalf("Active=%v ActiveModels=%v; a window with quota left benches nothing", h.store.Active(), h.store.ActiveModels())
	}
}

func TestProber_EachExhaustedModelGetsItsOwnBenchAndReset(t *testing.T) {
	h := newWindowHarness(t, "claude", claudeWithModelWeeks(modelWeek("Opus", 100, 5*time.Hour), modelWeek("Fable", 100, 9*time.Hour)))

	h.prober.Run(context.Background())

	opus, okOpus := modelBenchOf(h, "claude", "Opus")
	fable, okFable := modelBenchOf(h, "claude", "Fable")
	if !okOpus || !okFable {
		t.Fatalf("ActiveModels() = %v, want both claude/Opus and claude/Fable", h.store.ActiveModels())
	}
	if !fable.BenchedUntil.After(opus.BenchedUntil.Add(3 * time.Hour)) {
		t.Errorf("Opus until %v, Fable until %v; each keeps its own reset", opus.BenchedUntil, fable.BenchedUntil)
	}
}

func TestProber_ADrainedSessionStillBenchesTheFamilyBesideAModelBench(t *testing.T) {
	windows := append([]quotastate.UsageWindow{claudeWindows(100, 0)[0]}, modelWeek("Opus", 100, 5*time.Hour))
	h := newWindowHarness(t, "claude", windows)

	h.prober.Run(context.Background())

	if _, ok := h.store.Active()["claude"]; !ok {
		t.Errorf("Active() = %v; an exhausted session window still benches the whole family", h.store.Active())
	}
	if _, ok := modelBenchOf(h, "claude", "Opus"); !ok {
		t.Errorf("ActiveModels() = %v; the exhausted Opus week is benched too", h.store.ActiveModels())
	}
}

func TestProber_AModelWindowMappedToNoFamilyBenchesNoModel(t *testing.T) {
	orphan := modelWeek("Opus", 100, 5*time.Hour)
	orphan.Family = ""
	h := newWindowHarness(t, "claude", []quotastate.UsageWindow{orphan})

	h.prober.Run(context.Background())

	if len(h.store.ActiveModels()) != 0 || len(h.store.Active()) != 0 {
		t.Fatalf("Active=%v ActiveModels=%v; a window with no routing family benches nothing", h.store.Active(), h.store.ActiveModels())
	}
	if !strings.Contains(h.log.String(), "WARN") || !strings.Contains(h.log.String(), "Opus") {
		t.Errorf("the unmapped drained window must be named in a WARN:\n%s", h.log)
	}
}
