//go:build acs

package cycle2634

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	hugeFloorPolicy  = `{"disk":{"floor_gb":100000000,"warn_gb":100000000}}`
	smallFloorPolicy = `{"disk":{"floor_gb":1,"warn_gb":1}}`
	newFloorKey      = "disk.floor_gb"
	oldGCFloorKey    = "gc.disk_floor_gb"
	oldPreflightKey  = "preflight.min_free_gib"
)

var (
	buildOnce   sync.Once
	builtBinary string
	buildOutput string
	buildCode   int
)

func evolveBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "evolve-cycle2634-")
		if err != nil {
			buildCode = -1
			buildOutput = err.Error()
			return
		}
		builtBinary = filepath.Join(dir, "evolve")
		stdout, stderr, code, _ := acsassert.SubprocessOutput("go", "-C", acsassert.GoModuleDir(t), "build", "-o", builtBinary, "./cmd/evolve")
		buildCode = code
		buildOutput = stdout + stderr
	})
	if buildCode != 0 {
		t.Fatalf("go build ./cmd/evolve exit %d\n%s", buildCode, acsassert.OutputTail(buildOutput, 40))
	}
	return builtBinary
}

func projectWithPolicy(t *testing.T, policyBody string) (root, evolveDir string) {
	t.Helper()
	root = t.TempDir()
	evolveDir = filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "policy.json"), []byte(policyBody), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, evolveDir
}

func runGC(t *testing.T, policyBody string) (output string, code int) {
	t.Helper()
	root, _ := projectWithPolicy(t, policyBody)
	stdout, stderr, code, err := acsassert.SubprocessOutput(evolveBinary(t), "gc", "--dry-run", "--only", "cache", "--project-root", root)
	if code < 0 {
		t.Fatalf("evolve gc did not start: %v", err)
	}
	return stdout + stderr, code
}

func runPreflightOnly(t *testing.T, policyBody string) string {
	t.Helper()
	root, evolveDir := projectWithPolicy(t, policyBody)
	stdout, stderr, code, err := acsassert.SubprocessOutput(evolveBinary(t), "loop", "--preflight-only", "--skip-preflight-boot", "--project-root", root, "--evolve-dir", evolveDir)
	if code < 0 {
		t.Fatalf("evolve loop --preflight-only did not start: %v", err)
	}
	return stdout + stderr
}

func lineWith(output string, parts ...string) bool {
	for _, line := range strings.Split(output, "\n") {
		all := true
		for _, p := range parts {
			all = all && strings.Contains(line, p)
		}
		if all {
			return true
		}
	}
	return false
}

func TestC2634_001_TheNewKeyAloneSetsTheFloorOfTheRealGCAndTheRealLoopPreflight(t *testing.T) {
	gcOut, gcCode := runGC(t, hugeFloorPolicy)
	if gcCode != 0 || !lineWith(gcOut, "WARN", "is under", newFloorKey) {
		t.Errorf("evolve gc exit %d: want a WARN naming %s as the breached floor:\n%s", gcCode, newFloorKey, gcOut)
	}
	if gcOut, _ := runGC(t, smallFloorPolicy); lineWith(gcOut, "WARN", "is under") {
		t.Errorf("evolve gc warns under a 1 GB floor on a host that has more free:\n%s", gcOut)
	}
	if out := runPreflightOnly(t, hugeFloorPolicy); !lineWith(out, "[halt]", "disk-space") {
		t.Errorf("evolve loop --preflight-only does not halt disk-space under a floor above the free disk:\n%s", out)
	}
	if out := runPreflightOnly(t, smallFloorPolicy); !lineWith(out, "[pass]", "disk-space") {
		t.Errorf("evolve loop --preflight-only does not pass disk-space under a 1 GB floor:\n%s", out)
	}
}

