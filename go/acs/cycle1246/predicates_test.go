//go:build acs

package cycle1246

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const violationCode = "unreachable_frozen_pin"

const fixtureModulePath = "example.com/fixture"

const (
	aliasCyclicFrozenTest      = "go/internal/core/frozen_alias_cyclic_test.go"
	aliasReachableFrozenTest   = "go/internal/core/frozen_alias_reachable_test.go"
	aliasUnboundFrozenTest     = "go/internal/core/frozen_alias_unbound_test.go"
	aliasBlankImportFrozenTest = "go/internal/core/frozen_alias_blank_test.go"
	plainCyclicFrozenTest      = "go/internal/core/frozen_plain_cyclic_test.go"
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
		dir, err := os.MkdirTemp("", "cycle1246-evolve-bin")
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

	writeFile(t, wt, aliasCyclicFrozenTest,
		"package core\n\nimport (\n\t\"testing\"\n\n"+
			"\tst \""+fixtureModulePath+"/internal/storage\"\n)\n\n"+
			"// TestC644_AliasedStateUsesStorage is the frozen structural pin.\n"+
			"func TestC644_AliasedStateUsesStorage(t *testing.T) {\n"+
			"\t_ = st.UpdateStateMap\n"+
			"\tassertFileContains(t, \"go/internal/core/state.go\", \"st.UpdateStateMap(\")\n"+
			"}\n")

	writeFile(t, wt, aliasReachableFrozenTest,
		"package core\n\nimport (\n\t\"testing\"\n\n"+
			"\tlf \""+fixtureModulePath+"/internal/leafutil\"\n)\n\n"+
			"// TestAliasedReachable is a benign aliased structural pin.\n"+
			"func TestAliasedReachable(t *testing.T) {\n"+
			"\t_ = lf.Helper\n"+
			"\tassertFileContains(t, \"go/internal/core/state.go\", \"lf.Helper(\")\n"+
			"}\n")

	writeFile(t, wt, aliasUnboundFrozenTest,
		"package core\n\nimport \"testing\"\n\n"+
			"// TestUnboundIdentifier pins an identifier no import binds.\n"+
			"func TestUnboundIdentifier(t *testing.T) {\n"+
			"\tassertFileContains(t, \"go/internal/core/state.go\", \"zz.Whatever(\")\n"+
			"}\n")

	writeFile(t, wt, aliasBlankImportFrozenTest,
		"package core\n\nimport (\n\t\"testing\"\n\n"+
			"\t_ \""+fixtureModulePath+"/internal/storage\"\n)\n\n"+
			"// TestBlankImport pins through a blank-imported package.\n"+
			"func TestBlankImport(t *testing.T) {\n"+
			"\tassertFileContains(t, \"go/internal/core/state.go\", \"_.UpdateStateMap(\")\n"+
			"}\n")

	writeFile(t, wt, plainCyclicFrozenTest,
		"package core\n\nimport \"testing\"\n\n"+
			"// TestPlainCyclic is the unaliased cycle-644 shape.\n"+
			"func TestPlainCyclic(t *testing.T) {\n"+
			"\tassertFileContains(t, \"go/internal/core/state.go\", \"storage.UpdateStateMap(\")\n"+
			"}\n")

	return wt
}

