//go:build acs

// Package cycle1246 materialises the acceptance criteria for this lane's single
// fleet-scoped task, `reachabilityprobe-alias-resolution` (inbox item
// tdd-structural-test-reachability-probe, weight 0.92, root cause cycle-644).
//
// What already landed: the reachabilityprobe library (cycle-1226), the frozen-pin
// derivation and its wiring into `evolve phase verify tdd` (cycle-1238). Triage
// dropped the todo-id's core scope as already-shipped and committed exactly one
// residual: resolvePackage (frozenpins.go:256-271) resolves the bare identifier
// written at a pinned call site by matching path.Base(pkg) == ident. An
// identifier introduced by an IMPORT ALIAS matches no package's base name, so
// the pin resolves to nothing and is silently skipped — a genuine cycle-644
// shape written as
//
//	import st "example.com/fixture/internal/storage"
//	... FileContains(t, "go/internal/core/state.go", "st.UpdateStateMap(")
//
// fails OPEN today, and the permanently unsatisfiable acceptance criterion sails
// through the gate that exists to catch it.
//
// Where the alias binding can live — the design constraint Builder inherits:
// NOT in the pinned production file. If core/state.go already imported storage
// while storage imports core, the module is already cyclic, `go list` refuses to
// produce a graph, and the gate fails open on infra ambiguity by design. The one
// place the binding can exist while the module still lists cleanly is the FROZEN
// TEST FILE carrying the pin (`go list -deps -json` ignores _test.go imports) —
// which is exactly where a structural test that also exercises the symbol
// declares it. So: resolve aliases from the import block of the frozen test file
// the pin was extracted from.
//
// Predicate strategy — every predicate exercises a REAL production path, never a
// source-grep of production code (the cycle-85 degenerate-predicate ban):
//
//   - 001/002/003 drive the REAL operator entry point: a freshly built `evolve`
//     binary running `phase verify tdd --workspace ... --worktree ...` over a
//     real fixture Go module. This is House Rule 2's wiring proof — the library
//     fix must be reachable from the production caller
//     (internal/cli/phasecmd/phase_verify.go:141 withFrozenPinViolations), not
//     only from a unit test.
//   - 001 is the crux REJECTION case (aliased cycle-644 shape must be flagged).
//   - 002 is the false-positive guard: an aliased pin at a package that imports
//     nothing back is buildable and must still pass. Without it, a resolver that
//     flagged every aliased-or-unresolvable identifier would satisfy 001 and turn
//     the tdd gate into a false-HALT generator.
//   - 003 is the edge / fail-open axis: an identifier no import binds, and a
//     blank import (which binds no usable identifier), must both leave the
//     verdict untouched.
//   - 004 requires the DURABLE guard — the acs predicates here vanish with this
//     cycle; go/internal/reachabilityprobe/frozenpins_test.go is what keeps the
//     fix from silently rotting in a later refactor.
//   - 005 is House Rule 1's second half: ./internal/reachabilityprobe is already
//     enrolled in go/.apicover-enforce (line 495), so any NEW exported symbol
//     must be named and executed in its apicover_named_test.go.
//
// RED at authoring time is behavioural, not a compile failure: no new exported
// symbol is required (the fix is internal to resolvePackage/extractPins), so
// 001 and 004 fail on the verdict the CLI and the durable test actually produce
// today.
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

// violationCode is the stable deliverable-violation code the tdd reachability
// gate emits (phase_verify.go:123), so the agent reading stderr knows WHICH gate
// failed. It is part of the contract, not an implementation detail.
const violationCode = "unreachable_frozen_pin"

// fixtureModulePath is the module path of the throwaway fixture module.
const fixtureModulePath = "example.com/fixture"

