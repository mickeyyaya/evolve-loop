//go:build acs

package cycle1701

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const shipPkg = "./internal/phases/ship"

func goDir(t *testing.T) string { return filepath.Join(acsassert.RepoRoot(t), "go") }

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