func fixtureWorkspace(t *testing.T, frozen ...string) string {
	t.Helper()
	ws := t.TempDir()
	handoff := map[string]any{
		"testFiles":               frozen,
		"redRunConfirmed":         true,
		"allTestsMustPassForShip": true,
		"doNotModifyTests":        true,
	}
	blob, err := json.MarshalIndent(handoff, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	report := "# TDD Report — cycle1246 fixture\n\n" +
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

func TestC1246_001_AliasedPinFlaggedOnLiveCLI(t *testing.T) {
	wt := fixtureWorktree(t)
	ws := fixtureWorkspace(t, aliasCyclicFrozenTest)

	code, stderr := verifyTDD(t, ws, wt)
	if code != 1 {
		t.Fatalf("RED: `evolve phase verify tdd` exited %d, want 1 (a confirmed"+
			" violation). The frozen test pins st.UpdateStateMap( inside package"+
			" core and binds `st` to %s/internal/storage in its own import block,"+
			" while storage already imports core — the cycle-644 shape, merely"+
			" spelled through an alias.\nstderr:\n%s", code, fixtureModulePath, stderr)
	}
	if !strings.Contains(stderr, violationCode) {
		t.Errorf("RED: stderr does not name the stable violation code %q, so the"+
			" agent cannot tell WHICH gate failed.\nstderr:\n%s", violationCode, stderr)
	}
	for _, want := range []string{"internal/storage", "internal/core", "UpdateStateMap"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("RED: stderr omits %q — the diagnostic must name the RESOLVED"+
				" referenced package, the pinning package and the symbol, so the fix"+
				" is actionable (reporting the bare alias `st` is not).\nstderr:\n%s",
				want, stderr)
		}
	}
}

func TestC1246_002_AliasedBenignPinPassesLiveCLI(t *testing.T) {
	wt := fixtureWorktree(t)
	ws := fixtureWorkspace(t, aliasReachableFrozenTest)

	if code, stderr := verifyTDD(t, ws, wt); code != 0 {
		t.Fatalf("`evolve phase verify tdd` exited %d, want 0 — `lf` binds"+
			" internal/leafutil, which imports nothing, so the aliased pin closes"+
			" no cycle and must pass unchanged.\nstderr:\n%s", code, stderr)
	}
}

func TestC1246_003_AliasEdgeCasesFailOpen(t *testing.T) {
	wt := fixtureWorktree(t)

	t.Run("identifier_bound_by_nothing", func(t *testing.T) {
		ws := fixtureWorkspace(t, aliasUnboundFrozenTest)
		if code, stderr := verifyTDD(t, ws, wt); code != 0 {
			t.Errorf("exit %d, want 0 — `zz` is bound by no import at all, which is"+
				" infra ambiguity, not a compiler-provable cycle.\nstderr:\n%s", code, stderr)
		}
	})

	t.Run("blank_import_binds_no_identifier", func(t *testing.T) {
		ws := fixtureWorkspace(t, aliasBlankImportFrozenTest)
		if code, stderr := verifyTDD(t, ws, wt); code != 0 {
			t.Errorf("exit %d, want 0 — a blank import binds no usable identifier,"+
				" so `_` must never be resolved to internal/storage.\nstderr:\n%s", code, stderr)
		}
	})

	t.Run("unaliased_cyclic_pin_still_flagged", func(t *testing.T) {
		ws := fixtureWorkspace(t, plainCyclicFrozenTest)
		if code, stderr := verifyTDD(t, ws, wt); code != 1 {
			t.Errorf("exit %d, want 1 — the pre-existing unaliased base-name"+
				" resolution must not regress.\nstderr:\n%s", code, stderr)
		}
	})
}

func TestC1246_004_DurableRegressionGuard(t *testing.T) {
	out, _, code, err := acsassert.SubprocessOutput(
		"go", "test", "-C", goDir(t), "-count=1", "-v",
		"-run", "TestCheckFrozenPins_Alias|TestCheckFrozenPins_Unaliased|TestExtractFrozenPins_Alias",
		"./internal/reachabilityprobe")
	if err != nil || code != 0 {
		t.Fatalf("RED: the durable frozenpins alias guard does not pass (exit=%d,"+
			" err=%v):\n%s", code, err, tailLines(out, 40))
	}
	for _, want := range []string{
		"--- PASS: TestCheckFrozenPins_AliasedImportResolved",
		"--- PASS: TestCheckFrozenPins_AliasedImportNoFalsePositive",
		"--- PASS: TestCheckFrozenPins_AliasUnresolvableFailsOpen",
		"--- PASS: TestCheckFrozenPins_UnaliasedPathUnchanged",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("RED: no %q line in the output — the durable guard must keep"+
				" all four cases (crux, false-positive, fail-open, non-regression)"+
				" and they must PASS.\nOut (tail):\n%s", want, tailLines(out, 40))
		}
	}
}

func TestC1246_005_ApicoverNamedCoverage(t *testing.T) {
	out, _, code, err := acsassert.SubprocessOutput(
		"go", "test", "-C", goDir(t), "-count=1", "-v",
		"-run", "TestExported", "./internal/reachabilityprobe")
	if err != nil || code != 0 {
		t.Fatalf("the apicover named tests for internal/reachabilityprobe do not"+
			" pass (exit=%d, err=%v):\n%s", code, err, tailLines(out, 40))
	}
	if !strings.Contains(out, "PASS") {
		t.Fatalf("no PASS in the named-test output:\n%s", tailLines(out, 40))
	}
}

func tailLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
