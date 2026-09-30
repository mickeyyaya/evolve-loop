//go:build acs

package cycle1525

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

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
