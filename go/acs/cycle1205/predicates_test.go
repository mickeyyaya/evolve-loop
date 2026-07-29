//go:build acs

// Package cycle1205 materialises the cycle-1205 acceptance criteria for this
// lane's single fleet-scoped task:
//
//   - rootcause-rule-regression-test → pin DefaultRules() against reintroducing
//     the cycle-1204 audit-REJECTED root-cause binding design.
//
// Why the task is a guard-rail, not a feature test. Cycle-1204 proposed a
// `rootCauseRule` that bound inbox items by exact string equality on a
// free-form prose field and placed it in DefaultRules() (default-on). The audit
// rejected it: D1 — measured against the 67 live .evolve/inbox items, all 20
// non-empty root_cause values were unique prose (median 317 bytes), so the rule
// was a NO-OP on real data; D2 — it carried neither of the discriminative guards
// its siblings have (hubAreaMaxItems ceiling, minAreaDepth floor), so a future
// normalising producer would collapse the campaign-less backlog into one
// over-fused cluster. The production code never landed, so there is no feature
// to regression-test; the regression worth writing is defensive — DefaultRules()
// must stay the three bounded structural signals, and none of them may bind
// items on a shared free-form prose field.
//
// Predicate strategy — every predicate below EXERCISES the system under test
// (calls DefaultRules()/Rule.Edges, or runs the package's tests as a
// subprocess); none is a source-grep of production text (the cycle-85
// degenerate-predicate ban).
//
//   - 001 (AC2) calls DefaultRules() and asserts the rule set IS exactly the
//     three structural rules AND produces zero edges for items whose only
//     commonality is an identical free-form prose field.
//   - 002 (AC4, negative) case/whitespace-varied prose must also bind nothing —
//     the "a normaliser lands upstream" failure mode of D2.
//   - 003 (AC5, edge) empty prose on every item — zero edges.
//   - 004 (AC1) runs the real package test suite and requires the NAMED
//     regression test to have actually run and PASSED (a `-run` pattern that
//     matches nothing also exits 0 — the "--- PASS:" line is what rules that
//     no-op out).
//   - 005 (AC3) the CRUX anti-no-op predicate: it MUTATES rules.go in memory
//     (go build -overlay) to reintroduce the rejected 4th prose-binding rule and
//     requires the new regression test to FAIL on that mutant. A guard that
//     cannot fail on the exact design it exists to reject is decoration. A
//     control run under the same overlay pins that the mutant still compiles,
//     so the FAIL is attributable to the guard and not to a broken build.
//
// Predicates 001-003 pin the CURRENT, audited state of production code and are
// green before Builder writes anything (recorded as pre-existing GREEN in
// test-report.md); 004 and 005 are RED until the regression test file lands at
// go/internal/inboxbatch/rules_rootcause_regression_test.go.
package cycle1205

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// regressionTestName is the test the scout/triage contract commits to; the
// overlay-mutation predicate and the suite predicate both key off it.
const regressionTestName = "TestDefaultRules_DoesNotBindOnRootCauseProse"

// regressionTestRelPath is Builder's deliverable — the package placement is
// load-bearing (white-box access to campaignRule/fileAreaRule/depRule).
const regressionTestRelPath = "go/internal/inboxbatch/rules_rootcause_regression_test.go"

// wantRuleTypes is the audited DefaultRules() composition: campaign, file-area
// and dependency edges — every one bounded (campaign is an explicit operator
// declaration; fileArea has both a hub ceiling and a depth floor; deps are hard
// structural references).
var wantRuleTypes = []string{"inboxbatch.campaignRule", "inboxbatch.fileAreaRule", "inboxbatch.depRule"}

