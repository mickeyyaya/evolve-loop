//go:build acs

package cycle1978

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/repocontract"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const modulePath = "github.com/mickeyyaya/evolve-loop/go/"

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func runNamed(t *testing.T, pkg, tags string, names ...string) {
	t.Helper()
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     goDir(t),
		Package: modulePath + pkg,
		Pattern: "^(" + strings.Join(names, "|") + ")$",
		Names:   names,
		Tags:    tags,
	})
}

func TestC1978_001_GoTestEnvDropsEveryAmbientEvolveKeyAndKeepsTheToolchain(t *testing.T) {
	t.Setenv("EVOLVE_FLEET", "1")
	t.Setenv("EVOLVE_CYCLE_STATE_FILE", "/runs/cycle-1978/cycle-state.json")
	t.Setenv("PATH", "/usr/bin:/bin")

	env := ipcenv.GoTestEnv()

	for _, kv := range env {
		if strings.HasPrefix(kv, "EVOLVE_") {
			t.Errorf("GoTestEnv kept the ambient key %q", kv)
		}
	}
	if !slices.Contains(env, "PATH=/usr/bin:/bin") {
		t.Errorf("GoTestEnv dropped PATH: %q", env)
	}
}

func TestC1978_002_GoTestEnvAppendsItsExtrasAfterTheScrubbedAmbientEnv(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", "/ambient/root")
	t.Setenv("PATH", "/usr/bin:/bin")

	env := ipcenv.GoTestEnv("EVOLVE_PROJECT_ROOT=/suite/root", "CHANGED_PACKAGES=./internal/x")

	var roots []string
	for _, kv := range env {
		if strings.HasPrefix(kv, "EVOLVE_PROJECT_ROOT=") {
			roots = append(roots, kv)
		}
	}
	if !slices.Equal(roots, []string{"EVOLVE_PROJECT_ROOT=/suite/root"}) {
		t.Errorf("EVOLVE_PROJECT_ROOT entries = %q, want only the extra, never the ambient value", roots)
	}
	if n := len(env); n < 2 || env[n-2] != "EVOLVE_PROJECT_ROOT=/suite/root" || env[n-1] != "CHANGED_PACKAGES=./internal/x" {
		t.Errorf("GoTestEnv tail = %q, want the extras last and in order", env)
	}
}

func TestC1978_003_EveryProductionGoTestSpawnerPassesTheWholeTreePin(t *testing.T) {
	runNamed(t, "internal/ipcenv", "", "TestGoTestSpawners_EveryProductionSpawnerUsesTheNamedProjection")
}

func TestC1978_004_ThePinFlagsAPlantedSpawnerAndAcceptsTheNamedProjections(t *testing.T) {
	runNamed(t, "internal/ipcenv", "",
		"TestGoTestSpawners_APlantedSpawnerThatBuildsItsEnvAnyOtherWayIsFlagged",
		"TestGoTestSpawners_ASpawnerThatUsesANamedProjectionIsAccepted",
		"TestGoTestSpawners_ACommandThatIsNotGoTestIsLeftAlone",
		"TestGoTestSpawners_ATreeScanReadsProductionFilesOnly",
		"TestGoTestSpawners_TheTreeScanFindsEveryKnownProductionSpawner",
		"TestGoTestSpawners_EveryExemptionStillNamesAnUnprojectedSpawner",
	)
}

func TestC1978_005_PkgAcsassertGoTestsRunsItsChildWithoutTheCallersEvolveKeys(t *testing.T) {
	runNamed(t, "pkg/acsassert", "integration", "TestGoTests_TheChildGoTestSeesNoEvolveKeyOfItsCaller")
}

func TestC1978_006_TheCommitGateGoLaneRunsGoTestWithoutTheLaneProtocolKeys(t *testing.T) {
	runNamed(t, "internal/commitgate", "", "TestRun_AGoLaneTestCarriesNoLaneProtocolKeyAndKeepsTheToolchain")
}

func TestC1978_007_TheCIParityGatesRunEveryGoTestWithoutTheLaneProtocolKeys(t *testing.T) {
	runNamed(t, "internal/phases/audit/ciparitygate", "", "TestGoTestGates_NoGateRunsItsGoTestWithALaneProtocolKey")
}

func TestC1978_008_ThePinsPackageJoinsTheRepoContractPack(t *testing.T) {
	inPack := slices.ContainsFunc(repocontract.Packages(), func(pattern string) bool {
		return strings.TrimPrefix(strings.TrimSuffix(pattern, "..."), "./") == "internal/ipcenv/"
	})
	if !inPack {
		t.Errorf("repocontract.Packages() = %q, want the internal/ipcenv package because the pin reads the whole tree", repocontract.Packages())
	}
	runNamed(t, "internal/repocontract", "", "TestPackages_HoldEveryTestThatReadsTheWholeTree")
}

func TestC1978_009_TheSizeRatchetOffendersAreNotTouched(t *testing.T) {
	root := acsassert.RepoRoot(t)
	out, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "diff", "--name-only", "HEAD", "--", "go/internal/sizeratchet/offenders.json")
	if err != nil || code != 0 {
		t.Fatalf("git diff exit=%d: %v", code, err)
	}
	if strings.TrimSpace(out) != "" {
		t.Errorf("go/internal/sizeratchet/offenders.json changed: %s", out)
	}
	if _, err := os.Stat(filepath.Join(root, "go", "internal", "sizeratchet", "offenders.json")); err != nil {
		t.Errorf("offenders.json missing: %v", err)
	}
}
