//go:build acs

// Package cycle533 materialises the cycle-533 acceptance criteria.
//
// TASK BINDING & CROSS-ARTIFACT CONFLICT (resolved here, documented in
// test-report.md):
//
//   - triage-report.md `## top_n` names `cache-stable-prompt-prefixes` (a
//     fleet_scope inbox item for a SIBLING lane) and defers scout's two bugfix
//     tasks. But that task has NO materialised eval, NO fault-localization, and
//     the LOCKED phase-plan actually driving dispatch (fault-localization →
//     bug-reproduction → tdd → build → adversarial-review → coverage-gate,
//     coverage-gate justified "diff_loc >= 50 for a guard/leak fix") is the
//     tree-diff-guard BUGFIX. fault-localization (this cycle's phase immediately
//     before tdd) explicitly resolved the conflict toward the bug per
//     follow_approved_plan_no_relitigate. These predicates therefore bind to the
//     work the executing plan committed — the bugfix — which is what
//     bug-reproduction/tdd/build all target.
//
//   - Scout proposed TWO fix tasks (both evals materialised in the workspace):
//     Task 1  fix-role-gate-worktree-write-predicate      (go/internal/guards/role.go)
//     Task 2  fix-treediff-leak-recovery-catalog-predicate (go/internal/core/cyclerun_review.go)
//     CONTROL-PLANE FINDING (new this cycle, missed by scout/fault-localization):
//     `/go/internal/guards/` is a PROTECTED integrity surface
//     (guards.protectedSurfaceFragments / ADR-0064) — an autonomous `--class
//     cycle` run STRUCTURALLY cannot edit role.go (the role gate denies it, and
//     Builder would hit the same wall). Task 1 is therefore dispositioned
//     manual+checklist (operator-gated `evolve ship --class manual`), NOT a
//     predicate — a predicate requiring a control-plane edit would set Builder
//     up to fail the integrity boundary. Per scout hypothesis #2, Task 2 is also
//     the MORE load-bearing half (the only leak backstop for non-Claude drivers,
//     which bypass the role gate entirely), so the cycle-shippable fix closes the
//     directly-recurring failure mode.
//
// These predicates bind ONLY to the triage/plan-committed, cycle-implementable
// work: Task 2 (R9.3 — predicates for committed work only). Task 1 rides in
// test-report.md as a manual+checklist item.
//
// PREDICATE QUALITY (cycle-85): every load-bearing predicate RUNS the
// system-under-test as a real subprocess (`go test -tags integration` against
// the orchestrator's production RunCycle → recoverBuildLeak → tree-diff guard
// path on a real git repo) and asserts on its exit code — never a
// "source file contains text X" grep. The behavioural test
// TestGuardRecoversCatalogWritesSourcePhaseLeak is RED on the current tree (the
// scout-leak cycle aborts with "tree-diff guard: phase \"scout\" wrote to the
// main tree"); Builder makes it green by swapping the bare `WorktreePhase(next)`
// at cyclerun_review.go:263 for the catalog-aware `cr.o.worktreePhase(next)`.
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - Positive:  C533_001 — a catalog source-writer phase's leak is RECOVERED.
//   - Negative:  C533_002 — a non-source phase's leak still hard-ABORTS (the
//     anti-over-broadening pin; kills the "always recover" fake).
//   - Regression: C533_003 — the built-in tdd/build leak paths + guard
//     classification suite stay green (the fix must not disturb them).
package cycle533

import (
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// goTestCore runs `go test -tags integration -run <pattern>` against the real
// internal/core package (absolute path → module resolves regardless of cwd) and
// returns the exit code + combined diagnostics. This EXECUTES the SUT: the named
// tests drive the production Orchestrator.RunCycle recovery/guard path over a
// real git repo, so a green result requires the fix to actually work, not a
// magic string in source.
func goTestCore(t *testing.T, pattern string) (string, string, int, error) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available — orchestrator leak/guard integration tests require real git")
	}
	corePkg := filepath.Join(acsassert.RepoRoot(t), "go", "internal", "core")
	return acsassert.SubprocessOutput("go", "test", "-tags", "integration", "-count=1",
		"-run", pattern, corePkg)
}

