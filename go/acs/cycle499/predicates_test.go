//go:build acs

// Package cycle499 materialises the cycle-499 acceptance criteria.
//
// TRIAGE COMMITTED ONE ## top_n TASK this cycle:
//
//	latest-model-preference: Model-selection policy for ALL CLIs
//	(inbox 2026-07-02T02-00-00Z-latest-model-preference.json, D1-D9,
//	4-tier top-default vocabulary — assigned to THIS concurrent fleet
//	lane per triage's fleet_scope rationale).
//
// SCOPE NOTE (documented, not silent — see test-report.md "Scope Decision"):
// the inbox item is a 9-design-point campaign (D1-D9) spanning live CLI
// query, bridge-probe verification, family filtering, freshness triggers,
// and a conformance suite — far larger than one TDD->Builder cycle. This
// cycle materialises the two foundational, self-contained, zero-I/O design
// points that unblock the rest and are independently valuable:
//
//	D2 (newest-wins version comparator) → C499_001, C499_002
//	TIER VOCABULARY (top tier + high alias) → C499_003, C499_004
//
// D1, D3, D4, D5, D6, D7, D8, D9 and the docs deliverable are OUT OF SCOPE
// this cycle (they require live-CLI/bridge integration this slice does not
// touch) and are queued to carryoverTodos — see test-report.md.
//
// Predicate strategy (mirrors cycle488): behavioral predicates drive the
// new/changed code through in-package RED tests via subprocess `go test`,
// asserting non-degenerate failure (requireTestsRan guard closes the
// cycle-85 "no tests matched" trap) — never a source grep.
package cycle499

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const modelqueryPkg = "github.com/mickeyyaya/evolve-loop/go/internal/modelquery"
const modelcatalogPkg = "github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
const bridgePkg = "github.com/mickeyyaya/evolve-loop/go/internal/bridge"

// runGoTest runs `go test` on pkgs and returns the combined output + exit
// code. Behavioral predicates invoke the system under test through its own
// in-package tests — no source-grep gaming.
func runGoTest(t *testing.T, runFilter string, pkgs ...string) (out string, code int) {
	t.Helper()
	args := []string{"test", "-count=1", "-race", "-v"}
	if runFilter != "" {
		args = append(args, "-run", runFilter)
	}
	args = append(args, pkgs...)
	stdout, stderr, code, _ := acsassert.SubprocessOutput("go", args...)
	return stdout + "\n" + stderr, code
}

// requireTestsRan closes the degenerate-predicate trap: `go test -run X` with
// no matching test exits 0 with "no tests to run", which would green a
// predicate on unwritten (or renamed) work.
func requireTestsRan(t *testing.T, out string, min int) {
	t.Helper()
	if strings.Contains(out, "no tests to run") {
		t.Errorf("no tests matched the -run filter (\"no tests to run\") — required tests are unwritten or renamed")
		return
	}
	if got := strings.Count(out, "=== RUN"); got < min {
		t.Errorf("only %d test(s) ran, need >= %d", got, min)
	}
}

// TestC499_001_NewestWinsPicksHighestVersion (D2-AC1): the newest-wins
// comparator picks the numerically-highest version within a lineage across
// the exact opus/gpt/gemini examples in the inbox spec, order-independent,
// AND rejects a naive lexicographic compare (opus-4.10 > opus-4.9). Drives
// modelquery.NewestInLineage through its in-package test. RED today: the
// function does not exist (compile failure).
func TestC499_001_NewestWinsPicksHighestVersion(t *testing.T) {
	out, code := runGoTest(t, "TestNewestInLineage_PicksHighestVersionAcrossFormats|TestNewestInLineage_RejectsNaiveLexicographicOrdering", modelqueryPkg)
	requireTestsRan(t, out, 2)
	if code != 0 {
		t.Errorf("newest-wins version comparator is red (exit=%d) — modelquery.NewestInLineage is missing or naively string-compares versions\n%s", code, out)
	}
}

// TestC499_002_NewestWinsHandlesEffortMiniAndUnversioned (D2-AC2, adversarial
// diversity: mini-suffix + effort-parenthetical + unversioned-fallback are
// three DISTINCT behaviors, not one restated). RED today: the function does
// not exist (compile failure).
func TestC499_002_NewestWinsHandlesEffortMiniAndUnversioned(t *testing.T) {
	out, code := runGoTest(t, "TestNewestInLineage_IgnoresMiniSuffixWhenComparingVersions|TestNewestInLineage_EffortParentheticalTieFallsBackToInputOrder|TestNewestInLineage_UnversionedFallsBackAndNeverCrashes", modelqueryPkg)
	requireTestsRan(t, out, 3)
	if code != 0 {
		t.Errorf("newest-wins comparator's mini/effort/unversioned handling is red (exit=%d)\n%s", code, out)
	}
}

// TestC499_003_CanonicalTiersIncludesTop (TIER VOCABULARY-AC1): a 4th "top"
// tier joins fast/balanced/deep in modelcatalog.CanonicalTiers, with no
// stray/duplicate tokens. RED today: CanonicalTiers is still the 3-entry
// {fast,balanced,deep} slice.
func TestC499_003_CanonicalTiersIncludesTop(t *testing.T) {
	out, code := runGoTest(t, "TestCanonicalTiersIncludesTop", modelcatalogPkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("CanonicalTiers is red (exit=%d) — \"top\" tier not yet added\n%s", code, out)
	}
}

// TestC499_004_HighAliasesToDeepNotTop (TIER VOCABULARY-AC2): "high" is
// accepted as an input alias of the internal "deep" tier via
// translateV1TierKey, while "top" stays its OWN distinct tier (negative:
// guards against collapsing the two). RED today: translateV1TierKey has no
// "high" case (falls through to pass-through, returning "high" unchanged).
func TestC499_004_HighAliasesToDeepNotTop(t *testing.T) {
	out, code := runGoTest(t, "TestTranslateV1TierKey_HighAliasesToDeep|TestTranslateV1TierKey_TopIsNotConflatedWithHigh|TestTranslateV1TierKey_PreexistingMappingsUnchanged", bridgePkg)
	requireTestsRan(t, out, 3)
	if code != 0 {
		t.Errorf("translateV1TierKey \"high\" alias is red (exit=%d)\n%s", code, out)
	}
}
