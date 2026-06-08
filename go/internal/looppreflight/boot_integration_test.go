//go:build integration

package looppreflight

import (
	"os/exec"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

// TestIntegration_BridgeBoot_ClaudeTmux is the one test that reproduces the
// cycle-258 catch: it runs the readiness gate with a REAL claude-tmux boot
// (BootTester defaulted to bridge.BootSmokeTest) and asserts the bridge-boot
// check passes on a healthy host. On a host where the REPL cannot boot it would
// halt with the captured scrollback — exactly the signal the loop lacked.
//
// Opt-in only (`go test -tags integration ./internal/looppreflight/...`); skips
// when tmux or claude is absent so the default suite stays hermetic.
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
		SkipBoot:    false, // exercise the REAL boot — the whole point
		// Empty spine list: the test binary does not import the phase packages,
		// so registry.For would (correctly) report them unregistered. We are
		// exercising the boot path, not pipeline wiring.
		SpinePhases:   []string{},
		ProfileLister: func() ([]string, error) { return []string{"builder"}, nil },
		ProfileGetter: func(name string) (profiles.Profile, error) {
			return profiles.Profile{Name: name, CLI: "claude-tmux"}, nil
		},
		DriverKnown: func(string) bool { return true },
		// BootTester left nil → newDefaultBootTester → bridge.BootSmokeTest.
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
