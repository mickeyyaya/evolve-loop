package usageprobe

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/quotastate"
)

func TestWeekUsed_ReadsTheFamilyWeekWindowOfEachRecordedCLI(t *testing.T) {
	dir := t.TempDir()
	for _, obs := range []Observation{
		{CLI: "agy", ObservedAt: probeNow, Windows: agyWindows(10)},
		{CLI: "claude", ObservedAt: probeNow, Windows: claudeWindows(8, 100)},
	} {
		if err := RecordObservation(dir, obs); err != nil {
			t.Fatal(err)
		}
	}
	got, err := WeekUsed(dir, probeNow)
	if err != nil {
		t.Fatalf("WeekUsed: %v", err)
	}
	want := map[string]float64{"agy": 8.3, "agy-claude": 27.32, "claude": 64}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("WeekUsed = %v, want %v (a per-model window must not count)", got, want)
	}
}

func TestWeekUsed_SkipsAFailedReadAndAWindowWhoseResetHasPassed(t *testing.T) {
	dir := t.TempDir()
	stale := []quotastate.UsageWindow{{Scope: "all models", Kind: quotastate.KindWeek, PercentUsed: 100, ResetsAt: resetIn(-time.Hour), Family: "claude", Exhausted: true}}
	for _, obs := range []Observation{
		{CLI: "claude", ObservedAt: probeNow, Windows: stale},
		{CLI: "agy", ObservedAt: probeNow, Windows: agyWindows(10), Error: "pane timeout"},
	} {
		if err := RecordObservation(dir, obs); err != nil {
			t.Fatal(err)
		}
	}
	got, err := WeekUsed(dir, probeNow)
	if err != nil || len(got) != 0 {
		t.Fatalf("WeekUsed = %v, %v; want an empty map", got, err)
	}
}

func TestWeekUsed_TheHighestWindowOfAFamilyCountsAndAnExhaustedOneIsFull(t *testing.T) {
	dir := t.TempDir()
	windows := []quotastate.UsageWindow{
		{Kind: quotastate.KindWeek, PercentUsed: 20, Family: "agy"},
		{Kind: quotastate.KindWeek, PercentUsed: 40, Family: "agy"},
		{Kind: quotastate.KindWeek, PercentUsed: 3, Family: "agy-claude", Exhausted: true},
		{Kind: quotastate.KindWeek, PercentUsed: 50},
	}
	if err := RecordObservation(dir, Observation{CLI: "agy", ObservedAt: probeNow, Windows: windows}); err != nil {
		t.Fatal(err)
	}
	got, err := WeekUsed(dir, probeNow)
	if err != nil {
		t.Fatal(err)
	}
	if want := map[string]float64{"agy": 40, "agy-claude": 100}; !reflect.DeepEqual(got, want) {
		t.Fatalf("WeekUsed = %v, want %v", got, want)
	}
}

func TestWeekUsed_AnUnreadableFileIsAnError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, WindowsFile), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := WeekUsed(dir, probeNow); err == nil || !strings.Contains(err.Error(), WindowsFile) {
		t.Fatalf("a malformed %s must be an error that names it, got %v", WindowsFile, err)
	}
}
