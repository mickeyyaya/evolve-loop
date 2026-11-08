package usageprobe

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/quotastate"
)

func unmappedWord(w quotastate.UsageWindow) quotastate.UsageWindow {
	w.Unmapped = true
	return w
}

func TestProber_AnUnmappedScopeWordBenchesNoModelAndKeepsTheWarn(t *testing.T) {
	h := newWindowHarness(t, "claude", claudeWithModelWeeks(unmappedWord(modelWeek("Fable", 100, 9*time.Hour))))

	h.prober.Run(context.Background())

	if len(h.store.ActiveModels()) != 0 {
		t.Fatalf("ActiveModels() = %v; a word the manifest model_match does not map benches nothing", h.store.ActiveModels())
	}
	if len(h.store.Active()) != 0 {
		t.Errorf("Active() = %v; an unmapped model word never benches the family", h.store.Active())
	}
	log := h.log.String()
	if !strings.Contains(log, "WARN") || !strings.Contains(log, "Fable") {
		t.Errorf("the unmapped drained window must keep its WARN:\n%s", log)
	}
	if strings.Contains(log, "benched claude/Fable") {
		t.Errorf("the log claims a bench that was never written:\n%s", log)
	}
}

func TestProber_AMappedScopeWordStillBenchesItsModel(t *testing.T) {
	h := newWindowHarness(t, "claude", claudeWithModelWeeks(modelWeek("Opus", 100, 5*time.Hour)))

	h.prober.Run(context.Background())

	if _, ok := modelBenchOf(h, "claude", "Opus"); !ok {
		t.Fatalf("ActiveModels() = %v; the mapped Opus week must bench claude/Opus", h.store.ActiveModels())
	}
}

func TestProber_OnlyTheMappedOfTwoExhaustedModelsIsBenched(t *testing.T) {
	h := newWindowHarness(t, "claude", claudeWithModelWeeks(modelWeek("Opus", 100, 5*time.Hour), unmappedWord(modelWeek("Fable", 100, 9*time.Hour))))

	h.prober.Run(context.Background())

	active := h.store.ActiveModels()
	if _, ok := modelBenchOf(h, "claude", "Opus"); !ok || len(active) != 1 {
		t.Fatalf("ActiveModels() = %v; want exactly claude/Opus", active)
	}
	if !strings.Contains(h.log.String(), "Fable") {
		t.Errorf("the unmapped Fable week must still be named in the log:\n%s", h.log)
	}
}

func TestProber_AnUnmappedModelWordBesideADrainedSessionStillBenchesTheFamily(t *testing.T) {
	windows := append([]quotastate.UsageWindow{claudeWindows(100, 0)[0]}, unmappedWord(modelWeek("Fable", 100, 9*time.Hour)))
	h := newWindowHarness(t, "claude", windows)

	h.prober.Run(context.Background())

	if _, ok := h.store.Active()["claude"]; !ok {
		t.Errorf("Active() = %v; the exhausted session window is not model-scoped and still benches the family", h.store.Active())
	}
	if len(h.store.ActiveModels()) != 0 {
		t.Errorf("ActiveModels() = %v; the unmapped Fable word benches no model", h.store.ActiveModels())
	}
}

func TestProber_AnUnmappedScopeWordStillRecordsEveryWindow(t *testing.T) {
	h := newWindowHarness(t, "claude", claudeWithModelWeeks(unmappedWord(modelWeek("Fable", 100, 9*time.Hour))))

	h.prober.Run(context.Background())

	if recorded := h.recorded["claude"]; len(recorded) != 3 || !recorded[2].Exhausted {
		t.Errorf("recorded = %+v; the unmapped window is still recorded as read", recorded)
	}
}
