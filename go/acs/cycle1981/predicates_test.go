//go:build acs

package cycle1981

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/quotastate"
	"github.com/mickeyyaya/evolve-loop/go/internal/usageprobe"
)

var probeNow = time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)

func resetsIn(d time.Duration) *time.Time {
	at := probeNow.Add(d)
	return &at
}

func opusWeek(family string, usedPct float64) quotastate.UsageWindow {
	return quotastate.UsageWindow{
		Scope: "Opus", Kind: quotastate.KindWeek, PercentUsed: usedPct, ResetsText: "in 5h",
		ResetsAt: resetsIn(5 * time.Hour), Family: family, Model: "Opus", Exhausted: usedPct >= 100,
	}
}

func allModelsWeek() quotastate.UsageWindow {
	return quotastate.UsageWindow{
		Scope: "all models", Kind: quotastate.KindWeek, PercentUsed: 40, ResetsText: "in 90h",
		ResetsAt: resetsIn(90 * time.Hour), Family: "claude",
	}
}

type probeRun struct {
	store *clihealth.Store
	log   *bytes.Buffer
}

func runProbe(t *testing.T, windows []quotastate.UsageWindow) probeRun {
	t.Helper()
	run := probeRun{store: clihealth.NewStore(t.TempDir(), func() time.Time { return probeNow }), log: &bytes.Buffer{}}
	prober := &usageprobe.Prober{
		Families: []string{"claude"},
		Probe:    func(context.Context, string) (string, error) { return "the /usage screen", nil },
		Classify: func(string, string) bool { return false },
		Windows:  func(string, string) []quotastate.UsageWindow { return windows },
		Store:    run.store,
		Log:      run.log,
	}
	prober.Run(context.Background())
	return run
}

func TestC1981_001_ExhaustedOpusWeekBenchesClaudeOpusUntilResetAndNotTheFamily(t *testing.T) {
	run := runProbe(t, []quotastate.UsageWindow{allModelsWeek(), opusWeek("claude", 100)})

	models := run.store.ActiveModels()
	bench, ok := models[clihealth.Entry{Family: "claude", Model: "Opus"}.Key()]
	if !ok || len(models) != 1 {
		t.Fatalf("RED: ActiveModels() = %v, want exactly the claude/Opus bench\n%s", models, run.log)
	}
	if lo, hi := probeNow.Add(5*time.Hour), probeNow.Add(5*time.Hour+10*time.Minute); !bench.BenchedUntil.After(lo) || bench.BenchedUntil.After(hi) {
		t.Errorf("RED: benched until %v, want the Opus week's 5h reset", bench.BenchedUntil)
	}
	if _, familyBenched := run.store.Active()["claude"]; familyBenched {
		t.Errorf("RED: the claude family was benched for one model's drained week: %v", run.store.Active())
	}
}

func TestC1981_002_AModelBenchStaysOutOfEveryFamilyView(t *testing.T) {
	root := t.TempDir()
	store := clihealth.NewStore(root, func() time.Time { return probeNow })
	if _, err := store.BenchModelUntil("claude", "Opus", clihealth.Wall{Pattern: clihealth.UsageProbePattern, Evidence: "Opus week 100% used", Reset: probeNow.Add(time.Hour)}); err != nil {
		t.Fatalf("RED: BenchModelUntil: %v", err)
	}
	if len(store.Active()) != 0 {
		t.Errorf("RED: Active() = %v, want no family bench", store.Active())
	}

	later := clihealth.NewStore(root, func() time.Time { return probeNow.Add(3 * time.Hour) })
	if len(later.Expired()) != 0 {
		t.Errorf("RED: Expired() = %v; the canary probes each expired key as a family", later.Expired())
	}
}

func TestC1981_003_AWindowMappedToNoFamilyBenchesNothingAndKeepsTheWarn(t *testing.T) {
	run := runProbe(t, []quotastate.UsageWindow{allModelsWeek(), opusWeek("", 100)})

	if len(run.store.ActiveModels()) != 0 || len(run.store.Active()) != 0 {
		t.Errorf("RED: Active=%v ActiveModels=%v, want nothing benched", run.store.Active(), run.store.ActiveModels())
	}
	if !strings.Contains(run.log.String(), "WARN") || !strings.Contains(run.log.String(), "Opus") {
		t.Errorf("RED: the unmapped drained window is not named in a WARN:\n%s", run.log)
	}
}

func TestC1981_004_AHealthyOpusWeekBenchesNoModel(t *testing.T) {
	run := runProbe(t, []quotastate.UsageWindow{allModelsWeek(), opusWeek("claude", 64)})

	if len(run.store.ActiveModels()) != 0 {
		t.Errorf("RED: ActiveModels() = %v, want none while the Opus week has quota left", run.store.ActiveModels())
	}
}
