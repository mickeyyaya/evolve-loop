//go:build acs

// Package cycle1138 materialises the cycle-1138 acceptance criteria for this
// fleet lane's single assigned item:
//
//   - warn-failed-verify-still-requires-real-report → regression-lock that a
//     bare FAIL/WARN verdict sentinel does NOT buy a phase out of the
//     deliverable contract's required-Sections check.
//
// The invariant. `deliverable.verifyMarkdown` (go/internal/deliverable/deliverable.go:134-158)
// runs the required-`Sections` loop unconditionally, ahead of and independent of
// the verdict-sentinel / failure-context block. Nothing between the two returns
// early. So "I am reporting FAIL, therefore I need not write the real report" is
// not, and must never become, a legal reading of the contract — the report body
// is owed on every verdict. Scout's read says the invariant already holds in
// code but is untested: no case pairs "sentinel declares FAIL/WARN" with
// "required sections absent". This cycle's deliverable is the executable lock.
//
// Predicate strategy — each predicate exercises the system under test, never a
// source-grep of production code (the cycle-85 degenerate-predicate ban):
//
//   - 001 CRUX / behavioural: calls the real `deliverable.Verify` on a
//     build-report whose entire body is a FAIL sentinel (then a WARN sentinel)
//     and asserts the missing_section violation still fires. This is the
//     invariant itself, asserted against production code — it red-fails the
//     instant anyone adds a short-circuit that skips section-checking on
//     FAIL/WARN, whether or not the Builder's unit test survives.
//   - 002 NEGATIVE CONTROL / anti-no-op: the same FAIL-sentinel report WITH the
//     required `## Changes` section present must NOT report missing_section.
//     Without this, 001 would also pass against a hypothetical broken verifier
//     that flags missing_section unconditionally; together they pin the check to
//     the actual section content, not to the verdict.
//   - 003 DELIVERABLE: runs the Builder's new unit test as a real subprocess and
//     requires (a) exit 0 AND (b) a `--- PASS: TestVerify_WarnOrFailSentinel_StillRequiresSections`
//     line in the -v output. The second half is load-bearing: `go test -run` on a
//     name that matches nothing exits 0, so exit-code-alone would green on a
//     no-op that never wrote the test.
//   - 004 REGRESSION: the whole `internal/deliverable` package suite stays green,
//     so the new test cannot be paid for by breaking a neighbour.
//
// Predicates 001/002 are expected pre-existing GREEN (they assert an invariant
// scout read as already-true); 003/004 are the RED-on-arrival ones. Both kinds
// are load-bearing: the green pair is what actually guards the behaviour against
// future regression, which is the point of the todo.
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

// newTestName is the unit test the Builder owes, per scout-report Task 1
// (verifiableBy) and triage top_n.
const newTestName = "TestVerify_WarnOrFailSentinel_StillRequiresSections"

// testFileRel is the single targetFile for this lane.
const testFileRel = "go/internal/deliverable/deliverable_test.go"

// writeReport drops a build-report.md with the given body into a fresh
// workspace and returns that workspace dir.
func writeReport(t *testing.T, body string) string {
	t.Helper()
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "build-report.md"), []byte(body), 0o644); err != nil {
		t.Fatalf("write build-report.md: %v", err)
	}
	return ws
}

// hasCode reports whether res carries a violation with the given code.
func hasCode(res deliverable.Result, code string) bool {
	for _, v := range res.Violations {
		if v.Code == code {
			return true
		}
	}
	return false
}

// sentinelOnly is a report body consisting of NOTHING but a verdict sentinel —
// the exact "I failed, so I skipped the report" artifact the todo names.
func sentinelOnly(verdict string) string {
	return `<!-- evolve-verdict: {"phase":"build","verdict":"` + verdict + `"} -->` + "\n"
}

// TestC1138_001_SentinelFailOrWarnStillRequiresSections is the crux: a
// sentinel-declared FAIL/WARN must not suppress the required-section check.
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

// TestC1138_002_SectionsPresentUnderFailSentinelIsClean is the negative control:
// the section check must key off the sections, not off the verdict. A FAIL
// report that DOES carry its required section must not be flagged
// missing_section.
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

// TestC1138_003_RegressionUnitTestExistsAndPasses runs the Builder's new unit
// test for real. Exit 0 alone is insufficient — `go test -run` matching nothing
// also exits 0 — so the -v PASS line for the exact test name is required.
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

// TestC1138_004_DeliverablePackageSuiteGreen keeps the new test from being paid
// for with a broken neighbour.
func TestC1138_004_DeliverablePackageSuiteGreen(t *testing.T) {
	root := acsassert.RepoRoot(t)
	stdout, stderr, code, _ := acsassert.SubprocessOutput("go", "test",
		"-C", filepath.Join(root, "go"), "-count=1", "./internal/deliverable/")
	if code != 0 {
		t.Errorf("internal/deliverable package suite exited %d, want 0 (no regression)\n%s",
			code, tail(stdout+stderr, 40))
	}
}

// tail returns the last n lines of s, so failure output stays readable.
func tail(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
