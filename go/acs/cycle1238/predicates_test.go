//go:build acs

package cycle1238

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/reachabilityprobe"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const violationCode = "unreachable_frozen_pin"

const fixtureModulePath = "example.com/fixture"

const (
	cyclicFrozenTest    = "go/internal/core/frozen_cyclic_test.go"
	reachableFrozenTest = "go/internal/core/frozen_reachable_test.go"
)

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

var (
	evolveBinOnce sync.Once
	evolveBinPath string
	evolveBinErr  error
)

func evolveBinary(t *testing.T) string {
	t.Helper()
	evolveBinOnce.Do(func() {
		dir, err := os.MkdirTemp("", "cycle1238-evolve-bin")
		if err != nil {
			evolveBinErr = err
			return
		}
		bin := filepath.Join(dir, "evolve")
		cmd := exec.Command("go", "build", "-C", goDir(t), "-o", bin, "./cmd/evolve")
		if out, err := cmd.CombinedOutput(); err != nil {
			evolveBinErr = err
			t.Logf("go build ./cmd/evolve output:\n%s", out)
			return
		}
		evolveBinPath = bin
	})
	if evolveBinErr != nil {
		t.Fatalf("building the evolve CLI failed: %v", evolveBinErr)
	}
	return evolveBinPath
}

func writeFile(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func fixtureWorktree(t *testing.T) string {
	t.Helper()
	wt := t.TempDir()

	writeFile(t, wt, "go/go.mod", "module "+fixtureModulePath+"\n\ngo 1.23\n")
	writeFile(t, wt, "go/internal/core/state.go",
		"package core\n\n// State is the fixture core type.\ntype State struct{}\n")
	writeFile(t, wt, "go/internal/storage/storage.go",
		"package storage\n\nimport \""+fixtureModulePath+"/internal/core\"\n\n"+
			"// UpdateStateMap is the cycle-644 symbol: storage already imports core.\n"+
			"func UpdateStateMap(s *core.State) {}\n")
	writeFile(t, wt, "go/internal/leafutil/leafutil.go",
		"package leafutil\n\n// Helper imports nothing — pinning it closes no cycle.\n"+
			"func Helper() {}\n")

	writeFile(t, wt, cyclicFrozenTest,
		"package core\n\nimport \"testing\"\n\n"+
			"// TestC644_StateUsesStorage is the frozen structural pin.\n"+
			"func TestC644_StateUsesStorage(t *testing.T) {\n"+
			"\tassertFileContains(t, \"go/internal/core/state.go\", \"storage.UpdateStateMap(\")\n"+
			"}\n")
	writeFile(t, wt, reachableFrozenTest,
		"package core\n\nimport \"testing\"\n\n"+
			"// TestReachable_StateUsesLeafutil is a benign structural pin.\n"+
			"func TestReachable_StateUsesLeafutil(t *testing.T) {\n"+
			"\tassertFileContains(t, \"go/internal/core/state.go\", \"leafutil.Helper(\")\n"+
			"}\n")
	return wt
}

func fixtureWorkspace(t *testing.T, doNotModifyTests bool, frozen ...string) string {
	t.Helper()
	ws := t.TempDir()
	handoff := map[string]any{
		"testFiles":               frozen,
		"redRunConfirmed":         true,
		"allTestsMustPassForShip": true,
		"doNotModifyTests":        doNotModifyTests,
	}
	blob, err := json.MarshalIndent(handoff, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	report := "# TDD Report — cycle1238 fixture\n\n" +
		"## AC-Materialization\n\n| Criterion | Test | Status |\n|---|---|---|\n| fixture | fixture | RED |\n\n" +
		"## RED Run Output\n\n```\nfixture RED output\n```\n\n" +
		"## Handoff to Builder\n\n```json\n" + string(blob) + "\n```\n"
	writeFile(t, ws, "test-report.md", report)
	return ws
}

func verifyTDD(t *testing.T, workspace, worktree string) (int, string) {
	t.Helper()
	cmd := exec.Command(evolveBinary(t), "phase", "verify", "tdd",
		"--workspace", workspace, "--worktree", worktree)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	cmd.Stdout = &strings.Builder{}
	err := cmd.Run()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("running `evolve phase verify tdd`: %v\nstderr:\n%s", err, stderr.String())
	}
	return code, stderr.String()
}

func TestC1238_001_CLIFlagsCycle644FrozenPin(t *testing.T) {
	wt := fixtureWorktree(t)
	ws := fixtureWorkspace(t, true, cyclicFrozenTest)

	code, stderr := verifyTDD(t, ws, wt)
	if code != 1 {
		t.Fatalf("RED: `evolve phase verify tdd` exited %d, want 1 (a confirmed"+
			" violation) for the cycle-644 shape — a frozen test pinning"+
			" storage.UpdateStateMap( inside package core while storage already"+
			" imports core.\nstderr:\n%s", code, stderr)
	}
	if !strings.Contains(stderr, violationCode) {
		t.Errorf("RED: stderr does not name the stable violation code %q, so the"+
			" agent cannot tell WHICH gate failed.\nstderr:\n%s", violationCode, stderr)
	}
	for _, want := range []string{"storage", "core", "UpdateStateMap"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("RED: stderr omits %q — the diagnostic must name the pinning"+
				" package, the referenced package and the symbol so the fix is"+
				" actionable.\nstderr:\n%s", want, stderr)
		}
	}
}

