//go:build acs

package cycle2459

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/regressiontia"
)

const (
	cleanPredicate    = "./acs/regression/clean"
	directlyErrored   = "./acs/regression/directerr"
	transitivelyError = "./acs/regression/transitiveerr"
)

func writeModuleFile(t *testing.T, moduleDir, rel, body string) {
	t.Helper()
	path := filepath.Join(moduleDir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func corpusPredicate(name, importLine, call string) string {
	return "//go:build acs\n\npackage " + name + "\n\nimport (\n\t\"testing\"\n\n\t" + importLine + "\n)\n\nfunc TestIt(t *testing.T) { _ = " + call + " }\n"
}

func corpusWithErroredPackages(t *testing.T) (repoRoot, moduleDir string) {
	t.Helper()
	repoRoot = t.TempDir()
	moduleDir = filepath.Join(repoRoot, "go")
	writeModuleFile(t, moduleDir, "go.mod", "module example.com/m\n\ngo 1.21\n")
	writeModuleFile(t, moduleDir, "internal/leaf/leaf.go", "package leaf\n\nfunc Value() int { return 1 }\n")
	writeModuleFile(t, moduleDir, "internal/other/other.go", "package other\n\nfunc Value() int { return 2 }\n")
	writeModuleFile(t, moduleDir, "internal/mid/mid.go", "package mid\n\nimport \"example.com/m/internal/ghosttwo\"\n\nfunc Value() int { return ghosttwo.Value() }\n")
	writeModuleFile(t, moduleDir, "acs/regression/clean/clean_test.go", corpusPredicate("clean", "\"example.com/m/internal/leaf\"", "leaf.Value()"))
	writeModuleFile(t, moduleDir, "acs/regression/directerr/directerr_test.go", corpusPredicate("directerr", "\"example.com/m/internal/ghostone\"", "ghostone.Value()"))
	writeModuleFile(t, moduleDir, "acs/regression/transitiveerr/transitiveerr_test.go", corpusPredicate("transitiveerr", "\"example.com/m/internal/mid\"", "mid.Value()"))
	return repoRoot, moduleDir
}

func computeShadow(t *testing.T, changed ...string) regressiontia.Decision {
	t.Helper()
	repoRoot, moduleDir := corpusWithErroredPackages(t)
	d := regressiontia.Compute("shadow", repoRoot, moduleDir, changed)
	if len(d.ChangedPackages) == 0 {
		t.Fatalf("ChangedPackages is empty, so Select keeps every predicate before the dependency walk runs: %+v", d)
	}
	return d
}

func TestC2459_001_CorpusPackageWithAGoListErrorIsNeverWouldSkip(t *testing.T) {
	d := computeShadow(t, "./internal/other/...")

	if slices.Contains(d.WouldSkip, directlyErrored) {
		t.Errorf("WouldSkip = %v: %s imports a package that go list reports as an error, so its closure is unknown and the split must count it as io", d.WouldSkip, directlyErrored)
	}
	if !slices.Contains(d.Selected, directlyErrored) {
		t.Errorf("Selected = %v: %s must stay selected while its go list reports an error", d.Selected, directlyErrored)
	}
}

func TestC2459_002_AGoListErrorTwoHopsDownStillMakesThePackageUnskippable(t *testing.T) {
	d := computeShadow(t, "./internal/other/...")

	if slices.Contains(d.WouldSkip, transitivelyError) {
		t.Errorf("WouldSkip = %v: %s reaches an errored package through internal/mid, so skippableDeps must leave it out", d.WouldSkip, transitivelyError)
	}
	if !slices.Contains(d.Selected, transitivelyError) {
		t.Errorf("Selected = %v: %s must stay selected while its closure holds a go list error", d.Selected, transitivelyError)
	}
}

func TestC2459_003_AnErrorFreePackageStaysSkippableBesideErroredOnes(t *testing.T) {
	d := computeShadow(t, "./internal/other/...")

	if !slices.Contains(d.WouldSkip, cleanPredicate) {
		t.Errorf("WouldSkip = %v, Selected = %v: %s has no go list error and no escape hatch, so it must stay skippable", d.WouldSkip, d.Selected, cleanPredicate)
	}
	if slices.Contains(d.Selected, cleanPredicate) {
		t.Errorf("Selected = %v: %s must not be selected for a change it does not import", d.Selected, cleanPredicate)
	}
}

func TestC2459_004_AChangeInsideAnErroredClosureStillSelectsThePackage(t *testing.T) {
	d := computeShadow(t, "./internal/mid/...")

	if !slices.Contains(d.Selected, transitivelyError) {
		t.Errorf("Selected = %v: a change to internal/mid is inside the closure of %s and must select it", d.Selected, transitivelyError)
	}
	if slices.Contains(d.WouldSkip, transitivelyError) {
		t.Errorf("WouldSkip = %v: %s must never be would-skip", d.WouldSkip, transitivelyError)
	}
}