func TestC2634_002_TheNewKeyAloneIsOneFloorForTheGCTheLoopPreflightAndThePercentGuardOnTheirProductionCallers(t *testing.T) {
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     acsassert.GoModuleDir(t),
		Package: "./cmd/evolve",
		Pattern: "^(TestDiskFloorOneHome_NewKeyAloneIsOneFloorForEveryReader|TestDiskFloorOneHome_GCNamesTheNewKeyWhenTheFloorIsBreached|TestDiskFloorOneHome_TheSequentialLoopStopsBeforeACycleUnderTheNewFloor|TestDiskFloorOneHome_AFleetLaneStopsBeforeACycleUnderTheNewFloor|TestDiskFloorOneHome_TheWaveHaltReadsTheNewKey|TestDiskFloorOneHome_WaveNextRefusesUnderTheNewKey)$",
		Names: []string{
			"TestDiskFloorOneHome_NewKeyAloneIsOneFloorForEveryReader",
			"TestDiskFloorOneHome_GCNamesTheNewKeyWhenTheFloorIsBreached",
			"TestDiskFloorOneHome_TheSequentialLoopStopsBeforeACycleUnderTheNewFloor",
			"TestDiskFloorOneHome_AFleetLaneStopsBeforeACycleUnderTheNewFloor",
			"TestDiskFloorOneHome_TheWaveHaltReadsTheNewKey",
			"TestDiskFloorOneHome_WaveNextRefusesUnderTheNewKey",
		},
	})
}

func TestC2634_003_AnOldKeyStillSetsTheFloorAndWarnsNamingTheNewKeyThroughTheRealBinary(t *testing.T) {
	gcOut, gcCode := runGC(t, `{"gc":{"disk_floor_gb":100000000,"disk_warn_gb":100000000}}`)
	if gcCode != 0 || !lineWith(gcOut, "is under") {
		t.Errorf("evolve gc exit %d: the old key %s no longer sets the floor:\n%s", gcCode, oldGCFloorKey, gcOut)
	}
	if !lineWith(gcOut, "WARN", oldGCFloorKey, newFloorKey) {
		t.Errorf("evolve gc prints no WARN naming %s and %s:\n%s", oldGCFloorKey, newFloorKey, gcOut)
	}
	out := runPreflightOnly(t, `{"preflight":{"min_free_gib":100000000}}`)
	if !lineWith(out, "[halt]", "disk-space") {
		t.Errorf("the old key %s no longer halts disk-space in evolve loop --preflight-only:\n%s", oldPreflightKey, out)
	}
	if !lineWith(out, "WARN", oldPreflightKey, newFloorKey) {
		t.Errorf("evolve loop --preflight-only prints no WARN naming %s and %s:\n%s", oldPreflightKey, newFloorKey, out)
	}
}

func TestC2634_004_AnOldKeyStillSetsTheFloorForEveryReaderAndWarnsNamingTheNewKey(t *testing.T) {
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     acsassert.GoModuleDir(t),
		Package: "./cmd/evolve",
		Pattern: "^(TestDiskFloorOneHome_AnOldKeyStillSetsTheFloorAndWarnsNamingTheNewKey|TestDiskFloorOneHome_NoWarnWhenOnlyTheNewKeyIsSet)$",
		Names: []string{
			"TestDiskFloorOneHome_AnOldKeyStillSetsTheFloorAndWarnsNamingTheNewKey",
			"TestDiskFloorOneHome_NoWarnWhenOnlyTheNewKeyIsSet",
		},
	})
}