// TestC1205_001_DefaultRulesStaysThreeBoundedStructuralRules is AC2: the rule
// set is exactly the three structural signals, and a shared free-form prose
// field binds nothing through any of them.
func TestC1205_001_DefaultRulesStaysThreeBoundedStructuralRules(t *testing.T) {
	rules := inboxbatch.DefaultRules()
	if got := len(rules); got != len(wantRuleTypes) {
		t.Fatalf("DefaultRules() returned %d rules, want %d (%v) — cycle-1204 audit D1/D2 rejected adding a "+
			"free-form-prose rule here; a 4th default-on rule needs a hubAreaMaxItems-style ceiling or a "+
			"minAreaDepth-style floor and a non-tautological eval against real .evolve/inbox data",
			got, len(wantRuleTypes), wantRuleTypes)
	}
	for i, r := range rules {
		if got := ruleTypeName(r); got != wantRuleTypes[i] {
			t.Errorf("DefaultRules()[%d] is %s, want %s", i, got, wantRuleTypes[i])
		}
	}

	// Behavioural half: items whose ONLY commonality is identical free-form
	// prose (Title is the closest live analogue of the proposed root_cause
	// field — unstructured, author-written, not an enum). No Campaign, no
	// Files, no Deps: the three structural rules have nothing to bind on.
	const prose = "verdict incoherence under contention: the tier reported RED because SubstantiveError was never populated"
	items := []inboxbatch.Item{
		{ID: "a-item", Title: prose},
		{ID: "b-item", Title: prose},
		{ID: "c-item", Title: prose},
	}
	if edges := allEdges(rules, items); len(edges) != 0 {
		t.Errorf("DefaultRules() bound %d edge(s) on items sharing only a free-form prose field: %+v — "+
			"exact-match prose binding is the cycle-1204 audit-rejected design (D1 no-op on real data, "+
			"D2 unbounded fusion once a normaliser lands)", len(edges), edges)
	}
}

// TestC1205_002_ProseVariantsBindNothing is AC4 (negative axis): the guard must
// hold for prose that differs only in case and whitespace. This is the shape a
// future normalising producer for root_cause-like fields would emit, i.e. the
// exact input that would have tripped D2's over-fusion.
func TestC1205_002_ProseVariantsBindNothing(t *testing.T) {
	rules := inboxbatch.DefaultRules()
	items := []inboxbatch.Item{
		{ID: "a-item", Title: "Quota Regex Drift"},
		{ID: "b-item", Title: "quota regex drift"},
		{ID: "c-item", Title: "  QUOTA   REGEX drift  "},
		{ID: "d-item", Title: "\tquota\tregex\tdrift\n"},
	}
	if edges := allEdges(rules, items); len(edges) != 0 {
		t.Errorf("DefaultRules() bound %d edge(s) on case/whitespace-varied prose: %+v — "+
			"no default rule may derive grouping from a free-form prose field, normalised or not",
			len(edges), edges)
	}
}

// TestC1205_003_EmptyProseBindsNothing is AC5 (edge axis): the degenerate case
// where every item's prose field is empty or whitespace-only must yield zero
// edges rather than fusing the whole set on "" (the classic empty-key
// map-bucket collapse).
func TestC1205_003_EmptyProseBindsNothing(t *testing.T) {
	rules := inboxbatch.DefaultRules()
	items := []inboxbatch.Item{
		{ID: "a-item", Title: ""},
		{ID: "b-item", Title: ""},
		{ID: "c-item", Title: "   "},
		{ID: "d-item", Title: "\n\t"},
	}
	if edges := allEdges(rules, items); len(edges) != 0 {
		t.Errorf("DefaultRules() bound %d edge(s) on items with empty/whitespace prose: %+v — "+
			"an empty key must never become a grouping bucket", len(edges), edges)
	}
}

