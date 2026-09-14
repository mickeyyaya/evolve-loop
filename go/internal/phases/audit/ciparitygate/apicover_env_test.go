package ciparitygate

import (
	"context"
	"strings"
	"testing"
)

// TestCoverageProfile_RunsGoTestUnderTheScrubbedEnv — the apicover step's
// coverage run is a `go test` of the lane's own packages, spawned from the
// lane process exactly like the tier step, and it must carry the same
// scrubbed allowlist env. Inheriting os.Environ() leaks EVOLVE_CYCLE_STATE_FILE
// and EVOLVE_FLEET into core's env-sensitive tests and the step fails on every
// audit (wave 2, 2026-09-14: AUDIT_CIPARITY_GATE_STEP_FAILED cover_run on
// cycles 1673 and 1676 — the leak the ship gate had, #615, on the audit's
// spawn site). The tier step scrubs (TestTierAttempts_ScrubbedEnv…); this pins
// the coverage run to the same contract: an explicit env, allowlist only.
func TestCoverageProfile_RunsGoTestUnderTheScrubbedEnv(t *testing.T) {
	t.Setenv("EVOLVE_LEAK_CANARY", "1")
	root, goDir := goWorktree(t)
	fn, _, envs := seqRunFunc(t, []step{{0, "ok\n"}, {0, ""}, {0, ""}})
	g := New(fn, fixedSet("./internal/widget/..."))
	_, _, cleanup, _ := g.coverageProfile(context.Background(), tierRequest(root, t.TempDir()), goDir, []string{"./internal/widget/..."})
	defer cleanup()
	if len(*envs) == 0 || (*envs)[0] == nil {
		t.Fatalf("the coverage run inherited the lane's environment (nil env = os.Environ()): %v", *envs)
	}
	joined := strings.Join((*envs)[0], "\n")
	if strings.Contains(joined, "EVOLVE_LEAK_CANARY") || !strings.Contains(joined, "PATH=") {
		t.Fatalf("coverage run env is not the scrubbed allowlist: %v", (*envs)[0])
	}
}
