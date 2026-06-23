package gc

// L3.2 acceptance: "live-dir never touched in fuzz test". Property-based:
// random synthetic trees of run dirs (random ages, markers, lease states,
// one optionally-current workspace) → Discover → Plan with an aggressive
// policy. INVARIANTS: no live dir (or anything under one) is ever planned;
// no quarantine/ledger/archive path is ever planned; markerless dirs are
// never planned (they are invisible to discovery).

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolveloop/go/internal/runlease"
	"pgregory.net/rapid"
)

func TestPlanNeverTouchesLiveDirs_Property(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		dir, err := os.MkdirTemp("", "gcfuzz")
		if err != nil {
			rt.Fatal(err)
		}
		defer os.RemoveAll(dir)
		runs := filepath.Join(dir, "runs")

		n := rapid.IntRange(1, 12).Draw(rt, "n")
		liveDirs := map[string]bool{}
		markerless := map[string]bool{}
		var currentWS string

		for i := 0; i < n; i++ {
			name := "run-" + strconv.Itoa(i)
			p := filepath.Join(runs, name)
			if err := os.MkdirAll(p, 0o755); err != nil {
				rt.Fatal(err)
			}
			hasMarker := rapid.Bool().Draw(rt, "marker-"+name)
			if hasMarker {
				if err := os.WriteFile(filepath.Join(p, "run.json"), []byte(`{"cycle_id":1}`), 0o644); err != nil {
					rt.Fatal(err)
				}
			} else {
				markerless[p] = true
			}
			age := rapid.IntRange(0, 400).Draw(rt, "age-"+name)
			mod := t0.Add(-time.Duration(age) * 24 * time.Hour)
			if err := os.Chtimes(p, mod, mod); err != nil {
				rt.Fatal(err)
			}
			switch rapid.IntRange(0, 3).Draw(rt, "lease-"+name) {
			case 1: // fresh lease → live
				if err := runlease.Write(p, runlease.Lease{RunID: name}, t0.Add(-time.Minute)); err != nil {
					rt.Fatal(err)
				}
				if hasMarker {
					liveDirs[p] = true
				}
			case 2: // stale lease → dead
				if err := runlease.Write(p, runlease.Lease{RunID: name}, t0.Add(-2*time.Hour)); err != nil {
					rt.Fatal(err)
				}
			}
			// Chtimes again — lease writes touched the dir mtime.
			if err := os.Chtimes(p, mod, mod); err != nil {
				rt.Fatal(err)
			}
			if currentWS == "" && hasMarker && rapid.Bool().Draw(rt, "current-"+name) {
				currentWS = p
				liveDirs[p] = true
			}
		}
		if currentWS != "" {
			cs := `{"cycle_id":42,"phase":"build","workspace_path":"` + currentWS + `"}`
			if err := os.WriteFile(filepath.Join(dir, "cycle-state.json"), []byte(cs), 0o644); err != nil {
				rt.Fatal(err)
			}
		}
		// Distractors that must never be planned.
		for _, d := range []string{
			filepath.Join(dir, "quarantine", "cycle-1"),
			filepath.Join(dir, "archive", "runs", "old"),
		} {
			if err := os.MkdirAll(d, 0o755); err != nil {
				rt.Fatal(err)
			}
		}

		discovered, err := Discover(dir, DiscoverOptions{Now: nowT0})
		if err != nil {
			rt.Fatalf("Discover: %v", err)
		}
		m, err := Plan(Options{
			EvolveDir: dir,
			Policy: Policy{
				Runs:           RunsPolicy{KeepFull: 1, ArchiveAfterDays: 10, DeleteAfterDays: 30},
				SalvageTTLDays: 1, LogsTTLDays: 1, TrackerTTLDays: 1,
			},
			Runs: discovered,
			Now:  nowT0,
		})
		if err != nil {
			rt.Fatalf("Plan: %v", err)
		}

		for _, it := range m.Items {
			for live := range liveDirs {
				if it.Path == live || strings.HasPrefix(it.Path, live+string(os.PathSeparator)) {
					rt.Fatalf("INVARIANT BROKEN: live dir targeted: %+v (live=%s)", it, live)
				}
			}
			for ml := range markerless {
				if it.Path == ml || strings.HasPrefix(it.Path, ml+string(os.PathSeparator)) {
					rt.Fatalf("INVARIANT BROKEN: markerless (undiscoverable) dir targeted: %+v", it)
				}
			}
			rel, err := filepath.Rel(dir, it.Path)
			if err != nil || strings.HasPrefix(rel, "..") {
				rt.Fatalf("INVARIANT BROKEN: planned path outside evolve dir: %+v", it)
			}
			first := strings.Split(filepath.ToSlash(rel), "/")[0]
			if first == "quarantine" || first == "archive" || strings.HasPrefix(first, "ledger") {
				rt.Fatalf("INVARIANT BROKEN: protected class targeted: %+v", it)
			}
		}
	})
}
