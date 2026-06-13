package bridge

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"
	"time"
)

// TestLookupEnv_DepsEnvOverlay proves the env resolver consults the
// request-local Deps.Env overlay first — the same map driverEnv exports to
// the inner CLI subprocess — so the in-process and subprocess env views agree.
// Before the env-resolution SSOT fix, lookupEnv read only Deps.LookupEnv and
// silently ignored Deps.Env.
func TestLookupEnv_DepsEnvOverlay(t *testing.T) {
	t.Run("overlay is consulted", func(t *testing.T) {
		if v, ok := lookupEnv(Deps{Env: map[string]string{"K": "x"}}, "K"); !ok || v != "x" {
			t.Fatalf("lookupEnv(Deps.Env) = (%q,%v), want (x,true)", v, ok)
		}
	})
	t.Run("overlay wins over LookupEnv", func(t *testing.T) {
		d := Deps{Env: map[string]string{"K": "x"}, LookupEnv: mapLookup(map[string]string{"K": "y"})}
		if v, _ := lookupEnv(d, "K"); v != "x" {
			t.Fatalf("Deps.Env should win over LookupEnv: lookupEnv = %q, want x", v)
		}
	})
	t.Run("falls through to LookupEnv when absent from overlay", func(t *testing.T) {
		d := Deps{Env: map[string]string{"OTHER": "1"}, LookupEnv: mapLookup(map[string]string{"K": "y"})}
		if v, ok := lookupEnv(d, "K"); !ok || v != "y" {
			t.Fatalf("absent-from-overlay should fall to LookupEnv: lookupEnv = (%q,%v), want (y,true)", v, ok)
		}
	})
}

// TestEnvInt_DepsEnvOverlay proves the int reader (which resolves the
// artifact-wait deadline) honours Deps.Env. This is the unit-level proof that
// an operator EVOLVE_ARTIFACT_TIMEOUT_S override carried on the launch reaches
// the deadline — the knob-never-applies bug B3 fixes.
func TestEnvInt_DepsEnvOverlay(t *testing.T) {
	if got := envInt(Deps{Env: map[string]string{"K": "600"}}, "K", 300); got != 600 {
		t.Fatalf("envInt from Deps.Env = %d, want 600", got)
	}
	d := Deps{Env: map[string]string{"K": "600"}, LookupEnv: mapLookup(map[string]string{"K": "120"})}
	if got := envInt(d, "K", 300); got != 600 {
		t.Fatalf("Deps.Env should win over LookupEnv: envInt = %d, want 600", got)
	}
	d2 := Deps{Env: map[string]string{"OTHER": "1"}, LookupEnv: mapLookup(map[string]string{"K": "120"})}
	if got := envInt(d2, "K", 300); got != 120 {
		t.Fatalf("absent from Deps.Env should fall to LookupEnv: envInt = %d, want 120", got)
	}
}

// TestRunTmuxREPL_TimeoutFromDepsEnv is the end-to-end proof: an
// EVOLVE_ARTIFACT_TIMEOUT_S override placed in Deps.Env reaches the
// artifact-wait interval, observable on the StopEvent the reviewer receives.
// RED before the fix (Deps.Env ignored → IntervalS defaults to 300).
func TestRunTmuxREPL_TimeoutFromDepsEnv(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	tmux := &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}} // boots; artifact never appears
	rev := &scriptedReviewer{verdicts: []ReviewVerdict{{Action: ReviewPause, Reason: "stop now"}}}
	eng := NewEngine(Deps{
		Tmux:     tmux,
		Sleep:    func(time.Duration) {},
		Env:      map[string]string{"EVOLVE_ARTIFACT_TIMEOUT_S": "7"},
		Reviewer: rev,
	})
	var stdout, stderr bytes.Buffer
	code := eng.LaunchArgs(context.Background(), fx.args("claude-tmux", "--allow-bypass"), nil, &stdout, &stderr)

	if code != ExitArtifactTimeout {
		t.Fatalf("exit = %d, want ExitArtifactTimeout; stderr=%q", code, stderr.String())
	}
	if len(rev.events) == 0 {
		t.Fatalf("reviewer never consulted; stderr=%q", stderr.String())
	}
	if rev.events[0].IntervalS != 7 {
		t.Fatalf("StopEvent.IntervalS = %d, want 7 (resolved from Deps.Env override)", rev.events[0].IntervalS)
	}
}

// TestRunTmuxREPL_CfgTimeoutWinsOverEnv locks the precedence the SSOT fix must
// preserve: an explicit per-launch Config.ArtifactTimeoutS (set by livesmoke /
// integration callers) beats the EVOLVE_ARTIFACT_TIMEOUT_S overlay, so a stray
// global override cannot lengthen a deliberately-short wait.
func TestRunTmuxREPL_CfgTimeoutWinsOverEnv(t *testing.T) {
	ws := t.TempDir()
	pf := writeJSON(t, filepath.Join(ws, "p.txt"), "hi")
	cfg := &Config{Model: "m", PromptFile: pf, Workspace: ws,
		Artifact: filepath.Join(ws, "a"), StdoutLog: filepath.Join(ws, "o"), StderrLog: filepath.Join(ws, "e"),
		ArtifactTimeoutS: 3}
	deps := covDeps()
	deps.Tmux = &fakeTmux{paneSeq: []string{"❯"}}
	deps.Env = map[string]string{"EVOLVE_ARTIFACT_TIMEOUT_S": "600"}
	rev := &scriptedReviewer{verdicts: []ReviewVerdict{{Action: ReviewPause, Reason: "stop"}}}
	deps.Reviewer = rev
	lp := tmuxLaunch{name: "claude", session: "s", launchCmd: "x", promptMarker: "❯", bootIntervalS: 1}

	code, _ := runTmuxREPL(context.Background(), cfg, deps, lp)
	if code != ExitArtifactTimeout {
		t.Fatalf("want ExitArtifactTimeout, got %d", code)
	}
	if len(rev.events) == 0 || rev.events[0].IntervalS != 3 {
		t.Fatalf("StopEvent.IntervalS = %v, want 3 (cfg wins over env=600)", rev.events)
	}
}
