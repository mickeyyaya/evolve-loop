//go:build acs

// Package cycle1036 materialises the cycle-1036 acceptance criteria for this
// fleet lane's sole item, retro-role-gate-lessons-write-allowance (triage top_n
// slug identical). Per R9.3 no predicate here binds to any other lane's items.
//
// Defect (from scout-report.md). go/internal/guards/role.go's doc comment
// promises a `learn/retrospective: workspace_path + .evolve/lessons/**`
// allowance that Decide() never implements. PhaseRetro ("retro") is a valid
// cs.Phase but is NOT a core.WorktreePhase, so the role guard grants it only the
// WorkspacePath allowance — a retro-phase Edit/Write under the real lesson
// corpus path `<repoRoot>/.evolve/instincts/lessons/` is default-denied
// (role.go fallthrough). The doc comment also names the wrong path
// (`.evolve/lessons/**` vs the real `.evolve/instincts/lessons/`, per
// go/internal/research/kb.go:75). Fix: add a retro-phase branch allowing writes
// under `<repoRoot>/.evolve/instincts/lessons/`, evaluated STRICTLY AFTER the
// IsProtectedSurface deny (so a crafted lessons path cannot smuggle a
// control-plane edit), and correct the doc comment.
//
// Predicate strategy. These predicates EXERCISE THE SYSTEM UNDER TEST: 001-004
// construct the real guards.Role via NewRole and call Decide() with a synthetic
// core.Storage whose CycleState has Phase=="retro" and a canonical
// WorkspacePath (`<root>/.evolve/runs/cycle-1036`). The guard derives the lesson
// corpus dir from that canonical WorkspacePath (its repoRoot ancestor +
// `.evolve/instincts/lessons`), so no real filesystem is touched — Decide is
// pure path arithmetic (isUnderDir/filepath.Rel + IsProtectedSurface). This is
// NOT a source-grep proxy (cycle-85 ban): the assertions are on Decide's
// returned GuardDecision. Only 005 asserts on source text, and it carries the
// `// acs-predicate: config-check` waiver because the doc-comment correction is
// an inherent documentation-text criterion with no runtime code path (the same
// waiver cycle-1029/cycle-943 used for inherent doc criteria).
//
// Adversarial axes (adversarial-testing SKILL §6):
//   - positive  : 001 retro + lessons path → Allow.
//   - negative  : 002 retro + non-lessons/non-workspace path → Deny (proves the
//     fix does not over-broaden — a no-op that blanket-allows retro fails here).
//   - edge/smuggle: 003 retro + a path that is BOTH under the lessons dir AND a
//     protected surface (`…/.evolve/instincts/lessons/.evolve/policy.json`) →
//     Deny + Alarm (proves the allowance is checked after IsProtectedSurface).
//   - edge/traversal: 004 retro + `…/.evolve/instincts/lessons/../../etc/passwd`
//     (escapes the lessons dir after cleaning) → Deny (proves clean containment,
//     not a naive prefix match).
//
// RED today:
//   - 001 fails: retro is not a WorktreePhase, so Decide returns Allow:false for
//     the lessons path (the missing branch).
//   - 005 fails: role.go's doc comment still reads `.evolve/lessons/**` and lacks
//     the corrected `.evolve/instincts/lessons` path.
//   - 002/003/004 are PRE-EXISTING GREEN guard tests (they assert behaviour the
//     fix must PRESERVE): they lock in that the new allowance does not
//     over-broaden, does not bypass the control-plane deny, and does not leak via
//     path traversal.
package cycle1036

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// modPrefix is the module import prefix; a subprocess `go test` runs with
// cwd==this package dir, where a relative `./internal/...` target does not
// resolve — a fully-qualified import path works from anywhere in the module.
const modPrefix = "github.com/mickeyyaya/evolve-loop/go/"

// synthRoot is a synthetic, non-/tmp absolute repo root. Decide never stats the
// filesystem for these paths, so the dir need not exist; a non-/tmp root keeps
// isAlwaysSafe from short-circuiting the decision.
const synthRoot = "/synth/repo-cycle1036"

