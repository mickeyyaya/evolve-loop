//go:build acs

package cycle503

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const modelqueryPkg = "github.com/mickeyyaya/evolve-loop/go/internal/modelquery"

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

func TestC503_001_FamilyFilterEnforcesFamilyPurity(t *testing.T) {
	out, code := runGoTest(t,
		"TestFilterByFamily_GeminiOnlyDropsClaudeAndGPT|TestFilterByFamily_NoGeminiYieldsEmptyNotPassthrough",
		modelqueryPkg)
	requireTestsRan(t, out, 2)
	if code != 0 {
		t.Errorf("family filter is red (exit=%d) — modelquery.FilterByFamily is missing or does not enforce family purity (cross-family ids not dropped)\n%s", code, out)
	}
}

func TestC503_002_FamilyOfClassifiesFamilies(t *testing.T) {
	out, code := runGoTest(t,
		"TestFamilyOf_ClassifiesKnownFamilies|TestFamilyOf_UnknownReturnsEmpty",
		modelqueryPkg)
	requireTestsRan(t, out, 2)
	if code != 0 {
		t.Errorf("family classifier is red (exit=%d) — modelquery.FamilyOf is missing or misclassifies a family\n%s", code, out)
	}
}

func TestC503_003_FamilyFilterEdgeAndComposition(t *testing.T) {
	out, code := runGoTest(t,
		"TestFilterByFamily_EmptyAllowedIsPassthrough|TestFilterByFamily_MultiFamilyKeepsOrderDropsUnknown|TestFilterByFamily_ComposesWithNewestWins",
		modelqueryPkg)
	requireTestsRan(t, out, 3)
	if code != 0 {
		t.Errorf("family filter edge/composition is red (exit=%d) — passthrough, order-preservation, or newest-wins composition is broken\n%s", code, out)
	}
}
