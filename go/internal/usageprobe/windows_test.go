package usageprobe

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/quotastate"
)

var probeNow = time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)

func resetIn(d time.Duration) *time.Time {
	t := probeNow.Add(d)
	return &t
}

func agyWindows(claudeFiveHourUsed float64) []quotastate.UsageWindow {
	return []quotastate.UsageWindow{
		{Scope: "GEMINI MODELS", Kind: quotastate.KindWeek, PercentUsed: 8.3, ResetsText: "140h 56m", ResetsAt: resetIn(140 * time.Hour), Family: "agy"},
		{Scope: "CLAUDE AND GPT MODELS", Kind: quotastate.KindWeek, PercentUsed: 27.32, ResetsText: "163h 40m", ResetsAt: resetIn(163 * time.Hour), Family: "agy-claude"},
		{Scope: "CLAUDE AND GPT MODELS", Kind: quotastate.KindFiveHour, PercentUsed: claudeFiveHourUsed, ResetsText: "2h 15m", ResetsAt: resetIn(2*time.Hour + 15*time.Minute), Family: "agy-claude", Exhausted: claudeFiveHourUsed >= 99.5},
	}
}

func claudeWindows(sessionUsed, fableUsed float64) []quotastate.UsageWindow {
	return []quotastate.UsageWindow{
		{Scope: "session", Kind: quotastate.KindSession, PercentUsed: sessionUsed, ResetsText: "12:40am (Asia/Taipei)", ResetsAt: resetIn(4 * time.Hour), Family: "claude", Exhausted: sessionUsed >= 100},
		{Scope: "all models", Kind: quotastate.KindWeek, PercentUsed: 64, ResetsText: "Oct 11 at 9pm (Asia/Taipei)", ResetsAt: resetIn(100 * time.Hour), Family: "claude"},
		{Scope: "Fable", Kind: quotastate.KindWeek, PercentUsed: fableUsed, ResetsText: "Oct 11 at 9pm (Asia/Taipei)", ResetsAt: resetIn(100 * time.Hour), Family: "claude", Model: "Fable", Exhausted: fableUsed >= 100},
	}
}

type windowHarness struct {
	prober     *Prober
	store      *clihealth.Store
	log        *bytes.Buffer
	classified *atomic.Int32
	recorded   map[string][]quotastate.UsageWindow
}

func newWindowHarness(t *testing.T, family string, windows []quotastate.UsageWindow) *windowHarness {
	t.Helper()
	h := &windowHarness{log: &bytes.Buffer{}, classified: &atomic.Int32{}, recorded: map[string][]quotastate.UsageWindow{}}
	h.store = clihealth.NewStore(t.TempDir(), func() time.Time { return probeNow })
	h.prober = &Prober{
		Families: []string{family},
		Probe:    func(context.Context, string) (string, error) { return "the /usage screen", nil },
		Classify: func(string, string) bool { h.classified.Add(1); return true },
		Windows:  func(string, string) []quotastate.UsageWindow { return windows },
		Record: func(cli string, w []quotastate.UsageWindow) error {
			h.recorded[cli] = w
			return nil
		},
		Store: h.store,
		Log:   h.log,
	}
	return h
}

func TestProber_ADrainedAgyGroupBenchesItsOwnFamilyUntilItsReset(t *testing.T) {
	h := newWindowHarness(t, "agy", agyWindows(100))

	h.prober.Run(context.Background())

	active := h.store.Active()
	bench, ok := active["agy-claude"]
	if !ok {
		t.Fatalf("the drained Claude group did not bench agy-claude: %v\n%s", active, h.log)
	}
	if _, geminiBenched := active["agy"]; geminiBenched {
		t.Errorf("the Gemini group has quota, yet agy was benched: %+v", active["agy"])
	}
	if lo, hi := probeNow.Add(2*time.Hour+15*time.Minute), probeNow.Add(2*time.Hour+30*time.Minute); !bench.BenchedUntil.After(lo) || bench.BenchedUntil.After(hi) {
		t.Errorf("benched until %v; want the drained window's 2h15m reset", bench.BenchedUntil)
	}
	if !strings.Contains(bench.Evidence, "CLAUDE AND GPT MODELS") || bench.Reason != benchReason {
		t.Errorf("bench = %+v", bench)
	}
	if len(h.recorded["agy"]) != 3 {
		t.Errorf("recorded = %v; every parsed window is recorded under the probed CLI", h.recorded)
	}
}