// workspacePath is the canonical per-cycle workspace: `<root>/.evolve/runs/cycle-N`.
// The guard must derive the lesson corpus dir from this (its repoRoot ancestor).
func workspacePath() string {
	return filepath.Join(synthRoot, ".evolve", "runs", "cycle-1036")
}

// lessonsDir is the real lesson corpus path (kb.go:75): `<root>/.evolve/instincts/lessons`.
func lessonsDir() string {
	return filepath.Join(synthRoot, ".evolve", "instincts", "lessons")
}

// fakeStorage is a minimal in-memory core.Storage returning a fixed CycleState.
// Only ReadCycleState is exercised by Decide; the rest satisfy the interface.
type fakeStorage struct{ cs core.CycleState }

func (f fakeStorage) ReadState(context.Context) (core.State, error) { return core.State{}, nil }
func (f fakeStorage) WriteState(context.Context, core.State) error  { return nil }
func (f fakeStorage) ReadCycleState(context.Context) (core.CycleState, error) {
	return f.cs, nil
}
func (f fakeStorage) WriteCycleState(context.Context, core.CycleState) error { return nil }
func (f fakeStorage) AcquireLock(context.Context) (func() error, error) {
	return func() error { return nil }, nil
}

// retroRole builds a real Role guard (bypass=false) whose storage reports a
// retro-phase cycle with the canonical workspace.
func retroRole() *guards.Role {
	cs := core.CycleState{
		CycleID:       1036,
		Phase:         string(core.PhaseRetro),
		WorkspacePath: workspacePath(),
	}
	return guards.NewRole(fakeStorage{cs: cs}, false)
}

// decideWrite runs the role guard for a Write of file_path.
func decideWrite(path string) core.GuardDecision {
	return retroRole().Decide(context.Background(), core.GuardInput{
		ToolName:  "Write",
		ToolInput: map[string]any{"file_path": path},
	})
}

// AC1 (positive, behavioural): retro phase may write under the lesson corpus
// dir. RED today — PhaseRetro is not a WorktreePhase, so Decide default-denies.
func TestC1036_001_retro_may_write_under_lessons_dir(t *testing.T) {
	path := filepath.Join(lessonsDir(), "retro-role-gate-lessons-write-allowance.yaml")
	dec := decideWrite(path)
	if !dec.Allow {
		t.Errorf("RED: retro-phase write under the lesson corpus %s was denied (Reason=%q); "+
			"the role guard's promised learn/retrospective lessons-write allowance is unimplemented",
			path, dec.Reason)
	}
	if dec.Alarm {
		t.Errorf("RED: a legitimate lessons write raised an integrity Alarm (Reason=%q) — "+
			"the lessons path was misclassified as a protected surface", dec.Reason)
	}
}

// AC3a (negative/semantic): the allowance must NOT over-broaden. A retro write
// outside BOTH the workspace and the lesson corpus stays denied — a no-op that
// blanket-allows retro fails here. PRE-EXISTING GREEN; the fix must preserve it.
func TestC1036_002_retro_write_outside_lessons_and_workspace_denied(t *testing.T) {
	path := filepath.Join(synthRoot, "go", "internal", "core", "cyclerun.go")
	dec := decideWrite(path)
	if dec.Allow {
		t.Errorf("retro-phase write outside workspace+lessons (%s) was allowed — "+
			"the lessons allowance over-broadened into general repo-tree writes", path)
	}
}

