//go:build acs

package cycle1403

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const auditPkg = "./internal/phases/audit"

func runNamedTests(t *testing.T, root string, names ...string) (string, int) {
	t.Helper()
	args := []string{"test", "-count=1", "-v", "-run", "^(" + strings.Join(names, "|") + ")$", auditPkg}
	cmd := exec.Command("go", args...)
	cmd.Dir = filepath.Join(root, "go")
	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
		} else {
			t.Fatalf("go test could not be run at %s: %v", cmd.Dir, err)
		}
	}
	return string(out), code
}

func assertAllExecutedAndPassed(t *testing.T, out string, code int, names ...string) {
	t.Helper()
	for _, n := range names {
		if !strings.Contains(out, "--- PASS: "+n) {
			t.Errorf("test %s did not execute and pass — it must exist under go/internal/phases/audit and be GREEN (a deleted or renamed frozen test is a contract removal, not a fix). go test exit=%d, output:\n%s", n, code, out)
		}
	}
	if code != 0 {
		t.Errorf("go test %s exited %d; every frozen acceptance test for this cycle must be GREEN. output:\n%s", auditPkg, code, out)
	}
}

func TestC1403_001_DispositionEvidenceShapeTolerance(t *testing.T) {
	root := acsassert.RepoRoot(t)
	names := []string{
		"TestClassify_DispositionEvidenceStringShapeAccepted",
		"TestClassify_DispositionEvidenceArrayShapeAccepted",
		"TestClassify_DispositionEvidenceArrayShapeUnresolvableStillBlocks",
		"TestClassify_DispositionEvidenceEmptyArrayOnFixedStillBlocks",
		"TestClassify_DispositionEvidenceObjectShapeStillBlocks",
	}
	out, code := runNamedTests(t, root, names...)
	assertAllExecutedAndPassed(t, out, code, names...)
}

func TestC1403_002_DispositionParseErrorSurfacesSchemaInline(t *testing.T) {
	root := acsassert.RepoRoot(t)
	names := []string{
		"TestClassify_DispositionUnparseableErrorNamesSchema",
		"TestClassify_DispositionMissingDiagnosticNotRelabelledUnparseable",
	}
	out, code := runNamedTests(t, root, names...)
	assertAllExecutedAndPassed(t, out, code, names...)
}

func TestC1403_003_DispositionSchemaLiteralExample(t *testing.T) {
	root := acsassert.RepoRoot(t)
	names := []string{
		"TestAuditorPromptDispositionExampleIsAcceptedByProductionReader",
		"TestAuditorPromptAndArchDocDispositionExamplesAgree",
	}
	out, code := runNamedTests(t, root, names...)
	assertAllExecutedAndPassed(t, out, code, names...)
}

func TestC1403_004_AuditPackageNoRegression(t *testing.T) {
	root := acsassert.RepoRoot(t)
	cmd := exec.Command("go", "test", "-count=1", auditPkg)
	cmd.Dir = filepath.Join(root, "go")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("go test %s failed — the disposition fix must not regress any other case on the audit verdict path: %v\noutput:\n%s", auditPkg, err, string(out))
	}
}