// TestC1205_004_RegressionTestLandsGreenInTheNormalSuite is AC1: the inboxbatch
// package suite passes WITH the new regression test, and the named test really
// ran. Requiring the "--- PASS:" line is the anti-no-op: `go test -run <absent
// pattern>` also exits 0, so exit code alone cannot distinguish "passed" from
// "never existed".
func TestC1205_004_RegressionTestLandsGreenInTheNormalSuite(t *testing.T) {
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")

	stdout, stderr, code := goTest(t, goDir, nil, "-count=1", "./internal/inboxbatch/...")
	if code != 0 {
		t.Fatalf("go test ./internal/inboxbatch/... exited %d, want 0\nstdout:\n%s\nstderr:\n%s", code, tail(stdout), tail(stderr))
	}

	stdout, stderr, code = goTest(t, goDir, nil, "-count=1", "-v", "-run", "^"+regressionTestName+"$", "./internal/inboxbatch/")
	if code != 0 {
		t.Fatalf("go test -run %s exited %d, want 0\nstdout:\n%s\nstderr:\n%s", regressionTestName, code, tail(stdout), tail(stderr))
	}
	if !strings.Contains(stdout, "--- PASS: "+regressionTestName) {
		t.Errorf("%s never ran: no %q line in `go test -v -run ^%s$ ./internal/inboxbatch/` output — "+
			"an exit code of 0 from a -run pattern that matches nothing is not evidence the regression test exists\nstdout:\n%s",
			regressionTestName, "--- PASS: "+regressionTestName, regressionTestName, tail(stdout))
	}
}

// TestC1205_005_RegressionTestFailsOnTheRejectedDesign is AC3 and the crux
// anti-no-op predicate: the guard must be LOAD-BEARING inside
// go/internal/inboxbatch. rules.go is mutated in memory (`go test -overlay`,
// nothing written into the tree) to reintroduce the cycle-1204 rejected design —
// a 4th default-on rule binding items by exact match on a free-form prose field
// — and the regression test must FAIL on it. The control run under the same
// overlay proves the mutant compiles, so a FAIL is attributable to the guard
// rather than to a broken build.
func TestC1205_005_RegressionTestFailsOnTheRejectedDesign(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")

	if _, err := os.Stat(filepath.Join(root, regressionTestRelPath)); err != nil {
		t.Fatalf("regression test file missing at %s: %v — it must live in package inboxbatch for white-box "+
			"access to campaignRule/fileAreaRule/depRule", regressionTestRelPath, err)
	}

	overlay := writeProseRuleOverlay(t, goDir)

	// Control: a rule-set-independent test in the same package must still pass
	// under the overlay. If this fails, the mutant did not compile and the
	// guard run below would fail for the wrong reason.
	const controlTest = "TestFileArea_RequiresMinimumDepthToBeDiscriminative"
	stdout, stderr, code := goTest(t, goDir, nil, "-count=1", "-overlay="+overlay, "-run", "^"+controlTest+"$", "./internal/inboxbatch/")
	if code != 0 {
		t.Fatalf("control test %s failed under the mutant overlay (exit %d) — the mutant must compile for this "+
			"predicate to be meaningful\nstdout:\n%s\nstderr:\n%s", controlTest, code, tail(stdout), tail(stderr))
	}

	// The guard run: the regression test MUST reject the mutant.
	stdout, stderr, code = goTest(t, goDir, nil, "-count=1", "-v", "-overlay="+overlay, "-run", "^"+regressionTestName+"$", "./internal/inboxbatch/")
	if code == 0 {
		t.Fatalf("%s PASSED against a DefaultRules() that reintroduces the rejected free-form-prose rule — "+
			"the guard is decoration: it must assert both the rule-set composition and zero prose binding\nstdout:\n%s\nstderr:\n%s",
			regressionTestName, tail(stdout), tail(stderr))
	}
	if !strings.Contains(stdout, "--- FAIL: "+regressionTestName) {
		t.Errorf("mutant run exited %d but %q is absent — the non-zero exit is not attributable to the regression test\nstdout:\n%s\nstderr:\n%s",
			code, "--- FAIL: "+regressionTestName, tail(stdout), tail(stderr))
	}
}

// --- helpers -------------------------------------------------------------

// allEdges unions every rule's edges, mirroring what Classify consumes.
func allEdges(rules []inboxbatch.Rule, items []inboxbatch.Item) []inboxbatch.Edge {
	var edges []inboxbatch.Edge
	for _, r := range rules {
		edges = append(edges, r.Edges(items)...)
	}
	return edges
}

