//go:build acs

// Package cycle1355 materializes the acceptance criteria for the single
// fleet-scoped task pinned to this lane's todo-id (`chain-summary-refresh-event-field`),
// resolved by Scout to task `document-catalog-refresh-ledger-entry`:
//
// Scout verified the underlying feature — a `catalog_refresh` ledger entry
// stamped by `Orchestrator.planCycle` (go/internal/core/cyclerun.go:586-612)
// whenever `WithCatalogRefresher` is wired — is already fully implemented and
// covered by `go/internal/core/catalog_refresh_ledger_test.go` (4/4 PASS).
// The one legitimate gap is documentation: `docs/operations/runtime-reference.md`
// has zero mention of the `catalog_refresh` ledger kind, unlike its sibling
// kinds `contract_correction` (line 54) and `plan_mode_degraded` (line 111).
//
// This is a DOC-ONLY task: zero Go source changes are in scope. Per
// go/acs/README.md's "Generated-from-source docs" note, the "system under
// test" for AC-1/AC-2 below genuinely IS the documentation file's content and
// placement — there is no separate behavior to invoke, so a content/structure
// assertion over the doc is the correct predicate shape here (the
// config-check waiver: `// acs-predicate: config-check`), not a source-grep
// standing in for an unexercised behavior. AC-3 is a true behavioral
// subprocess predicate: it proves the doc-only change didn't regress the
// already-shipped ledger-stamping behavior.
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
//
// TestC1355_001_DocDescribesCatalogRefreshLedgerSemantics is the primary,
// load-bearing predicate. It requires the doc to state the FULL Action/Message
// contract from cyclerun.go:586-612 — not merely the bare string
// "catalog_refresh" (which a no-op doc edit could add without conveying
// anything). All of: the kind name, both Action outcomes, the Message
// semantics (resolved catalog.refresh_stage), and the best-effort/never-blocks
// guarantee must be present. RED today: the doc has zero mention of
// catalog_refresh (verified live via `grep -n catalog_refresh
// docs/operations/runtime-reference.md` -> no hits).
func TestC1355_001_DocDescribesCatalogRefreshLedgerSemantics(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, runtimeRefDoc)

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", runtimeRefDoc, err)
	}
	body := string(raw)

	required := []string{
		"catalog_refresh",       // the ledger Kind
		"\"ok\"",                // Action outcome 1
		"\"failed\"",            // Action outcome 2
		"catalog.refresh_stage", // Message semantics
		"best-effort",           // never-blocks guarantee (matches sibling rows' vocabulary)
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
//
// TestC1355_002_DocEntryPlacedNearExistingLedgerRows is the structural
// (negative-shape) predicate: it rejects a lazily-appended standalone section
// far from the existing ledger-entry documentation. The scout report and eval
// require the addition to sit BESIDE the existing plan_mode_degraded /
// contract_correction rows (around line 111), not bolted onto the end of the
// file. RED today: catalog_refresh is entirely absent, so proximity is
// trivially unsatisfied (index -1).
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

	const maxDistance = 100 // lines; keeps the addition in the same doc neighborhood, not a new standalone section
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

// TestC1355_003_PinnedCatalogRefreshTestsStillPass is the behavioral
// regression predicate: it actually invokes the go test binary (not a
// source-grep) to prove the doc-only change did not regress the
// already-shipped ledger-stamping implementation. Expected pre-existing
// GREEN (the feature already ships and is tested) — pinned here so the
// Builder's doc edit cannot accidentally touch go/internal/core and break it.
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
