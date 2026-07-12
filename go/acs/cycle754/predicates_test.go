//go:build acs

// Package cycle754 materializes the cycle-754 acceptance criteria for the sole
// committed top_n task token-resolver-production-wiring (triage-report.md
// ## top_n; the scout's second selection, retro-bridge-timeout-width10, was
// DEFERRED by triage, so per R9.3 no predicates bind to it).
//
// Task source: inbox id token-resolver-production-wiring (weight 0.96),
// corrected by cycle-754 scout: composition-root wiring already landed, but
// tokenusage.DefaultResolver chains ONLY the transcript tier, so every
// tmux-driven launch (the production majority) records "source":"none" with
// zero tokens — confirmed live across 124 .evolve/runs/*/llm-calls.ndjson.
//
// AC map (1:1), from scout-report.md "Acceptance Criteria Summary" Task 1:
//
//	AC1 DefaultResolver chains all 3 tiers in fidelity order → C754_002
//	    (scrollback tier reachable) + C754_003 (transcript still wins —
//	    anti-reorder pin)
//	AC2 no-transcript launch with events log resolves via
//	    EventsResultCollector                             → C754_001 (resolver
//	    layer) + C754_006 (engine end-to-end)
//	AC3 SourceNone when no tier has data (no fabrication) → C754_004 +
//	    C754_007 (engine-level negative anti-stamp)
//	AC4 malformed events log falls through cleanly        → C754_005
//	AC5 real-cycle `evolve tokens report` shows non-zero  → manual+checklist
//	    (needs a genuinely tmux-driven cycle; see test-report.md)
//
// Each predicate shells `go test -race -count=1 -v -run '^<name>$'` over the
// unit contract in the target package, which EXERCISES DefaultResolver /
// Engine.recordTokenUsage against real on-disk fixtures — behavioral via
// subprocess, no source-grep predicates (cycle-85 rule). The `-v` +
// "--- PASS:" guard rejects a rename/no-tests-matched silent green.
package cycle754

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	tokenusagePkg = "github.com/mickeyyaya/evolve-loop/go/internal/tokenusage"
	bridgePkg     = "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
)

// runGoTest executes the named unit test under -race and requires an explicit
// verbose PASS marker so the predicate fails on: compile failure, test
// failure, a race report, a missing package, OR the test not existing
// (rename gaming).
func runGoTest(t *testing.T, pkg, name string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-race", "-count=1", "-v", "-run", "^"+name+"$", pkg)
	if code != 0 || err != nil {
		t.Fatalf("go test -race %s -run %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			pkg, name, code, err, stdout, stderr)
	}
	if !strings.Contains(stdout, "--- PASS: "+name) {
		t.Fatalf("go test reported no PASS for %s (renamed or not run?)\nstdout:\n%s", name, stdout)
	}
}

// AC2 (resolver layer) — the production bug shape: no transcript + a present
// events log must resolve via the eventsResult tier with the envelope's exact
// counts, not fall through to SourceNone.
func TestC754_001_EventsTierWinsWhenNoTranscript(t *testing.T) {
	runGoTest(t, tokenusagePkg, "TestDefaultResolver_EventsLogTier_WinsWhenNoTranscript")
}

// AC1 (tier-3 reachability) — with no transcript and no events log, a
// scrollback capture with the "↓ N tokens" marker resolves via the
// scrollbackPeak tier as an output-only floor (non-output fields zero).
func TestC754_002_ScrollbackTierOutputOnlyFloor(t *testing.T) {
	runGoTest(t, tokenusagePkg, "TestDefaultResolver_ScrollbackTier_OutputOnlyFloor")
}

// AC1 (fidelity-order pin — anti-reorder) — when ALL tiers have data the
// transcript must still win. Wiring the new tiers must not shadow tier 1.
func TestC754_003_TranscriptFidelityStillWins(t *testing.T) {
	runGoTest(t, tokenusagePkg, "TestDefaultResolver_TranscriptTier_StillWinsOverLowerTiers")
}

// AC3 (resolver-layer negative / anti-fabrication) — all tiers empty must
// yield SourceNone, zero usage, nil error (fail open, never invent tokens).
func TestC754_004_AllTiersEmptySourceNone(t *testing.T) {
	runGoTest(t, tokenusagePkg, "TestDefaultResolver_AllTiersEmpty_SourceNoneNilError")
}

// AC4 (edge / malformed input) — a corrupt events log neither errors nor
// poisons the result: falls through to scrollback when present, to SourceNone
// otherwise.
func TestC754_005_MalformedEventsLogFallsThrough(t *testing.T) {
	runGoTest(t, tokenusagePkg, "TestDefaultResolver_MalformedEventsLog_FallsThroughCleanly")
}

// AC2/AC5 (engine end-to-end, hermetic form) — Engine.recordTokenUsage with
// the REAL production resolver must thread the workspace <agent>-events.ndjson
// into the Window so the llm-calls.ndjson record carries
// "source":"events_result" with the envelope counts — the flip from the
// permanently-zero "source":"none" production shows today.
func TestC754_006_EngineRecordsEventsResultSource(t *testing.T) {
	runGoTest(t, bridgePkg, "TestRecordTokenUsage_EventsLogFallback_EndToEnd")
}

// AC3 (engine-level negative anti-stamp; expected pre-existing GREEN) — a
// launch with genuinely no telemetry source must keep recording
// "source":"none" with zero tokens. A stub stamping events_result (or
// inventing counts) to satisfy C754_006 must fail here.
func TestC754_007_EngineNoSourcesRecordsNone(t *testing.T) {
	runGoTest(t, bridgePkg, "TestRecordTokenUsage_NoSources_RecordsSourceNoneZeroTokens")
}