// Worktree-relative paths of the fixture frozen test files, in the exact form a
// tdd handoff JSON writes them.
const (
	aliasCyclicFrozenTest      = "go/internal/core/frozen_alias_cyclic_test.go"
	aliasReachableFrozenTest   = "go/internal/core/frozen_alias_reachable_test.go"
	aliasUnboundFrozenTest     = "go/internal/core/frozen_alias_unbound_test.go"
	aliasBlankImportFrozenTest = "go/internal/core/frozen_alias_blank_test.go"
	plainCyclicFrozenTest      = "go/internal/core/frozen_plain_cyclic_test.go"
)

// goDir is the Go module root of the tree under audit.
func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

var (
	evolveBinOnce sync.Once
	evolveBinPath string
	evolveBinErr  error
)

// evolveBinary builds the `evolve` CLI from the tree under audit ONCE per test
// binary. A stale go/bin/evolve would prove nothing about THIS diff, so the
// binary is always rebuilt from source.
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

// writeFile materialises rel (slash-separated, relative to root) with body.
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

// fixtureWorktree builds a throwaway worktree whose go/ subdirectory is a real,
// `go list`-resolvable module carrying both shapes the gate must tell apart:
//
//	internal/storage  imports internal/core  → pinning it inside a core file is
//	                                           the cycle-644 shape (a cycle).
//	internal/leafutil imports nothing        → pinning it inside a core file
//	                                           closes no cycle.
//
// Every frozen test file pins a call site into go/internal/core/state.go; they
// differ only in how the referenced package's identifier is bound. The pins are
// REQUIREMENTS on production code, not calls the fixture itself makes, so the
// fixture module stays buildable and `go list` stays clean.
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

	// ALIASED cycle-644 shape: `st` -> internal/storage, bound in this frozen
	// test file's own import block.
	writeFile(t, wt, aliasCyclicFrozenTest,
		"package core\n\nimport (\n\t\"testing\"\n\n"+
			"\tst \""+fixtureModulePath+"/internal/storage\"\n)\n\n"+
			"// TestC644_AliasedStateUsesStorage is the frozen structural pin.\n"+
			"func TestC644_AliasedStateUsesStorage(t *testing.T) {\n"+
			"\t_ = st.UpdateStateMap\n"+
			"\tassertFileContains(t, \"go/internal/core/state.go\", \"st.UpdateStateMap(\")\n"+
			"}\n")

	// ALIASED benign shape: `lf` -> internal/leafutil, which imports nothing.
	writeFile(t, wt, aliasReachableFrozenTest,
		"package core\n\nimport (\n\t\"testing\"\n\n"+
			"\tlf \""+fixtureModulePath+"/internal/leafutil\"\n)\n\n"+
			"// TestAliasedReachable is a benign aliased structural pin.\n"+
			"func TestAliasedReachable(t *testing.T) {\n"+
			"\t_ = lf.Helper\n"+
			"\tassertFileContains(t, \"go/internal/core/state.go\", \"lf.Helper(\")\n"+
			"}\n")

	// UNBOUND identifier: `zz` is bound by nothing ⇒ unresolvable ⇒ fail open.
	writeFile(t, wt, aliasUnboundFrozenTest,
		"package core\n\nimport \"testing\"\n\n"+
			"// TestUnboundIdentifier pins an identifier no import binds.\n"+
			"func TestUnboundIdentifier(t *testing.T) {\n"+
			"\tassertFileContains(t, \"go/internal/core/state.go\", \"zz.Whatever(\")\n"+
			"}\n")

	// BLANK import binds no usable identifier ⇒ `_` must never resolve to
	// internal/storage.
	writeFile(t, wt, aliasBlankImportFrozenTest,
		"package core\n\nimport (\n\t\"testing\"\n\n"+
			"\t_ \""+fixtureModulePath+"/internal/storage\"\n)\n\n"+
			"// TestBlankImport pins through a blank-imported package.\n"+
			"func TestBlankImport(t *testing.T) {\n"+
			"\tassertFileContains(t, \"go/internal/core/state.go\", \"_.UpdateStateMap(\")\n"+
			"}\n")

	// UNALIASED control: the pre-existing behaviour that must not regress.
	writeFile(t, wt, plainCyclicFrozenTest,
		"package core\n\nimport \"testing\"\n\n"+
			"// TestPlainCyclic is the unaliased cycle-644 shape.\n"+
			"func TestPlainCyclic(t *testing.T) {\n"+
			"\tassertFileContains(t, \"go/internal/core/state.go\", \"storage.UpdateStateMap(\")\n"+
			"}\n")

	return wt
}

