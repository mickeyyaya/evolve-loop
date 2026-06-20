package bridge

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"
)

// scrollback_lines_test.go — contract for scrollback depth configurability.
//
// `tmuxArtifactScrollback = 10000` is the built-in default at the two final-
// capture sites (artifact-completion + tmuxCleanup). This is now configured
// via Deps.ScrollbackLines (BridgePolicy.ScrollbackLines); zero or negative →
// defaultIfZero → 10000. claude-tmux's bootScrollback is 0, so any NON-ZERO
// value recorded by the fake is a final/cleanup capture — the distinguisher
// this test keys on. Behavioral: inspects the actual scrollback argument
// passed to CapturePane (not a source string check).

// runScrollbackPhase runs a happy-path claude-tmux launch (artifact pre-seeded)
// with the given scrollbackLines typed field and returns recorded CapturePane
// scrollback arguments.
func runScrollbackPhase(t *testing.T, scrollbackLines int) []int {
	t.Helper()
	fx := newFixture(t, "claude-tmux", "")
	if err := os.WriteFile(fx.artifact, []byte("<!-- challenge-token: "+fx.token+" -->\nDONE\n"), 0o644); err != nil {
		t.Fatalf("seed artifact: %v", err)
	}
	tmux := &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}}
	eng := NewEngine(Deps{
		Tmux:            tmux,
		Sleep:           func(time.Duration) {},
		ScrollbackLines: scrollbackLines,
	})
	var stdout, stderr bytes.Buffer
	code := eng.LaunchArgs(context.Background(), fx.args("claude-tmux", "--allow-bypass"), nil, &stdout, &stderr)
	if code != ExitOK {
		t.Fatalf("exit = %d, want ExitOK; stderr=%q", code, stderr.String())
	}
	return tmux.captureScrollback
}

// containsInt reports whether xs holds v.
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
		got := runScrollbackPhase(t, 2000)
		if !containsInt(got, 2000) {
			t.Errorf("no CapturePane used scrollback=2000; recorded=%v", got)
		}
		if containsInt(got, tmuxArtifactScrollback) {
			t.Errorf("override ignored: a CapturePane still used the %d default; recorded=%v", tmuxArtifactScrollback, got)
		}
	})

	t.Run("zero → final capture keeps the 10000 default", func(t *testing.T) {
		got := runScrollbackPhase(t, 0)
		if !containsInt(got, tmuxArtifactScrollback) {
			t.Errorf("default depth %d not used when ScrollbackLines=0; recorded=%v", tmuxArtifactScrollback, got)
		}
	})

	// Non-positive values must fall back to the 10000 default (defaultIfZero semantics).
	for _, bad := range []int{0, -5} {
		bad := bad
		t.Run("non-positive falls back to 10000", func(t *testing.T) {
			got := runScrollbackPhase(t, bad)
			if !containsInt(got, tmuxArtifactScrollback) {
				t.Errorf("non-positive value %d should fall back to %d; recorded=%v", bad, tmuxArtifactScrollback, got)
			}
		})
	}
}
