package ciparitygate

import (
	"context"
	"strings"
	"testing"
)

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
