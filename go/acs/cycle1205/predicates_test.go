//go:build acs

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

const regressionTestName = "TestDefaultRules_DoesNotBindOnRootCauseProse"

const regressionTestRelPath = "go/internal/inboxbatch/rules_rootcause_regression_test.go"

var wantRuleTypes = []string{"inboxbatch.campaignRule", "inboxbatch.fileAreaRule"}

func TestC1205_001_DefaultRulesStaysBoundedStructuralRules(t *testing.T) {
	rules := inboxbatch.DefaultRules()
	if got := len(rules); got != len(wantRuleTypes) {
		t.Fatalf("DefaultRules() returned %d rules, want %d (%v) — cycle-1204 audit D1/D2 rejected adding a "+
			"free-form-prose rule here; another default-on rule needs a hubAreaMaxItems-style ceiling or a "+
			"minAreaDepth-style floor and a non-tautological eval against real .evolve/inbox data",
			got, len(wantRuleTypes), wantRuleTypes)
	}
	for i, r := range rules {
		if got := ruleTypeName(r); got != wantRuleTypes[i] {
			t.Errorf("DefaultRules()[%d] is %s, want %s", i, got, wantRuleTypes[i])
		}
	}

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

func TestC1205_005_RegressionTestFailsOnTheRejectedDesign(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")

	if _, err := os.Stat(filepath.Join(root, regressionTestRelPath)); err != nil {
		t.Fatalf("regression test file missing at %s: %v — it must live in package inboxbatch for white-box "+
			"access to campaignRule/fileAreaRule", regressionTestRelPath, err)
	}

	overlay := writeProseRuleOverlay(t, goDir)

	const controlTest = "TestFileArea_RequiresMinimumDepthToBeDiscriminative"
	stdout, stderr, code := goTest(t, goDir, nil, "-count=1", "-overlay="+overlay, "-run", "^"+controlTest+"$", "./internal/inboxbatch/")
	if code != 0 {
		t.Fatalf("control test %s failed under the mutant overlay (exit %d) — the mutant must compile for this "+
			"predicate to be meaningful\nstdout:\n%s\nstderr:\n%s", controlTest, code, tail(stdout), tail(stderr))
	}

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

func allEdges(rules []inboxbatch.Rule, items []inboxbatch.Item) []inboxbatch.Edge {
	var edges []inboxbatch.Edge
	for _, r := range rules {
		edges = append(edges, r.Edges(items)...)
	}
	return edges
}

func ruleTypeName(r inboxbatch.Rule) string {
	return strings.TrimPrefix(fmt.Sprintf("%T", r), "*")
}

func writeProseRuleOverlay(t *testing.T, goDir string) string {
	t.Helper()
	rulesPath := filepath.Join(goDir, "internal", "inboxbatch", "rules.go")
	raw, err := os.ReadFile(rulesPath)
	if err != nil {
		t.Fatalf("read %s: %v", rulesPath, err)
	}
	const target = "return []Rule{campaignRule{}, fileAreaRule{}}"
	src := string(raw)
	if !strings.Contains(src, target) {
		t.Fatalf("rules.go no longer contains the DefaultRules() body %q — this predicate's mutant is stale; "+
			"re-derive it from the current DefaultRules()", target)
	}
	mutant := strings.Replace(src, target,
		"return []Rule{campaignRule{}, fileAreaRule{}, proseRule{}}", 1) + proseRuleSrc

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

func tail(s string) string {
	const max = 4000
	if len(s) <= max {
		return s
	}
	return "…(truncated)…\n" + s[len(s)-max:]
}
