//go:build acs

// Package cycle503 materialises the cycle-503 acceptance criteria.
//
// TRIAGE COMMITTED ONE ## top_n TASK this cycle:
//
//	latest-model-preference: Model-selection policy for ALL CLIs
//	(inbox 2026-07-02T02-00-00Z-latest-model-preference.json, D1-D9,
//	4-tier top-default vocabulary — the sole fleet-scoped commit per
//	triage's rationale; every other backlog item deferred).
//
// SCOPE DECISION (documented, not silent — see test-report.md "Scope
// Decision"): the inbox item is a 9-design-point campaign (D1-D9) far larger
// than one TDD→Builder cycle. Cycle 499 already shipped the first two
// foundational, zero-I/O points (D2 newest-wins comparator + the 4-tier
// vocabulary with the high→deep alias); those are GREEN and out of scope here.
// This cycle advances to the NEXT self-contained, deterministic, zero-I/O
// point that is independently valuable and unblocks the rest:
//
//	D7 (FAMILY CONSTRAINT — family-pure candidate sets) → C503_001..003
//
// D1, D3, D4, D5, D6, D8, D9 and the docs deliverable remain OUT OF SCOPE this
// cycle (they require live-CLI/bridge integration or policy control-plane edits
// this slice does not touch) and stay queued to carryoverTodos — see
// test-report.md.
//
// Predicate strategy (mirrors cycle499/cycle488): behavioral predicates drive
// the new code through its in-package RED tests via subprocess `go test`,
// asserting a non-degenerate pass (requireTestsRan closes the cycle-85
// "no tests matched" trap) — never a source grep. The in-package tests
// (internal/modelquery/family_test.go) were authored by the TDD engineer;
// the Builder implements internal/modelquery/family.go only.
package cycle503

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const modelqueryPkg = "github.com/mickeyyaya/evolve-loop/go/internal/modelquery"

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

// TestC503_001_FamilyFilterEnforcesFamilyPurity (D7-AC1): the family filter
// keeps only ids in an allowed family AND removes cross-family ids. The positive
// case is the exact agy live-evidence scenario (a Gemini-only CLI's mixed list
// must shed its GPT-OSS + Claude ids); the paired negative case (a list with no
// gemini model → empty result, not passthrough) is the anti-no-op guard that a
// stub returning its input verbatim cannot satisfy. Drives
// modelquery.FilterByFamily through its in-package tests. RED today:
// FilterByFamily does not exist (modelquery build failure).
func TestC503_001_FamilyFilterEnforcesFamilyPurity(t *testing.T) {
	out, code := runGoTest(t,
		"TestFilterByFamily_GeminiOnlyDropsClaudeAndGPT|TestFilterByFamily_NoGeminiYieldsEmptyNotPassthrough",
		modelqueryPkg)
	requireTestsRan(t, out, 2)
	if code != 0 {
		t.Errorf("family filter is red (exit=%d) — modelquery.FilterByFamily is missing or does not enforce family purity (cross-family ids not dropped)\n%s", code, out)
	}
}

// TestC503_002_FamilyOfClassifiesFamilies (D7-AC2): the family classifier maps
// raw model ids into claude/gpt/gemini (case- and display-name-insensitive) and
// returns "" for an unknown family. This is the vocabulary the filter enforces.
// Drives modelquery.FamilyOf through its in-package tests. RED today: FamilyOf
// does not exist (modelquery build failure).
func TestC503_002_FamilyOfClassifiesFamilies(t *testing.T) {
	out, code := runGoTest(t,
		"TestFamilyOf_ClassifiesKnownFamilies|TestFamilyOf_UnknownReturnsEmpty",
		modelqueryPkg)
	requireTestsRan(t, out, 2)
	if code != 0 {
		t.Errorf("family classifier is red (exit=%d) — modelquery.FamilyOf is missing or misclassifies a family\n%s", code, out)
	}
}

// TestC503_003_FamilyFilterEdgeAndComposition (D7-AC3, adversarial diversity:
// empty-allowed passthrough + multi-family order-preservation + composition with
// the D2 newest-wins comparator are three DISTINCT behaviors, not one restated).
// The composition case encodes the D2+D7+D8 acceptance scenario — family-filter
// then newest-wins auto-promotes agy to the frontier Gemini 3.5 Pro when the
// picker lists it. RED today: FilterByFamily does not exist (build failure).
func TestC503_003_FamilyFilterEdgeAndComposition(t *testing.T) {
	out, code := runGoTest(t,
		"TestFilterByFamily_EmptyAllowedIsPassthrough|TestFilterByFamily_MultiFamilyKeepsOrderDropsUnknown|TestFilterByFamily_ComposesWithNewestWins",
		modelqueryPkg)
	requireTestsRan(t, out, 3)
	if code != 0 {
		t.Errorf("family filter edge/composition is red (exit=%d) — passthrough, order-preservation, or newest-wins composition is broken\n%s", code, out)
	}
}
