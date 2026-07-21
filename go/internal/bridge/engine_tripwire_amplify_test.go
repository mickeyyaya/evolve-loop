package bridge

// engine_tripwire_amplify_test.go — Test Amplification pass for cycle-1005
// (telemetry-coverage-tripwire-nonclaude-success). Black-box adversarial cases
// designed from the TDD contract in test-report.md / build-report.md ONLY —
// no implementation file was read while designing these. Reuses the
// tripwireCase / runTripwireCase / tripwireStderrLine harness defined in
// engine_tripwire_test.go (same package); that file is NOT modified.
//
// Contract under test (engine.go recordTokenUsage tripwire escalation):
//
//	trip_when := result.Source==SourceNone && code==0 &&
//	             end.Sub(start) > 60*time.Second && !isClaudeDriver(req.CLI)
//
// These cases probe boundaries and semantics the RED contract's 7 cases did
// not isolate: the exact 60s threshold edge, exit codes other than the
// documented 85 quota-abort, case sensitivity and substring-vs-prefix on the
// "claude" driver check, an empty CLI identity, coexistence of the tripwire
// line with the pre-existing generic WARN, ndjson field presence across every
// silent path (not just one), and a pathologically large duration.

import (
	"strings"
	"testing"
)

// TestRecordTokenUsage_Tripwire_ExactlySixtySeconds_Silent — BOUNDARY. The
// contract specifies a strict "> 60s", not ">= 60s". A launch that ran for
// exactly the threshold duration must NOT trip; a naive off-by-one (>=
// instead of >) would false-positive here.
func TestRecordTokenUsage_Tripwire_ExactlySixtySeconds_Silent(t *testing.T) {
	stderr, _ := runTripwireCase(t, tripwireCase{
		cli: "agy", agent: "builder", code: 0, durSecs: 60, cycleInDir: true,
	})
	if line, ok := tripwireStderrLine(stderr); ok {
		t.Errorf("duration of exactly 60s must NOT trip (contract is strictly '> 60s'), got: %s", line)
	}
}

// TestRecordTokenUsage_Tripwire_OneSecondOverThreshold_Warns — BOUNDARY. One
// second past the 60s floor is the smallest margin that must trip, pairing
// with the exact-60s negative above to pin the inequality's direction.
func TestRecordTokenUsage_Tripwire_OneSecondOverThreshold_Warns(t *testing.T) {
	stderr, _ := runTripwireCase(t, tripwireCase{
		cli: "agy", agent: "builder", code: 0, durSecs: 61, cycleInDir: true,
	})
	if _, ok := tripwireStderrLine(stderr); !ok {
		t.Fatalf("duration of 61s (1s past the 60s floor) must trip; stderr:\n%s", stderr)
	}
}

// TestRecordTokenUsage_Tripwire_NonZeroExitCodeOtherThanQuotaAbort_Silent —
// NEGATIVE. AC3's RED test only proves exit 85 (the documented quota-abort
// code) stays silent. The contract gates on "code==0" generically, so any
// non-zero exit — not just 85 — combined with a long duration must also stay
// silent. Isolates the exit-code gate from a single hardcoded value.
func TestRecordTokenUsage_Tripwire_NonZeroExitCodeOtherThanQuotaAbort_Silent(t *testing.T) {
	for _, code := range []int{1, 2, 137} {
		stderr, _ := runTripwireCase(t, tripwireCase{
			cli: "agy", agent: "builder", code: code, durSecs: 90, cycleInDir: true,
		})
		if line, ok := tripwireStderrLine(stderr); ok {
			t.Errorf("exit code %d with long duration must NOT trip (code==0 gate), got: %s", code, line)
		}
	}
}

// TestRecordTokenUsage_Tripwire_ClaudeDriverCaseInsensitive_Silent —
// NEGATIVE (SEMANTIC). The RED contract's claude-baseline case only exercises
// lowercase "claude-tmux". Driver identity strings are not guaranteed
// lowercase at every call site; an uppercase or mixed-case claude driver must
// still be recognized as the measured baseline and stay silent.
func TestRecordTokenUsage_Tripwire_ClaudeDriverCaseInsensitive_Silent(t *testing.T) {
	for _, cli := range []string{"CLAUDE-TMUX", "Claude", "ClAuDe-Headless"} {
		stderr, _ := runTripwireCase(t, tripwireCase{
			cli: cli, agent: "builder", code: 0, durSecs: 90, cycleInDir: true,
		})
		if line, ok := tripwireStderrLine(stderr); ok {
			t.Errorf("cli %q must be recognized as a claude driver regardless of case, got: %s", cli, line)
		}
	}
}

