package ship

import (
	"context"
	"io"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
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

func writeGatePolicy(t *testing.T, root, gate string) {
	t.Helper()
	body := `{"gates":{"repo_contract_gate":"` + gate + `"}}`
	if gate == "" {
		body = `{"gates":{}}`
	}
	mustWrite(t, filepath.Join(root, ".evolve", "policy.json"), body)
}

func shipRunsThePack(t *testing.T, root string) bool {
	t.Helper()
	p, err := policy.Load(filepath.Join(root, ".evolve", "policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	dirs := swapRepoContractTest(t, greenPack())
	_ = runRepoContractGateAt(context.Background(), p.GatesConfig().RepoContractGate, root, "HEAD", "", io.Discard, nil)
	return len(*dirs) > 0
}

func floorRunsThePack(t *testing.T, root string) bool {
	t.Helper()
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
		for _, gate := range []string{"enforce", "off", "shadwo", ""} {
			dir := repoDeclaring(t, root.goMod)
			writeGatePolicy(t, dir, gate)
			want := root.own && gate != "off"
			shipRan, floorRan := shipRunsThePack(t, dir), floorRunsThePack(t, dir)
			if shipRan != want || floorRan != want {
				t.Errorf("%s, gate %q: ship ran the pack=%v, the build floor ran it=%v, want both %v", root.name, gate, shipRan, floorRan, want)
			}
		}
	}
}
