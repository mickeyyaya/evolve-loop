package opscmd

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
)

func TestLiveFailingStage_MapsEachExitCodeAndTrustDialog(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		rc         int
		scrollback string
		want       string
	}{
		{"missing binary", bridge.ExitMissingBinary, "", "launch"},
		{"safety gate", bridge.ExitSafetyGate, "", "launch"},
		{"cost leak", bridge.ExitCostLeak, "", "launch"},
		{"launch wins over a trust dialog", bridge.ExitMissingBinary, "Yes, I trust this folder", "launch"},
		{"claude trust dialog", bridge.ExitREPLBootTimeout, "❯ 1. Yes, I trust this folder\nEnter to confirm", "trust prompt"},
		{"codex trust dialog", bridge.ExitCmdTimeout, "Do you trust the contents of this directory?", "trust prompt"},
		{"agy trust dialog", 1, "Do you trust the contents of this project?", "trust prompt"},
		{"repl boot timeout", bridge.ExitREPLBootTimeout, "booting", "REPL prompt"},
		{"model mismatch", bridge.ExitModelMismatch, "", "REPL prompt"},
		{"artifact timeout", bridge.ExitArtifactTimeout, "", "artifact"},
		{"require-full unmet", bridge.ExitRequireFullUnmet, "", "artifact"},
		{"unknown prompt", bridge.ExitUnknownPrompt, "", "delivery"},
		{"respond loop guard without a trust dialog", bridge.ExitRespondLoopGuard, "Allow this edit?", "delivery"},
		{"command timeout", bridge.ExitCmdTimeout, "", "delivery"},
		{"generic failure", 1, "", "delivery"},
		{"untrusted is not a trust dialog", 1, "Working with untrusted contents", "delivery"},
	}
	for _, c := range cases {
		if got := liveFailingStage(c.rc, c.scrollback); got != c.want {
			t.Errorf("%s: liveFailingStage(%d) = %q, want %q", c.name, c.rc, got, c.want)
		}
	}
}
