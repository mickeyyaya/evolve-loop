//go:build integration

package looppreflight

import (
	"os/exec"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/internal/testhost"
)

func TestIntegration_BridgeBoot_ClaudeTmux(t *testing.T) {
	testhost.Tool(t, "tmux")
	if _, err := exec.LookPath("claude"); err != nil {
		t.Skip("claude not on PATH")
	}

	opts := Options{
		ProjectRoot:   t.TempDir(),
		EvolveDir:     t.TempDir(),
		SkipBoot:      false,
		SpinePhases:   []string{},
		ProfileLister: func() ([]string, error) { return []string{"builder"}, nil },
		ProfileGetter: func(name string) (profiles.Profile, error) {
			return profiles.Profile{Name: name, CLI: "claude-tmux"}, nil
		},
		DriverKnown:        func(string) bool { return true },
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
