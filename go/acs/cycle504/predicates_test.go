//go:build acs

// Package cycle504 materialises the cycle-504 acceptance criteria.
//
// TRIAGE COMMITTED ONE ## top_n TASK this cycle:
//
//	latest-model-preference: model-selection policy for all CLIs (conformance)
//	(inbox 2026-07-02T02-00-00Z-latest-model-preference.json, D1-D9, 4-tier
//	top-default vocabulary — the sole fleet-scoped commit per triage's
//	rationale; every other backlog/inbox item, including this cycle's own
//	scout-report.md fleet-width tasks, deferred).
//
// SCOPE DECISION (documented, not silent — see test-report.md "Scope
// Decision"): scout-report.md for cycle 504 researched a DIFFERENT inbox item
// (triage-supply-disjoint-topn-for-fleet-width); triage instead committed
// latest-model-preference to top_n (assigned fleet scope), so this cycle's
// tests are authored directly against the inbox item + triage-report.md
// rationale, not scout-report.md. The inbox item is a 9-design-point campaign
// (D1-D9) far larger than one TDD->Builder cycle. Prior cycles already shipped
// two foundational, zero-I/O slices: cycle499 (D2 newest-wins comparator + the
// 4-tier vocabulary with the high->deep alias) and cycle503 (D7's standalone
// FamilyOf/FilterByFamily helpers, built but NOT wired into production). This
// cycle advances to the natural next increment: WIRE D7 into production —
//
//	D7 PRODUCTION WIRING (policy.json catalog.allowed_families -> the live
//	refresh pipeline, so a CLI's candidate ids are family-filtered BEFORE they
//	reach the Classifier) -> C504_001..003
//
// D1, D2(wiring), D3, D4, D5, D6, D8, D9 and the docs deliverable remain OUT OF
// SCOPE this cycle (they require live-CLI/bridge integration this slice does
// not touch) and stay queued to carryoverTodos — see test-report.md.
//
// Predicate strategy (mirrors cycle499/cycle503): behavioral predicates drive
// the new code through its in-package RED tests via subprocess `go test`,
// asserting a non-degenerate pass (requireTestsRan closes the cycle-85 "no
// tests matched" trap) — never a source grep. The in-package tests
// (internal/policy/policy_test.go, internal/modelquery/query_test.go) were
// authored by the TDD engineer; the Builder implements
// internal/policy/policy.go + internal/modelquery/query.go only.
package cycle504

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	policyPkg     = "github.com/mickeyyaya/evolve-loop/go/internal/policy"
	modelqueryPkg = "github.com/mickeyyaya/evolve-loop/go/internal/modelquery"
)

// runGoTest runs `go test` on pkgs and returns the combined output + exit code.
// Behavioral predicates invoke the system under test through its own in-package
// tests — no source-grep gaming.
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

// requireTestsRan closes the degenerate-predicate trap: `go test -run X` with no
// matching test exits 0 with "no tests to run", which would green a predicate on
// unwritten (or renamed) work.
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

// TestC504_001_CatalogPolicySchemaRoundTrips (D7-AC1): `.evolve/policy.json`
// `catalog.allowed_families` (per-CLI family allow-list) round-trips through
// Load() -> Policy.CatalogConfig(), and an absent/unconfigured block resolves
// to nil (no constraint) — the byte-identical-default regression pin every
// existing cycle without this key depends on. Drives policy.CatalogConfig and
// policy.Load through their in-package tests. RED today: CatalogPolicy has no
// AllowedFamilies field (policy package build failure).
func TestC504_001_CatalogPolicySchemaRoundTrips(t *testing.T) {
	out, code := runGoTest(t,
		"TestCatalogConfig_AllowedFamilies|TestLoad_ParsesCatalogAllowedFamilies",
		policyPkg)
	requireTestsRan(t, out, 2)
	if code != 0 {
		t.Errorf("catalog.allowed_families schema is red (exit=%d) — CatalogPolicy.AllowedFamilies is missing, not resolved by CatalogConfig(), or not parsed by Load()\n%s", code, out)
	}
}

// TestC504_002_RefreshFiltersByFamilyBeforeClassify (D7-AC2, the live-evidence
// acceptance scenario): RefreshDeps.AllowedFamilies must filter a CLI's
// live-queried id list down to its allowed families BEFORE the ids reach the
// Classifier — reproducing the exact agy incident (a mixed Claude/Gemini/GPT
// list must never let a cross-family id reach classification, the root cause
// of the observed Sonnet-4.6->GPT-OSS-120B flap). Drives modelquery.Refresh
// through its in-package test. RED today: RefreshDeps has no AllowedFamilies
// field (modelquery package build failure).
func TestC504_002_RefreshFiltersByFamilyBeforeClassify(t *testing.T) {
	out, code := runGoTest(t,
		"TestRefreshFamilyFilterAppliedBeforeClassify",
		modelqueryPkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("family-filter production wiring is red (exit=%d) — RefreshDeps.AllowedFamilies is missing or not applied before Classify\n%s", code, out)
	}
}

// TestC504_003_RefreshNoFamiliesIsPassthroughRegression (D7-AC3, the anti-
// no-op guard): a CLI with no AllowedFamilies entry — every cycle before this
// feature, and every CLI that opts out — must be byte-identical to today:
// every listed id reaches the classifier unfiltered. Paired with C504_002 as
// the negative/positive pair the adversarial-testing skill requires (a
// wiring that always filters, or never filters, fails one of the two). Drives
// modelquery.Refresh through its in-package test. RED today: RefreshDeps has
// no AllowedFamilies field (modelquery package build failure).
func TestC504_003_RefreshNoFamiliesIsPassthroughRegression(t *testing.T) {
	out, code := runGoTest(t,
		"TestRefreshNoAllowedFamiliesIsPassthrough",
		modelqueryPkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("no-constraint passthrough regression is red (exit=%d) — an unconfigured CLI's ids must reach Classify unfiltered\n%s", code, out)
	}
}
