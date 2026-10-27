package usageprobe

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/quotastate"
)

func TestHistoryPathNamesTheFileUnderTheEvolveDir(t *testing.T) {
	dir := t.TempDir()
	if got, want := HistoryPath(dir), filepath.Join(dir, HistoryFile); got != want {
		t.Fatalf("HistoryPath = %q, want %q", got, want)
	}
	if HistoryFile != "usage-history.ndjson" {
		t.Fatalf("HistoryFile = %q", HistoryFile)
	}
}

func TestRecordObservationAppendsOneLinePerWindowWithObservedAt(t *testing.T) {
	dir := t.TempDir()
	at := time.Date(2026, 10, 27, 7, 0, 0, 0, time.UTC)
	obs := Observation{CLI: "claude", ObservedAt: at, Windows: []quotastate.UsageWindow{
		{Scope: "session", Kind: quotastate.KindSession, PercentUsed: 3},
		{Scope: "all models", Kind: quotastate.KindWeek, PercentUsed: 9},
	}}
	if err := RecordObservation(dir, obs); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(HistoryPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("want 2 lines, got %d: %q", len(lines), data)
	}
	if !strings.Contains(lines[0], `"observed_at":"2026-10-27T07:00:00Z"`) || !strings.Contains(lines[1], `"percent_used":9`) {
		t.Fatalf("lines lack observed_at or percent: %q", data)
	}
}

func TestAppendHistoryReportsAWindowThatCannotBeEncoded(t *testing.T) {
	obs := Observation{CLI: "claude", Windows: []quotastate.UsageWindow{{Scope: "s", PercentUsed: math.NaN()}}}
	err := appendHistory(t.TempDir(), obs)
	if err == nil || !strings.Contains(err.Error(), "usage history") {
		t.Fatalf("want a usage history error, got %v", err)
	}
}

func TestAppendHistoryReportsAHistoryPathItCannotOpen(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(HistoryPath(dir), 0o755); err != nil {
		t.Fatal(err)
	}
	obs := Observation{CLI: "claude", Windows: []quotastate.UsageWindow{{Scope: "s"}}}
	err := appendHistory(dir, obs)
	if err == nil || !strings.Contains(err.Error(), "usage history") {
		t.Fatalf("want a usage history error, got %v", err)
	}
}

func TestRecordObservationReturnsTheSnapshotWriteError(t *testing.T) {
	dir := t.TempDir()
	obs := Observation{CLI: "claude", Windows: []quotastate.UsageWindow{{Scope: "s", PercentUsed: math.NaN()}}}
	if err := RecordObservation(dir, obs); err == nil {
		t.Fatal("want an error for an unencodable snapshot")
	}
	if _, err := os.Stat(HistoryPath(dir)); err == nil {
		t.Fatal("history must not be written when the snapshot write fails")
	}
}
