package bridge

import (
	"os"
	"testing"
)

// scrollback_lines_test.go — RED contract for cycle-256 task
// `scrollback-lines-configurable`.
//
// `tmuxArtifactScrollback = 10000` is hardcoded at the two final-capture sites
// (artifact-completion capture + tmuxCleanup). This task resolves the depth via
// envInt(deps, "EVOLVE_SCROLLBACK_LINES", tmuxArtifactScrollback), honoring the
// existing positive-int fallback semantics (unset/0/negative/non-numeric →
// 10000). claude-tmux's bootScrollback is 0, so any NON-ZERO value recorded by
// the fake is a final/cleanup capture — the clean distinguisher this test keys
// on. Behavioral: it runs the real REPL engine and inspects the actual
// scrollback argument passed to CapturePane (not a source string check).

// runScrollbackPhase runs a happy-path claude-tmux launch (artifact pre-seeded)
// with the given EVOLVE_SCROLLBACK_LINES env value and returns the recorded
// CapturePane scrollback arguments.
func runScrollbackPhase(t *testing.T, env map[string]string) []int {
	t.Helper()
	fx := newFixture(t, "claude-tmux", "")
	if err := os.WriteFile(fx.artifact, []byte("<!-- challenge-token: "+fx.token+" -->\nDONE\n"), 0o644); err != nil {
		t.Fatalf("seed artifact: %v", err)
	}
	tmux := &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}}
	code, stderr := runTmux(t, fx, tmux, env, "--allow-bypass")
	if code != ExitOK {
		t.Fatalf("exit = %d, want ExitOK; stderr=%q", code, stderr)
	}
	return tmux.captureScrollback
}

// contains reports whether xs holds v.
func containsInt(xs []int, v int) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

func TestScrollbackLines(t *testing.T) {
	t.Run("override 2000 → final capture uses 2000, not the 10000 default", func(t *testing.T) {
		got := runScrollbackPhase(t, map[string]string{"EVOLVE_SCROLLBACK_LINES": "2000"})
		if !containsInt(got, 2000) {
			t.Errorf("no CapturePane used scrollback=2000; recorded=%v", got)
		}
		if containsInt(got, tmuxArtifactScrollback) {
			t.Errorf("override ignored: a CapturePane still used the %d default; recorded=%v", tmuxArtifactScrollback, got)
		}
	})

	t.Run("unset → final capture keeps the 10000 default", func(t *testing.T) {
		got := runScrollbackPhase(t, nil)
		if !containsInt(got, tmuxArtifactScrollback) {
			t.Errorf("default depth %d not used when env unset; recorded=%v", tmuxArtifactScrollback, got)
		}
	})

	// Invalid values must fall back to the 10000 default via the existing
	// positive-int envInt semantics (negative / zero / non-numeric).
	for _, bad := range []string{"0", "-5", "abc"} {
		bad := bad
		t.Run("invalid '"+bad+"' → falls back to 10000", func(t *testing.T) {
			got := runScrollbackPhase(t, map[string]string{"EVOLVE_SCROLLBACK_LINES": bad})
			if !containsInt(got, tmuxArtifactScrollback) {
				t.Errorf("invalid value %q should fall back to %d; recorded=%v", bad, tmuxArtifactScrollback, got)
			}
		})
	}
}
