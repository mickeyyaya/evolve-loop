//go:build integration

package ciparity

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// These tests exercise the real Make recipes and Go coverage tools. A percentage
// alone is not a verdict: Go writes coverage even when a test assertion fails.
func TestMakeCoverageGates_PropagateTestFailure(t *testing.T) {
	for _, target := range []string{"cover-strict", "apicover-enforce"} {
		for _, failing := range []bool{false, true} {
			name := "passing"
			if failing {
				name = "failing_with_full_coverage"
			}
			t.Run(target+"/"+name, func(t *testing.T) {
				root := makeFixture(t)
				body := `if Value() != 1 { t.Fatal("wrong value") }`
				if failing {
					body += `; t.Fatal("intentional assertion failure")`
				}
				writeMakeFixture(t, root, "internal/probe/probe.go", "package probe\nfunc Value() int { return 1 }\n")
				writeMakeFixture(t, root, "internal/probe/probe_test.go", "package probe\nimport \"testing\"\nfunc TestValue(t *testing.T) { "+body+" }\n")
				out, err := runMakeFixture(t, root, target)
				if (err != nil) != failing {
					t.Fatalf("%s: error = %v, want failure %v\n%s", target, err, failing, out)
				}
			})
		}
	}
}

func TestMakeCoverageGates_RejectToolFailure(t *testing.T) {
	for _, stage := range []string{"test", "tool", "list"} {
		t.Run(stage, func(t *testing.T) {
			root := makeFixture(t)
			writeMakeFixture(t, root, "internal/probe/probe.go", "package probe\nfunc Value() int { return 1 }\n")
			writeMakeFixture(t, root, "internal/probe/probe_test.go", "package probe\nimport \"testing\"\nfunc TestValue(t *testing.T) { if Value() != 1 { t.Fatal(\"wrong value\") } }\n")
			// Run the real tool first so the failure leaves plausible output/profile.
			// The gate must honor the exit status instead of accepting that output.
			wrapper := "#!/bin/sh\nif [ \"$1\" = \"" + stage + "\" ]; then go \"$@\"; echo intentional-tool-failure >&2; exit 23; fi\n" + makeGoBuildRedirect(t) + "exec go \"$@\"\n"
			writeMakeFixture(t, root, "probe-go", wrapper)
			out, err := runMakeFixture(t, root, "apicover-enforce")
			if err == nil {
				t.Fatalf("gate accepted failing %s tool\n%s", stage, out)
			}
			if !strings.Contains(out, "intentional-tool-failure") {
				t.Fatalf("failure lost its diagnostic: %s", out)
			}
		})
	}
}

func TestMakeCoverStrict_PreservesFloor(t *testing.T) {
	root := makeFixture(t)
	writeMakeFixture(t, root, "internal/probe/probe.go", "package probe\nfunc Value(b bool) int { if b { return 1 }; return 2 }\n")
	writeMakeFixture(t, root, "internal/probe/probe_test.go", "package probe\nimport \"testing\"\nfunc TestValue(t *testing.T) { if Value(true) != 1 { t.Fatal(\"wrong value\") } }\n")
	if out, err := runMakeFixture(t, root, "cover-strict"); err == nil {
		t.Fatalf("partially covered package passed its 100%% floor\n%s", out)
	}
}

func TestMakeAPIEnforce_ExcludesCyclePredicatesFromMeasurement(t *testing.T) {
	root := makeFixture(t)
	writeMakeFixture(t, root, "internal/probe/probe.go", "package probe\nfunc Value() int { return 1 }\n")
	writeMakeFixture(t, root, "internal/probe/probe_test.go", "package probe\nimport \"testing\"\nfunc TestValue(t *testing.T) { if Value() != 1 { t.Fatal(\"wrong value\") } }\n")
	writeMakeFixture(t, root, "acs/probe/probe_test.go", "//go:build acs\n\npackage probe\nimport \"testing\"\nfunc TestLiveArtifacts(t *testing.T) { t.Fatal(\"cycle-artifact-required\") }\n")
	writeMakeFixture(t, root, ".apicover-enforce", "./internal/probe\n./acs/probe\n")
	if out, err := runMakeFixture(t, root, "apicover-enforce"); err != nil {
		t.Fatalf("test-only cycle predicates must remain inspectable without executing them: %v\n%s", err, out)
	}
}