// TestC533_001_CatalogSourceWriterLeakRecovered (Task 2, positive — THE defect).
// A catalog phase with writes_source:true that is NOT a tdd/build literal (scout,
// marked via WithCatalog) leaks a tracked-file edit into the main tree; the
// leak-recovery gate must relocate it and let the cycle proceed to ship. RED on
// the current tree (the gate keys off the catalog-blind WorktreePhase, so the
// cycle aborts at the scout leak). Builder makes it green with the one-line
// catalog-aware gate swap.
func TestC533_001_CatalogSourceWriterLeakRecovered(t *testing.T) {
	stdout, stderr, code, err := goTestCore(t, "TestGuardRecoversCatalogWritesSourcePhaseLeak")
	if err != nil || code != 0 {
		t.Errorf("catalog source-writer phase leak must be recovered (leak-recovery gate must be catalog-aware), but the behavioural test failed (code=%d err=%v)\nstdout:\n%s\nstderr:\n%s", code, err, stdout, stderr)
	}
}

// TestC533_002_NonSourcePhaseLeakStillAborts (Task 2, negative / anti-gaming).
// A phase that is NOT a declared source-writer (audit, absent from the catalog)
// leaking a source file must STILL hard-abort via the tree-diff guard. Kills the
// cheapest fake — deleting the gate condition so recoverBuildLeak always runs.
// Green today AND after the fix (the fix must not broaden recovery to all
// phases); pins that scoping.
func TestC533_002_NonSourcePhaseLeakStillAborts(t *testing.T) {
	stdout, stderr, code, err := goTestCore(t, "TestGuardStillAbortsNonSourcePhaseLeak")
	if err != nil || code != 0 {
		t.Errorf("non-source phase leak must still abort (recovery must stay scoped to catalog source-writers), but the behavioural test failed (code=%d err=%v)\nstdout:\n%s\nstderr:\n%s", code, err, stdout, stderr)
	}
}

// TestC533_003_BuiltinLeakAndGuardSuiteStaysGreen (Task 2, regression). The
// existing tdd/build leak-recovery paths and the tree-diff guard classification
// suite — the literal paths the fix must NOT disturb — stay green. Runs the
// curated pre-existing guard/leak cases as one subprocess.
func TestC533_003_BuiltinLeakAndGuardSuiteStaysGreen(t *testing.T) {
	pattern := "TestTDDLeakRecover|TestOrchestrator_AuditLeakRecover|" +
		"TestGuardIgnoresOrchestratorSelfWrite_WorktreePhase|" +
		"TestGuardCatchesDeliverableRenameSmuggle_WorktreePhase|" +
		"TestGuardCatchesInsertedPhaseLeak|TestGuardIgnoresLegitimateWorkspaceWrite|" +
		"TestGuardIgnoresScoutEvalMaterialization|TestIsLegitimateMainTreePath"
	stdout, stderr, code, err := goTestCore(t, pattern)
	if err != nil || code != 0 {
		t.Errorf("the pre-existing leak/guard suite must stay green through the fix (built-in tdd/build + classifier paths), but it failed (code=%d err=%v)\nstdout:\n%s\nstderr:\n%s", code, err, stdout, stderr)
	}
}

// TestC533_004_CoreVetClean (Task 2, repo-gate pin). internal/core — including
// the new integration guard tests — must stay `go vet`-clean through the change.
// Subprocess against the real toolchain, not a text scan.
func TestC533_004_CoreVetClean(t *testing.T) {
	corePkg := filepath.Join(acsassert.RepoRoot(t), "go", "internal", "core")
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "vet", "-tags", "integration", corePkg)
	if err != nil || code != 0 {
		t.Errorf("go vet -tags integration ./internal/core/ reported problems (code=%d err=%v)\nstdout:\n%s\nstderr:\n%s", code, err, stdout, stderr)
	}
}
