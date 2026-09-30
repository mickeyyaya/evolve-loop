//go:build acs

package cycle1513

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const lockRelPath = "go/internal/core/retry_backoff_test.go"

const productRelPath = "go/internal/core/retry_backoff.go"

const corePkg = "./internal/core"

var lockedTests = []string{
	"TestComposeCorrection_CarriesReasonVerbatim",
	"TestComposeCorrection_FramingSurroundsTheReason",
	"TestComposeCorrection_EmptyReasonStillProducesADirective",
}

var verbatimSubtests = []string{
	"single_line_with_code_token",
	"multiline_multi_violation_summarize_rendering",
	"unicode_and_punctuation",
	"trailing_and_leading_whitespace_is_preserved",
	"percent_and_backslash_are_not_format_interpreted",
}

func TestC1513_001_LockFileLandedAndTracked(t *testing.T) {
	root := acsassert.RepoRoot(t)
	if !acsassert.FileExists(t, filepath.Join(root, lockRelPath)) {
		t.Fatalf("RED: %s missing on disk under %s — the salvage reland did not happen", lockRelPath, root)
	}
	_, stderr, code, err := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", lockRelPath)
	if err != nil || code != 0 {
		t.Errorf("RED: %s is UNTRACKED (git ls-files exit=%d err=%v stderr=%q) — an untracked lock is dropped at ship and locks nothing",
			lockRelPath, code, err, strings.TrimSpace(stderr))
	}
}

func TestC1513_002_LockedTestsExecuteAndPass(t *testing.T) {
	root := acsassert.RepoRoot(t)
	pattern := "^(" + strings.Join(lockedTests, "|") + ")$"
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "-C", filepath.Join(root, "go"), "test", "-count=1", "-v", "-run", pattern, corePkg)
	if err != nil || code != 0 {
		t.Fatalf("RED: locked tests did not pass (exit=%d err=%v)\nstdout:\n%s\nstderr:\n%s", code, err, stdout, stderr)
	}
	for _, name := range lockedTests {
		if !strings.Contains(stdout, fmt.Sprintf("--- PASS: %s ", name)) {
			t.Errorf("RED: %s did not report PASS — it is missing, renamed, or never ran.\nstdout:\n%s", name, stdout)
		}
	}
	for _, sub := range verbatimSubtests {
		want := fmt.Sprintf("--- PASS: %s/%s ", lockedTests[0], sub)
		if !strings.Contains(stdout, want) {
			t.Errorf("RED: verbatim table row %q did not report PASS — the lock has been narrowed to a weaker table.\nstdout:\n%s", sub, stdout)
		}
	}
}

func TestC1513_003_ProductFileUntouched(t *testing.T) {
	root := acsassert.RepoRoot(t)
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"git", "-C", root, "diff", "origin/main", "--", productRelPath)
	if err != nil || code != 0 {
		t.Fatalf("could not diff %s against origin/main (exit=%d err=%v stderr=%q)", productRelPath, code, err, strings.TrimSpace(stderr))
	}
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("RED: %s CHANGED vs origin/main — this is a test-only reland; a product edit makes the lock tautological.\ndiff:\n%s",
			productRelPath, stdout)
	}
}

func TestC1513_004_LockFileGofmtClean(t *testing.T) {
	root := acsassert.RepoRoot(t)
	stdout, stderr, code, err := acsassert.SubprocessOutput("gofmt", "-l", filepath.Join(root, lockRelPath))
	if err != nil || code != 0 {
		t.Fatalf("gofmt failed to run (exit=%d err=%v stderr=%q)", code, err, strings.TrimSpace(stderr))
	}
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("RED: gofmt -l reported %s as unformatted", strings.TrimSpace(stdout))
	}
}
