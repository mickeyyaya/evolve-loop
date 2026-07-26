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

// Every tmux CLI that ships an exhausted_regex must also ship a drift_probe_regex:
// the watcher is fail-OPEN, so an unconfigured CLI has NO drift diagnostic at all —
// the same silent-burn class the alarm exists to prevent, one level up. This drives
// the real warnExhaustionRegexDrift against each newly-armed CLI's own shipped
// patterns, asserting the full firing condition (probe ∧ ¬exhausted): fire on a
// drifted wall, silence on a wall exhausted_regex already catches, silence on a
// benign pane. Subtest names carry the CLI so per-CLI coverage is greppable.
func TestDriftProbeArmedPerCLI(t *testing.T) {
	const driftMarker = "POSSIBLE EXHAUSTION-REGEX DRIFT"
	cases := []struct {
		cli string
		// driftedPane: a plausible future wall wording exhausted_regex misses.
		driftedPane string
		// matchedWall: a wall exhausted_regex DOES catch — no drift to report.
		matchedWall string
		benignPane  string
	}{
		{
			cli:         "codex-tmux",
			driftedPane: "You've hit your usage limit for this week.",
			matchedWall: "Usage limit reached for this account.",
			benignPane:  "Applying patch to usageclassify.go; 2 hunks staged.",
		},
		{
			cli:         "agy-tmux",
			driftedPane: "You are out of credits. Upgrade to continue.",
			matchedWall: "quota exceeded for this billing period",
			benignPane:  "Running tests... 42/50 passing, still working.",
		},
	}
	for _, tc := range cases {
		t.Run(tc.cli, func(t *testing.T) {
			if probe := manifestDriftProbePattern(tc.cli); probe == "" {
				t.Fatalf("%s has no controls.usage.drift_probe_regex — the drift alarm is inert for this CLI", tc.cli)
			}
			m, err := LoadManifest(tc.cli)
			if err != nil {
				t.Fatalf("cannot load %s manifest: %v", tc.cli, err)
			}
			exhausted := manifestExhaustedPattern(m)
			if exhausted == "" {
				t.Fatalf("%s has no controls.usage.exhausted_regex — nothing to drift-guard", tc.cli)
			}
			// Guard the premise of the positive case: if exhausted_regex already
			// matched the "drifted" pane there would be no gap to detect and the
			// fire assertion below would be vacuous.
			if matchExhausted(exhausted, tc.driftedPane) {
				t.Fatalf("%s exhausted_regex already matches %q — the drifted fixture no longer models a drift", tc.cli, tc.driftedPane)
			}

			panes := []struct {
				name, pane string
				wantDrift  bool
			}{
				{"drifted wall exhausted_regex misses -> DRIFT fires", tc.driftedPane, true},
				{"wall exhausted_regex DOES match -> no drift", tc.matchedWall, false},
				{"benign working pane -> no drift", tc.benignPane, false},
			}
			for _, p := range panes {
				t.Run(p.name, func(t *testing.T) {
					var buf bytes.Buffer
					warnExhaustionRegexDrift(&buf, "[test]", tc.cli, p.pane, exhausted)
					if got := strings.Contains(buf.String(), driftMarker); got != p.wantDrift {
						t.Errorf("%s: drift alarm fired=%v, want %v — pane=%q output=%q", tc.cli, got, p.wantDrift, p.pane, buf.String())
					}
				})
			}
		})
	}
}