func TestMakeAPIEnforce_AcceptsTestOnlyOrEmptyEnrollment(t *testing.T) {
	for _, manifest := range []string{"./acs/probe\n", "# no enrolled packages\n"} {
		t.Run(strings.TrimSpace(manifest), func(t *testing.T) {
			root := makeFixture(t)
			writeMakeFixture(t, root, "acs/probe/probe_test.go", "//go:build acs\n\npackage probe\nimport \"testing\"\nfunc TestLiveArtifacts(t *testing.T) { t.Fatal(\"cycle-artifact-required\") }\n")
			writeMakeFixture(t, root, ".apicover-enforce", manifest)
			if out, err := runMakeFixture(t, root, "apicover-enforce"); err != nil {
				t.Fatalf("zero production exports should pass API inspection: %v\n%s", err, out)
			}
		})
	}
}

func TestMakeAPICheck_RejectsMissingOrMalformedProfile(t *testing.T) {
	for _, missing := range []bool{true, false} {
		name := "malformed"
		if missing {
			name = "missing"
		}
		t.Run(name, func(t *testing.T) {
			root := makeFixture(t)
			writeMakeFixture(t, root, "internal/probe/probe.go", "package probe\nfunc Value() int { return 1 }\n")
			if !missing {
				writeMakeFixture(t, root, "coverage.txt", "not a coverage profile\n")
			}
			out, err := runMakeFixture(t, root, "apicover-check")
			if err == nil || !strings.Contains(out, "cover:") {
				t.Fatalf("invalid profile must fail at the cover tool: %v\n%s", err, out)
			}
		})
	}
}

func TestMakeCoverStrict_ChecksFinalLineWithoutNewline(t *testing.T) {
	root := makeFixture(t)
	writeMakeFixture(t, root, "internal/probe/probe.go", "package probe\nfunc Value() int { return 1 }\n")
	writeMakeFixture(t, root, "internal/probe/probe_test.go", "package probe\nimport \"testing\"\nfunc TestValue(t *testing.T) { _ = Value(); t.Fatal(\"intentional assertion failure\") }\n")
	writeMakeFixture(t, root, ".cover-strict", "./internal/probe 100")
	if out, err := runMakeFixture(t, root, "cover-strict"); err == nil {
		t.Fatalf("gate skipped final manifest entry\n%s", out)
	}
}

func TestMakeCoverageGates_RejectMissingManifest(t *testing.T) {
	for _, target := range []string{"cover-strict", "apicover-enforce"} {
		t.Run(target, func(t *testing.T) {
			root := makeFixture(t)
			if err := os.Remove(filepath.Join(root, "."+target)); err != nil {
				t.Fatal(err)
			}
			if out, err := runMakeFixture(t, root, target); err == nil {
				t.Fatalf("gate accepted missing manifest\n%s", out)
			}
		})
	}
}