// ruleTypeName renders a Rule's concrete type as package.Type. The rule types
// are unexported, so %T is the only handle a predicate outside the package has.
func ruleTypeName(r inboxbatch.Rule) string {
	return strings.TrimPrefix(fmt.Sprintf("%T", r), "*")
}

// writeProseRuleOverlay builds an in-memory mutant of rules.go that adds the
// rejected prose-binding rule to DefaultRules(), and returns the path of the
// `go build -overlay` JSON describing the substitution. Nothing is written into
// the repository tree (t.TempDir only) — worktree isolation is preserved.
func writeProseRuleOverlay(t *testing.T, goDir string) string {
	t.Helper()
	rulesPath := filepath.Join(goDir, "internal", "inboxbatch", "rules.go")
	raw, err := os.ReadFile(rulesPath)
	if err != nil {
		t.Fatalf("read %s: %v", rulesPath, err)
	}
	const target = "return []Rule{campaignRule{}, fileAreaRule{}, depRule{}}"
	src := string(raw)
	if !strings.Contains(src, target) {
		t.Fatalf("rules.go no longer contains the DefaultRules() body %q — this predicate's mutant is stale; "+
			"re-derive it from the current DefaultRules()", target)
	}
	mutant := strings.Replace(src, target,
		"return []Rule{campaignRule{}, fileAreaRule{}, depRule{}, proseRule{}}", 1) + proseRuleSrc

	dir := t.TempDir()
	mutantPath := filepath.Join(dir, "rules_mutant.go")
	if err := os.WriteFile(mutantPath, []byte(mutant), 0o644); err != nil {
		t.Fatalf("write mutant: %v", err)
	}
	payload, err := json.Marshal(map[string]map[string]string{"Replace": {rulesPath: mutantPath}})
	if err != nil {
		t.Fatalf("marshal overlay: %v", err)
	}
	overlayPath := filepath.Join(dir, "overlay.json")
	if err := os.WriteFile(overlayPath, payload, 0o644); err != nil {
		t.Fatalf("write overlay: %v", err)
	}
	return overlayPath
}

// proseRuleSrc is the cycle-1204 audit-REJECTED design, reconstructed ONLY as an
// in-memory mutant for predicate 005. It binds items by exact match on a
// free-form prose field with no ceiling and no floor — D1 (no-op on real,
// all-unique prose) and D2 (total fusion once the values normalise) in ten
// lines. It must never exist in the tree.
const proseRuleSrc = `

type proseRule struct{}

func (proseRule) Edges(items []Item) []Edge {
	byProse := map[string][]int{}
	for i, it := range items {
		if p := strings.TrimSpace(it.Title); p != "" {
			byProse[p] = append(byProse[p], i)
		}
	}
	var edges []Edge
	for p, idx := range byProse {
		for k := 1; k < len(idx); k++ {
			edges = append(edges, Edge{A: idx[k-1], B: idx[k], Reason: "prose " + p})
		}
	}
	return edges
}
`

// goTest runs the go tool in dir and returns stdout, stderr and the exit code.
// acsassert.SubprocessOutput cannot set a working directory, and every
// invocation here must run inside the worktree's go module.
func goTest(t *testing.T, dir string, env []string, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command("go", append([]string{"test"}, args...)...)
	cmd.Dir = dir
	if env != nil {
		cmd.Env = append(os.Environ(), env...)
	}
	var sout, serr strings.Builder
	cmd.Stdout = &sout
	cmd.Stderr = &serr
	err := cmd.Run()
	code := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			code = exitErr.ExitCode()
		} else {
			t.Fatalf("go test %v in %s: %v\nstderr:\n%s", args, dir, err, serr.String())
		}
	}
	return sout.String(), serr.String(), code
}

// tail trims long tool output to the last few KB so a failure message stays
// readable in the audit transcript.
func tail(s string) string {
	const max = 4000
	if len(s) <= max {
		return s
	}
	return "…(truncated)…\n" + s[len(s)-max:]
}
