//go:build acs

package cycle1138

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const newTestName = "TestVerify_WarnOrFailSentinel_StillRequiresSections"

const testFileRel = "go/internal/deliverable/deliverable_test.go"

func writeReport(t *testing.T, body string) string {
	t.Helper()
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "build-report.md"), []byte(body), 0o644); err != nil {
		t.Fatalf("write build-report.md: %v", err)
	}
	return ws
}

func hasCode(res deliverable.Result, code string) bool {
	for _, v := range res.Violations {
		if v.Code == code {
			return true
		}
	}
	return false
}

func sentinelOnly(verdict string) string {
	return `<!-- evolve-verdict: {"phase":"build","verdict":"` + verdict + `"} -->` + "\n"
}

func TestC1138_001_SentinelFailOrWarnStillRequiresSections(t *testing.T) {
	for _, verdict := range []string{"FAIL", "WARN"} {
		ws := writeReport(t, sentinelOnly(verdict))
		res, err := deliverable.Verify("build", phasecontract.Roots{Workspace: ws})
		if err != nil {
			t.Fatalf("verdict %s: Verify returned infra error: %v", verdict, err)
		}
		if res.OK {
			t.Errorf("verdict %s: sentinel-only report was accepted as well-formed — a %s verdict must not waive the report body", verdict, verdict)
			continue
		}
		if !hasCode(res, deliverable.CodeMissingSection) {
			t.Errorf("verdict %s: want %s violation for a report with no required sections; got %+v",
				verdict, deliverable.CodeMissingSection, res.Violations)
		}
	}
}

func TestC1138_002_SectionsPresentUnderFailSentinelIsClean(t *testing.T) {
	body := sentinelOnly("FAIL") + "\n# Build Report\n\n## Changes\n- foo.go\n"
	ws := writeReport(t, body)
	res, err := deliverable.Verify("build", phasecontract.Roots{Workspace: ws})
	if err != nil {
		t.Fatalf("Verify returned infra error: %v", err)
	}
	if hasCode(res, deliverable.CodeMissingSection) {
		t.Errorf("FAIL report WITH its required section must not be flagged %s; got %+v",
			deliverable.CodeMissingSection, res.Violations)
	}
}

func TestC1138_003_RegressionUnitTestExistsAndPasses(t *testing.T) {
	root := acsassert.RepoRoot(t)
	if !acsassert.FileExists(t, filepath.Join(root, testFileRel)) {
		t.Fatalf("%s is missing", testFileRel)
	}
	stdout, stderr, code, _ := acsassert.SubprocessOutput("go", "test",
		"-C", filepath.Join(root, "go"), "-count=1", "-v",
		"-run", "^"+newTestName+"$", "./internal/deliverable/")
	out := stdout + stderr
	if code != 0 {
		t.Errorf("go test -run %s exited %d, want 0\n%s", newTestName, code, tail(out, 40))
		return
	}
	if !strings.Contains(out, "--- PASS: "+newTestName) {
		t.Errorf("no `--- PASS: %s` line in -v output — the test does not exist (go test -run matching nothing also exits 0)\n%s",
			newTestName, tail(out, 40))
	}
}

func TestC1138_004_DeliverablePackageSuiteGreen(t *testing.T) {
	root := acsassert.RepoRoot(t)
	stdout, stderr, code, _ := acsassert.SubprocessOutput("go", "test",
		"-C", filepath.Join(root, "go"), "-count=1", "./internal/deliverable/")
	if code != 0 {
		t.Errorf("internal/deliverable package suite exited %d, want 0 (no regression)\n%s",
			code, tail(stdout+stderr, 40))
	}
}

func tail(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