func TestMakeCoverStrict_RejectsManifestDirectory(t *testing.T) {
	root := makeFixture(t)
	manifest := filepath.Join(root, ".cover-strict")
	if err := os.Remove(manifest); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(manifest, 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := runMakeFixture(t, root, "cover-strict"); err == nil {
		t.Fatalf("gate accepted a directory as its manifest\n%s", out)
	}
}

func TestMakeFixture_CancellationFailsHarness(t *testing.T) {
	const control = "EVOLVE_MAKE_CANCELLATION_CONTROL"
	if os.Getenv(control) == "1" {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, _ = runMakeFixtureContext(t, ctx, makeFixture(t), "cover-strict")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestMakeFixture_CancellationFailsHarness$")
	cmd.Env = append(os.Environ(), control+"=1")
	out, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "fixture execution canceled") {
		t.Fatalf("canceled fixture was accepted as a test result: %v\n%s", err, out)
	}
}

func TestMakeCoverStrict_RejectsInvalidFloorOrEvaluator(t *testing.T) {
	for _, fault := range []string{"invalid_floor", "failed_evaluator"} {
		t.Run(fault, func(t *testing.T) {
			root := makeFixture(t)
			writeMakeFixture(t, root, "internal/probe/probe.go", "package probe\nfunc Value() int { return 1 }\n")
			writeMakeFixture(t, root, "internal/probe/probe_test.go", "package probe\nimport \"testing\"\nfunc TestValue(t *testing.T) { if Value() != 1 { t.Fatal(\"wrong value\") } }\n")
			if fault == "invalid_floor" {
				writeMakeFixture(t, root, ".cover-strict", "./internal/probe invalid\n")
			} else {
				writeMakeFixture(t, root, "awk", "#!/bin/sh\necho intentional-evaluator-failure >&2\nexit 23\n")
			}
			if out, err := runMakeFixture(t, root, "cover-strict"); err == nil {
				t.Fatalf("gate accepted %s\n%s", fault, out)
			}
		})
	}
}

func TestMakeIntegration_SelectsEntireRuntime(t *testing.T) {
	root := makeFixture(t)
	packages := []string{"internal/probe", "cmd/probe", "pkg/probe", "test/component", "test/fixtures", "test/trustkernel", "test/integration", "examples/probe"}
	for _, pkg := range packages {
		writeMakeFixture(t, root, pkg+"/probe_test.go", "package probe\nimport \"testing\"\nfunc TestSelected(t *testing.T) { t.Fatal(\"selected-"+pkg+"\") }\n")
	}
	writeMakeFixture(t, root, "acs/probe/probe_test.go", "package probe\nimport \"testing\"\nfunc TestExcluded(t *testing.T) { t.Fatal(\"unselected-acs\") }\n")
	out, err := runMakeFixture(t, root, "test-integration")
	if err == nil {
		t.Fatalf("known-red selected suite passed\n%s", out)
	}
	for _, pkg := range packages {
		if !strings.Contains(out, "selected-"+pkg) {
			t.Errorf("required package %s did not execute\n%s", pkg, out)
		}
	}
	if strings.Contains(out, "unselected-acs") {
		t.Fatalf("integration ran artifact-dependent ACS\n%s", out)
	}
}

func TestMakeAll_ExecutesCompoundE2ETag(t *testing.T) {
	root := makeFixture(t)
	writeMakeFixture(t, root, "internal/probe/probe.go", "package probe\n")
	writeMakeFixture(t, root, "cmd/probe/probe.go", "package probe\n")
	writeMakeFixture(t, root, "test/e2e/probe.go", "package probe\n")
	writeMakeFixture(t, root, "cmd/probe/probe_test.go", "//go:build e2e && evolve_test_phases\n\npackage probe\nimport \"testing\"\nfunc TestRequiredPhase(t *testing.T) { t.Fatal(\"compound-tag-executed\") }\n")
	out, err := runMakeFixture(t, root, "test-all")
	if err == nil || !strings.Contains(out, "compound-tag-executed") {
		t.Fatalf("test-all did not execute the required compound-tag case: %v\n%s", err, out)
	}
}

func makeFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeMakeFixture(t, root, "go.mod", "module gateprobe\n\ngo 1.23\n")
	writeMakeFixture(t, root, ".cover-strict", "./internal/probe 100\n")
	writeMakeFixture(t, root, ".apicover-enforce", "./internal/probe\n")
	writeMakeFixture(t, root, "probe-go", "#!/bin/sh\n"+makeGoBuildRedirect(t)+"exec go \"$@\"\n")
	return root
}

func makeGoBuildRedirect(t *testing.T) string {
	t.Helper()
	// Build the actual apicover command, with its output still in the fixture.
	module := filepath.Join(workflowRepoRoot(t), "go")
	return "if [ \"$1\" = build ]; then exec go -C '" + strings.ReplaceAll(module, "'", "'\"'\"'") + "' \"$@\"; fi\n"
}

func writeMakeFixture(t *testing.T, root, name, body string) {
	t.Helper()
	p := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func runMakeFixture(t *testing.T, root, target string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	return runMakeFixtureContext(t, ctx, root, target)
}

func runMakeFixtureContext(t *testing.T, ctx context.Context, root, target string) (string, error) {
	t.Helper()
	cmd := exec.CommandContext(ctx, "make", "--no-print-directory", "-s", "-f", filepath.Join(workflowRepoRoot(t), "go/Makefile"), "GO="+filepath.Join(root, "probe-go"), "BIN_DIR="+filepath.Join(root, "bin"), target)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=", "APICOVER_PKGS=", "MAKEFLAGS=", "PATH="+root+string(os.PathListSeparator)+os.Getenv("PATH"))
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("fixture execution canceled: %v\n%s", ctx.Err(), out)
	}
	return string(out), err
}