func TestC2634_005_ConflictingOldAndNewValuesAreRefusedWithAnErrorNamingBothThroughTheRealBinary(t *testing.T) {
	cases := []struct{ name, policy, oldKey string }{
		{"gc floor", `{"gc":{"disk_floor_gb":5},"disk":{"floor_gb":8}}`, oldGCFloorKey},
		{"preflight floor", `{"preflight":{"min_free_gib":2},"disk":{"floor_gb":8}}`, oldPreflightKey},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gcOut, gcCode := runGC(t, c.policy)
			if gcCode == 0 {
				t.Errorf("evolve gc ran with conflicting keys:\n%s", gcOut)
			}
			if !strings.Contains(gcOut, c.oldKey) || !strings.Contains(gcOut, newFloorKey) {
				t.Errorf("evolve gc refusal does not name both %s and %s:\n%s", c.oldKey, newFloorKey, gcOut)
			}
			out := runPreflightOnly(t, c.policy)
			if !strings.Contains(out, c.oldKey) || !strings.Contains(out, newFloorKey) {
				t.Errorf("evolve loop --preflight-only does not name both %s and %s:\n%s", c.oldKey, newFloorKey, out)
			}
			if lineWith(out, "[pass]", "disk-space") {
				t.Errorf("evolve loop --preflight-only passes disk-space under conflicting keys:\n%s", out)
			}
		})
	}
}

func TestC2634_006_ConflictingOldAndNewValuesAreRefusedByEveryReader(t *testing.T) {
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     acsassert.GoModuleDir(t),
		Package: "./cmd/evolve",
		Pattern: "^(TestDiskFloorOneHome_ConflictingOldAndNewValuesAreRefusedByGC|TestDiskFloorOneHome_ConflictingOldAndNewValuesHaltTheLoopPreflight|TestDiskFloorOneHome_ConflictingOldAndNewValuesRefuseTheCycleStart|TestDiskFloorOneHome_EqualOldAndNewValuesAreNotAConflict)$",
		Names: []string{
			"TestDiskFloorOneHome_ConflictingOldAndNewValuesAreRefusedByGC",
			"TestDiskFloorOneHome_ConflictingOldAndNewValuesHaltTheLoopPreflight",
			"TestDiskFloorOneHome_ConflictingOldAndNewValuesRefuseTheCycleStart",
			"TestDiskFloorOneHome_EqualOldAndNewValuesAreNotAConflict",
		},
	})
}

func TestC2634_007_AGuardTestFailsWhenASecondReaderOfADiskFloorAppearsOutsideTheResolver(t *testing.T) {
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     acsassert.GoModuleDir(t),
		Package: "./internal/policy",
		Pattern: "^(TestSingleReader_NoDiskFloorReaderLivesOutsideTheResolver|TestSingleReader_AGuardFindsASecondReaderOutsideTheResolver)$",
		Names: []string{
			"TestSingleReader_NoDiskFloorReaderLivesOutsideTheResolver",
			"TestSingleReader_AGuardFindsASecondReaderOutsideTheResolver",
		},
	})
}

// acs-predicate: config-check
func TestC2634_008_TheResolverPackageIsEnrolledInTheApicoverGateWithItsNamedTest(t *testing.T) {
	module := acsassert.GoModuleDir(t)
	enforce, err := os.ReadFile(filepath.Join(module, ".apicover-enforce"))
	if err != nil {
		t.Fatal(err)
	}
	if !lineWith(string(enforce), "./internal/diskfloor") {
		t.Errorf("go/.apicover-enforce does not list ./internal/diskfloor")
	}
	if !acsassert.PathExists(filepath.Join(module, "internal", "diskfloor", "apicover_named_test.go")) {
		t.Errorf("go/internal/diskfloor/apicover_named_test.go is missing")
	}
	entries, err := os.ReadDir(filepath.Join(module, "internal", "diskfloor"))
	if err != nil {
		t.Fatal(err)
	}
	production := 0
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".go") && !strings.HasSuffix(e.Name(), "_test.go") {
			production++
		}
	}
	if production == 0 {
		t.Errorf("go/internal/diskfloor holds no production file: the resolver is absent")
	}
}

func TestC2634_009_TheRepoContractPackHoldsEveryTreeReadingTestSoShipDoesNotRefuseTheGuard(t *testing.T) {
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     acsassert.GoModuleDir(t),
		Package: "./internal/repocontract",
		Pattern: "^TestPackages_HoldEveryTestThatReadsTheWholeTree$",
		Names:   []string{"TestPackages_HoldEveryTestThatReadsTheWholeTree"},
	})
}