func TestC1238_002_CLIPassesReachablePin(t *testing.T) {
	wt := fixtureWorktree(t)
	ws := fixtureWorkspace(t, true, reachableFrozenTest)

	code, stderr := verifyTDD(t, ws, wt)
	if code != 0 {
		t.Fatalf("RED: `evolve phase verify tdd` exited %d, want 0 — pinning"+
			" leafutil.Helper( inside package core closes no cycle (leafutil"+
			" imports nothing) and must pass unchanged.\nstderr:\n%s", code, stderr)
	}
}

func TestC1238_003_GateScopeAndFailOpen(t *testing.T) {
	t.Run("unfrozen_handoff_does_not_fire", func(t *testing.T) {
		wt := fixtureWorktree(t)
		ws := fixtureWorkspace(t, false, cyclicFrozenTest)
		if code, stderr := verifyTDD(t, ws, wt); code != 0 {
			t.Errorf("RED: exit %d, want 0 — doNotModifyTests:false means the pin"+
				" is not frozen and the gate must stay silent.\nstderr:\n%s", code, stderr)
		}
	})

	t.Run("no_go_module_fails_open", func(t *testing.T) {
		wt := t.TempDir()
		ws := fixtureWorkspace(t, true, cyclicFrozenTest)
		if code, stderr := verifyTDD(t, ws, wt); code != 0 {
			t.Errorf("RED: exit %d, want 0 — an underivable import graph is infra"+
				" ambiguity and must fail OPEN, never a confirmed violation."+
				"\nstderr:\n%s", code, stderr)
		}
	})
}

