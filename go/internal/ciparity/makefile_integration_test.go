//go:build integration

package ciparity

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

// These tests exercise the real Make recipes and Go coverage tools. A percentage
// alone is not a verdict: Go writes coverage even when a test assertion fails.
func TestMakeCoverageGates_PropagateTestFailure(t *testing.T) {
	t.Parallel()
	for _, failing := range []bool{false, true} {
		name := "passing"
		if failing {
			name = "failing_with_full_coverage"
		}
		t.Run("apicover-enforce/"+name, func(t *testing.T) {
			root := probeFixture(t, failing)
			out, err := runMakeFixture(t, root, "apicover-enforce")
			if (err != nil) != failing {
				t.Fatalf("apicover-enforce: error = %v, want failure %v\n%s", err, failing, out)
			}
		})
		t.Run("test-integration+cover-strict/"+name, func(t *testing.T) {
			root := probeFixture(t, failing)
			out, err := runMakeFixture(t, root, "test-integration")
			if (err != nil) != failing {
				t.Fatalf("test-integration: error = %v, want failure %v\n%s", err, failing, out)
			}
			strictOut, strictErr := runMakeFixture(t, root, "cover-strict")
			if (strictErr != nil) != failing {
				t.Fatalf("cover-strict after test-integration: error = %v, want failure %v\n%s", strictErr, failing, strictOut)
			}
			if failing && !strings.Contains(strictOut, "no coverage profile") {
				t.Errorf("a failed run left a profile for cover-strict to read, or the refusal is unclear:\n%s", strictOut)
			}
		})
	}
}

func probeFixture(t *testing.T, failing bool) string {
	t.Helper()
	root := makeFixture(t)
	body := `if Value() != 1 { t.Fatal("wrong value") }`
	if failing {
		body += `; t.Fatal("intentional assertion failure")`
	}
	writeMakeFixture(t, root, "internal/probe/probe.go", "package probe\nfunc Value() int { return 1 }\n")
	writeMakeFixture(t, root, "internal/probe/probe_test.go", "package probe\nimport \"testing\"\nfunc TestValue(t *testing.T) { "+body+" }\n")
	return root
}

func requireMake(t *testing.T, root, target string) {
	t.Helper()
	if out, err := runMakeFixture(t, root, target); err != nil {
		t.Fatalf("%s: %v\n%s", target, err, out)
	}
}

