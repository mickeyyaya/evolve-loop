//go:build acs

package cycle1913

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const cycle1911Pkg = "github.com/mickeyyaya/evolve-loop/go/acs/cycle1911"

var (
	scopedBenchTests = []string{
		"TestC1911_003_ArgonScopedBenchLeavesBalancedFlashDispatchable",
		"TestC1911_006_DeepArgonWallThroughTheWalkBenchesOnlyTheArgonGroup",
	}
	allCycle1911Tests = []string{
		"TestC1911_001_AgyCompactResetKeepsTheWallLineAndCapsAt24h",
		"TestC1911_002_AgyCompactResetParsesShortAndRejectsGarbage",
		"TestC1911_003_ArgonScopedBenchLeavesBalancedFlashDispatchable",
		"TestC1911_004_FamilyWideEntryStillBenchesEveryAgyTier",
		"TestC1911_005_ScopedAndFamilyEntriesCoexistInTheStore",
		"TestC1911_006_DeepArgonWallThroughTheWalkBenchesOnlyTheArgonGroup",
	}
)

func moduleRoot(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func hostProjectWithAgyCatalog(t *testing.T, agyTierModels map[string]string) string {
	t.Helper()
	project := t.TempDir()
	evolveDir := filepath.Join(project, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if agyTierModels == nil {
		return project
	}
	catalog := map[string]any{
		"fetched_at": "2026-10-17T00:00:00Z",
		"clis": map[string]any{
			"agy": map[string]any{"tier_models": agyTierModels, "source": "live"},
		},
	}
	raw, err := json.Marshal(catalog)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "model-catalog.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return project
}

func runCycle1911TestsUnderHost(t *testing.T, project string, names []string) {
	t.Helper()
	t.Setenv("EVOLVE_PROJECT_ROOT", project)
	t.Setenv("EVOLVE_PLUGIN_ROOT", project)
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     moduleRoot(t),
		Package: cycle1911Pkg,
		Pattern: "^(" + strings.Join(names, "|") + ")$",
		Names:   names,
		Tags:    "acs",
	})
}

func TestC1913_001_ScopedBenchTestsPassWhenHostCatalogPutsDeepOnFlash(t *testing.T) {
	deepCollapsedOntoFlash := hostProjectWithAgyCatalog(t, map[string]string{
		"fast":     "Gemini 3.8 Flash (Low)",
		"balanced": "Gemini 3.8 Flash (High)",
		"deep":     "Gemini 3.8 Flash (High)",
		"top":      "Gemini 3.8 Flash (High)",
	})
	runCycle1911TestsUnderHost(t, deepCollapsedOntoFlash, scopedBenchTests)
}

func TestC1913_002_ScopedBenchTestsPassWhenHostCatalogPutsDeepOnPro(t *testing.T) {
	deepOnGeminiPro := hostProjectWithAgyCatalog(t, map[string]string{
		"fast":     "Gemini 3.5 Flash (Low)",
		"balanced": "Gemini 3.5 Flash (Medium)",
		"deep":     "Gemini 3.1 Pro (High)",
		"top":      "Gemini 3.1 Pro (High)",
	})
	runCycle1911TestsUnderHost(t, deepOnGeminiPro, scopedBenchTests)
}

func TestC1913_003_AllCycle1911PredicatesPassWithNoHostCatalog(t *testing.T) {
	runCycle1911TestsUnderHost(t, hostProjectWithAgyCatalog(t, nil), allCycle1911Tests)
}

func TestC1913_004_AllCycle1911PredicatesPassUnderAMalformedHostCatalog(t *testing.T) {
	project := hostProjectWithAgyCatalog(t, nil)
	if err := os.WriteFile(filepath.Join(project, ".evolve", "model-catalog.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	runCycle1911TestsUnderHost(t, project, allCycle1911Tests)
}
