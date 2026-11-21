//go:build integration

package ciparity

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const passingProbeTest = "package probe\nimport \"testing\"\nfunc TestValue(t *testing.T) { if Value() != 1 { t.Fatal(\"wrong value\") } }\n"

func treewalkProbeFixture(t *testing.T, probeTestBody string) string {
	t.Helper()
	root := makeFixture(t)
	writeMakeFixture(t, root, "internal/probe/probe.go", "package probe\nfunc Value() int { return 1 }\n")
	writeMakeFixture(t, root, "internal/probe/probe_test.go", probeTestBody)
	return root
}

func leakingProbeTest(leak string) string {
	return "package probe\nimport (\"os\"; \"testing\")\nfunc TestValue(t *testing.T) {\n\tif Value() != 1 { t.Fatal(\"wrong value\") }\n\t" + leak + "\n}\n"
}

func TestMakeIntegration_FailsWhenATestLeaksAPathOutsideTheAllowList(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		leak     string
		wantLeak string
	}{
		{
			"evolve ledger directory next to the test",
			"if err := os.MkdirAll(\".evolve/ledger\", 0o755); err != nil { t.Fatal(err) }; if err := os.WriteFile(\".evolve/ledger/ledger.ndjson\", []byte(\"x\"), 0o644); err != nil { t.Fatal(err) }",
			"LEAK " + filepath.Join("internal", "probe", ".evolve"),
		},
		{
			"stray file next to the test",
			"if err := os.WriteFile(\"stray.txt\", []byte(\"x\"), 0o644); err != nil { t.Fatal(err) }",
			"LEAK " + filepath.Join("internal", "probe", "stray.txt"),
		},
		{
			"empty directory at the module root",
			"if err := os.Mkdir(\"../../leaked-empty-dir\", 0o755); err != nil { t.Fatal(err) }",
			"LEAK leaked-empty-dir",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			root := treewalkProbeFixture(t, leakingProbeTest(c.leak))
			out, err := runMakeFixture(t, root, "test-integration")
			if err == nil {
				t.Fatalf("test-integration passed although a test leaked a path outside the allow list\n%s", out)
			}
			if !strings.Contains(out, c.wantLeak) {
				t.Errorf("test-integration failed without naming the leak %q:\n%s", c.wantLeak, out)
			}
		})
	}
}

func TestMakeIntegration_PassesOnACleanTreeWithItsOwnOutputsPresent(t *testing.T) {
	t.Parallel()
	root := treewalkProbeFixture(t, passingProbeTest)
	rusage := filepath.Join(root, "rusage.ndjson")
	out, err := runMakeFixture(t, root, "test-integration", "TEST_JSON=test-integration.json", "TEST_RUSAGE=1", "TESTRUSAGE_OUT="+rusage)
	if err != nil {
		t.Fatalf("test-integration on a clean tree: %v\n%s", err, out)
	}
	if strings.Contains(out, "LEAK") {
		t.Errorf("the guard flagged an output the target writes itself:\n%s", out)
	}
	if !strings.Contains(out, "treewalk: clean") {
		t.Errorf("the target ran no treewalk check, so a leak would pass unseen:\n%s", out)
	}
	for _, rel := range []string{"coverage.txt", "test-integration.json", "rusage.ndjson", filepath.Join("bin", "coversplit"), filepath.Join("bin", "testrusage"), filepath.Join("bin", "testlatency")} {
		if _, statErr := os.Stat(filepath.Join(root, rel)); statErr != nil {
			t.Errorf("output %s is missing after a green run: %v", rel, statErr)
		}
	}
}

func TestMakeIntegration_PassesWithTheDefaultOutputsOnly(t *testing.T) {
	t.Parallel()
	root := treewalkProbeFixture(t, passingProbeTest)
	out, err := runMakeFixture(t, root, "test-integration")
	if err != nil {
		t.Fatalf("test-integration with default outputs: %v\n%s", err, out)
	}
	if !strings.Contains(out, "treewalk: clean") || strings.Contains(out, "LEAK") {
		t.Errorf("default run: want a clean treewalk verdict and no LEAK line:\n%s", out)
	}
	if _, statErr := os.Stat(filepath.Join(root, "coverage.txt")); statErr != nil {
		t.Errorf("coverage.txt is missing after a green run: %v", statErr)
	}
}

func TestMakeIntegration_IgnoresPathsThatExistedBeforeTheRun(t *testing.T) {
	t.Parallel()
	root := treewalkProbeFixture(t, passingProbeTest)
	writeMakeFixture(t, root, "internal/probe/.evolve/old-ledger.ndjson", "x")
	out, err := runMakeFixture(t, root, "test-integration")
	if err != nil {
		t.Fatalf("a path present before the run was reported as a leak: %v\n%s", err, out)
	}
}

func TestMakeIntegration_KeepsTheTestFailureWhenNothingLeaks(t *testing.T) {
	t.Parallel()
	root := treewalkProbeFixture(t, "package probe\nimport \"testing\"\nfunc TestValue(t *testing.T) { t.Fatal(\"intentional assertion failure\") }\n")
	out, err := runMakeFixture(t, root, "test-integration")
	if err == nil {
		t.Fatalf("a failing test no longer fails test-integration\n%s", out)
	}
	if !strings.Contains(out, "intentional assertion failure") {
		t.Errorf("the failing test lost its diagnostic:\n%s", out)
	}
	if _, statErr := os.Stat(filepath.Join(root, "coverage.txt")); statErr == nil {
		t.Errorf("a failed run left coverage.txt behind")
	}
}

func TestMakeIntegration_FailsOnALeakEvenWhenATestAlsoFails(t *testing.T) {
	t.Parallel()
	body := "package probe\nimport (\"os\"; \"testing\")\nfunc TestValue(t *testing.T) {\n\tif err := os.WriteFile(\"stray.txt\", []byte(\"x\"), 0o644); err != nil { t.Fatal(err) }\n\tt.Fatal(\"intentional assertion failure\")\n}\n"
	root := treewalkProbeFixture(t, body)
	out, err := runMakeFixture(t, root, "test-integration")
	if err == nil {
		t.Fatalf("test-integration passed with a failing test and a leak\n%s", out)
	}
	if !strings.Contains(out, "intentional assertion failure") || !strings.Contains(out, "LEAK "+filepath.Join("internal", "probe", "stray.txt")) {
		t.Errorf("want both the test failure and the leak reported:\n%s", out)
	}
}
