//go:build integration

package ciparity

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func splitProbeFixture(t *testing.T, failing bool) string {
	t.Helper()
	root := makeFixture(t)
	body := `if probe.Value() != 1 { t.Fatal("wrong value") }`
	if failing {
		body += `; t.Fatal("intentional split failure")`
	}
	writeMakeFixture(t, root, ".cover-split", "./internal/probe/probetest ./internal/probe\n")
	writeMakeFixture(t, root, "internal/probe/probe.go", "package probe\nfunc Value() int { return 1 }\nfunc Other() int { return 2 }\n")
	writeMakeFixture(t, root, "internal/probe/probe_test.go", "package probe\nimport \"testing\"\nfunc TestOther(t *testing.T) { if Other() != 2 { t.Fatal(\"other\") } }\n")
	writeMakeFixture(t, root, "internal/probe/probetest/value_test.go", "package probetest_test\nimport (\n\t\"testing\"\n\n\t\"gateprobe/internal/probe\"\n)\nfunc TestValue(t *testing.T) { "+body+" }\n")
	return root
}

func requireNoRunFiles(t *testing.T, root string) {
	t.Helper()
	left, err := filepath.Glob(filepath.Join(root, "coverage.run.*"))
	if err != nil || len(left) != 0 {
		t.Errorf("the run left %v (glob err %v), want no coverage.run.* file", left, err)
	}
}

func TestMakeTestIntegration_ASplitPackageCountsForThePackageItTests(t *testing.T) {
	t.Parallel()
	root := splitProbeFixture(t, false)

	out, err := runMakeFixture(t, root, "test-integration", "TEST_JSON=stream.json")

	if err != nil {
		t.Fatalf("test-integration: %v\n%s", err, out)
	}
	requireMake(t, root, "cover-strict")
	stream, err := os.ReadFile(filepath.Join(root, "stream.json"))
	if err != nil || !strings.Contains(string(stream), `"Action":"pass","Package":"gateprobe/internal/probe/probetest"`) || !strings.Contains(string(stream), `"Package":"gateprobe/internal/probe"`) {
		t.Errorf("the -json stream must hold the main run and the pass of the split package (err %v):\n%s", err, stream)
	}
	requireNoRunFiles(t, root)
}

func TestMakeTestIntegration_AFailedSplitRunFailsTheRunAndLeavesNoProfile(t *testing.T) {
	t.Parallel()
	root := splitProbeFixture(t, true)

	out, err := runMakeFixture(t, root, "test-integration", "TEST_JSON=stream.json")

	if err == nil || !strings.Contains(out, "intentional split failure") {
		t.Fatalf("a red split run must fail test-integration and show its output: err %v\n%s", err, out)
	}
	if _, statErr := os.Stat(filepath.Join(root, "coverage.txt")); !os.IsNotExist(statErr) {
		t.Errorf("a failed split run left coverage.txt (stat err %v)", statErr)
	}
	requireNoRunFiles(t, root)
}

func TestMakeTestIntegration_ABadSplitListStopsTheRunBeforeAnyTest(t *testing.T) {
	t.Parallel()
	root := splitProbeFixture(t, false)
	writeMakeFixture(t, root, ".cover-split", "./internal/probe/probetest\n")

	out, err := runMakeFixture(t, root, "test-integration")

	if err == nil || !strings.Contains(out, ".cover-split") || strings.Contains(out, "TestValue") {
		t.Fatalf("a bad split list must stop the run before any test and name the list: err %v\n%s", err, out)
	}
	requireNoRunFiles(t, root)
}
