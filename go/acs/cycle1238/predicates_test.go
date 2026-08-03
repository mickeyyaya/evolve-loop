//go:build acs

// Package cycle1238 materialises the acceptance criteria for this lane's single
// fleet-scoped task, `wire-reachability-gate-into-tdd-verify` (inbox item
// tdd-structural-test-reachability-probe, weight 0.92, root cause cycle-644).
//
// What already landed (scout-report.md): the `reachabilityprobe` library
// (CheckCallSite/BuildImportGraph), the `evolve reachability check-pin`
// subcommand, and the obligation text in agents/evolve-tdd-engineer.md:132.
// What has NOT landed — and is the whole point of this cycle — is a
// DETERMINISTIC caller: nothing in the phase-gate pipeline runs the probe, so
// the cycle-644 shape is caught only if the TDD agent remembers to probe by
// hand. That is the exact LLM judgment lapse cycle-644 already demonstrated.
//
// The cycle-644 shape, restated so the predicates below are readable: a frozen
// (`doNotModifyTests: true`) structural test pinned `storage.UpdateStateMap(`
// as a required call site inside a file belonging to package `core`, while
// `storage` already imported `core`. Satisfying that pin would require
// core -> storage -> core: a compiler-proven import cycle, so the acceptance
// criterion was permanently unsatisfiable and the cycle burned.
//
// Predicate strategy — every predicate exercises a REAL production path, never
// a source-grep of production code (the cycle-85 degenerate-predicate ban):
//
//   - 001/002/003 drive the REAL CLI entry point: a freshly built `evolve`
//     binary running `phase verify tdd --workspace ... --worktree ...` over a
//     real fixture Go module, asserting the exit code and stderr an operator
//     actually sees. This is the wiring proof — a gate reachable only from a
//     unit test is dead code (House Rule 2).
//   - 001 is the crux REJECTION case (cycle-644 shape must be flagged).
//   - 002 is the false-positive regression guard (a reachable pin passes
//     unchanged) — the inbox item's explicit acceptance criterion #3.
//   - 003 is the edge/fail-open axis: an unfrozen handoff and a worktree with
//     no Go module must both leave the verdict untouched.
//   - 004 exercises the new library seam directly (extraction + resolution +
//     handoff parsing), so a CLI regression and a library regression are
//     distinguishable.
//   - 005 is House Rule 1's second half: `internal/reachabilityprobe` is
//     already enrolled in go/.apicover-enforce (line 495), so every NEW
//     exported symbol must be named and executed in its apicover_named_test.go.
//
// RED at authoring time is a COMPILE failure for 004 (the three new exported
// symbols do not exist yet) and a behavioural failure for 001 (the CLI exits 0
// today: `grep -n reachability go/internal/cli/phasecmd/phase_verify.go`
// returns nothing).
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

// violationCode is the stable deliverable-violation code the tdd gate must
// emit, so the agent reading stderr knows what to fix. It is part of the
// contract Builder inherits — not an implementation detail.
const violationCode = "unreachable_frozen_pin"

// fixtureModulePath is the module path of the throwaway module the fixtures
// build. Predicates assert on import paths derived from it.
const fixtureModulePath = "example.com/fixture"

