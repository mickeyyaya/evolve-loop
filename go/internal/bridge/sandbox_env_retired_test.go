package bridge

import "testing"

func TestDefaultSandboxWrap_TheRetiredEnvironmentNameNoLongerTurnsTheSandboxOff(t *testing.T) {
	t.Parallel()
	deps := Deps{Env: map[string]string{"EVOLVE_SANDBOX": "off"}}
	wrap := defaultSandboxWrapWithProbe(deps, fakeProbe("darwin", true))
	prefix, ok := wrap(SandboxWrapRequest{Phase: "build", Worktree: t.TempDir(), Workspace: t.TempDir(), RepoRoot: t.TempDir()})
	if !ok || len(prefix) == 0 {
		t.Fatalf("EVOLVE_SANDBOX=off in the environment turned the sandbox off with no audit record; got (%v, %v)", prefix, ok)
	}
}
