//go:build acs

package cycle2580

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

type rawManifest struct {
	ModelTierMap     map[string]string `json:"model_tier_map"`
	ModelTierMapFrom string            `json:"model_tier_map_from"`
}

func readRawManifest(t *testing.T, cli string) rawManifest {
	t.Helper()
	path := filepath.Join(acsassert.GoModuleDir(t), "internal", "bridge", "manifests", cli+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var m rawManifest
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return m
}

func goTestPackage(t *testing.T, pkg string, extra ...string) string {
	t.Helper()
	args := append([]string{"test", "-count=1"}, extra...)
	args = append(args, pkg)
	cmd := exec.Command("go", args...)
	cmd.Dir = acsassert.GoModuleDir(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go %s: %v\n%s", strings.Join(args, " "), err, acsassert.OutputTail(string(out), 40))
	}
	return string(out)
}

// acs-predicate: config-check
func TestC2580_001_AgyHeadlessManifestDeclaresNoTierMapAndPointsAtTheFamily(t *testing.T) {
	agy := readRawManifest(t, "agy")
	if len(agy.ModelTierMap) != 0 {
		t.Errorf("agy.json still declares model_tier_map %v", agy.ModelTierMap)
	}
	if agy.ModelTierMapFrom != "agy-tmux" {
		t.Errorf("agy.json model_tier_map_from = %q, want agy-tmux", agy.ModelTierMapFrom)
	}
	family := readRawManifest(t, "agy-tmux")
	if len(family.ModelTierMap) == 0 {
		t.Fatal("agy-tmux.json declares no model_tier_map: the family table must stay the one declaration")
	}
	if family.ModelTierMapFrom != "" {
		t.Errorf("agy-tmux.json model_tier_map_from = %q: the family manifest must own its table", family.ModelTierMapFrom)
	}
}

func TestC2580_002_LoadedAgyTierMapEqualsTheFamilyTable(t *testing.T) {
	bridge.SetModelCatalogDirFn(func() string { return t.TempDir() })
	headless, err := bridge.LoadManifest("agy")
	if err != nil {
		t.Fatalf("LoadManifest(agy): %v", err)
	}
	family, err := bridge.LoadManifest("agy-tmux")
	if err != nil {
		t.Fatalf("LoadManifest(agy-tmux): %v", err)
	}
	if len(family.ModelTierMap) == 0 {
		t.Fatal("agy-tmux loads with an empty tier map")
	}
	if len(headless.ModelTierMap) != len(family.ModelTierMap) {
		t.Fatalf("agy tier map %v, want the family table %v", headless.ModelTierMap, family.ModelTierMap)
	}
	for tier, want := range family.ModelTierMap {
		if got := headless.ModelTierMap[tier]; got != want {
			t.Errorf("LoadManifest(agy)[%s] = %q, want the family's %q", tier, got, want)
		}
	}
}

func TestC2580_003_BridgeRelationalTierMapTestRunsAndPasses(t *testing.T) {
	out := goTestPackage(t, "./internal/bridge", "-v", "-run", "^TestAgyHeadlessManifest_PointsAtFamilyTable$")
	if !strings.Contains(out, "--- PASS: TestAgyHeadlessManifest_PointsAtFamilyTable") {
		t.Errorf("the relational test did not run and pass:\n%s", acsassert.OutputTail(out, 20))
	}
}

func TestC2580_004_ProfilesPackageStaysGreen(t *testing.T) {
	goTestPackage(t, "./internal/profiles")
}

func TestC2580_005_SetupPackageStaysGreen(t *testing.T) {
	goTestPackage(t, "./internal/setup")
}

func TestC2580_006_BridgeAgyHeadlessConsumersStayGreen(t *testing.T) {
	goTestPackage(t, "./internal/bridge", "-run", "^(TestAgyModelTier|TestAgyEveryLaunch_DoctorProbeCarriesTheManifestFastModel|TestLaunchLLM_LogsTheManifestReasonForAnUnsandboxedExec|TestAgyTierDeepLaunchCarriesModelToPane|TestManifests_EachFamilyPassesItsConfigDirAndProxies|TestEmbeddedManifests_DecodeStrictlyAndNameOnlyKnownFormatIDs)")
}
