//go:build acs

// Package cycle1403 materialises the cycle-1403 acceptance criteria for the
// three fleet-scoped tasks pinned to this lane, all serving one inbox item
// (`defect-disposition-contract-unsatisfiable`):
//
//   - disposition-evidence-tolerant-unmarshal   → the JSON-type fix (crux)
//   - disposition-schema-literal-example        → prompt + arch-doc example
//   - disposition-parse-error-surfaced-inline   → self-sufficient rejection
//
// Predicate strategy. The subject is `readDispositions` / the disposition
// branch of `hooks.Classify` in package `audit` — all unexported, so an
// external predicate package cannot call them. Each predicate therefore drives
// the RED contract this cycle's TDD phase froze into
// go/internal/phases/audit/*_test.go, which itself reaches the subject through
// the production seam `hooks{}.Classify` (and, for the doc example, the
// production reader `readDispositions`). No predicate greps production source
// for a magic string — the cycle-85 degenerate-predicate ban.
//
// Vacuity guard. `go test -run` exits 0 when the pattern matches NOTHING, so a
// builder who deletes or renames a frozen test would turn every predicate here
// green. runNamedTests therefore asserts each expected test name appears as an
// executed `--- PASS:` line, not merely that the process exited 0.
//
// Flaky-shape posture. Each predicate invokes ONE named package
// (./internal/phases/audit — ~22s cold, well under the banned 40s+ suites) with
// `-run` narrowing where the criterion is specific; cmd.Dir is set explicitly
// rather than relying on process cwd, which differs between main tree,
// worktree, and fleet lane. No wall-clock assertions, no literal PIDs, no
// unreaped load generators.
package cycle1403

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// auditPkg is the single named package under test. Never a `/...` sweep.
const auditPkg = "./internal/phases/audit"

// runNamedTests runs exactly `names` in auditPkg and reports (combinedOutput,
// exitCode). It sets cmd.Dir to <root>/go so the invocation is independent of
// the caller's working directory.
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

// assertAllExecutedAndPassed fails unless every name appears as an executed
// passing test — the guard against a `-run` pattern that matches nothing.
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

// TestC1403_001_DispositionEvidenceShapeTolerance — Task 1, the crux. The
// array-shaped `evidence` that killed cycle-1399 must be READ and honoured,
// while an unresolvable array, an empty array, and a non-array/non-string shape
// must all still block. Tolerance widens the accepted shape, never the accepted
// claim.
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

// TestC1403_002_DispositionParseErrorSurfacesSchemaInline — Task 3. A rejected
// disposition file must name the expected schema inline (fields plus the two
// legal statuses), and the absent-file branch must keep its own distinct
// marker.
func TestC1403_002_DispositionParseErrorSurfacesSchemaInline(t *testing.T) {
	root := acsassert.RepoRoot(t)
	names := []string{
		"TestClassify_DispositionUnparseableErrorNamesSchema",
		"TestClassify_DispositionMissingDiagnosticNotRelabelledUnparseable",
	}
	out, code := runNamedTests(t, root, names...)
	assertAllExecutedAndPassed(t, out, code, names...)
}

// TestC1403_003_DispositionSchemaLiteralExample — Task 2. The example in
// agents/evolve-auditor.md must be a document the production reader accepts,
// must show both dispositions with literal values, and must match the
// architecture doc's example as parsed JSON.
func TestC1403_003_DispositionSchemaLiteralExample(t *testing.T) {
	root := acsassert.RepoRoot(t)
	names := []string{
		"TestAuditorPromptDispositionExampleIsAcceptedByProductionReader",
		"TestAuditorPromptAndArchDocDispositionExamplesAgree",
	}
	out, code := runNamedTests(t, root, names...)
	assertAllExecutedAndPassed(t, out, code, names...)
}

// TestC1403_004_AuditPackageNoRegression — the no-regression floor. The
// disposition decoder sits on the audit verdict path that every cycle crosses,
// so the whole (single, named) package must stay green: a tolerant decoder that
// breaks an unrelated reconcile case has traded one outage for another.
func TestC1403_004_AuditPackageNoRegression(t *testing.T) {
	root := acsassert.RepoRoot(t)
	cmd := exec.Command("go", "test", "-count=1", auditPkg)
	cmd.Dir = filepath.Join(root, "go")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("go test %s failed — the disposition fix must not regress any other case on the audit verdict path: %v\noutput:\n%s", auditPkg, err, string(out))
	}
}
