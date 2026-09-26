//go:build acs

// Package cycle1701 materialises the acceptance criteria for
// audit-binding-report-comment-fallback-is-dead-code: the auditor-report
// `audit_bound_tree_sha:` comment must stop being a source for the ship
// package's audit-bound tree, leaving the ledger's worktree_tree_sha as the
// single binding. Each predicate drives verifyAuditBinding (or scans the ship
// package's parsed source) through one named in-package test.
package cycle1701

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const shipPkg = "./internal/phases/ship"

// goDir is the worktree's Go module root, so the shelled tests compile the
// cycle's tree rather than main's copy.
func goDir(t *testing.T) string { return filepath.Join(acsassert.RepoRoot(t), "go") }

// runShipTest runs one named integration-tier test in the ship package and
// requires a real PASS line, so a renamed or skipped test cannot pass silently.
func runShipTest(t *testing.T, name string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-C", goDir(t), "-tags", "integration", "-count=1", "-v", "-run", "^"+name+"$", shipPkg)
	out := stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to launch (not a test failure): %v\n%s", err, out)
	}
	if code != 0 {
		t.Fatalf("%s -run %s exited %d\n%s", shipPkg, name, code, out)
	}
	if !strings.Contains(out, "--- PASS: "+name) {
		t.Fatalf("no PASS line for %s in %s (renamed, skipped, or never ran?)\n%s", name, shipPkg, out)
	}
}

func TestC1701_001_ReportCommentNeverBindsAuditTree(t *testing.T) {
	runShipTest(t, "TestVerifyAuditBinding_ReportCommentNeverBindsTree")
}

func TestC1701_002_EmptyLedgerTreeRefusedBeforeConsumption(t *testing.T) {
	runShipTest(t, "TestVerifyAuditBinding_EmptyLedgerTreeRefusedBeforeConsumption")
}

func TestC1701_003_LedgerTreeWinsOverReportComment(t *testing.T) {
	runShipTest(t, "TestVerifyAuditBinding_LedgerTreeWinsOverReportComment")
}

func TestC1701_004_ReportCommentParserDeletedAndShipVets(t *testing.T) {
	runShipTest(t, "TestShipSource_NoAuditReportTreeCommentParser")

	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "vet", "-C", goDir(t), "-tags", "integration", shipPkg)
	if code != 0 {
		t.Errorf("go vet %s exited %d (err=%v)\n%s%s", shipPkg, code, err, stdout, stderr)
	}
	acsassert.FileNotContains(t, filepath.Join(goDir(t), "internal", "phases", "ship", "audit.go"), "auditBoundTreeSHARe")
}
