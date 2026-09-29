package ship

import (
	"context"
	"io"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

const evolveLoopGoMod = "module github.com/mickeyyaya/evolve-loop/go\n\ngo 1.23\n"

func repoDeclaring(t *testing.T, goMod string) string {
	t.Helper()
	repo := makeRepo(t)
	if goMod != "" {
		mustWrite(t, filepath.Join(repo, "go", "go.mod"), goMod)
		runGit(t, repo, "add", "go/go.mod")
		runGit(t, repo, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "declare the module")
	}
	return repo
}

func evolveLoopLane(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "go", "go.mod"), evolveLoopGoMod)
	return root
}

func shipRunsThePack(t *testing.T, gate, root string) bool {
	t.Helper()
	dirs := swapRepoContractTest(t, greenPack())
	_ = runRepoContractGateAt(context.Background(), gate, root, "HEAD", "", io.Discard, nil)
	return len(*dirs) > 0
}

func floorRunsThePack(t *testing.T, gate, root string) bool {
	t.Helper()
	mustWrite(t, filepath.Join(root, ".evolve", "policy.json"), `{"gates":{"repo_contract_gate":"`+gate+`"}}`)
	ran := false
	pack := func(context.Context, string) ([]string, string, error) {
		ran = true
		return nil, "", nil
	}
	core.RepoContractFloorChecks(pack)(context.Background(), core.ReviewInput{Phase: string(core.PhaseBuild), Worktree: root, ProjectRoot: root})
	return ran
}

func TestRepoContractPack_ShipAndTheBuildFloorMakeOneDecision(t *testing.T) {
	for _, root := range []struct {
		name, goMod string
		own         bool
	}{
		{"evolve-loop's module", evolveLoopGoMod, true},
		{"a foreign module, the e2e fixture's shape", "module e2e.local/fixture\n\ngo 1.23\n", false},
		{"a tree without go/", "", false},
	} {
		for _, gate := range []string{"enforce", "off", "shadwo"} {
			dir := repoDeclaring(t, root.goMod)
			want := root.own && gate != "off"
			shipRan, floorRan := shipRunsThePack(t, gate, dir), floorRunsThePack(t, gate, dir)
			if shipRan != want || floorRan != want {
				t.Errorf("%s, gate %q: ship ran the pack=%v, the build floor ran it=%v, want both %v", root.name, gate, shipRan, floorRan, want)
			}
		}
	}
}
