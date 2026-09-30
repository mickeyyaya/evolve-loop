//go:build acs

package cycle1522

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const auditPkg = "github.com/mickeyyaya/evolve-loop/go/internal/phases/audit"

var lengthSubTests = []string{
	"TestAuditReportLength/cap_value_sane",
	"TestAuditReportLength/under_cap_silent",
	"TestAuditReportLength/exact_boundary_silent",
	"TestAuditReportLength/over_cap_warns_once",
	"TestAuditReportLength/over_cap_does_not_flip_verdict",
	"TestAuditReportLength/over_cap_does_not_mutate_artifact",
}

func TestC1522_001_audit_report_length_contract_green(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-run", "TestAuditReportLength", "-v", auditPkg)
	if code != 0 || err != nil {
		t.Fatalf("go test -run TestAuditReportLength %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			auditPkg, code, err, stdout, stderr)
	}
	for _, name := range lengthSubTests {
		if !strings.Contains(stdout, "--- PASS: "+name) {
			t.Errorf("sub-test %s did not report PASS (renamed, skipped, or not run) — "+
				"exit 0 alone cannot prove the size contract ran\nstdout:\n%s", name, stdout)
		}
	}
}

var constValueRE = regexp.MustCompile(`auditReportMaxBytes\s*=\s*([0-9]+(?:\s*\*\s*[0-9]+)*)`)

func capBytesFromSource(t *testing.T, root string) int {
	t.Helper()
	src := filepath.Join(root, "go", "internal", "phases", "audit", "audit.go")
	body, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read %s: %v", src, err)
	}
	m := constValueRE.FindSubmatch(body)
	if m == nil {
		t.Fatalf("auditReportMaxBytes is not declared with a numeric value in %s — "+
			"the cap has no single source for the doc to track", src)
	}
	value := 1
	for _, factor := range strings.Split(string(m[1]), "*") {
		n, convErr := strconv.Atoi(strings.TrimSpace(factor))
		if convErr != nil {
			t.Fatalf("cannot parse cap expression %q: %v", string(m[1]), convErr)
		}
		value *= n
	}
	return value
}

// acs-predicate: config-check — this is an inherent cross-artifact consistency
func TestC1522_002_doc_cap_matches_code_cap(t *testing.T) {
	root := acsassert.RepoRoot(t)
	capBytes := capBytesFromSource(t, root)
	if capBytes < 8*1024 || capBytes > 1<<20 {
		t.Errorf("auditReportMaxBytes=%d out of the sane range [8KiB, 1MiB]", capBytes)
	}
	doc := filepath.Join(root, "agents", "evolve-auditor-reference.md")
	forms := []string{strconv.Itoa(capBytes)}
	if capBytes%1024 == 0 {
		kib := capBytes / 1024
		forms = append(forms,
			fmt.Sprintf("%dKB", kib), fmt.Sprintf("%d KB", kib),
			fmt.Sprintf("%dKiB", kib), fmt.Sprintf("%d KiB", kib))
	}
	if !acsassert.FileContainsAny(doc, forms...) {
		t.Errorf("agents/evolve-auditor-reference.md documents no budget matching the code cap "+
			"auditReportMaxBytes=%d (accepted forms: %v) — prompt and gate have drifted, which is "+
			"the failure mode the doc-sync AC exists to prevent", capBytes, forms)
	}
}

func TestC1522_003_verdict_semantics_unregressed(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-run", "TestVerdictConflict", "-v", auditPkg)
	if code != 0 || err != nil {
		t.Fatalf("verdict-conflict suite regressed: go test -run TestVerdictConflict %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			auditPkg, code, err, stdout, stderr)
	}
	if !strings.Contains(stdout, "--- PASS: TestVerdictConflict") {
		t.Errorf("no TestVerdictConflict PASS marker — the regression axis did not actually run "+
			"(renamed or filtered out)\nstdout:\n%s", stdout)
	}
}
