package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
)

func TestWaveStatus_ADeniedRunsDirectoryIsAWarningEvenWhenALandingIntentNamesACycle(t *testing.T) {
	t.Parallel()
	p := newWavePlane(t)
	if err := p.store().Save(wave.Record{Number: 1, RunID: "r1", StartedAt: waveTestClock.Add(-time.Hour)}); err != nil {
		t.Fatal(err)
	}
	p.write(".evolve/landing/cycle-5.json", `{"cycle":5,"status":"complete"}`)
	runs := filepath.Join(p.root, ".evolve", "runs")
	if err := os.MkdirAll(runs, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(runs, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(runs, 0o755) })
	h := newWaveHarness()

	h.run("status", "--json", "--project-root", p.root)

	var got waveStatusReport
	if err := json.Unmarshal(h.stdout.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	for _, w := range got.Warnings {
		if strings.HasPrefix(w, runs+": ") {
			return
		}
	}
	t.Errorf("warnings = %q, want one that starts with %s: the runs directory could not be listed", got.Warnings, runs)
}
