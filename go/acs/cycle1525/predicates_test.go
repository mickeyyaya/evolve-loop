//go:build acs

// Package cycle1525 materializes the cycle-1525 acceptance criterion for the
// fleet-scoped task `cap-audit-report-length`: doc/code drift protection for
// the whole-report size budget introduced in the cycle-1522 salvage
// (audit_report_length_test.go / audit.go:auditReportMaxBytes).
//
// Everything else this task's eval pins (cap-exists, overflow-recorded,
// non-lossy, no-regression — .evolve/evals/cap-audit-report-length.md) is
// already covered, live and GREEN, by
// go/internal/phases/audit/audit_report_length_test.go's TestAuditReportLength
// table (verified pre-existing GREEN this cycle, see test-report.md). The one
// AC that eval pins but no test file yet materializes is criterion 5,
// "doc-code-sync": agents/evolve-auditor-reference.md must document the SAME
// numeric budget the gate enforces (auditReportMaxBytes in
// internal/phases/audit/audit.go). This predicate is that missing piece.
package cycle1525

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// TestC1525_001_DocCapMatchesCodeCap cross-references the numeric budget
// documented in agents/evolve-auditor-reference.md against the live
// auditReportMaxBytes constant in internal/phases/audit/audit.go. It is a
// static-text comparison, not a subprocess/behavioral call — there is no
// runtime seam to drive for "does the doc match the code" — so it is declared
// as a waived config-check predicate (cycle-85 classification table) rather
// than dressed up as a fake behavioral test. It still FAILS the moment either
// side drifts, which is the load-bearing property: bumping the code constant
// without updating the doc (or vice versa) breaks this predicate immediately.
//
// acs-predicate: config-check
func TestC1525_001_DocCapMatchesCodeCap(t *testing.T) {
	root := acsassert.RepoRoot(t)

	codePath := filepath.Join(root, "go", "internal", "phases", "audit", "audit.go")
	codeRE := regexp.MustCompile(`const auditReportMaxBytes = (\d+) \* 1024`)
	codeKiB := extractInt(t, codePath, codeRE, 1)
	codeBytes := codeKiB * 1024

	docPath := filepath.Join(root, "agents", "evolve-auditor-reference.md")
	docRE := regexp.MustCompile(`Whole-file size budget: (\d+)KB \((\d+) bytes\)`)
	docKB := extractInt(t, docPath, docRE, 1)
	docBytes := extractInt(t, docPath, docRE, 2)

	if docKB*1024 != docBytes {
		t.Fatalf("agents/evolve-auditor-reference.md is internally inconsistent: %dKB does not equal %d bytes", docKB, docBytes)
	}
	if docBytes != codeBytes {
		t.Errorf("doc/code drift on the audit-report.md size budget: "+
			"agents/evolve-auditor-reference.md documents %d bytes, but "+
			"internal/phases/audit/audit.go's auditReportMaxBytes is %d bytes — "+
			"a reader following the documented budget would trip the real gate at a different size",
			docBytes, codeBytes)
	}
}

// extractInt reads path, matches re against its content, and parses capture
// group idx as an integer. Fails the test (not the predicate silently) when
// the file is missing, the pattern doesn't match, or the capture isn't
// numeric — a shape change on either side of the doc/code pair must be loud.
func extractInt(t *testing.T, path string, re *regexp.Regexp, idx int) int {
	t.Helper()
	if !acsassert.FileExists(t, path) {
		t.Fatalf("required file missing: %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	sub := re.FindStringSubmatch(string(data))
	if sub == nil {
		t.Fatalf("%s does not match expected pattern %s — the constant/doc line may have been renamed or reshaped", path, re.String())
	}
	n, err := strconv.Atoi(sub[idx])
	if err != nil {
		t.Fatalf("%s: captured value %q is not an integer: %v", path, sub[idx], err)
	}
	return n
}
