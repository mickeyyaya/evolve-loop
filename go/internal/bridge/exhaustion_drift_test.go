package bridge

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// exhaustedRegex is passed explicitly so the test stays decoupled from the
// manifest's current wording.
func TestWarnExhaustionRegexDrift(t *testing.T) {
	const driftMarker = "POSSIBLE EXHAUSTION-REGEX DRIFT"
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

func TestClaudeTmuxDriftProbe_MatchesRealWall(t *testing.T) {
	probe := manifestDriftProbePattern("claude-tmux")
	if probe == "" {
		t.Fatal("claude-tmux has no controls.usage.drift_probe_regex — the drift alarm is inert")
	}
	realWall := "You've reached your Fable 5 limit. Run /usage-credits to continue or switch models with /model."
	if !matchExhausted(probe, realWall) {
		t.Errorf("drift_probe_regex %q does not match the real captured wall %q", probe, realWall)
	}
	if matchExhausted(probe, "Writing the audit report now; 3 files reviewed.") {
		t.Errorf("drift_probe_regex %q false-matched a benign working pane", probe)
	}
}

func TestDriftProbeArmedPerCLI(t *testing.T) {
	const driftMarker = "POSSIBLE EXHAUSTION-REGEX DRIFT"
	cases := []struct {
		cli         string
		driftedPane string
		matchedWall string
		benignPane  string
	}{
		{
			cli:         "codex-tmux",
			driftedPane: "Too many requests — please retry in a moment.",
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
			// Guards against a vacuous positive case: if exhausted_regex already
			// matched driftedPane there would be no gap left to detect.
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

const driftAlarmMarker = "POSSIBLE EXHAUSTION-REGEX DRIFT"

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

// A genuine drifted CLI-chrome wall is neither a diff line nor a prompt echo,
// so it survives strippedForExhaustionScan and must still fire the alarm.
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
