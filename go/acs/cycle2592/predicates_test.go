//go:build acs

package cycle2592

import (
	"bytes"
	"context"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	treewalkPackage = "./internal/treewalk"
	cmdPackage      = "./cmd/evolve"
	ciparityPackage = "./internal/ciparity"
)

func exactPattern(names []string) string {
	return "^(" + strings.Join(names, "|") + ")$"
}

func requireTests(t *testing.T, pkg, tags string, names ...string) {
	t.Helper()
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     acsassert.GoModuleDir(t),
		Package: pkg,
		Pattern: exactPattern(names),
		Names:   names,
		Tags:    tags,
	})
}

func coverFunc(t *testing.T, pkg, run string) string {
	t.Helper()
	dir := acsassert.GoModuleDir(t)
	profile := filepath.Join(t.TempDir(), "cover.out")
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	testArgs := []string{"test", "-count=1", "-coverprofile=" + profile}
	if run != "" {
		testArgs = append(testArgs, "-run", run)
	}
	for _, args := range [][]string{append(testArgs, pkg), {"tool", "cover", "-func=" + profile}} {
		cmd := exec.CommandContext(ctx, "go", args...)
		cmd.Dir = dir
		cmd.Env = ipcenv.GoTestEnv()
		cmd.WaitDelay = 10 * time.Second
		var out, errOut bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &errOut
		if err := cmd.Run(); err != nil {
			t.Fatalf("go %s: %v\n%s%s", strings.Join(args, " "), err, out.String(), errOut.String())
		}
		if args[0] == "tool" {
			return out.String()
		}
	}
	return ""
}

func functionsOf(coverOutput, file string) map[string]float64 {
	found := map[string]float64{}
	for _, line := range strings.Split(coverOutput, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || !strings.Contains(fields[0], "/"+file+":") {
			continue
		}
		pct, err := acsassert.FuncCoverage(line, fields[1])
		if err == nil {
			found[fields[1]] = pct
		}
	}
	return found
}

func requireFullCoverage(t *testing.T, coverOutput, file string, mustCover []string, unchangedBefore []string) {
	t.Helper()
	funcs := functionsOf(coverOutput, file)
	if len(funcs) == 0 {
		t.Fatalf("cover output lists no function of %s", file)
	}
	for _, name := range mustCover {
		if pct, ok := funcs[name]; !ok {
			t.Errorf("%s has no function %s in the cover output", file, name)
		} else if pct != 100 {
			t.Errorf("%s: %s is at %.1f%%, want 100%%", file, name, pct)
		}
	}
	for name, pct := range funcs {
		if slices.Contains(unchangedBefore, name) || slices.Contains(mustCover, name) {
			continue
		}
		if pct != 100 {
			t.Errorf("%s: new function %s is at %.1f%%, want 100%%", file, name, pct)
		}
	}
}

func TestC2592_001_ACheckThatSeesANewPathOutsideTheAllowListFailsInTheLibraryTheCommandAndTheTarget(t *testing.T) {
	requireTests(t, treewalkPackage, "",
		"TestFilterDropsPathsAtOrUnderAnAllowedPrefix",
		"TestFilterKeepsOrderAndLeavesItsInputsUntouched",
		"TestFilterAppliedToADiffHidesOnlyTheAllowedLeaks")
	requireTests(t, cmdPackage, "",
		"TestRunTreewalkCheckAllowSkipsOnlyTheListedPrefixes",
		"TestRunTreewalkCheckCountsOnlyTheLeaksThatRemainAfterTheAllowList",
		"TestRunTreewalkCheckRejectsAnEmptyAllowEntry",
		"TestRunTreewalkCheckRejectsAFlagWithoutAValueAndHelpListsAllow",
		"TestRunTreewalkCheckReportsAnUnreadableBaselineOrRoot",
		"TestRunTreewalkCheckRejectsAPositionalArgument")
	requireTests(t, ciparityPackage, "integration",
		"TestMakeIntegration_FailsOnALeakEvenWhenATestAlsoFails")
}

func TestC2592_002_ACleanTreePassesTheTargetWithItsOwnCoverageBinAndRusageOutputsPresent(t *testing.T) {
	requireTests(t, ciparityPackage, "integration",
		"TestMakeIntegration_PassesOnACleanTreeWithItsOwnOutputsPresent",
		"TestMakeIntegration_PassesWithTheDefaultOutputsOnly",
		"TestMakeIntegration_IgnoresPathsThatExistedBeforeTheRun",
		"TestMakeIntegration_KeepsTheTestFailureWhenNothingLeaks")
}

func TestC2592_003_APlantedLeakedEvolveDirectoryFailsTheCITarget(t *testing.T) {
	requireTests(t, ciparityPackage, "integration",
		"TestMakeIntegration_FailsWhenATestLeaksAPathOutsideTheAllowList")
}

func TestC2592_004_EveryChangedFunctionOfTheTreewalkLibraryAndCommandIsAtFullCoverage(t *testing.T) {
	requireFullCoverage(t, coverFunc(t, treewalkPackage, ""), "treewalk.go",
		[]string{"Filter"}, []string{"Take", "Diff", "Write", "Load"})
	requireFullCoverage(t, coverFunc(t, cmdPackage, "Treewalk"), "cmd_treewalk.go",
		[]string{"runTreewalk", "runTreewalkCheck"}, []string{"runTreewalkSnapshot"})
}

func TestC2592_005_TheFastTestTargetKeepsItsTreewalkSnapshotAndCheck(t *testing.T) {
	cmd := exec.Command("make", "-n", "-C", acsassert.GoModuleDir(t), "test")
	cmd.Env = ipcenv.GoTestEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("make -n test: %v\n%s", err, out)
	}
	for _, want := range []string{"treewalk snapshot", "treewalk check"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("make test no longer runs %q:\n%s", want, out)
		}
	}
}
