package bridge

// This file is package bridge, not bridge_test, because detectorFor is
// unexported; calling it without exporting it requires being in-package.

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

func TestDriverLivenessRouting_ClaudeTmux(t *testing.T) {
	lp := tmuxLaunch{name: "claude-tmux"}
	probe := detectorFor(lp)
	if _, ok := probe.(*panestream.ClaudeDetector); !ok {
		t.Errorf("claude-tmux: got %T, want *panestream.ClaudeDetector", probe)
	}
}

func TestDriverLivenessRouting_CodexTmux(t *testing.T) {
	lp := tmuxLaunch{name: "codex-tmux"}
	probe := detectorFor(lp)
	if _, ok := probe.(*panestream.DefaultDetector); !ok {
		t.Errorf("codex-tmux: got %T, want *panestream.DefaultDetector", probe)
	}
}

func TestDriverLivenessRouting_AgyTmux(t *testing.T) {
	lp := tmuxLaunch{name: "agy-tmux"}
	probe := detectorFor(lp)
	if _, ok := probe.(*panestream.AgyDetector); !ok {
		t.Errorf("agy-tmux: got %T, want *panestream.AgyDetector", probe)
	}
}

func TestDriverLivenessRouting_OllamaTmux(t *testing.T) {
	lp := tmuxLaunch{name: "ollama-tmux"}
	probe := detectorFor(lp)
	if _, ok := probe.(*panestream.OllamaDetector); !ok {
		t.Errorf("ollama-tmux: got %T, want *panestream.OllamaDetector", probe)
	}
}

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
