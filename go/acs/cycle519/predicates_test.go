//go:build acs

package cycle519

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	cmdEvolvePkg = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"
	corePkg      = "github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func runGoTest(t *testing.T, runFilter, pkg string) (out string, code int) {
	t.Helper()
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-v", "-run", runFilter, pkg)
	return stdout + "\n" + stderr, code
}

func requireTestsRan(t *testing.T, out string, min int) {
	t.Helper()
	if strings.Contains(out, "no tests to run") {
		t.Errorf("no tests matched the -run filter (\"no tests to run\") — required tests are unwritten or renamed")
		return
	}
	if got := strings.Count(out, "=== RUN"); got < min {
		t.Errorf("only %d test(s) ran, need >= %d (package build failure or renamed tests)", got, min)
	}
}

func TestC519_001_QuarantineStashLabelTimestamped(t *testing.T) {
	out, code := runGoTest(t,
		"TestDefaultBootRecovery_QuarantineStashLabelIsTimestamped", cmdEvolvePkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("boot quarantine stash label is NOT timestamped (still the fixed 'boot-quarantine' constant) — successive boot quarantines are indistinguishable/unrecoverable.\n%s", out)
	}
}

func TestC519_002_DetectsAndQuarantinesLeakedSource(t *testing.T) {
	out, code := runGoTest(t,
		"TestDefaultBootRecovery_QuarantinesDirtyTree", cmdEvolvePkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("boot pre-flight no longer detects+quarantines leaked tracked source (detection/wiring regressed).\n%s", out)
	}
}

func TestC519_003_ExcludesLoopManagedDirs(t *testing.T) {
	out, code := runGoTest(t,
		"TestClassifyDirtyPaths_IgnoresLoopManaged", corePkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf(".evolve/ and knowledge-base/ must be EXCLUDED from the boot dirty-scan; exclusion regressed.\n%s", out)
	}
}

func TestC519_004_QuarantineNonDestructivePreservesContent(t *testing.T) {
	out, code := runGoTest(t,
		"TestQuarantineDirtyTree_LeavesStatusCleanAndPreservesContent", corePkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("boot quarantine must stay NON-DESTRUCTIVE (stash-preserves-content) and leave the tree clean; regressed.\n%s", out)
	}
}
