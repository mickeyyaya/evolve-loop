//go:build acs

package cycle357

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func TestC357_001_DispatchBridgeFlagsAbsentFromLookup(t *testing.T) {
	for _, name := range []string{"EVOLVE_DISPATCH_STOP_ON_FAIL", "EVOLVE_DISPATCH_VERIFY"} {
		if f, ok := flagregistry.Lookup(name); ok {
			t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — deprecated bridge flag still registered.\n"+
				"Builder must remove this row from go/internal/flagregistry/registry_table.go.\n"+
				"Current entry: Status=%q Cluster=%q Doc=%q",
				name, f.Status, f.Cluster, f.Doc)
		}
	}
}

func TestC357_003_ResolveDispatchPolicyNoLegacyCases(t *testing.T) {
	dir := goDir(t)
	out, errOut, code, err := acsassert.SubprocessOutput(
		"go", "test",
		"-C", dir,
		"-count=1", "-v",
		"./cmd/evolve/...",
		"-run", "TestResolveDispatchPolicy",
	)
	combined := out + "\n" + errOut
	if code != 0 || err != nil {
		t.Fatalf("go test -run TestResolveDispatchPolicy failed (exit=%d): %v\nOutput:\n%s",
			code, err, combined)
	}
	for _, marker := range []string{"STOP_ON_FAIL=1_bridges", "VERIFY=0_bridges", "both_legacy"} {
		if strings.Contains(combined, marker) {
			t.Errorf("RED: go test output contains legacy bridge case marker %q — legacy cases still present.\n"+
				"Builder must remove the 4 legacy bridge test cases from cmd_loop_m4_test.go lines 103-106.",
				marker)
		}
	}
}

// acs-predicate: config-check
func TestC357_004_BridgeTestCasesAbsentFromM4Test(t *testing.T) {
	root := acsassert.RepoRoot(t)
	m4test := filepath.Join(root, "go", "cmd", "evolve", "cmd_loop_m4_test.go")
	for _, name := range []string{"EVOLVE_DISPATCH_STOP_ON_FAIL", "EVOLVE_DISPATCH_VERIFY"} {
		if !acsassert.FileNotContains(t, m4test, name) {
			t.Errorf("RED: cmd_loop_m4_test.go still references %q.\n"+
				"Builder must remove the bridge test cases (lines 103-106) and envKeys entries (lines 131-132).\n"+
				"File: %s", name, m4test)
		}
	}
}

// acs-predicate: config-check
func TestC357_005_ClaudeRuntimeDocNoBridgeFlagReference(t *testing.T) {
	root := acsassert.RepoRoot(t)
	claudeRuntime := filepath.Join(root, "skills", "loop", "reference", "claude-runtime.md")
	if !acsassert.FileNotContains(t, claudeRuntime, "EVOLVE_DISPATCH_VERIFY=0") {
		t.Errorf("RED: skills/loop/reference/claude-runtime.md still contains EVOLVE_DISPATCH_VERIFY=0.\n"+
			"Builder must replace line 72 with EVOLVE_DISPATCH_POLICY=off.\n"+
			"File: %s", claudeRuntime)
	}
}

func TestC357_006_FlagsCheckExitsZero(t *testing.T) {
	root := acsassert.RepoRoot(t)
	binPath := filepath.Join(root, "go", "bin", "evolve")
	out, errOut, code, err := acsassert.SubprocessOutput(
		"bash", "-c", "cd "+root+" && "+binPath+" flags check",
	)
	combined := out + "\n" + errOut
	if code != 0 || err != nil {
		t.Errorf("evolve flags check exited %d: %v\nOutput:\n%s\n"+
			"Builder must run `evolve flags generate` after removing the 2 deprecated registry rows.",
			code, err, combined)
	}
}

func TestC357_007_NoProductionGoCodeReferencesBridgeFlags(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goSrc := filepath.Join(root, "go")
	for _, flag := range []string{"EVOLVE_DISPATCH_STOP_ON_FAIL", "EVOLVE_DISPATCH_VERIFY"} {
		out, _, _, _ := acsassert.SubprocessOutput("bash", "-c",
			`grep -rl "`+flag+`" "`+goSrc+`" --include="*.go" 2>/dev/null | grep -v "_test.go"; true`)
		if strings.TrimSpace(out) != "" {
			t.Errorf("RED: non-test Go files still reference deprecated bridge flag %q:\n%s\n"+
				"Builder must remove all production references (bridge function body, doc comment, registry row).",
				flag, strings.TrimSpace(out))
		}
	}
}

func TestC357_008_FlagRegistryTestsPass(t *testing.T) {
	dir := goDir(t)
	out, errOut, code, err := acsassert.SubprocessOutput(
		"go", "test",
		"-C", dir,
		"-count=1",
		"./internal/flagregistry/...",
	)
	combined := out + "\n" + errOut
	if code != 0 || err != nil {
		t.Errorf("RED: go test ./internal/flagregistry/... failed (exit=%d): %v\nOutput:\n%s",
			code, err, combined)
	}
}