// fixtureWorkspace writes a WELL-FORMED tdd deliverable freezing the named test
// files. Well-formedness matters: the reachability gate must be the ONLY thing
// that can turn these cases red, never a missing-section gap.
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

// verifyTDD runs the REAL operator command and returns (exitCode, stderr).
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

// TestC1246_001_AliasedPinFlaggedOnLiveCLI is the CRUX and the negative
// (rejection) predicate, asserted on the live production path: a cycle-644 shape
// whose referenced package is named through an import alias must be a CONFIRMED
// violation, exactly as the unaliased spelling already is.
//
// RED today: resolvePackage cannot map `st` to any package, returns ok=false,
// CheckFrozenPins skips the pin, and `evolve phase verify tdd` exits 0 on a
// permanently unsatisfiable acceptance criterion.
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

// TestC1246_002_AliasedBenignPinPassesLiveCLI is the false-positive regression
// guard on the same resolution path. `lf` binds internal/leafutil, which imports
// nothing, so pinning lf.Helper( inside package core closes no cycle and is
// perfectly buildable. A resolver that reported every aliased identifier — or
// every identifier it could not base-name-match — would satisfy 001 and make the
// tdd phase unusable; this predicate forbids that.
func TestC1246_002_AliasedBenignPinPassesLiveCLI(t *testing.T) {
	wt := fixtureWorktree(t)
	ws := fixtureWorkspace(t, aliasReachableFrozenTest)

	if code, stderr := verifyTDD(t, ws, wt); code != 0 {
		t.Fatalf("`evolve phase verify tdd` exited %d, want 0 — `lf` binds"+
			" internal/leafutil, which imports nothing, so the aliased pin closes"+
			" no cycle and must pass unchanged.\nstderr:\n%s", code, stderr)
	}
}

// TestC1246_003_AliasEdgeCasesFailOpen pins the edge / OOD axis: two alias
// shapes that are genuinely ambiguous must leave the verdict untouched, because
// a false HALT in this gate taxes every cycle. It also re-asserts that the
// UNALIASED path is unchanged by the fix (the acceptance-criteria summary's
// explicit no-behaviour-change requirement), on the live CLI.
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

// TestC1246_004_DurableRegressionGuard requires the PERMANENT protection. The
// acs predicates above are cycle-scoped and vanish with this cycle; without an
// ordinary (non-acs) test in package reachabilityprobe, the alias resolution can
// silently rot in a later refactor and nothing in `go test ./...` notices.
//
// The guard is go/internal/reachabilityprobe/frozenpins_test.go, authored this
// phase and frozen: it must pass, and its crux case
// TestCheckFrozenPins_AliasedImportResolved must be among the tests that pass —
// a suite that passes because the crux was deleted proves nothing.
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

// TestC1246_005_ApicoverNamedCoverage is House Rule 1's second half.
// ./internal/reachabilityprobe is ALREADY enrolled in go/.apicover-enforce (line
// 495), so the repo-wide gate fails the tree unless every exported symbol is
// named by identifier in that package's apicover_named_test.go and exercised
// there. The alias fix is expected to be internal (resolvePackage/extractPins),
// but if Builder exports a new seam, this predicate is what makes the enrollment
// obligation bite in the same diff rather than at the repo-wide gate.
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

// tailLines returns the last n lines of s, for readable failure output.
func tailLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
