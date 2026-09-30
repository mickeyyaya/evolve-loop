//go:build acs

package cycle359

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func controlFlagsPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "docs", "architecture", "control-flags.md")
}

func TestC359_001_PlatformHybridFlagsAbsentFromRegistry(t *testing.T) {
	deadFlags := []string{
		"EVOLVE_GEMINI_CLAUDE_PATH",
		"EVOLVE_GEMINI_REQUIRE_FULL",
		"EVOLVE_CODEX_CLAUDE_PATH",
		"EVOLVE_ALLOW_INTERACTIVE_FALLBACK",
		"EVOLVE_FORCE_BARE",
	}
	for _, name := range deadFlags {
		if f, ok := flagregistry.Lookup(name); ok {
			t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — dead Platform/CLI Hybrid flag still registered.\n"+
				"Builder must remove this row from go/internal/flagregistry/registry_table.go.\n"+
				"Current entry: Status=%q Cluster=%q Doc=%q",
				name, f.Status, f.Cluster, f.Doc)
		}
	}
}

func TestC359_002_FlagsCheckExitsZero(t *testing.T) {
	root := acsassert.RepoRoot(t)
	binPath := filepath.Join(root, "go", "bin", "evolve")
	out, errOut, code, err := acsassert.SubprocessOutput(
		"bash", "-c", "cd "+root+" && "+binPath+" flags check",
	)
	combined := strings.TrimSpace(out + "\n" + errOut)
	if code != 0 || err != nil {
		t.Errorf("evolve flags check exited %d: %v\nOutput:\n%s\n"+
			"Builder must run `evolve flags generate` after removing registry_table.go rows.",
			code, err, combined)
	}
}

// acs-predicate: config-check
func TestC359_004_PlatformHybridFlagsAbsentFromControlFlagsDoc(t *testing.T) {
	doc := controlFlagsPath(t)
	removedFlagPatterns := []string{
		"| `EVOLVE_GEMINI_CLAUDE_PATH` |",
		"| `EVOLVE_GEMINI_REQUIRE_FULL` |",
		"| `EVOLVE_CODEX_CLAUDE_PATH` |",
		"| `EVOLVE_ALLOW_INTERACTIVE_FALLBACK` |",
		"| `EVOLVE_FORCE_BARE` |",
	}
	for _, pattern := range removedFlagPatterns {
		if !acsassert.FileNotContains(t, doc, pattern) {
			t.Errorf("RED: control-flags.md still contains table row for %q.\n"+
				"Builder must remove the registry rows and run `evolve flags generate` "+
				"to drop these entries from the Generated Flag Index, and manually remove "+
				"the corresponding hand-maintained cluster section rows.\nFile: %s",
				pattern, doc)
		}
	}
}

func TestC359_005_NoProductionReadersOfRemovedFlags(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goSrc := filepath.Join(root, "go")
	out, _, _, _ := acsassert.SubprocessOutput("bash", "-c",
		`grep -Erl "EVOLVE_GEMINI_CLAUDE_PATH|EVOLVE_GEMINI_REQUIRE_FULL|EVOLVE_CODEX_CLAUDE_PATH|EVOLVE_ALLOW_INTERACTIVE_FALLBACK|EVOLVE_FORCE_BARE" "`+
			goSrc+`" --include="*.go" 2>/dev/null | grep -v "_test.go" | grep -v "/acs/"; true`)
	if strings.TrimSpace(out) != "" {
		t.Errorf("RED: non-test, non-acs Go files still reference the removed Platform/CLI Hybrid flags:\n%s\n"+
			"Builder must remove all 5 rows from go/internal/flagregistry/registry_table.go.\n"+
			"Only acs/cycle354 and acs/cycle359 test files should reference these flag names.",
			strings.TrimSpace(out))
	}
}