func TestMakeCoverageGates_RejectToolFailure(t *testing.T) {
	t.Parallel()
	for _, stage := range []string{"test", "tool", "list"} {
		t.Run(stage, func(t *testing.T) {
			root := makeFixture(t)
			writeMakeFixture(t, root, "internal/probe/probe.go", "package probe\nfunc Value() int { return 1 }\n")
			writeMakeFixture(t, root, "internal/probe/probe_test.go", "package probe\nimport \"testing\"\nfunc TestValue(t *testing.T) { if Value() != 1 { t.Fatal(\"wrong value\") } }\n")
			// Run the real tool first so the failure leaves plausible output/profile.
			// The gate must honor the exit status instead of accepting that output.
			wrapper := "#!/bin/sh\nif [ \"$1\" = \"" + stage + "\" ]; then go \"$@\"; echo intentional-tool-failure >&2; exit 23; fi\n" + makeGoBuildRedirect(t) + "exec go \"$@\"\n"
			fakeclitest.Install(t, filepath.Join(root, "probe-go"), wrapper)
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
	t.Parallel()
	root := makeFixture(t)
	writeMakeFixture(t, root, "internal/probe/probe.go", "package probe\nfunc Value(b bool) int { if b { return 1 }; return 2 }\n")
	writeMakeFixture(t, root, "internal/probe/probe_test.go", "package probe\nimport \"testing\"\nfunc TestValue(t *testing.T) { if Value(true) != 1 { t.Fatal(\"wrong value\") } }\n")
	requireMake(t, root, "test-integration")
	out, err := runMakeFixture(t, root, "cover-strict")
	if err == nil || !strings.Contains(out, "FAIL") {
		t.Fatalf("partially covered package passed its 100%% floor: %v\n%s", err, out)
	}
}

func TestMakeAPIEnforce_ExcludesCyclePredicatesFromMeasurement(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	root := makeFixture(t)
	writeMakeFixture(t, root, "internal/probe/probe.go", "package probe\nfunc Value(b bool) int { if b { return 1 }; return 2 }\n")
	writeMakeFixture(t, root, "internal/probe/probe_test.go", "package probe\nimport \"testing\"\nfunc TestValue(t *testing.T) { if Value(true) != 1 { t.Fatal(\"wrong value\") } }\n")
	writeMakeFixture(t, root, ".cover-strict", "# first\n./internal/probe 100")
	requireMake(t, root, "test-integration")
	if out, err := runMakeFixture(t, root, "cover-strict"); err == nil {
		t.Fatalf("gate skipped final manifest entry\n%s", out)
	}
}

func TestMakeCoverageGates_RejectMissingManifest(t *testing.T) {
	t.Parallel()
	for _, target := range []string{"cover-strict", "apicover-enforce"} {
		t.Run(target, func(t *testing.T) {
			root := makeFixture(t)
			if err := os.Remove(filepath.Join(root, "."+target)); err != nil {
				t.Fatal(err)
			}
			out, err := runMakeFixture(t, root, target)
			if err == nil {
				t.Fatalf("gate accepted missing manifest\n%s", out)
			}
			if target == "cover-strict" && !strings.Contains(out, "floors file") {
				t.Fatalf("cover-strict does not name the missing floors file\n%s", out)
			}
		})
	}
}

func TestMakeCoverStrict_RejectsManifestDirectory(t *testing.T) {
	t.Parallel()
	root := makeFixture(t)
	manifest := filepath.Join(root, ".cover-strict")
	if err := os.Remove(manifest); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(manifest, 0o755); err != nil {
		t.Fatal(err)
	}
	out, err := runMakeFixture(t, root, "cover-strict")
	if err == nil || !strings.Contains(out, "floors file") {
		t.Fatalf("gate accepted a directory as its manifest: %v\n%s", err, out)
	}
}

func TestMakeFixture_CancellationFailsHarness(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	for _, fault := range []string{"invalid_floor", "floor_over_100", "failed_evaluator"} {
		t.Run(fault, func(t *testing.T) {
			root := probeFixture(t, false)
			requireMake(t, root, "test-integration")
			want := "floors line 1"
			switch fault {
			case "invalid_floor":
				writeMakeFixture(t, root, ".cover-strict", "./internal/probe invalid\n")
			case "floor_over_100":
				writeMakeFixture(t, root, ".cover-strict", "./internal/probe 101\n")
			default:
				want = "intentional-evaluator-failure"
				fakeclitest.Install(t, filepath.Join(root, "probe-go"), "#!/bin/sh\nif [ \"$1\" = run ]; then echo intentional-evaluator-failure >&2; exit 23; fi\nexec go \"$@\"\n")
			}
			out, err := runMakeFixture(t, root, "cover-strict")
			if err == nil || !strings.Contains(out, want) {
				t.Fatalf("gate accepted %s or lost its diagnostic %q: %v\n%s", fault, want, err, out)
			}
		})
	}
}

func TestMakeCoverStrict_RefusesAMissingOrStaleProfile(t *testing.T) {
	t.Parallel()
	root := probeFixture(t, false)
	out, err := runMakeFixture(t, root, "cover-strict")
	if err == nil || !strings.Contains(out, "no coverage profile") {
		t.Fatalf("cover-strict without a profile: %v\n%s", err, out)
	}
	requireMake(t, root, "test-integration")
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(filepath.Join(root, "internal/probe/probe.go"), later, later); err != nil {
		t.Fatal(err)
	}
	out, err = runMakeFixture(t, root, "cover-strict")
	if err == nil || !strings.Contains(out, "older than") {
		t.Fatalf("cover-strict over a profile older than the source: %v\n%s", err, out)
	}
}

func TestMakeIntegration_LeavesAProfileOnlyAfterAGreenRun(t *testing.T) {
	t.Parallel()
	root := probeFixture(t, true)
	writeMakeFixture(t, root, "coverage.txt", "mode: atomic\nstale profile of an earlier run\n")
	if out, err := runMakeFixture(t, root, "test-integration"); err == nil {
		t.Fatalf("known-red suite passed\n%s", out)
	}
	for _, name := range []string{"coverage.txt", "coverage.partial.txt"} {
		if _, err := os.Stat(filepath.Join(root, name)); !os.IsNotExist(err) {
			t.Errorf("%s exists after a failed run (stat error %v)", name, err)
		}
	}
	writeMakeFixture(t, root, "internal/probe/probe_test.go", "package probe\nimport \"testing\"\nfunc TestValue(t *testing.T) { if Value() != 1 { t.Fatal(\"wrong value\") } }\n")
	requireMake(t, root, "test-integration")
	got, err := os.ReadFile(filepath.Join(root, "coverage.txt"))
	if err != nil || !strings.HasPrefix(string(got), "mode: atomic") || !strings.Contains(string(got), "gateprobe/internal/probe/probe.go") {
		t.Errorf("coverage.txt after a green run = %q, %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(root, "coverage.partial.txt")); !os.IsNotExist(err) {
		t.Errorf("coverage.partial.txt remains after a green run (stat error %v)", err)
	}
}

func TestMakeIntegration_SelectsEntireRuntime(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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

func TestMakeTestRecipes_RunGitWithBackgroundMaintenanceOff(t *testing.T) {
	for _, entry := range gittest.ConfigEnv() {
		key, _, _ := strings.Cut(entry, "=")
		t.Setenv(key, "")
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	probe := gitMaintenanceProbe + fmt.Sprintf("\nvar maintenanceConfig = %#v\n", gittest.MaintenanceConfig())
	for _, target := range []string{"test", "test-integration"} {
		t.Run(target, func(t *testing.T) {
			root := makeFixture(t)
			writeMakeFixture(t, root, "internal/probe/probe_test.go", probe)
			if out, err := runMakeFixture(t, root, target); err != nil {
				t.Fatalf("%s runs git with background maintenance on: %v\n%s", target, err, out)
			}
		})
	}
}

const gitMaintenanceProbe = `package probe

import (
	"os/exec"
	"strings"
	"testing"
)

func TestGitMaintenanceIsOff(t *testing.T) {
	for _, kv := range maintenanceConfig {
		out, err := exec.Command("git", "config", "--get", kv[0]).Output()
		if got := strings.TrimSpace(string(out)); err != nil || got != kv[1] {
			t.Errorf("git config %s = %q (%v), want %q", kv[0], got, err, kv[1])
		}
	}
}
`

func makeFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeMakeFixture(t, root, "go.mod", "module gateprobe\n\ngo 1.23\n")
	writeMakeFixture(t, root, ".cover-strict", "./internal/probe 100\n")
	writeMakeFixture(t, root, ".apicover-enforce", "./internal/probe\n")
	fakeclitest.Install(t, filepath.Join(root, "probe-go"), "#!/bin/sh\n"+makeGoBuildRedirect(t)+makeGoRunRedirect(t)+"exec go \"$@\"\n")
	return root
}

func makeGoBuildRedirect(t *testing.T) string {
	t.Helper()
	// Build the actual apicover command, with its output still in the fixture.
	module := filepath.Join(workflowRepoRoot(t), "go")
	return "if [ \"$1\" = build ]; then exec go -C '" + strings.ReplaceAll(module, "'", "'\"'\"'") + "' \"$@\"; fi\n"
}

func makeGoRunRedirect(t *testing.T) string {
	t.Helper()
	module := filepath.Join(workflowRepoRoot(t), "go")
	return "if [ \"$1\" = run ]; then shift 2; exec go -C '" + strings.ReplaceAll(module, "'", "'\"'\"'") + "' run ./cmd/evolve \"$@\" --root \"$PWD\"; fi\n"
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

func runMakeFixture(t *testing.T, root, target string, vars ...string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	return runMakeFixtureContext(t, ctx, root, target, vars...)
}

func runMakeFixtureContext(t *testing.T, ctx context.Context, root, target string, vars ...string) (string, error) {
	t.Helper()
	args := append([]string{"--no-print-directory", "-s", "-f", filepath.Join(workflowRepoRoot(t), "go/Makefile"), "GO=" + filepath.Join(root, "probe-go"), "BIN_DIR=" + filepath.Join(root, "bin")}, vars...)
	cmd := exec.CommandContext(ctx, "make", append(args, target)...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=", "APICOVER_PKGS=", "MAKEFLAGS=", "PATH="+root+string(os.PathListSeparator)+os.Getenv("PATH"))
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("fixture execution canceled: %v\n%s", ctx.Err(), out)
	}
	return string(out), err
}

func TestMakeAPIEnforce_LeavesTheIntegrationProfileForCoverChanged(t *testing.T) {
	t.Parallel()
	root := makeFixture(t)
	writeMakeFixture(t, root, "internal/probe/probe.go", "package probe\nfunc Value() int { return 1 }\n")
	writeMakeFixture(t, root, "internal/probe/probe_test.go", "package probe\nimport \"testing\"\nfunc TestValue(t *testing.T) { if Value() != 1 { t.Fatal(\"wrong value\") } }\n")
	integration := "mode: atomic\ngateprobe/internal/probe/probe.go:2.20,2.30 1 7\n"
	writeMakeFixture(t, root, "coverage.txt", integration)

	out, err := runMakeFixture(t, root, "apicover-enforce")

	if err != nil {
		t.Fatalf("apicover-enforce: %v\n%s", err, out)
	}
	if got, _ := os.ReadFile(filepath.Join(root, "coverage.txt")); string(got) != integration {
		t.Errorf("apicover-enforce rewrote coverage.txt, the profile cover-changed reads:\n%s", got)
	}
	if got, err := os.ReadFile(filepath.Join(root, "coverage.apicover.txt")); err != nil || !strings.HasPrefix(string(got), "mode: ") {
		t.Errorf("coverage.apicover.txt = %q, %v; want the scoped profile there", got, err)
	}
}

func TestMakeApicover_NeverFeedsTheCoverageGates(t *testing.T) {
	t.Parallel()
	root := probeFixture(t, false)
	scoped := "APICOVER_PKGS=./internal/probe"
	if out, err := runMakeFixture(t, root, "apicover", scoped); err != nil {
		t.Fatalf("apicover: %v\n%s", err, out)
	}
	if _, err := os.Stat(filepath.Join(root, "coverage.txt")); !os.IsNotExist(err) {
		t.Errorf("apicover wrote coverage.txt (stat error %v)", err)
	}
	if got, err := os.ReadFile(filepath.Join(root, "coverage.apicover.txt")); err != nil || !strings.HasPrefix(string(got), "mode: ") {
		t.Errorf("coverage.apicover.txt = %q, %v; want the scoped profile there", got, err)
	}
	if out, err := runMakeFixture(t, root, "cover-strict"); err == nil || !strings.Contains(out, "no coverage profile") {
		t.Fatalf("cover-strict trusted a profile that apicover left: %v\n%s", err, out)
	}
	requireMake(t, root, "test-integration")
	before, err := os.ReadFile(filepath.Join(root, "coverage.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if out, err := runMakeFixture(t, root, "apicover", scoped); err != nil {
		t.Fatalf("apicover after test-integration: %v\n%s", err, out)
	}
	if after, _ := os.ReadFile(filepath.Join(root, "coverage.txt")); string(after) != string(before) {
		t.Errorf("apicover rewrote the integration profile:\n%s", after)
	}
}

func TestMakeIntegration_PassesTheTestTimeout(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		vars []string
		want string
	}{
		{"default", nil, "-timeout 20m"},
		{"override", []string{"TEST_TIMEOUT=7m"}, "-timeout 7m"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := probeFixture(t, false)
			argLog := filepath.Join(root, "go-test-args.txt")
			wrapper := "#!/bin/sh\nif [ \"$1\" = test ]; then echo \"$*\" > '" + argLog + "'; fi\n" + makeGoBuildRedirect(t) + makeGoRunRedirect(t) + "exec go \"$@\"\n"
			fakeclitest.Install(t, filepath.Join(root, "probe-go"), wrapper)
			out, err := runMakeFixture(t, root, "test-integration", tc.vars...)
			if err != nil {
				t.Fatalf("test-integration: %v\n%s", err, out)
			}
			got, err := os.ReadFile(argLog)
			if err != nil || !strings.Contains(string(got), tc.want) {
				t.Fatalf("go test arguments = %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}

func TestMakeIntegration_TeesAJSONStreamWithoutLosingTheVerdict(t *testing.T) {
	t.Parallel()
	for _, failing := range []bool{false, true} {
		name := "green"
		if failing {
			name = "red"
		}
		t.Run(name, func(t *testing.T) {
			root := probeFixture(t, failing)
			stream := filepath.Join(root, "stream.json")
			out, err := runMakeFixture(t, root, "test-integration", "TEST_JSON="+stream)
			if (err != nil) != failing {
				t.Fatalf("test-integration with TEST_JSON: error = %v, want failure %v\n%s", err, failing, out)
			}
			if strings.Contains(out, `{"Action"`) {
				t.Errorf("the log carries JSON events; want the plain go test text:\n%s", out)
			}
			if failing && (!strings.Contains(out, "--- FAIL: TestValue") || !strings.Contains(out, "FAIL\tgateprobe/internal/probe")) {
				t.Errorf("the log of a red run lacks the plain failure lines the CI classifier reads:\n%s", out)
			}
			if !failing && !strings.Contains(out, "ok  \tgateprobe/internal/probe") {
				t.Errorf("the log of a green run lacks the plain ok line:\n%s", out)
			}
			got, readErr := os.ReadFile(stream)
			action := `"Action":"pass"`
			if failing {
				action = `"Action":"fail"`
			}
			if readErr != nil || !strings.Contains(string(got), action) || !strings.Contains(string(got), `"Package":"gateprobe/internal/probe"`) {
				t.Fatalf("stream %q, %v; want a go test -json stream holding %s", got, readErr, action)
			}
			_, statErr := os.Stat(filepath.Join(root, "coverage.txt"))
			if failing == (statErr == nil) {
				t.Errorf("coverage.txt presence = %v after a %s run", statErr == nil, name)
			}
		})
	}
}
