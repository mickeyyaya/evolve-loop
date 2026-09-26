//go:build integration

package looppreflight

import (
	"os/exec"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

func TestIntegration_BridgeBoot_ClaudeTmux(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not on PATH")
	}
	if _, err := exec.LookPath("claude"); err != nil {
		t.Skip("claude not on PATH")
	}

	opts := Options{
		ProjectRoot: t.TempDir(),
		EvolveDir:   t.TempDir(),
		SkipBoot:    false,
		// Empty still falls back to DefaultSpinePhases; only bridge-boot is asserted.
		SpinePhases:   []string{},
		ProfileLister: func() ([]string, error) { return []string{"builder"}, nil },
		ProfileGetter: func(name string) (profiles.Profile, error) {
			return profiles.Profile{Name: name, CLI: "claude-tmux"}, nil
		},
		DriverKnown: func(string) bool { return true },
		// BootTester nil: the real bridge.BootSmokeTest runs.
		// Freeze seams stubbed so the test never reads the real home dir or runs brew.
		SelfUpdateEvidence: func(string) (bool, string, error) { return false, "", nil },
		PinnedLister:       func() ([]string, error) { return nil, nil },
	}

	r, err := Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	c := findCheck(t, r, "bridge-boot")
	if c.Level != LevelPass {
		t.Fatalf("real claude-tmux boot should pass; got %s\n%s", c.Level, c.Detail)
	}
}
