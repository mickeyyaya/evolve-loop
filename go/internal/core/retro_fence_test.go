package core_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/treefence"
)

type failingBuild struct{}

func (failingBuild) Name() string { return string(core.PhaseBuild) }
func (failingBuild) Run(context.Context, core.PhaseRequest) (core.PhaseResponse, error) {
	return core.PhaseResponse{Phase: string(core.PhaseBuild)}, errors.New("build crashed")
}

type writingRetro struct {
	fenced  bool
	outcome treefence.Outcome
}

func (r *writingRetro) Name() string { return string(core.PhaseRetro) }
func (r *writingRetro) Run(ctx context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	r.fenced = req.Fence != nil
	fence := treefence.Begin(ctx, req.Worktree, req.Fence != nil, req.Fence.WritablePaths()...)
	if err := os.WriteFile(filepath.Join(req.Worktree, "README.md"), []byte("the retro rewrote this\n"), 0o644); err != nil {
		return core.PhaseResponse{}, err
	}
	r.outcome = fence.End(ctx)
	return core.PhaseResponse{Phase: string(core.PhaseRetro), Verdict: core.VerdictPASS}, nil
}

func TestOutOfBandRetro_AWritingRetroOfAFailedBuildIsHeldByTheFence(t *testing.T) {
	t.Parallel()
	repo := gittest.Fixture(t)
	repo.Write("README.md", "the tree the ship binds\n")
	repo.Git("add", "README.md")
	repo.Git("commit", "-q", "-m", "seed")
	retro := &writingRetro{}
	o := core.NewOrchestrator(&recStorage{}, &fakeLedger{}, newRunners(map[core.Phase]core.PhaseRunner{
		core.PhaseBuild: failingBuild{},
		core.PhaseRetro: retro,
	}), core.WithWorktreeProvisioner(fixedWorktree{dir: repo.Dir}))

	_ = runCompositionCycle(t, o)

	got, err := os.ReadFile(filepath.Join(repo.Dir, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !retro.fenced || string(got) != "the tree the ship binds\n" || len(retro.outcome.Restored) == 0 {
		t.Fatalf("fenced=%v README=%q outcome=%+v; the out-of-band retro is read-only, so the fence reverts its write", retro.fenced, got, retro.outcome)
	}
}
