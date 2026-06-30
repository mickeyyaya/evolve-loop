package bridge

// driver_liveness_routing_test.go — pins the bridge-level detectorFor seam (T2).
//
// Pre-GREEN: detectorFor already delegates correctly to panestream.DetectorFor;
// these tests pin the routing at the consumer boundary (tmux_pane_checks.go:27)
// against future regression (e.g. a new driver silently falling to the coarse
// boolean path).
//
// Why package bridge (not bridge_test): detectorFor is unexported; the only way
// to call it without making it exported is from within the package itself.

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
)

func repoRootForBridge(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Skipf("not in a git work tree: %v", err)
	}
	return strings.TrimSpace(string(out))
}

// TestDriverLivenessRouting_ClaudeTmux asserts that claude-tmux maps to ClaudeDetector,
// enabling the monotonic-↓-token-counter Converging override for claude sessions.
func TestDriverLivenessRouting_ClaudeTmux(t *testing.T) {
	lp := tmuxLaunch{name: "claude-tmux"}
	probe := detectorFor(lp)
	if _, ok := probe.(*panestream.ClaudeDetector); !ok {
		t.Errorf("claude-tmux: got %T, want *panestream.ClaudeDetector", probe)
	}
}

// TestDriverLivenessRouting_CodexTmux asserts that codex-tmux maps to DefaultDetector
// (codex has no busy affordance; growth-velocity-only is the documented fallback).
func TestDriverLivenessRouting_CodexTmux(t *testing.T) {
	lp := tmuxLaunch{name: "codex-tmux"}
	probe := detectorFor(lp)
	if _, ok := probe.(*panestream.DefaultDetector); !ok {
		t.Errorf("codex-tmux: got %T, want *panestream.DefaultDetector", probe)
	}
}

// TestDriverLivenessRouting_AgyTmux asserts that agy-tmux maps to AgyDetector,
// enabling the ⣯-Generating-spinner override for agy sessions.
func TestDriverLivenessRouting_AgyTmux(t *testing.T) {
	lp := tmuxLaunch{name: "agy-tmux"}
	probe := detectorFor(lp)
	if _, ok := probe.(*panestream.AgyDetector); !ok {
		t.Errorf("agy-tmux: got %T, want *panestream.AgyDetector", probe)
	}
}

// TestDriverLivenessRouting_OllamaTmux asserts that ollama-tmux maps to OllamaDetector,
// enabling the "Thinking..." header Converging override for ollama sessions.
func TestDriverLivenessRouting_OllamaTmux(t *testing.T) {
	lp := tmuxLaunch{name: "ollama-tmux"}
	probe := detectorFor(lp)
	if _, ok := probe.(*panestream.OllamaDetector); !ok {
		t.Errorf("ollama-tmux: got %T, want *panestream.OllamaDetector", probe)
	}
}

// TestDriverLivenessRouting_UnknownTmux is the load-bearing negative test: an
// unknown driver must never return nil (which would panic in the reviewer) and must
// never fall to a coarse boolean-only path — it routes to DefaultDetector so the
// growth-velocity strategy is always available.
func TestDriverLivenessRouting_UnknownTmux(t *testing.T) {
	lp := tmuxLaunch{name: "unknown-tmux"}
	probe := detectorFor(lp)
	if probe == nil {
		t.Fatal("unknown-tmux: detectorFor returned nil (would panic in reviewer)")
	}
	if _, ok := probe.(*panestream.DefaultDetector); !ok {
		t.Errorf("unknown-tmux: got %T, want *panestream.DefaultDetector", probe)
	}
}

// TestDriverLivenessRouting_StopReviewHasNoCLILiterals is the grep-assert that
// stopreview.go contains zero CLI-name literals ("claude", "codex", "agy", "ollama").
// All per-CLI branching must live in panestream.DetectorFor (ADR-0047 §3), not the
// reviewer — this test pins that invariant at the file level.
func TestDriverLivenessRouting_StopReviewHasNoCLILiterals(t *testing.T) {
	root := repoRootForBridge(t)
	path := filepath.Join(root, "go", "internal", "bridge", "stopreview.go")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read stopreview.go: %v", err)
	}
	for _, lit := range []string{`"claude"`, `"codex"`, `"agy"`, `"ollama"`} {
		if bytes.Contains(src, []byte(lit)) {
			t.Errorf("stopreview.go contains CLI literal %s; all per-CLI strategy selection must be in panestream.DetectorFor", lit)
		}
	}
}
