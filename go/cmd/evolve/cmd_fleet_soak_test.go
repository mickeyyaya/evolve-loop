//go:build integration

// cmd_fleet_soak_test.go — TDD RED tests for Slice 5 (fleet soak harness).
// All tests reference runFleetSoak, soakLaunchFn, and soakKiller which are
// declared in cmd_fleet_soak.go (not yet created) → compile error → RED.
package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/internal/sessionreaper"
	"github.com/mickeyyaya/evolve-loop/go/internal/sessionrecord"
	"github.com/mickeyyaya/evolve-loop/go/internal/swarm"
)

type soakKillRec struct {
	runDir  string
	session string
}

// TestFleetSoak_AllFourInvariants is the system-level proof that Slices 1–4
// compose correctly under concurrency. Wires an in-process fake LaunchFn and
// TmuxKiller (no real LLM, no real tmux) and verifies all four invariants:
//
//  1. Distinct branches: N RunScope.CycleBranch() values are pairwise distinct.
//  2. Distinct+reaped: post-soak ReapOrphans with stale leases finds 0 live
//     orphans and exactly N reaped sessions.
//  3. No cross-run reap: every kill resolved to the session's owning runDir.
//  4. No torn config: TOML file has exactly N [projects.*] entries.
//
// Do NOT call t.Parallel(): this test mutates the soakLaunchFn and soakKiller
// package-level injection vars (declared in cmd_fleet_soak.go) and must not
// race with TestFleetSoakArgs_* or TestDispatch_FleetSoakRegistered.
func TestFleetSoak_AllFourInvariants(t *testing.T) {
	const n = 4

	evolveDir := t.TempDir()
	tomlPath := filepath.Join(t.TempDir(), "config.toml")

	// Fake TmuxKiller: records (runDir, session) pairs for invariant 3 check.
	var killMu sync.Mutex
	var killLog []soakKillRec
	fakeKiller := swarm.TmuxKiller(func(_ context.Context, session string) error {
		rd := resolveOwningRunDir(evolveDir, session)
		killMu.Lock()
		killLog = append(killLog, soakKillRec{runDir: rd, session: session})
		killMu.Unlock()
		return nil
	})

	// Fake LaunchFn: creates one "run" (stale lease + session record + TOML entry).
	// Protected by callMu so concurrent goroutines get distinct indices (race-safe).
	var (
		callMu  sync.Mutex
		callIdx int
		tomlMu  sync.Mutex
	)
	fakeLaunch := fleet.LaunchFn(func(_ context.Context, _ fleet.CycleSpec) (int, error) {
		callMu.Lock()
		idx := callIdx
		callIdx++
		callMu.Unlock()

		cycleN := idx + 1
		runID := fmt.Sprintf("soaktest%08d", idx)
		runDir := filepath.Join(evolveDir, "runs", fmt.Sprintf("cycle-%d", cycleN))
		if err := os.MkdirAll(runDir, 0o755); err != nil {
			return 1, fmt.Errorf("mkdir runDir: %w", err)
		}
		// Stale lease (heartbeat at Unix epoch — far beyond any DefaultTTL).
		if err := runlease.Write(runDir, runlease.Lease{RunID: runID}, time.Unix(1, 0)); err != nil {
			return 1, fmt.Errorf("write lease: %w", err)
		}
		// Session record with evolve-bridge- prefix (required by ReapRunSessions).
		sess := fmt.Sprintf("evolve-bridge-soak-%s", runID[:8])
		if err := sessionrecord.Append(sessionrecord.PathIn(runDir), sessionrecord.Record{
			Session: sess, RunID: runID, Cycle: cycleN, Agent: "soak-fake",
		}); err != nil {
			return 1, fmt.Errorf("append session: %w", err)
		}
		// Atomic TOML entry (mutex-serialized to prove atomic-write composes safely).
		if err := appendSoakTomlEntry(&tomlMu, tomlPath, runID, runDir); err != nil {
			return 1, fmt.Errorf("toml append: %w", err)
		}
		return 0, nil
	})

	// Wire fakes into the package-level injection points (cmd_fleet_soak.go).
	soakLaunchFn = fakeLaunch
	soakKiller = fakeKiller
	t.Cleanup(func() {
		soakLaunchFn = nil
		soakKiller = nil
	})

	var out, errBuf bytes.Buffer
	rc := runFleetSoak(
		[]string{"--count", fmt.Sprintf("%d", n), "--evolve-dir", evolveDir, "--toml-path", tomlPath},
		nil, &out, &errBuf,
	)
	if rc != 0 {
		t.Fatalf("runFleetSoak returned %d\nstdout: %s\nstderr: %s", rc, out.String(), errBuf.String())
	}

	// AC8: soakreport verdict table printed to stdout — must show 4 PASS rows.
	stdout := out.String()
	passCount := strings.Count(stdout, "PASS")
	if passCount < 4 {
		t.Errorf("AC8: soakreport should contain 4 PASS rows, got %d; stdout:\n%s", passCount, stdout)
	}

	// Invariant 2 (external behavioral check): ReapOrphans on evolveDir with
	// the same fakeKiller finds 0 live runs and exactly N orphaned runs.
	reapRep, err := sessionreaper.ReapOrphans(context.Background(), evolveDir, sessionreaper.Options{
		Now:      func() time.Time { return time.Now().Add(24 * time.Hour) },
		LeaseTTL: runlease.DefaultTTL,
		Kill:     fakeKiller,
	})
	if err != nil {
		t.Fatalf("Invariant 2: ReapOrphans: %v", err)
	}
	if reapRep.LiveRunsSkipped != 0 {
		t.Errorf("Invariant 2: %d live runs found — all leases should be stale", reapRep.LiveRunsSkipped)
	}
	if len(reapRep.Orphaned) != n {
		t.Errorf("Invariant 2: %d orphaned runs, want %d", len(reapRep.Orphaned), n)
	}
	for _, o := range reapRep.Orphaned {
		if o.Report.Killed != 1 {
			t.Errorf("Invariant 2: run %s killed=%d sessions, want 1", o.RunDir, o.Report.Killed)
		}
	}

	// Invariant 3 (external behavioral check): every kill resolved to a runDir
	// whose registry contains the killed session (no cross-run kill).
	killMu.Lock()
	defer killMu.Unlock()
	for _, k := range killLog {
		if k.runDir == "UNKNOWN" {
			t.Errorf("Invariant 3: session %q resolved to UNKNOWN runDir — cross-run reap", k.session)
			continue
		}
		recs, _ := sessionrecord.ReadAll(sessionrecord.PathIn(k.runDir))
		found := false
		for _, r := range recs {
			if r.Session == k.session {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Invariant 3: session %q killed by %q but not in that run's registry", k.session, k.runDir)
		}
	}

	// Invariant 4 (external behavioral check): TOML has exactly N [projects.*] headers.
	tomlData, rerr := os.ReadFile(tomlPath)
	if rerr != nil {
		t.Fatalf("Invariant 4: read TOML %s: %v", tomlPath, rerr)
	}
	hdrCount := 0
	for _, line := range strings.Split(string(tomlData), "\n") {
		if strings.HasPrefix(line, "[projects.") {
			hdrCount++
		}
	}
	if hdrCount != n {
		t.Errorf("Invariant 4: TOML has %d [projects.*] headers, want %d\ncontent:\n%s",
			hdrCount, n, tomlData)
	}
}

// resolveOwningRunDir searches all run registries under evolveDir/runs/ for the
// given session name and returns the run dir that owns it, or "UNKNOWN".
func resolveOwningRunDir(evolveDir, session string) string {
	entries, _ := os.ReadDir(filepath.Join(evolveDir, "runs"))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		rd := filepath.Join(evolveDir, "runs", e.Name())
		recs, _ := sessionrecord.ReadAll(sessionrecord.PathIn(rd))
		for _, r := range recs {
			if r.Session == session {
				return rd
			}
		}
	}
	return "UNKNOWN"
}

