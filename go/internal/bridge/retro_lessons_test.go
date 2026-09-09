package bridge

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRetrospectiveGrantRejectsRetargetedLessonScope(t *testing.T) {
	for _, scope := range []string{".evolve", ".evolve/instincts", ".evolve/instincts/lessons"} {
		for _, outside := range []bool{false, true} {
			t.Run(scope+map[bool]string{false: "/sibling", true: "/outside"}[outside], func(t *testing.T) {
				root := t.TempDir()
				target := filepath.Join(root, "protected")
				if outside {
					target = t.TempDir()
				}
				if err := os.MkdirAll(target, 0700); err != nil {
					t.Fatal(err)
				}
				link := filepath.Join(root, scope)
				if err := os.MkdirAll(filepath.Dir(link), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, link); err != nil {
					t.Fatal(err)
				}
				wrap := defaultSandboxWrapWithProbe(Deps{}, fakeProbe("darwin", true))
				if prefix, ok := wrap(SandboxWrapRequest{Phase: "retrospective", RepoRoot: root, Worktree: t.TempDir(), Workspace: t.TempDir()}); ok {
					t.Fatalf("retargeted grant accepted: %v", prefix)
				}
			})
		}
	}
}
