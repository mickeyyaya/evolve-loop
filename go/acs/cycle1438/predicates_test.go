//go:build acs

package cycle1438

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const targetPkg = "./internal/deliverable"

const regressionTestName = "TestClassifyBadVerdict_UnmatchedBacktickDoesNotMisclassify"

const guardTestName = "TestNoQuotedEchoRegression"

const testFileRel = "go/internal/deliverable/salvage_instrument_test.go"

const strayBacktickPreamble = "The auditor noted a stray ` tick in the transcript and moved on.\n\n"

func goTest(t *testing.T, root string, args ...string) (string, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	full := append([]string{"test", "-count=1"}, args...)
	cmd := exec.CommandContext(ctx, "go", full...)
	cmd.Dir = filepath.Join(root, "go")
	cmd.WaitDelay = 10 * time.Second
	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		code = cmd.ProcessState.ExitCode()
		if code < 0 {
			code = 1
		}
	}
	return string(out), code
}

func requireNamedTestPasses(t *testing.T, root, name string) {
	t.Helper()
	out, code := goTest(t, root, "-v", "-run", "^"+name+"$", targetPkg)
	if !strings.Contains(out, "--- PASS: "+name) {
		t.Errorf("RED: %s did not run-and-pass in %s (exit=%d).\n"+
			"A `--- PASS: %s` line is required — exit 0 with \"no tests to run\" means the test is absent.\n"+
			"---- go test output ----\n%s", name, targetPkg, code, name, out)
		return
	}
	if code != 0 {
		t.Errorf("RED: %s passed but the package run exited %d:\n%s", name, code, out)
	}
}

func TestC1438_001_RegressionTestRunsAndPasses(t *testing.T) {
	root := acsassert.RepoRoot(t)
	if !acsassert.FileExists(t, filepath.Join(root, testFileRel)) {
		t.Fatalf("RED: %s missing on disk", testFileRel)
	}
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", testFileRel); code != 0 {
		t.Errorf("RED: %s is untracked — an untracked test is dropped at ship and pins nothing", testFileRel)
	}
	requireNamedTestPasses(t, root, regressionTestName)
}

func TestC1438_002_SymbolReintroductionGuardRunsAndPasses(t *testing.T) {
	root := acsassert.RepoRoot(t)
	requireNamedTestPasses(t, root, guardTestName)
}

func backtickCases() []struct {
	name    string
	content string
} {
	return []struct {
		name    string
		content string
	}{
		{
			name:    "sentinel-trailing-comma",
			content: "# Audit Report\n\n<!-- evolve-verdict: {\"verdict\":\"FAIL\",} -->\n",
		},
		{
			name:    "fenced-json",
			content: "# Audit Report\n\n```json\n{\"verdict\":\"PASS\"}\n```\n",
		},
		{
			name:    "displaced-line",
			content: "# Audit Report\n\nThe verdict object is {\"verdict\":\"PASS\"} inline in prose.\n",
		},
		{
			name:    "none",
			content: "# Audit Report\n\nProse only. No verdict payload of any kind was emitted.\n",
		},
	}
}

func TestC1438_003_UnmatchedBacktickDoesNotPerturbClassification(t *testing.T) {
	for _, tc := range backtickCases() {
		control := deliverable.ClassifyBadVerdict(tc.content)
		poisoned := deliverable.ClassifyBadVerdict(strayBacktickPreamble + tc.content)
		if poisoned.Recoverable != control.Recoverable || poisoned.Pattern != control.Pattern {
			t.Errorf("RED[%s]: a stray unmatched backtick perturbed classification — "+
				"control{recoverable=%v pattern=%q} vs poisoned{recoverable=%v pattern=%q}",
				tc.name, control.Recoverable, control.Pattern, poisoned.Recoverable, poisoned.Pattern)
		}
		if poisoned.Reason == "" {
			t.Errorf("RED[%s]: classification carries an empty Reason — a silent classification is not observability", tc.name)
		}
	}

	for _, content := range []string{"", "`", strayBacktickPreamble} {
		if got := deliverable.ClassifyBadVerdict(content); got.Recoverable {
			t.Errorf("RED: content %q classified Recoverable=true pattern=%q — a verdict-free deliverable is not recoverable",
				content, got.Pattern)
		}
	}
}

func TestC1438_004_ClassifierContractUnchanged(t *testing.T) {
	want := map[string]struct {
		recoverable bool
		pattern     deliverable.SalvagePattern
	}{
		"sentinel-trailing-comma": {true, deliverable.SalvagePatternTrailingComma},
		"fenced-json":             {true, deliverable.SalvagePatternFencedJSON},
		"displaced-line":          {true, deliverable.SalvagePatternDisplaced},
		"none":                    {false, deliverable.SalvagePatternNone},
	}
	for _, tc := range backtickCases() {
		exp, ok := want[tc.name]
		if !ok {
			t.Fatalf("golden table is missing case %q", tc.name)
		}
		got := deliverable.ClassifyBadVerdict(tc.content)
		if got.Recoverable != exp.recoverable || got.Pattern != exp.pattern {
			t.Errorf("RED[%s]: classifier contract changed — want{recoverable=%v pattern=%q} got{recoverable=%v pattern=%q}. "+
				"This cycle must NOT edit salvage_instrument.go; the production fix is already correct.",
				tc.name, exp.recoverable, exp.pattern, got.Recoverable, got.Pattern)
		}
	}

	root := acsassert.RepoRoot(t)
	prod := filepath.Join(root, "go", "internal", "deliverable", "salvage_instrument.go")
	for _, sym := range []string{"isQuotedEcho", "insideStringLiteral"} {
		if !acsassert.FileNotContains(t, prod, sym) {
			t.Errorf("RED: %s reintroduced into salvage_instrument.go — that is the cycle-1406/1407 defect returning", sym)
		}
	}
}

func TestC1438_005_PackageSuiteGreen(t *testing.T) {
	root := acsassert.RepoRoot(t)
	out, code := goTest(t, root, targetPkg)
	if code != 0 {
		t.Errorf("RED: `go test %s` exited %d — regression in the touched package:\n%s", targetPkg, code, out)
	}
}
