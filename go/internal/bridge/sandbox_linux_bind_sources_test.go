package bridge

import (
	"os"
	"strings"
	"testing"
)

func TestDefaultSandboxWrap_LinuxNeverStrictBindsAMissingGrantOfAnyProfile(t *testing.T) {
	t.Parallel()
	for name, prof := range realSandboxProfiles(t) {
		t.Run(strings.TrimSuffix(name, ".json"), func(t *testing.T) {
			root, worktree, workspace := t.TempDir(), t.TempDir(), t.TempDir()
			prefix, outcome := defaultSandboxWrapWithProbe(Deps{}, fakeProbe("linux", true))(SandboxWrapRequest{
				Phase: strings.TrimSuffix(name, ".json"), RepoRoot: root, Worktree: worktree, Workspace: workspace,
				WriteSubpaths: prof.Sandbox.WriteSubpaths,
			})
			if !outcome.Applied {
				t.Fatalf("wrapper refused the launch: %+v", outcome)
			}
			for i := 0; i+2 < len(prefix); i++ {
				if prefix[i] != "--bind" {
					continue
				}
				if _, err := os.Stat(prefix[i+1]); err != nil {
					t.Errorf("bwrap fails the launch on `--bind %s`: %v (profile %s declares %v)", prefix[i+1], err, name, prof.Sandbox.WriteSubpaths)
				}
			}
		})
	}
}
