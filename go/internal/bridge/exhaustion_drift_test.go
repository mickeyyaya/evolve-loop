package bridge

import (
	"bytes"
	"os"
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

// --- Call-site integration: the drift alarm must scan the AGENT-STRIPPED pane.
//
// The primary exhaustion detector runs on strippedForExhaustionScan (the
// fast-poll and the 300s checkpoint both do, so a working agent that merely
// QUOTES wall text is never benched — cycles 254/255/314/641). The drift alarm
// one hop downstream (driver_tmux_repl.go, post exit-81 teardown) is fed the RAW
// lastGoodPane, so the same agent-authored content that the real detector
// correctly ignored can still trip "POSSIBLE EXHAUSTION-REGEX DRIFT" — a false
// alarm that sends an operator chasing a regex that is working exactly as
// intended. These tests drive the REAL driver loop to its exit-81 teardown and
// assert on what the alarm actually printed, so they pin the call site's pane
// treatment, not a helper signature.

const driftAlarmMarker = "POSSIBLE EXHAUSTION-REGEX DRIFT"

// Agent DIFF content that matches the broad drift probe (but not the shipped
// exhausted_regex) must NOT raise the drift alarm: an agent editing this very
// package renders such lines constantly. RED today — the call site passes the
// raw pane, so the alarm fires on the agent's own edit.
func TestTmuxREPL_DriftAlarm_AgentDiffContent_NoFalseAlarm(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	// A unified-diff content line the agent is WRITING (quota-wall wording inside
	// a test fixture), framed by prompt markers so the session reads as booted.
	pane := "❯\n+\tfmt.Println(\"switch models with /model\")\n❯"
	tmux := &nudgeRecordingTmux{fakeTmux: &fakeTmux{paneSeq: []string{pane}}}
	code, stderr := runTmuxNudge(t, fx, tmux)
	if code != ExitArtifactTimeout {
		t.Fatalf("exit = %d, want %d (ExitArtifactTimeout — agent-authored wall text must not fast-fail either); stderr=%q", code, ExitArtifactTimeout, stderr)
	}
	if strings.Contains(stderr, driftAlarmMarker) {
		t.Errorf("drift alarm fired on AGENT-AUTHORED diff content — the teardown call site is scanning the raw pane instead of strippedForExhaustionScan; stderr=%q", stderr)
	}
}

// Prompt-echo content (the agent's own injected instructions rendered back into
// the pane) must NOT raise the drift alarm either — the second half of what
// strippedForExhaustionScan removes (cycle-641/642). RED today.
func TestTmuxREPL_DriftAlarm_PromptEchoContent_NoFalseAlarm(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	// A prompt line that is wall-SHAPED to the broad probe. The pane echoes it
	// verbatim, exactly as a CLI that re-renders its injected instructions does.
	const echoed = "Do not treat \"switch models with /model\" as a real wall."
	body := "Use your Write tool to create artifact containing:\n<!-- challenge-token: " + fx.token + " -->\n" + echoed + "\nPROTOTYPE OK\n"
	if err := os.WriteFile(fx.promptFile, []byte(body), 0o644); err != nil {
		t.Fatalf("rewrite prompt: %v", err)
	}
	pane := "❯\n" + echoed + "\n❯"
	tmux := &nudgeRecordingTmux{fakeTmux: &fakeTmux{paneSeq: []string{pane}}}
	code, stderr := runTmuxNudge(t, fx, tmux)
	if code != ExitArtifactTimeout {
		t.Fatalf("exit = %d, want %d (ExitArtifactTimeout); stderr=%q", code, ExitArtifactTimeout, stderr)
	}
	if strings.Contains(stderr, driftAlarmMarker) {
		t.Errorf("drift alarm fired on an ECHOED PROMPT line — the teardown call site is scanning the raw pane instead of strippedForExhaustionScan; stderr=%q", stderr)
	}
}

// Anti-no-op pin: stripping must not silence the alarm's REASON FOR EXISTING.
// A genuine CLI-chrome wall whose wording drifted ahead of exhausted_regex is
// neither a diff line nor a prompt echo, so it survives stripping and must still
// fire. Guards the fix against being "implemented" by deleting the alarm call,
// blanking the pane, or over-stripping.
func TestTmuxREPL_DriftAlarm_RealDriftedWallStillFires(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	// Plausible future wording: the shipped exhausted_regex misses "hit your
	// <model> quota" (it keys on "reached/hit your usage|weekly limit"), while
	// the broad probe sees "hit your" + "/usage-credits" — the drift signature.
	pane := "❯\nYou've hit your Fable 5 quota. Run /usage-credits to continue.\n❯"
	tmux := &nudgeRecordingTmux{fakeTmux: &fakeTmux{paneSeq: []string{pane}}}
	code, stderr := runTmuxNudge(t, fx, tmux)
	if code != ExitArtifactTimeout {
		t.Fatalf("exit = %d, want %d (ExitArtifactTimeout — a drifted wall the exhausted_regex misses cannot fast-fail); stderr=%q", code, ExitArtifactTimeout, stderr)
	}
	if !strings.Contains(stderr, driftAlarmMarker) {
		t.Errorf("drift alarm did NOT fire on a genuine drifted CLI wall — the diagnostic that turns an 8-cycle silent burn into one loud line is dead; stderr=%q", stderr)
	}
}