func TestC1238_004_LibrarySeam(t *testing.T) {
	wt := fixtureWorktree(t)

	t.Run("frozen_test_files_honours_the_freeze_flag", func(t *testing.T) {
		frozenWS := fixtureWorkspace(t, true, cyclicFrozenTest)
		got, err := reachabilityprobe.FrozenTestFiles(filepath.Join(frozenWS, "test-report.md"))
		if err != nil {
			t.Fatalf("FrozenTestFiles(frozen report) returned error: %v", err)
		}
		if len(got) != 1 || got[0] != cyclicFrozenTest {
			t.Errorf("FrozenTestFiles = %v, want [%s]", got, cyclicFrozenTest)
		}

		unfrozenWS := fixtureWorkspace(t, false, cyclicFrozenTest)
		got, err = reachabilityprobe.FrozenTestFiles(filepath.Join(unfrozenWS, "test-report.md"))
		if err != nil {
			t.Fatalf("FrozenTestFiles(unfrozen report) returned error: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("FrozenTestFiles(doNotModifyTests:false) = %v, want no files", got)
		}
	})

	t.Run("extract_resolves_pinning_package_and_symbol", func(t *testing.T) {
		pins, err := reachabilityprobe.ExtractFrozenPins(wt, []string{cyclicFrozenTest})
		if err != nil {
			t.Fatalf("ExtractFrozenPins returned error: %v", err)
		}
		want := reachabilityprobe.CallSite{
			PinningPackage:    fixtureModulePath + "/internal/core",
			ReferencedPackage: "storage",
			Symbol:            "UpdateStateMap",
		}
		found := false
		for _, p := range pins {
			if p == want {
				found = true
			}
		}
		if !found {
			t.Errorf("ExtractFrozenPins = %+v, want it to contain %+v — the pin"+
				" names go/internal/core/state.go, so the pinning package is the"+
				" package owning THAT file, not the test's own package", pins, want)
		}
	})

	t.Run("check_separates_cyclic_from_reachable", func(t *testing.T) {
		bad, err := reachabilityprobe.CheckFrozenPins(wt, []string{cyclicFrozenTest})
		if err != nil {
			t.Fatalf("CheckFrozenPins(cyclic) returned error: %v", err)
		}
		if len(bad) != 1 {
			t.Fatalf("CheckFrozenPins(cyclic) = %d violation(s), want exactly 1", len(bad))
		}
		if len(bad[0].Cycle) == 0 {
			t.Error("Violation.Cycle is empty — the proving import chain must be reported")
		}

		ok, err := reachabilityprobe.CheckFrozenPins(wt, []string{reachableFrozenTest})
		if err != nil {
			t.Fatalf("CheckFrozenPins(reachable) returned error: %v", err)
		}
		if len(ok) != 0 {
			t.Errorf("CheckFrozenPins(reachable) = %+v, want no violations", ok)
		}
	})
}

func TestC1238_006_PermanentRegressionGuard(t *testing.T) {
	out, _, code, err := acsassert.SubprocessOutput(
		"go", "test", "-C", goDir(t), "-count=1", "-v",
		"-run", "TestPhaseVerifyTDD_FrozenPin", "./internal/cli/phasecmd")
	if err != nil || code != 0 {
		t.Fatalf("RED: the permanent phasecmd regression guard does not pass"+
			" (exit=%d, err=%v):\n%s", code, err, tailLines(out, 40))
	}
	if !strings.Contains(out, "--- PASS: TestPhaseVerifyTDD_FrozenPin") {
		t.Errorf("RED: no `--- PASS: TestPhaseVerifyTDD_FrozenPin...` line in the"+
			" output — Builder must add the permanent regression guard to package"+
			" phasecmd, not only the cycle-scoped acs predicates.\nOut (tail):\n%s",
			tailLines(out, 40))
	}
}

func tailLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

func TestC1238_005_ApicoverNamedCoverage(t *testing.T) {
	out, _, code, err := acsassert.SubprocessOutput(
		"go", "test", "-C", goDir(t), "-count=1", "-v",
		"-run", "TestExported", "./internal/reachabilityprobe")
	if err != nil || code != 0 {
		t.Fatalf("RED: the apicover named tests for internal/reachabilityprobe do"+
			" not pass (exit=%d, err=%v):\n%s", code, err, out)
	}
	if !strings.Contains(out, "PASS") {
		t.Fatalf("RED: no PASS in the named-test output:\n%s", out)
	}

	named := filepath.Join(goDir(t), "internal", "reachabilityprobe", "apicover_named_test.go")
	body, readErr := os.ReadFile(named)
	if readErr != nil {
		t.Fatalf("reading %s: %v", named, readErr)
	}
	for _, sym := range []string{"FrozenTestFiles", "ExtractFrozenPins", "CheckFrozenPins"} {
		if !strings.Contains(string(body), sym) {
			t.Errorf("RED: %s does not name %s — an enrolled package's new export"+
				" must be named and exercised there or the repo-wide apicover gate"+
				" fails the tree (ADR-0069).", filepath.Base(named), sym)
		}
	}
}
