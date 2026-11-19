package core_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
)

const cycle2508Claim = `BLOCKER protected-path go/cmd/evolve/request_channel_test.go:178 the role guard denies a build edit of this file; TestWireOrchestratorDeps_RequestChannelWired needs serial("calls process-global setter SetModelCatalogDirFn") as its first statement, so TestParallelDefault_EveryTopLevelTestIsParallelOrListedSerial and TestC2508_006 stay red`

func TestBlockerProver_Cycle2508ProtectedTestFileIsProvenOnTheRealSurface(t *testing.T) {
	t.Parallel()
	const protectedTest = "go/cmd/evolve/request_channel_test.go"
	wt := t.TempDir()
	target := filepath.Join(wt, filepath.FromSlash(protectedTest))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	failure := "./acs/cycle2508 (-tags acs): unit tests FAIL\n" +
		"--- FAIL: TestC2508_006 (0.10s)\n" +
		"    " + protectedTest + ":178: TestWireOrchestratorDeps_RequestChannelWired has no serial line\n"
	c := core.BlockerCase{
		Phase:       string(core.PhaseBuild),
		Cycle:       2508,
		Worktree:    wt,
		Protected:   guards.IsProtectedSurface,
		Failures:    []string{failure},
		Defects:     []string{cycle2508Claim},
		Explainable: func(string) bool { return true },
	}

	proof := core.NewBlockerProver().Prove(c)

	if !proof.Blocked() || len(proof.Proven) != 1 || proof.Proven[0].Path != protectedTest {
		t.Fatalf("proof = %+v; the cycle 2508 blocker on a test file of the integrity surface is a proven blocker", proof)
	}
}
