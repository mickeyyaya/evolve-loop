package ship

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
)

const (
	orphanGuardPackage = "./internal/tmuxtest"
	orphanGuardTest    = "TestEveryTmuxTestPackageUsesTheOrphanGuard"
)

func addedFakeTmuxTestLane(t *testing.T) string {
	t.Helper()
	return dataLaneFixture(t, func(repo string) {
		mustWrite(t, filepath.Join(repo, "go", "internal", "faketmux", "contract_test.go"), "package faketmux\n")
	})
}

func TestRepoContractGate_AnAddedTmuxTestRunsTheOrphanGuardByName(t *testing.T) {
	calls := recordGoTestJSON(t)
	repo := addedFakeTmuxTestLane(t)
	var out strings.Builder
	if err := runRepoContractGateAt(context.Background(), "enforce", repo, "HEAD", "", &out, nil); err != nil {
		t.Fatalf("a green pack ships: %v\n%s", err, out.String())
	}
	args := byNameCall(t, *calls)
	if !slices.Contains(args, orphanGuardPackage) || !strings.Contains(strings.Join(args, " "), orphanGuardTest) {
		t.Fatalf("a Go change in a package tmuxtest does not import must still run %s in %s by name;\n%s", orphanGuardTest, orphanGuardPackage, strings.Join(args, " "))
	}
}

func TestRepoContractGate_ARedOrphanGuardBlocksShipAndNamesItself(t *testing.T) {
	prev := goTestJSONFn
	goTestJSONFn = func(_ context.Context, _ string, _ packLog, args []string) packOutcome {
		if slices.Contains(args, "-run") && slices.Contains(args, orphanGuardPackage) && strings.Contains(strings.Join(args, " "), orphanGuardTest) {
			return redPack("github.com/mickeyyaya/evolve-loop/go/internal/tmuxtest." + orphanGuardTest)
		}
		return greenPack()
	}
	t.Cleanup(func() { goTestJSONFn = prev })
	err := runRepoContractGateAt(context.Background(), "enforce", addedFakeTmuxTestLane(t), "HEAD", "", &strings.Builder{}, nil)
	wantShipErr(t, err, shiperr.CodeRepoContractGate, core.ShipClassPrecondition, orphanGuardTest)
}