// TestRecordTokenUsage_Tripwire_ClaudeSubstringNotPrefix_Warns — EDGE
// (SEMANTIC). "claude" appearing anywhere in the driver string is not the
// same as the driver BEING claude. A CLI identity that merely contains
// "claude" as a substring (not a prefix) is not the measured baseline and
// must still trip like any other non-claude driver — this catches a
// substring-match implementation in place of the documented prefix check.
func TestRecordTokenUsage_Tripwire_ClaudeSubstringNotPrefix_Warns(t *testing.T) {
	for _, cli := range []string{"my-claude-wrapper", "wrapper-claude", "not-claude-at-all"} {
		stderr, _ := runTripwireCase(t, tripwireCase{
			cli: cli, agent: "builder", code: 0, durSecs: 90, cycleInDir: true,
		})
		if _, ok := tripwireStderrLine(stderr); !ok {
			t.Errorf("cli %q does not start with \"claude\" and must trip like any non-claude driver; stderr:\n%s", cli, stderr)
		}
	}
}

// TestRecordTokenUsage_Tripwire_EmptyCLI_NoPanicStillWarns — EDGE (robustness).
// An empty CLI identity is not documented, but recordTokenUsage must never
// panic on it, and an empty string trivially does not have a "claude" prefix,
// so the escalation must still fire (fail-open extends to malformed identity,
// not just a missing cycle).
func TestRecordTokenUsage_Tripwire_EmptyCLI_NoPanicStillWarns(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("recordTokenUsage panicked on empty CLI: %v", r)
		}
	}()
	stderr, _ := runTripwireCase(t, tripwireCase{
		cli: "", agent: "builder", code: 0, durSecs: 90, cycleInDir: true,
	})
	if _, ok := tripwireStderrLine(stderr); !ok {
		t.Errorf("empty CLI (not a claude prefix) with exit-0/long-duration/uncovered must still trip; stderr:\n%s", stderr)
	}
}

// TestRecordTokenUsage_Tripwire_GenericWarnCoexistsWithEscalation — CONTRACT
// (keep_generic_warn). The RED contract's positive test only asserts the
// TRIPWIRE line's presence; it does not check that the pre-existing generic
// per-driver coverage WARN survives alongside it. Losing the generic WARN
// while adding the escalation would be a regression for any tooling that
// still greps for the original message.
func TestRecordTokenUsage_Tripwire_GenericWarnCoexistsWithEscalation(t *testing.T) {
	stderr, _ := runTripwireCase(t, tripwireCase{
		cli: "agy", agent: "builder", code: 0, durSecs: 90, cycleInDir: true,
	})
	if _, ok := tripwireStderrLine(stderr); !ok {
		t.Fatalf("expected a TRIPWIRE line; stderr:\n%s", stderr)
	}
	hasGenericWarn := false
	for _, ln := range strings.Split(stderr, "\n") {
		if strings.Contains(ln, "WARN") && !strings.Contains(ln, "TRIPWIRE") {
			hasGenericWarn = true
			break
		}
	}
	if !hasGenericWarn {
		t.Errorf("the pre-existing generic coverage WARN must still appear alongside the TRIPWIRE escalation; stderr:\n%s", stderr)
	}
}

// TestRecordTokenUsage_TripwireRecord_FalseFieldPresentAcrossAllSilentPaths —
// CONTRACT (AC5 amplification). The RED contract's FieldFlips test proves
// "tripwire":false is present for exactly ONE silent case (quota-abort). AC5
// requires the field "always present" — this checks the other three
// documented silent categories (short-duration, claude-baseline, covered)
// each also carry an explicit, non-omitted "tripwire":false rather than
// silently dropping the field for some code paths and not others.
func TestRecordTokenUsage_TripwireRecord_FalseFieldPresentAcrossAllSilentPaths(t *testing.T) {
	cases := map[string]tripwireCase{
		"exit0-short-duration": {cli: "agy", agent: "builder", code: 0, durSecs: 5, cycleInDir: true},
		"claude-baseline":      {cli: "claude-tmux", agent: "builder", code: 0, durSecs: 90, cycleInDir: true},
		"covered-nonclaude":    {cli: "agy", agent: "builder", code: 0, durSecs: 90, covered: true, cycleInDir: true},
	}
	for name, c := range cases {
		_, record := runTripwireCase(t, c)
		if !strings.Contains(record, `"tripwire":false`) {
			t.Errorf("%s: silent record must still carry an explicit \"tripwire\":false, got: %s", name, record)
		}
	}
}

// TestRecordTokenUsage_Tripwire_VeryLongDuration_NoOverflowStillWarns — EDGE
// (robustness). A pathologically long-running launch (well beyond any
// realistic agent phase) must not overflow or misbehave in the duration
// comparison; the escalation must still fire cleanly.
func TestRecordTokenUsage_Tripwire_VeryLongDuration_NoOverflowStillWarns(t *testing.T) {
	stderr, _ := runTripwireCase(t, tripwireCase{
		cli: "agy", agent: "builder", code: 0, durSecs: 365 * 24 * 60 * 60, cycleInDir: true, // 1 year
	})
	if _, ok := tripwireStderrLine(stderr); !ok {
		t.Errorf("a pathologically long non-claude exit-0 source=none launch must still trip without overflow; stderr:\n%s", stderr)
	}
}
