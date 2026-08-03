package phasecmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Permanent (non-acs) regression guard for the cycle-644 reachability gate
// wired into `evolve phase verify tdd` (cycle-1238, inbox item
// tdd-structural-test-reachability-probe).
//
// The acs predicates for this cycle vanish with the cycle; without these the
// gate could silently rot in a later refactor and `go test ./...` would not
// notice. Precedent: phase_verify_docsfloor_test.go (cycle-1150). Like that
// one, these drive the REAL CLI entry point (runPhaseVerify) over a real Go
// module, so they assert on the exit code and stderr an operator actually sees
// — a gate reachable only from a unit test is dead code.

// frozenPinWrite materialises rel (slash separated, relative to root).
func frozenPinWrite(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// frozenPinWorktree builds a throwaway worktree whose go/ subdirectory is a
// real, resolvable module carrying both shapes the gate must tell apart:
// storage imports core (so pinning storage.UpdateStateMap( inside a core file
// is the cycle-644 shape), and leafutil imports nothing (so pinning
// leafutil.Helper( inside a core file is perfectly buildable).
func frozenPinWorktree(t *testing.T) string {
	t.Helper()
	wt := t.TempDir()
	const module = "example.com/frozenpin"

	frozenPinWrite(t, wt, "go/go.mod", "module "+module+"\n\ngo 1.23\n")
	frozenPinWrite(t, wt, "go/internal/core/state.go",
		"package core\n\n// State is the fixture core type.\ntype State struct{}\n")
	frozenPinWrite(t, wt, "go/internal/storage/storage.go",
		"package storage\n\nimport \""+module+"/internal/core\"\n\n"+
			"// UpdateStateMap is the cycle-644 symbol: storage already imports core.\n"+
			"func UpdateStateMap(s *core.State) {}\n")
	frozenPinWrite(t, wt, "go/internal/leafutil/leafutil.go",
		"package leafutil\n\n// Helper imports nothing.\nfunc Helper() {}\n")

	frozenPinWrite(t, wt, frozenPinCyclicTest,
		"package core\n\nimport \"testing\"\n\n"+
			"func TestC644_StateUsesStorage(t *testing.T) {\n"+
			"\tassertFileContains(t, \"go/internal/core/state.go\", \"storage.UpdateStateMap(\")\n}\n")
	frozenPinWrite(t, wt, frozenPinReachableTest,
		"package core\n\nimport \"testing\"\n\n"+
			"func TestReachable_StateUsesLeafutil(t *testing.T) {\n"+
			"\tassertFileContains(t, \"go/internal/core/state.go\", \"leafutil.Helper(\")\n}\n")
	return wt
}

const (
	frozenPinCyclicTest    = "go/internal/core/frozen_cyclic_test.go"
	frozenPinReachableTest = "go/internal/core/frozen_reachable_test.go"
)

// frozenPinWorkspace writes a WELL-FORMED tdd deliverable freezing (or not)
// the named test file. Well-formedness matters: the reachability gate must be
// the only thing that can turn these cases red, never a missing section.
func frozenPinWorkspace(t *testing.T, doNotModifyTests bool, frozen string) string {
	t.Helper()
	ws := t.TempDir()
	flag := "false"
	if doNotModifyTests {
		flag = "true"
	}
	report := "# TDD Report — frozen-pin fixture\n\n" +
		"## AC-Materialization\n\n| Criterion | Test | Status |\n|---|---|---|\n| fixture | fixture | RED |\n\n" +
		"## RED Run Output\n\n```\nfixture RED output\n```\n\n" +
		"## Handoff to Builder\n\n```json\n{\n  \"testFiles\": [\"" + frozen + "\"],\n" +
		"  \"redRunConfirmed\": true,\n  \"doNotModifyTests\": " + flag + "\n}\n```\n"
	frozenPinWrite(t, ws, "test-report.md", report)
	return ws
}

// TestPhaseVerifyTDD_FrozenPinCycle_Exit1 is the crux rejection contract: the
// cycle-644 shape is a CONFIRMED violation on the live CLI path, before the
// build phase ever starts, with the stable code and all three identifiers an
// agent needs to act on it.
func TestPhaseVerifyTDD_FrozenPinCycle_Exit1(t *testing.T) {
	wt := frozenPinWorktree(t)
	ws := frozenPinWorkspace(t, true, frozenPinCyclicTest)

	code, _, errb := runVerify(t, "tdd", "--workspace="+ws, "--worktree="+wt)
	if code != 1 {
		t.Fatalf("exit=%d want 1 (confirmed unreachable-frozen-pin violation); stderr=%s", code, errb)
	}
	if !strings.Contains(errb, codeUnreachableFrozenPin) {
		t.Errorf("stderr must name %q so the agent knows which gate failed; got %q", codeUnreachableFrozenPin, errb)
	}
	for _, want := range []string{"storage", "core", "UpdateStateMap"} {
		if !strings.Contains(errb, want) {
			t.Errorf("stderr omits %q — the diagnostic must name the pinning package, the referenced package and the symbol; got %q", want, errb)
		}
	}
}

// TestPhaseVerifyTDD_FrozenPinReachable_Exit0 is the false-positive regression
// guard: a pin closing no cycle passes the same gate unchanged. A gate that
// flagged every package-qualified pin would satisfy the case above and make
// the tdd phase unusable.
func TestPhaseVerifyTDD_FrozenPinReachable_Exit0(t *testing.T) {
	wt := frozenPinWorktree(t)
	ws := frozenPinWorkspace(t, true, frozenPinReachableTest)

	if code, _, errb := runVerify(t, "tdd", "--workspace="+ws, "--worktree="+wt); code != 0 {
		t.Fatalf("exit=%d want 0 — leafutil imports nothing, so the pin closes no cycle; stderr=%s", code, errb)
	}
}

// TestPhaseVerifyTDD_FrozenPinScopeAndFailOpen pins the two edge cases that
// keep the gate from becoming a new false-HALT source: it keys off the freeze
// flag rather than scanning every test it can find, and an underivable import
// graph is infra ambiguity that must fail OPEN.
func TestPhaseVerifyTDD_FrozenPinScopeAndFailOpen(t *testing.T) {
	t.Run("unfrozen_handoff_does_not_fire", func(t *testing.T) {
		wt := frozenPinWorktree(t)
		ws := frozenPinWorkspace(t, false, frozenPinCyclicTest)
		if code, _, errb := runVerify(t, "tdd", "--workspace="+ws, "--worktree="+wt); code != 0 {
			t.Errorf("exit=%d want 0 — doNotModifyTests:false means the pin is not frozen; stderr=%s", code, errb)
		}
	})

	t.Run("no_go_module_fails_open", func(t *testing.T) {
		wt := t.TempDir() // no go/go.mod at all
		ws := frozenPinWorkspace(t, true, frozenPinCyclicTest)
		if code, _, errb := runVerify(t, "tdd", "--workspace="+ws, "--worktree="+wt); code != 0 {
			t.Errorf("exit=%d want 0 — an underivable import graph must never become a confirmed violation; stderr=%s", code, errb)
		}
	})

	t.Run("no_worktree_leaves_verdict_untouched", func(t *testing.T) {
		ws := frozenPinWorkspace(t, true, frozenPinCyclicTest)
		if code, _, errb := runVerify(t, "tdd", "--workspace="+ws); code != 0 {
			t.Errorf("exit=%d want 0 — without --worktree there are no pinned files to judge; stderr=%s", code, errb)
		}
	})
}