// cyclicFrozenTest / reachableFrozenTest are the worktree-relative paths of the
// two fixture frozen tests, in the exact form a tdd handoff JSON writes them.
const (
	cyclicFrozenTest    = "go/internal/core/frozen_cyclic_test.go"
	reachableFrozenTest = "go/internal/core/frozen_reachable_test.go"
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
// binary and returns its path. Predicates must exercise the real operator
// entry point; a stale go/bin/evolve would prove nothing about THIS diff, so
// the binary is always rebuilt from source.
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

// fixtureWorktree builds a throwaway worktree whose `go/` subdirectory is a
// real, resolvable Go module carrying BOTH shapes the gate must tell apart:
//
//	internal/storage  imports internal/core  → pinning storage.UpdateStateMap(
//	                                           inside a core file is the
//	                                           cycle-644 shape (a cycle).
//	internal/leafutil imports nothing        → pinning leafutil.Helper(
//	                                           inside a core file is fine.
//
// The frozen test files pin their call sites the way this repo's structural
// tests actually do — a source path plus a package-qualified needle on one line
// (the acsassert.FileContains idiom) — which is exactly the cycle-644 artefact:
// the pin is a REQUIREMENT on production code, not a call the test itself
// makes, so the fixture module stays buildable and `go list` stays clean.
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

// fixtureWorkspace writes a WELL-FORMED tdd deliverable naming frozen as the
// frozen test files. Well-formedness matters: the reachability gate must be the
// ONLY thing that can turn these cases red, never a missing-section gap.
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

// TestC1238_001_CLIFlagsCycle644FrozenPin is the CRUX and the negative
// (rejection) predicate: the cycle-644 shape must be a CONFIRMED violation on
// the live CLI path, before the build phase ever starts.
//
// RED today: `evolve phase verify tdd` has no reachability step at all
// (phase_verify.go has zero `reachability` references), so it exits 0 on this
// fixture and the unsatisfiable criterion sails through.
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

// TestC1238_002_CLIPassesReachablePin is the false-positive regression guard
// (inbox acceptance criterion #3): a pin whose referenced package does NOT
// import the pinning package back is perfectly buildable and must pass the
// same gate unchanged. A gate that flags every package-qualified pin would
// satisfy 001 and make the tdd phase unusable — this predicate forbids that.
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

// TestC1238_003_GateScopeAndFailOpen pins the two edge cases that keep the gate
// from becoming a new source of false HALTs:
//
//	(a) doNotModifyTests:false — the tests are NOT frozen, so no pin is a
//	    permanent commitment and the gate must not fire, even on the cycle-644
//	    fixture. This proves the gate keys off the freeze flag rather than
//	    scanning every test it can find.
//	(b) a worktree with no Go module — the import graph is underivable, and an
//	    infra gap must fail OPEN (phase_verify.go's standing philosophy:
//	    ambiguity never becomes a confirmed violation).
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
		wt := t.TempDir() // no go/go.mod at all
		ws := fixtureWorkspace(t, true, cyclicFrozenTest)
		if code, stderr := verifyTDD(t, ws, wt); code != 0 {
			t.Errorf("RED: exit %d, want 0 — an underivable import graph is infra"+
				" ambiguity and must fail OPEN, never a confirmed violation."+
				"\nstderr:\n%s", code, stderr)
		}
	})
}

// TestC1238_004_LibrarySeam exercises the three new exported symbols directly,
// so a CLI-layer regression and a library-layer regression are distinguishable.
// The contract Builder inherits:
//
//	FrozenTestFiles(reportPath) ([]string, error)
//	    parses a tdd test-report.md handoff JSON; returns testFiles when
//	    doNotModifyTests is true, nil when it is false.
//	ExtractFrozenPins(worktreeRoot, frozenTestFiles) ([]CallSite, error)
//	    returns one CallSite per package-qualified pin, with PinningPackage as
//	    the FULL import path of the package owning the pinned source file and
//	    ReferencedPackage as the bare identifier as written.
//	CheckFrozenPins(worktreeRoot, frozenTestFiles) ([]Violation, error)
//	    resolves those identifiers against the module at <worktreeRoot>/go and
//	    returns one Violation per pin that would close an import cycle.
//
// RED today: this file does not compile — none of the three symbols exist.
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

// TestC1238_006_PermanentRegressionGuard requires the DURABLE protection: an
// ordinary (non-acs) test in package phasecmd, named with the
// `TestPhaseVerifyTDD_FrozenPin` prefix, that drives runPhaseVerify over the
// cycle-644 shape. The acs predicates above are cycle-scoped and vanish with
// this cycle; without this guard the gate can silently rot in a later refactor
// and nothing in `go test ./...` notices. The docsfloor gate (cycle-1150,
// phase_verify_docsfloor_test.go) is the precedent to copy.
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

// tailLines returns the last n lines of s, for readable failure output.
func tailLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// TestC1238_005_ApicoverNamedCoverage is House Rule 1's second half.
// `./internal/reachabilityprobe` is ALREADY enrolled in go/.apicover-enforce
// (line 495), so the repo-wide gate fails the tree unless every newly exported
// symbol is named by identifier in that package's apicover_named_test.go and
// exercised there. This predicate runs those named tests (executed proof, not a
// grep) and then confirms the three new identifiers are among what they name.
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
