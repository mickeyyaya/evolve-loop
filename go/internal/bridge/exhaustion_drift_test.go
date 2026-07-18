package bridge

import (
	"bytes"
	"strings"
	"testing"
)

// The drift alarm must fire exactly when the pane shows a quota-wall signal the
// exhausted_regex MISSED (the drift signature that cost 8 cycles), and stay
// silent otherwise — a genuine stall, a wall the regex DOES match, or no signal.
// exhausted_regex is passed explicitly so the test is decoupled from whatever
// wording the manifest currently ships (it will change as walls drift).
func TestWarnExhaustionRegexDrift(t *testing.T) {
	const driftMarker = "POSSIBLE EXHAUSTION-REGEX DRIFT"
	// A narrow pattern matching only the LEGACY wording — it misses the per-model
	// wall, reproducing the exact gap the per-model incident hit.
	const narrowExhausted = `(?i)reached your (usage|weekly) limit`
	perModelWall := "You've reached your Fable 5 limit. Run /usage-credits to continue or switch models with /model."

	cases := []struct {
		name, cli, pane, exhaustedRegex string
		wantDrift                       bool
	}{
		{"per-model wall the narrow regex misses -> DRIFT fires", "claude-tmux", perModelWall, narrowExhausted, true},
		{"wall the exhausted_regex DOES match -> no drift", "claude-tmux", "reached your usage limit", narrowExhausted, false},
		{"ordinary stall pane (no wall signal) -> no drift", "claude-tmux", "Running tests... 42/50 passing, still working.", narrowExhausted, false},
		{"blank pane -> no drift", "claude-tmux", "   ", narrowExhausted, false},
		{"unknown cli (no drift_probe configured) -> no drift", "nonexistent-tmux", perModelWall, narrowExhausted, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			warnExhaustionRegexDrift(&buf, "[test]", tc.cli, tc.pane, tc.exhaustedRegex)
			if got := strings.Contains(buf.String(), driftMarker); got != tc.wantDrift {
				t.Errorf("drift alarm fired=%v, want %v — output=%q", got, tc.wantDrift, buf.String())
			}
		})
	}
}

// The claude-tmux manifest must actually carry a drift_probe_regex (the alarm is
// inert without it), it must compile, and — the load-bearing property — it must
// match the real captured per-model wall that the pre-fix exhausted_regex missed.
func TestClaudeTmuxDriftProbe_MatchesRealWall(t *testing.T) {
	probe := manifestDriftProbePattern("claude-tmux")
	if probe == "" {
		t.Fatal("claude-tmux has no controls.usage.drift_probe_regex — the drift alarm is inert")
	}
	realWall := "You've reached your Fable 5 limit. Run /usage-credits to continue or switch models with /model."
	if !matchExhausted(probe, realWall) {
		t.Errorf("drift_probe_regex %q does not match the real captured wall %q", probe, realWall)
	}
	// A benign working pane must NOT match the broad probe (keeps the alarm from
	// crying wolf on every ordinary exit-81 stall).
	if matchExhausted(probe, "Writing the audit report now; 3 files reviewed.") {
		t.Errorf("drift_probe_regex %q false-matched a benign working pane", probe)
	}
}
