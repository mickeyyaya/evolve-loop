package bridge

import (
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// budget_scale_test.go — ADR-0076 slice A (A3): a build launch for a large
// cycle carries a scaled --artifact-timeout-s. The scaling seam is
// scaledArtifactBudget (pure) consumed by launchArgs (the pure extraction of
// Launch's inline arg construction), so the composed flag emission is testable
// without driving a real CLI.

func TestScaledArtifactBudget(t *testing.T) {
	cases := []struct {
		name  string
		base  int
		scale float64
		want  int
	}{
		{"unset scale keeps base", 600, 0, 600},
		{"scale one keeps base", 600, 1.0, 600},
		{"scales and rounds", 600, 1.25, 750},
		{"no base: builtin scaled", 0, 1.5, 450},
		{"no base no scale: zero (no flag)", 0, 0, 0},
		{"sub-one scale never shrinks", 600, 0.5, 600},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := scaledArtifactBudget(tc.base, tc.scale); got != tc.want {
				t.Errorf("scaledArtifactBudget(%d, %v) = %d, want %d", tc.base, tc.scale, got, tc.want)
			}
		})
	}
}

func TestLaunchArgs_ArtifactBudgetFlag(t *testing.T) {
	req := core.BridgeRequest{
		CLI: "claude-tmux", Profile: "/p.json", Model: "auto",
		Workspace: "/ws", ArtifactPath: "/ws/build-report.md", Agent: "builder",
	}
	deps := Deps{PhaseArtifactTimeoutS: map[string]int{"builder": 600}}

	t.Run("unscaled emits the policy base", func(t *testing.T) {
		args := launchArgs(req, "/ws/p.txt", "/ws/o.log", "/ws/e.log", deps)
		if !slices.Contains(args, "--artifact-timeout-s=600") {
			t.Errorf("want --artifact-timeout-s=600 in %v", args)
		}
	})
	t.Run("BudgetScale scales the flag", func(t *testing.T) {
		r := req
		r.BudgetScale = 1.5
		args := launchArgs(r, "/ws/p.txt", "/ws/o.log", "/ws/e.log", deps)
		if !slices.Contains(args, "--artifact-timeout-s=900") {
			t.Errorf("want --artifact-timeout-s=900 in %v", args)
		}
	})
	t.Run("scale without policy base scales the builtin", func(t *testing.T) {
		r := req
		r.BudgetScale = 1.5
		args := launchArgs(r, "/ws/p.txt", "/ws/o.log", "/ws/e.log", Deps{})
		if !slices.Contains(args, "--artifact-timeout-s=450") {
			t.Errorf("want --artifact-timeout-s=450 (builtin 300 scaled) in %v", args)
		}
	})
	t.Run("no base no scale: no flag", func(t *testing.T) {
		args := launchArgs(req, "/ws/p.txt", "/ws/o.log", "/ws/e.log", Deps{})
		for _, a := range args {
			if strings.HasPrefix(a, "--artifact-timeout-s=") {
				t.Errorf("unexpected artifact-timeout flag: %v", args)
			}
		}
	})
	t.Run("core flags preserved by the extraction", func(t *testing.T) {
		args := launchArgs(req, "/ws/p.txt", "/ws/o.log", "/ws/e.log", deps)
		for _, want := range []string{"--cli=claude-tmux", "--profile=/p.json", "--artifact=/ws/build-report.md", "--agent=builder", "--allow-bypass"} {
			if !slices.Contains(args, want) {
				t.Errorf("want %q in %v", want, args)
			}
		}
	})
}