// AC1 (edge/smuggle): a path that is BOTH under the lessons dir AND a protected
// control-plane surface must still be denied AND alarmed — proving the lessons
// allowance is evaluated strictly AFTER the IsProtectedSurface deny and can
// never be used to smuggle a control-plane edit. PRE-EXISTING GREEN; the fix
// must preserve the ordering.
func TestC1036_003_lessons_path_cannot_smuggle_protected_surface(t *testing.T) {
	// `<lessonsDir>/.evolve/policy.json` contains the protected fragment
	// "/.evolve/policy.json" yet resolves under the lesson corpus dir.
	path := filepath.Join(lessonsDir(), ".evolve", "policy.json")
	if !guards.IsProtectedSurface(path) {
		t.Fatalf("test premise broken: %s is expected to be a protected surface", path)
	}
	dec := decideWrite(path)
	if dec.Allow {
		t.Errorf("SMUGGLE: a protected control-plane path under the lessons dir (%s) was ALLOWED — "+
			"the lessons allowance bypassed the IsProtectedSurface deny", path)
	}
	if !dec.Alarm {
		t.Errorf("a protected-surface deny under the lessons dir (%s) did not raise an integrity Alarm", path)
	}
}

// AC3c (edge/traversal): a path that escapes the lessons dir via `..` after
// cleaning must be denied — the allowance uses clean containment (isUnderDir),
// not a naive string prefix. PRE-EXISTING GREEN; the fix must preserve it.
func TestC1036_004_lessons_path_traversal_escape_denied(t *testing.T) {
	// filepath.Join cleans the `..` segments, resolving OUTSIDE lessonsDir.
	path := filepath.Join(lessonsDir(), "..", "..", "..", "etc", "passwd")
	dec := decideWrite(path)
	if dec.Allow {
		t.Errorf("TRAVERSAL: a path escaping the lessons dir via `..` (%s) was allowed — "+
			"the allowance used a naive prefix match instead of clean containment", path)
	}
}

// AC2 (doc-comment correction): role.go's Role doc comment must name the REAL
// lesson corpus path `.evolve/instincts/lessons` (matching kb.go:75) and must no
// longer carry the stale `.evolve/lessons/**`.
//
// acs-predicate: config-check — the deliverable of AC2 IS the documentation
// text; there is no runtime code path to exercise for a doc-comment string, so
// (uniquely among these predicates) it asserts on the emitted source doc. RED
// today: the comment still reads `.evolve/lessons/**`.
func TestC1036_005_role_doc_comment_names_real_lessons_path(t *testing.T) {
	roleGo := filepath.Join(acsassert.RepoRoot(t), "go", "internal", "guards", "role.go")
	if !acsassert.FileExists(t, roleGo) {
		return // FileExists already failed with the path
	}
	raw, rerr := os.ReadFile(roleGo)
	if rerr != nil {
		t.Fatalf("read role.go: %v", rerr)
	}
	// Plain (non-asserting) reads: FileContains is an ASSERTING helper, so its
	// negation still fails the test when the needle is rightly absent — the
	// authoring bug that kept this predicate red after the fix landed.
	if !strings.Contains(string(raw), ".evolve/instincts/lessons") {
		t.Errorf("role.go doc comment does not name the real lesson corpus path `.evolve/instincts/lessons` (kb.go:75)")
	}
	if strings.Contains(string(raw), ".evolve/lessons/**") {
		t.Errorf("role.go doc comment still carries the stale `.evolve/lessons/**` path")
	}
}

// AC4 (behavioural/no-regression): the guards package's role suite passes N/N in
// this tree — the retro allowance must not regress any existing role guard
// behaviour. Runs the SUT's own tests as a subprocess and requires exit 0 with
// at least one PASS marker (a -run matching zero tests exits 0 — that gaming
// vector is rejected by requiring a PASS line). PRE-EXISTING GREEN today; stays
// green iff the Builder's role.go change breaks nothing.
func TestC1036_006_role_guard_suite_no_regression(t *testing.T) {
	pkg := modPrefix + "internal/guards"
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", pkg, "-run", "TestRole", "-count=1", "-v")
	if code != 0 || err != nil {
		t.Errorf("`go test %s -run TestRole` exited %d (err=%v) — role guard regression\nstdout:\n%s\nstderr:\n%s",
			pkg, code, err, stdout, stderr)
		return
	}
	if !strings.Contains(stdout, "--- PASS: TestRole") {
		t.Errorf("`go test %s -run TestRole` reported no PASS marker — the filter matched zero tests", pkg)
	}
}