func TestProber_AnExhaustedClaudeSessionBenchesClaudeUntilItsReset(t *testing.T) {
	h := newWindowHarness(t, "claude", claudeWindows(100, 0))

	h.prober.Run(context.Background())

	bench, ok := h.store.Active()["claude"]
	if !ok || !bench.BenchedUntil.After(probeNow.Add(4*time.Hour)) || bench.BenchedUntil.After(probeNow.Add(4*time.Hour+10*time.Minute)) {
		t.Fatalf("bench = %+v (present=%v); want claude benched until the session's reset", bench, ok)
	}
}

func TestProber_AnExhaustedPerModelWindowIsAWarningAndARecordedFactNotABench(t *testing.T) {
	h := newWindowHarness(t, "claude", claudeWindows(7, 100))

	h.prober.Run(context.Background())

	if len(h.store.Active()) != 0 {
		t.Fatalf("benches = %v; benches are per family, so one model's drained week benches nothing", h.store.Active())
	}
	for _, want := range []string{"WARN", "claude", "Fable", "not benched"} {
		if !strings.Contains(h.log.String(), want) {
			t.Errorf("the log lacks %q:\n%s", want, h.log)
		}
	}
	fable := h.recorded["claude"][2]
	if fable.Model != "Fable" || !fable.Exhausted {
		t.Errorf("the recorded fact = %+v; want the exhausted Fable window", fable)
	}
}

func TestProber_ParsedWindowsOwnTheVerdictSoTheRegexIsNotConsulted(t *testing.T) {
	h := newWindowHarness(t, "agy", agyWindows(45.36))

	h.prober.Run(context.Background())

	if len(h.store.Active()) != 0 || h.classified.Load() != 0 {
		t.Fatalf("benches=%v classify calls=%d; healthy windows bench nothing and leave the whole-family regex unasked", h.store.Active(), h.classified.Load())
	}
}

func TestProber_ADrainedScopeMappedToNoFamilyIsWarnedNotBenched(t *testing.T) {
	orphan := []quotastate.UsageWindow{{Scope: "OTHER MODELS", Kind: quotastate.KindWeek, PercentUsed: 100, Exhausted: true}}
	h := newWindowHarness(t, "agy", orphan)

	h.prober.Run(context.Background())

	if len(h.store.Active()) != 0 {
		t.Fatalf("a scope no manifest maps to a family benched %v", h.store.Active())
	}
	if !strings.Contains(h.log.String(), "WARN") || !strings.Contains(h.log.String(), "OTHER MODELS") {
		t.Errorf("an unmapped drained scope must be named in a WARN:\n%s", h.log)
	}
}

func TestProber_APaneWithNoWindowsFallsBackToTheRegexClassifier(t *testing.T) {
	h := newWindowHarness(t, "agy", nil)

	h.prober.Run(context.Background())

	if _, ok := h.store.Active()["agy"]; !ok || h.classified.Load() != 1 || len(h.recorded) != 0 {
		t.Fatalf("benches=%v classify calls=%d recorded=%v; an unread pane keeps the regex verdict and records nothing", h.store.Active(), h.classified.Load(), h.recorded)
	}
}

func TestRecordObservation_KeepsTheLatestWindowsPerCLI(t *testing.T) {
	dir := t.TempDir()
	first := Observation{CLI: "claude", ObservedAt: probeNow, Windows: claudeWindows(7, 100)}
	agy := Observation{CLI: "agy", ObservedAt: probeNow, Windows: agyWindows(100)}
	later := Observation{CLI: "claude", ObservedAt: probeNow.Add(time.Hour), Windows: claudeWindows(9, 0)}

	for _, obs := range []Observation{first, agy, later} {
		if err := RecordObservation(dir, obs); err != nil {
			t.Fatal(err)
		}
	}

	all, err := LoadObservations(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 || !all["claude"].ObservedAt.Equal(later.ObservedAt) || all["claude"].Windows[0].PercentUsed != 9 || len(all["agy"].Windows) != 3 {
		t.Fatalf("observations = %+v; want the latest per CLI", all)
	}
	if WindowsPath(dir) != filepath.Join(dir, WindowsFile) {
		t.Errorf("WindowsPath(%s) = %s; the record lives at <evolve-dir>/%s", dir, WindowsPath(dir), WindowsFile)
	}
	if empty, err := LoadObservations(t.TempDir()); err != nil || len(empty) != 0 {
		t.Errorf("no file = %v, %v; want none and no error", empty, err)
	}
}
