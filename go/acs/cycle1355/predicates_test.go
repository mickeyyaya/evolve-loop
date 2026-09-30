//go:build acs

package cycle1355

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const runtimeRefDoc = "docs/operations/runtime-reference.md"

// acs-predicate: config-check
func TestC1355_001_DocDescribesCatalogRefreshLedgerSemantics(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, runtimeRefDoc)

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", runtimeRefDoc, err)
	}
	body := string(raw)

	required := []string{
		"catalog_refresh",
		"\"ok\"",
		"\"failed\"",
		"catalog.refresh_stage",
		"best-effort",
	}
	var missing []string
	for _, s := range required {
		if !strings.Contains(body, s) {
			missing = append(missing, s)
		}
	}
	if len(missing) > 0 {
		t.Errorf("%s missing catalog_refresh ledger semantics, absent substrings: %v", runtimeRefDoc, missing)
	}
}

// acs-predicate: config-check
func TestC1355_002_DocEntryPlacedNearExistingLedgerRows(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, runtimeRefDoc)

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", runtimeRefDoc, err)
	}
	lines := strings.Split(string(raw), "\n")

	indexOf := func(needle string) int {
		for i, l := range lines {
			if strings.Contains(l, needle) {
				return i
			}
		}
		return -1
	}

	refreshLine := indexOf("catalog_refresh")
	if refreshLine < 0 {
		t.Fatalf("catalog_refresh not found in %s (doc row not added yet)", runtimeRefDoc)
	}

	contractLine := indexOf("contract_correction")
	planModeLine := indexOf("plan_mode_degraded")
	if contractLine < 0 || planModeLine < 0 {
		t.Fatalf("sibling ledger-entry anchors not found in %s (contract_correction=%d, plan_mode_degraded=%d) — doc structure changed unexpectedly", runtimeRefDoc, contractLine, planModeLine)
	}

	const maxDistance = 100
	dist := refreshLine - contractLine
	if dist < 0 {
		dist = -dist
	}
	distPlanMode := refreshLine - planModeLine
	if distPlanMode < 0 {
		distPlanMode = -distPlanMode
	}
	if dist > maxDistance && distPlanMode > maxDistance {
		t.Errorf("catalog_refresh row (line %d) is not near the existing ledger-entry rows (contract_correction line %d, plan_mode_degraded line %d) — looks like a standalone section instead of an addition beside the existing documentation", refreshLine+1, contractLine+1, planModeLine+1)
	}
}

func TestC1355_003_PinnedCatalogRefreshTestsStillPass(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")

	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-C", goDir, "./internal/core", "-run", "CatalogRefresh", "-v",
	)
	if err != nil {
		t.Fatalf("running pinned CatalogRefresh tests: %v (stderr: %s)", err, stderr)
	}
	if code != 0 {
		t.Errorf("pinned CatalogRefresh tests failed (exit %d)\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	const wantPass = 4
	got := strings.Count(stdout, "--- PASS: TestOrchestrator_CatalogRefresh")
	if got < wantPass {
		t.Errorf("expected >= %d pinned CatalogRefresh PASS lines, got %d\nstdout:\n%s", wantPass, got, stdout)
	}
}
