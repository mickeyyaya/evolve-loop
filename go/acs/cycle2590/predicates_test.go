//go:build acs

package cycle2590

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/stelint"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	researchDoc  = "docs/research/ci-wall-variance-2026-11.md"
	refreshTest  = "TestPRRefresh_TheRealGenerators"
	wallBudget   = "20s"
	notMeasured  = "not measured"
	shareOfTotal = `\d+(?:\.\d+)?\s?%`
)

var percent = regexp.MustCompile(shareOfTotal)

func refreshLines(t *testing.T) []string {
	t.Helper()
	path := filepath.Join(acsassert.RepoRoot(t), researchDoc)
	if !acsassert.FileExists(t, path) {
		t.Fatalf("%s is missing", researchDoc)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	for _, line := range strings.Split(string(body), "\n") {
		if strings.Contains(line, "TestPRRefresh") {
			lines = append(lines, line)
		}
	}
	if len(lines) == 0 {
		t.Fatalf("%s no longer mentions TestPRRefresh", researchDoc)
	}
	return lines
}

// acs-predicate: config-check
func TestC2590_001_ResearchDocNamesTheCheckoutAndGeneratorShares(t *testing.T) {
	lines := refreshLines(t)
	named := false
	for _, line := range lines {
		if strings.Contains(strings.ToLower(line), notMeasured) {
			t.Errorf("%s still says the PRRefresh split is %q: %s", researchDoc, notMeasured, line)
		}
		lower := strings.ToLower(line)
		if strings.Contains(lower, "checkout") && strings.Contains(lower, "generator") && len(percent.FindAllString(line, -1)) >= 2 {
			named = true
		}
	}
	if !named {
		t.Errorf("no TestPRRefresh line of %s names the share of the checkout and the share of the generators as percentages", researchDoc)
	}
}

func TestC2590_002_RefreshTestPassesInsideTheWallBudget(t *testing.T) {
	moduleDir := acsassert.GoModuleDir(t)
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "-C", moduleDir, "test", "-count=1", "-tags", "integration",
		"-timeout", wallBudget, "-v", "-run", "^"+refreshTest, "./cmd/evolve/")
	if err != nil || code != 0 {
		t.Fatalf("the refresh test did not pass inside -timeout %s (exit %d, err %v)\nstdout:\n%s\nstderr:\n%s",
			wallBudget, code, err, acsassert.OutputTail(stdout, 20), acsassert.OutputTail(stderr, 20))
	}
	if !strings.Contains(stdout, "--- PASS: "+refreshTest) {
		t.Errorf("no %s test reported PASS:\n%s", refreshTest, acsassert.OutputTail(stdout, 20))
	}
	if strings.Contains(stdout, "--- SKIP") {
		t.Errorf("the refresh test skipped instead of running:\n%s", acsassert.OutputTail(stdout, 20))
	}
}

func TestC2590_003_ResearchDocStaysInSTE(t *testing.T) {
	root := acsassert.RepoRoot(t)
	words, err := stelint.LoadStandard(root)
	if err != nil {
		t.Fatal(err)
	}
	findings, err := stelint.LintFile(filepath.Join(root, researchDoc), stelint.Options{Words: words})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range findings {
		if !f.IsSkipNotice() {
			t.Errorf("%s line %d %s: %s", researchDoc, f.Line, f.Rule, f.Excerpt)
		}
	}
}
