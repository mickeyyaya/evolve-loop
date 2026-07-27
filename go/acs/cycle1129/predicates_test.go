//go:build acs

// Package cycle1129 materialises the cycle-1129 acceptance criteria for the one
// triage-committed top_n task (see scout-report.md / triage-report.md):
//
//   - exhaustion-checkpoint-raw-pane-stripping: the exhaustion-regex DRIFT alarm
//     (exhaustion_drift.go, diagnostic-only, fired on the exit-81 teardown) is
//     handed the RAW lastGoodPane at driver_tmux_repl.go:813, while the primary
//     exhaustion detector one line up (704) correctly scans
//     strippedForExhaustionScan(pane, ar.injectedPrompt). Agent-authored content
//     — an edit diff, an echoed prompt — that the real detector deliberately
//     ignores can therefore still raise "POSSIBLE EXHAUSTION-REGEX DRIFT",
//     sending an operator after a regex that is working as intended.
//
// Predicate strategy: behavioural-via-subprocess (the cycle-549…574 / cycle-976
// precedent). Each predicate shells `go test -run` over the RED integration
// tests authored this cycle in internal/bridge (exhaustion_drift_test.go). None
// is a source-grep: every one drives the REAL tmux driver loop (Engine.LaunchArgs
// over a fake tmux) to its exit-81 teardown and asserts on the stderr the alarm
// actually emitted — so they pin the CALL SITE's pane treatment, not a helper
// signature. RED now on 001/002 (raw pane ⇒ false alarm); GREEN once Builder
// strips at the call site. 003 is the anti-no-op pin (already GREEN) that keeps
// the fix from being "achieved" by deleting or blanketing the alarm.
package cycle1129

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const bridgePkg = "github.com/mickeyyaya/evolve-loop/go/internal/bridge"

// runGoTest shells `go test -run '^(<pattern>)$' -count=1 <pkg>` and reports
// whether it exited cleanly plus the combined output. -count=1 defeats the test
// cache so the predicate always exercises current source. A compile failure or an
// assertion failure surfaces as a non-zero exit — the intended RED signal. A
// negative code is a genuine launch failure (binary missing / killed), never a
// test verdict, so it is a hard error rather than a silent RED.
func runGoTest(t *testing.T, pkg, pattern string) (ok bool, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", "^("+pattern+")$", "-count=1", pkg)
	out = stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to launch for %s (%s): code=%d err=%v\n%s", pkg, pattern, code, err, out)
	}
	return code == 0, out
}

// TestC1129_001_DriftAlarmIgnoresAgentDiffContent — AC-1 (primary axis).
// Driving the real driver loop to exit-81 with a pane whose only wall-shaped
// text is an agent-authored unified-diff line must produce NO drift alarm. RED
// now: the teardown call site scans the raw pane, so the agent's own edit trips
// the diagnostic.
func TestC1129_001_DriftAlarmIgnoresAgentDiffContent(t *testing.T) {
	ok, out := runGoTest(t, bridgePkg, "TestTmuxREPL_DriftAlarm_AgentDiffContent_NoFalseAlarm")
	if !ok {
		t.Errorf("the exit-81 drift alarm still fires on AGENT-AUTHORED diff content — driver_tmux_repl.go is passing the raw lastGoodPane instead of strippedForExhaustionScan:\n%s", out)
	}
}

// TestC1129_002_DriftAlarmIgnoresPromptEcho — AC-1 (second axis).
// The other half of strippedForExhaustionScan: a pane line that is a verbatim
// echo of the injected prompt must not raise the alarm either (cycle-641/642
// class). RED now for the same single root cause; a call-site fix that strips
// only diff lines would leave this one red.
func TestC1129_002_DriftAlarmIgnoresPromptEcho(t *testing.T) {
	ok, out := runGoTest(t, bridgePkg, "TestTmuxREPL_DriftAlarm_PromptEchoContent_NoFalseAlarm")
	if !ok {
		t.Errorf("the exit-81 drift alarm still fires on an ECHOED PROMPT line — the teardown pane is not being run through strippedForExhaustionScan:\n%s", out)
	}
}

// TestC1129_003_DriftAlarmStillFiresOnRealDriftedWall — AC-2, the anti-no-op pin.
// Stripping must not cost the alarm its reason for existing: a genuine CLI-chrome
// wall whose wording drifted past exhausted_regex is neither a diff line nor a
// prompt echo, so it must survive stripping and still fire. Pre-existing GREEN by
// design — it exists to fail if the fix is "achieved" by deleting the call,
// blanking the pane, or over-stripping.
func TestC1129_003_DriftAlarmStillFiresOnRealDriftedWall(t *testing.T) {
	ok, out := runGoTest(t, bridgePkg, "TestTmuxREPL_DriftAlarm_RealDriftedWallStillFires")
	if !ok {
		t.Errorf("the drift alarm no longer fires on a genuine drifted CLI wall — the 8-cycle-silent-burn diagnostic has been silenced rather than de-noised:\n%s", out)
	}
}

// TestC1129_004_ExistingDriftAlarmContractIntact — AC-3 (no-regression).
// The alarm's own unit contract (fire on probe ∧ ¬exhausted, silence otherwise,
// armed for every tmux CLI that ships an exhausted_regex) must be unchanged by
// this cycle: the fix is a pane-treatment correction at one call site, not a
// change to when the alarm fires.
func TestC1129_004_ExistingDriftAlarmContractIntact(t *testing.T) {
	ok, out := runGoTest(t, bridgePkg, "TestWarnExhaustionRegexDrift|TestClaudeTmuxDriftProbe_MatchesRealWall|TestDriftProbeArmedPerCLI")
	if !ok {
		t.Errorf("the pre-existing drift-alarm contract regressed — the call-site pane fix must not change the alarm's firing condition:\n%s", out)
	}
}

// TestC1129_005_ExhaustionDetectionUnregressed — AC-3 (no-regression, detector).
// The primary exhaustion path — fast-poll fast-fail, persistence gate, and the
// healthy-idle negative pin — shares strippedForExhaustionScan with the fix, so
// it is the blast radius. It must stay green.
func TestC1129_005_ExhaustionDetectionUnregressed(t *testing.T) {
	ok, out := runGoTest(t, bridgePkg, "TestTmuxREPL_ExhaustionWall_FailsOverFast|TestTmuxREPL_ExhaustionWall_FastFailNotAtCheckpoint|TestTmuxREPL_HealthyIdle_NotExhausted")
	if !ok {
		t.Errorf("the primary exhaustion fast-fail path regressed — stripping the drift alarm's pane must not touch the detector that already stripped:\n%s", out)
	}
}
