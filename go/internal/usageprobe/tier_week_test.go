package usageprobe

import (
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/quotastate"
)

func argonWindow(used float64, resetsIn time.Duration) quotastate.UsageWindow {
	return quotastate.UsageWindow{Scope: "Gemini 4 Argon", Kind: quotastate.KindWeek, PercentUsed: used, ResetsAt: resetIn(resetsIn), Family: "agy", Model: "Gemini 4 Argon"}
}

func recordAll(t *testing.T, dir string, obs ...Observation) {
	t.Helper()
	for _, o := range obs {
		if err := RecordObservation(dir, o); err != nil {
			t.Fatal(err)
		}
	}
}

func TestTierWeek_TheModelWindowOfTheTierModelWinsOverTheFamilyWindow(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	recordAll(t, dir, Observation{CLI: "agy", ObservedAt: probeNow, Windows: append(agyWindows(10), argonWindow(37, 100*time.Hour))})

	w, ok, err := TierWeek(dir, "agy", "Gemini 4 Argon (Medium)", probeNow)

	if err != nil || !ok || w.Model != "Gemini 4 Argon" || w.PercentUsed != 37 {
		t.Fatalf("TierWeek = %+v, %v, %v; want the Argon window at 37%%", w, ok, err)
	}
}

func TestTierWeek_AModelWithoutItsOwnWindowReadsTheFamilyWindow(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	recordAll(t, dir, Observation{CLI: "agy", ObservedAt: probeNow, Windows: append(agyWindows(10), argonWindow(37, 100*time.Hour))})

	w, ok, err := TierWeek(dir, "agy", "Gemini 3.8 Flash (High)", probeNow)

	if err != nil || !ok || w.Model != "" || w.PercentUsed != 8.3 {
		t.Fatalf("TierWeek = %+v, %v, %v; want the GEMINI MODELS family window at 8.3%%", w, ok, err)
	}
}

func TestTierWeek_SkipsAFailedReadAPassedResetAndAnUnreadableFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	recordAll(t, dir,
		Observation{CLI: "agy", ObservedAt: probeNow, Windows: []quotastate.UsageWindow{argonWindow(90, -time.Hour)}},
		Observation{CLI: "agy-bad", ObservedAt: probeNow, Windows: []quotastate.UsageWindow{argonWindow(50, time.Hour)}, Error: "pane timeout"},
	)

	if w, ok, err := TierWeek(dir, "agy", "Gemini 4 Argon (Medium)", probeNow); err != nil || ok {
		t.Fatalf("TierWeek = %+v, %v, %v; want no live window", w, ok, err)
	}
	if _, _, err := TierWeek(t.TempDir()+"/missing/\x00", "agy", "m", probeNow); err == nil {
		t.Fatal("an unreadable windows file must be an error")
	}
}

func TestTierWeek_TwoWindowsOfOneModelReadTheMostUsed(t *testing.T) {
	t.Parallel()
	for _, order := range [][2]float64{{30, 70}, {70, 30}} {
		dir := t.TempDir()
		recordAll(t, dir, Observation{CLI: "agy", ObservedAt: probeNow, Windows: []quotastate.UsageWindow{argonWindow(order[0], 100*time.Hour), argonWindow(order[1], 100*time.Hour)}})

		if w, ok, err := TierWeek(dir, "agy", "Gemini 4 Argon (Medium)", probeNow); err != nil || !ok || w.PercentUsed != 70 {
			t.Fatalf("order %v: TierWeek = %+v, %v, %v; want the most-used window at 70%%", order, w, ok, err)
		}
	}
}

func TestTierWeek_TheLongestMatchingWindowModelWins(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	short := argonWindow(90, 100*time.Hour)
	short.Model = "Gemini 4"
	recordAll(t, dir, Observation{CLI: "agy", ObservedAt: probeNow, Windows: append(agyWindows(10), short, argonWindow(37, 100*time.Hour))})

	if w, ok, err := TierWeek(dir, "agy", "Gemini 4 Argon (Medium)", probeNow); err != nil || !ok || w.Model != "Gemini 4 Argon" {
		t.Fatalf("TierWeek = %+v, %v, %v; want the Argon window, not the shorter Gemini 4 name", w, ok, err)
	}
	if w, ok, err := TierWeek(dir, "agy", "Gemini 4 Argon", probeNow); err != nil || !ok || w.PercentUsed != 37 {
		t.Fatalf("TierWeek = %+v, %v, %v; want the exact model name to match", w, ok, err)
	}
	if w, ok, err := TierWeek(dir, "agy", "Gemini 4Turbo", probeNow); err != nil || !ok || w.Model != "" {
		t.Fatalf("TierWeek = %+v, %v, %v; a window model matches only at a word boundary", w, ok, err)
	}
}
