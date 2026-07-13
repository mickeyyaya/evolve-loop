package tokenusage

// driver_coverage_test.go — cycle-779 AC2 contract (named by ACS predicates
// C779_003/C779_004): per-driver fail-open extraction with an explicit
// uncovered/WARN signal. The 2026-07-13 baseline recorded agy/codex-driven
// launches as zero-usage-as-if-covered; these tests pin the fix — a driver
// whose sources carry no usage yields Source==SourceNone WITH a per-driver
// Warn (unmeasured, not free), a driver whose driver-agnostic tiers do carry
// data is extracted normally, and an unknown driver never errors a launch.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

// TestScanner_PerDriverCoverageWarnsNotZeros: a codex-driven launch with no
// observable sources resolves uncovered WITH an explicit per-driver Warn —
// never a silent zero attributed as covered — while the same driver WITH an
// events-log envelope is extracted through the fail-open generic tier.
func TestScanner_PerDriverCoverageWarnsNotZeros(t *testing.T) {
	resolver := DefaultResolver(t.TempDir()) // no transcripts anywhere

	// Uncovered: codex driver, no events log, no scrollback.
	res, err := resolver(Window{
		Driver:   "codex",
		Worktree: "/repo/worktrees/cycle-779-codex",
		Start:    mustParse(t, launchWindowStart),
		End:      mustParse(t, launchWindowEnd),
	})
	if err != nil {
		t.Fatalf("resolver errored for an uncovered driver: %v (must fail open)", err)
	}
	if res.Source != SourceNone {
		t.Errorf("Source = %q, want %q for a driver with no observable sources", res.Source, SourceNone)
	}
	if res.Usage != (cyclestate.TokenUsage{}) {
		t.Errorf("Usage = %+v, want zero (no fabrication for an uncovered driver)", res.Usage)
	}
	if res.Warn == "" {
		t.Fatal("uncovered driver resolved with no Warn — silent zeros masquerade as covered (the 2026-07-13 baseline defect)")
	}
	if !strings.Contains(res.Warn, "codex") {
		t.Errorf("Warn %q does not name the uncovered driver — coverage signal must be per-driver", res.Warn)
	}

	// Covered via the fail-open generic tier: codex driver with an events log.
	ws := t.TempDir()
	logPath := filepath.Join(ws, "build-events.ndjson")
	envelope := `{"kind":"result","data":{"cost_usd":0.4,"tokens":{"in":900,"out":210,"cache_r":30,"cache_c":7}}}` + "\n"
	if err := os.WriteFile(logPath, []byte(envelope), 0o644); err != nil {
		t.Fatalf("write events fixture: %v", err)
	}
	covered, err := resolver(Window{
		Driver:        "codex",
		Worktree:      "/repo/worktrees/cycle-779-codex",
		EventsLogPath: logPath,
		Start:         mustParse(t, launchWindowStart),
		End:           mustParse(t, launchWindowEnd),
	})
	if err != nil {
		t.Fatalf("resolver errored for a covered driver: %v", err)
	}
	if covered.Source != SourceEventsResult {
		t.Errorf("Source = %q, want %q (per-driver fail-open extraction via events tier)", covered.Source, SourceEventsResult)
	}
	if covered.Usage.Input != 900 || covered.Usage.Output != 210 {
		t.Errorf("Usage = %+v, want Input=900 Output=210 from the events envelope", covered.Usage)
	}
	if covered.Warn != "" {
		t.Errorf("covered driver carries Warn %q, want none", covered.Warn)
	}
}

// TestScanner_UnknownDriverFailsOpenNoError: a driver identity the resolver
// has never heard of fails OPEN — nil error (telemetry must never fail a
// launch), zero usage, and an explicit uncovered Warn naming the driver.
func TestScanner_UnknownDriverFailsOpenNoError(t *testing.T) {
	resolver := DefaultResolver(t.TempDir())
	res, err := resolver(Window{
		Driver:   "some-future-cli",
		Worktree: "/repo/worktrees/cycle-779-unknown",
		Start:    mustParse(t, launchWindowStart),
		End:      mustParse(t, launchWindowEnd),
	})
	if err != nil {
		t.Fatalf("unknown driver returned error %v — telemetry must never fail a launch", err)
	}
	if res.Source != SourceNone {
		t.Errorf("Source = %q, want %q for an unknown driver with no data", res.Source, SourceNone)
	}
	if res.Usage != (cyclestate.TokenUsage{}) {
		t.Errorf("Usage = %+v, want zero (no fabrication)", res.Usage)
	}
	if res.Warn == "" || !strings.Contains(res.Warn, "some-future-cli") {
		t.Errorf("Warn = %q, want an uncovered signal naming the unknown driver", res.Warn)
	}
}