// appendSoakTomlEntry atomically appends one [projects."<runID>"] entry to
// the shared TOML file, serialized via mu to prove no torn-write races.
func appendSoakTomlEntry(mu *sync.Mutex, tomlPath, runID, runDir string) error {
	mu.Lock()
	defer mu.Unlock()
	entry := fmt.Sprintf("\n[projects.%q]\npath = %q\n", "soak/"+runID, runDir)
	existing, _ := os.ReadFile(tomlPath)
	tmp := tomlPath + ".tmp." + runID[:8]
	if err := os.WriteFile(tmp, append(existing, []byte(entry)...), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, tomlPath)
}

// TestFleetSoakArgs_RejectsZeroCount is the negative/adversarial test for AC7:
// --count 0 must be rejected with exit code 1 and an error mentioning "count".
func TestFleetSoakArgs_RejectsZeroCount(t *testing.T) {
	var out, errBuf bytes.Buffer
	rc := runFleetSoak([]string{"--count", "0"}, nil, &out, &errBuf)
	if rc != 1 {
		t.Fatalf("runFleetSoak --count 0 returned %d, want 1", rc)
	}
	if !strings.Contains(errBuf.String(), "count") {
		t.Errorf("stderr %q should mention 'count'", errBuf.String())
	}
}

// TestFleetSoakArgs_RejectsNegativeCount is the edge-case adversarial test:
// negative --count values must also be rejected with exit code 1.
func TestFleetSoakArgs_RejectsNegativeCount(t *testing.T) {
	var out, errBuf bytes.Buffer
	rc := runFleetSoak([]string{"--count", "-1"}, nil, &out, &errBuf)
	if rc != 1 {
		t.Fatalf("runFleetSoak --count -1 returned %d, want 1", rc)
	}
}

// TestDispatch_FleetSoakRegistered verifies that `evolve fleet soak` routes to
// runFleetSoak (not the "unknown command" path). A --count 0 call must exit 1
// with a "count" error from runFleetSoak's validation — not a flag-parse error
// and not rc=2.
func TestDispatch_FleetSoakRegistered(t *testing.T) {
	var out, errBuf bytes.Buffer
	rc := dispatch([]string{"fleet", "soak", "--count", "0"}, nil, &out, &errBuf)
	if rc == 2 {
		t.Fatalf("dispatch fleet soak: rc=2 (unknown command) — soak not wired in fleet dispatch: %s", errBuf.String())
	}
	if rc != 1 {
		t.Fatalf("dispatch fleet soak --count 0: rc=%d, want 1 from count validation", rc)
	}
	if !strings.Contains(errBuf.String(), "count") {
		t.Errorf("expected --count rejection from runFleetSoak, got: %q", errBuf.String())
	}
}
