//go:build acs

package cycle2134

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	argonMedium = "Gemini 4 Argon (Medium)"
	argonHigh   = "Gemini 4 Argon (High)"
)

func hostileCatalogRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cat := modelcatalog.Catalog{CLIs: map[string]modelcatalog.CLIEntry{
		"agy": {
			Source: modelcatalog.SourceLive,
			TierModels: map[string]string{
				"fast":     "Gemini 3.8 Flash (Low)",
				"balanced": "Gemini 3.8 Flash (High)",
				"deep":     argonMedium,
				"top":      argonHigh,
			},
		},
	}}
	if err := modelcatalog.Write(evolveDir, cat); err != nil {
		t.Fatal(err)
	}
	return root
}

func isolateCatalog(t *testing.T) {
	t.Helper()
	empty := t.TempDir()
	bridge.SetModelCatalogDirFn(func() string { return empty })
}

func goTestOnePackageUnderHostileCatalog(t *testing.T, pkg, run string) {
	t.Helper()
	moduleDir, err := acsassert.ModuleDir(filepath.Join(acsassert.RepoRoot(t), "go"))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "test", "-count=1", "-run", run, pkg)
	cmd.Dir = moduleDir
	cmd.Env = append(os.Environ(), "EVOLVE_PROJECT_ROOT="+hostileCatalogRoot(t))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("go test %s -run %s fails when the live catalog maps agy top to %q: %v\n%s", pkg, run, argonHigh, err, acsassert.OutputTail(string(out), 25))
	}
}

func pinnedTopInSource(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), rel))
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`"top":\s*"([^"]*)"`).FindSubmatch(data)
	if m == nil {
		t.Fatalf("%s pins no \"top\" tier model", rel)
	}
	return string(m[1])
}

func TestC2134_001_AgyTopTierLoadsAsArgonMedium(t *testing.T) {
	isolateCatalog(t)
	m, err := bridge.LoadManifest("agy-tmux")
	if err != nil {
		t.Fatalf("LoadManifest(agy-tmux): %v", err)
	}
	if got := m.ModelTierMap["top"]; got != argonMedium {
		t.Errorf("agy top tier = %q, want %q", got, argonMedium)
	}
}

func TestC2134_002_AgyTopTierIsNoLongerArgonHigh(t *testing.T) {
	isolateCatalog(t)
	m, err := bridge.LoadManifest("agy-tmux")
	if err != nil {
		t.Fatalf("LoadManifest(agy-tmux): %v", err)
	}
	if got := m.ModelTierMap["top"]; got == argonHigh {
		t.Errorf("agy top tier still %q", got)
	}
}

func TestC2134_003_OtherAgyTiersKeepTheirModels(t *testing.T) {
	isolateCatalog(t)
	m, err := bridge.LoadManifest("agy-tmux")
	if err != nil {
		t.Fatalf("LoadManifest(agy-tmux): %v", err)
	}
	for tier, want := range map[string]string{
		"fast":     "Gemini 3.8 Flash (Low)",
		"balanced": "Gemini 3.8 Flash (High)",
		"deep":     argonMedium,
	} {
		if got := m.ModelTierMap[tier]; got != want {
			t.Errorf("agy tier %q = %q, want %q (only top may change)", tier, got, want)
		}
	}
}

func TestC2134_004_BridgePinTestPassesAndPinsMediumAtTop(t *testing.T) {
	goTestOnePackageUnderHostileCatalog(t, "./internal/bridge", "^TestAgyTierModelsPinnedToVerifiedNames$")
	if got := pinnedTopInSource(t, "go/internal/bridge/agy_model_names_test.go"); got != argonMedium {
		t.Errorf("bridge pin test expects top %q, want %q", got, argonMedium)
	}
}

func TestC2134_005_SetupTierModelsTestPassesAndPinsMediumAtTop(t *testing.T) {
	goTestOnePackageUnderHostileCatalog(t, "./internal/setup", "^TestTierModelsFor$")
	if got := pinnedTopInSource(t, "go/internal/setup/setup_test.go"); got != argonMedium {
		t.Errorf("setup tier test expects top %q, want %q", got, argonMedium)
	}
}

func TestC2134_007_HostileCatalogFixtureOverlaysArgonHighOnTheManifest(t *testing.T) {
	root := hostileCatalogRoot(t)
	evolveDir := filepath.Join(root, ".evolve")
	bridge.SetModelCatalogDirFn(func() string { return evolveDir })
	m, err := bridge.LoadManifest("agy-tmux")
	if err != nil {
		t.Fatalf("LoadManifest(agy-tmux): %v", err)
	}
	if got := m.ModelTierMap["top"]; got != argonHigh {
		t.Errorf("hostile catalog fixture does not overlay: top = %q, want %q (predicates 004 and 005 would pass vacuously)", got, argonHigh)
	}
}

// acs-predicate: config-check
func TestC2134_006_PlanMarksP1BuiltAndLeavesP2Open(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), "docs/plans/model-tier-policy-2026-10.md"))
	if err != nil {
		t.Fatal(err)
	}
	rows := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		for _, id := range []string{"P1", "P2"} {
			if strings.HasPrefix(line, "| "+id+" |") {
				rows[id] = line
			}
		}
	}
	p1, ok := rows["P1"]
	if !ok {
		t.Fatal("component table has no P1 row")
	}
	if !strings.Contains(p1, "**Built:") || strings.Contains(p1, "**Open:") {
		t.Errorf("P1 row is not marked done (need a **Built: ...** marker and no **Open:** marker): %s", p1)
	}
	if p2 := rows["P2"]; !strings.Contains(p2, "**Open:") {
		t.Errorf("P2 row must stay open (out of scope): %s", p2)
	}
}
