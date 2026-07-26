//go:build acs

// Package cycle1101 materialises the cycle-1101 acceptance criteria for the
// single fleet-scoped task pinned to this lane:
//
//	persona-budget-inlane-gate → the in-lane build-floor gate that runs the
//	`internal/prompts` persona line-budget test when a lane's diff touches
//	`agents/evolve-*.md`.
//
// The defect (third instance of the "per-cycle-gate ≠ repo-wide-gate" class,
// after warnship-apicover-ci-gap and acs-predicate-compile-gate-at-build-exit):
// `changedPackageFloorChecks` derives its test set purely from
// `changedGoTestPackages(paths)`, which keeps only paths matching `go/**.go`.
// A lane that grows a persona doc past the 751-line budget pinned by
// `TestPersonaStopCriterionDedupe_CombinedLineCountReduced` therefore produces
// ZERO packages, hits the `len(pkgs) == 0 → return nil` early return
// (build_floor_reviewer.go:110-112), and sails through build handoff — the
// breach only lands on main's CI, reddening the build for every concurrent lane
// sharing the branch (observed twice on 2026-07-23).
//
// Predicate strategy — every predicate EXERCISES the real production entrypoint
// `core.DefaultBuildFloorChecks(ctx, ReviewInput{Phase: "build", ...})` against
// a purpose-built git worktree fixture and asserts on its RETURN VALUE. No
// predicate greps `build_floor_reviewer.go` for a magic string (the cycle-85
// degenerate-predicate ban): an implementer who writes the word "persona" into
// the source without wiring the check fails all three.
//
// The fixture is a minimal but REAL repo: a git-initialised tree with an
// `agents/evolve-scout.md` persona doc and a self-contained `go/` module whose
// `internal/prompts` package carries one test that is either RED or GREEN by
// construction. That lets each predicate vary exactly one input:
//
//   - 001 (positive / crux): persona doc touched + `internal/prompts` RED
//     ⇒ DefaultBuildFloorChecks MUST return a failure naming `internal/prompts`.
//     A no-op implementation returns an empty slice and fails here.
//   - 002 (negative / regression-safety): persona doc UNTOUCHED (only a
//     `docs/` file changed) + `internal/prompts` RED ⇒ MUST return zero
//     failures. An implementation that runs the prompts tests unconditionally
//     (or on any changed path) fails here — this is the fail-open guarantee for
//     lanes that never touch persona docs.
//   - 003 (negative / anti-false-positive): persona doc touched but
//     `internal/prompts` GREEN ⇒ MUST return zero failures. An implementation
//     that rejects on the mere PRESENCE of an `agents/evolve-*.md` path,
//     without actually running the budget test, fails here.
//
// 002 and 003 together pin the gate to the CONJUNCTION (path matched AND test
// red), which is the only behaviour the acceptance criteria admit.
//
// RED today: no persona-budget check exists in `DefaultBuildFloorChecks`, so
// 001 fails (empty failure list) while 002 and 003 pass vacuously — the
// expected RED shape for a gate that is entirely absent. 002/003 are recorded
// as pre-existing GREEN in test-report.md; they are the guards that keep the
// forthcoming implementation from over-firing.
package cycle1101

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// personaBudgetTestName is the fixture's stand-in for
// TestPersonaStopCriterionDedupe_CombinedLineCountReduced — the real
// go/internal/prompts test that pins the 751-line combined persona budget.
const personaBudgetTestName = "TestPersonaLineBudgetFixture"

// promptsPkgMarker is the substring the floor failure must carry so an operator
// can tell WHICH gate rejected the handoff. The acceptance criterion requires
// the reason to identify the internal/prompts persona-budget failure rather
// than rejecting anonymously.
const promptsPkgMarker = "internal/prompts"

// fixtureRepo builds a real git repo fixture and returns its root.
//
// Layout:
//
//	<root>/agents/evolve-scout.md          persona doc (committed at base)
//	<root>/docs/notes.md                   non-persona doc (committed at base)
//	<root>/go/go.mod                       self-contained module, zero deps
//	<root>/go/internal/prompts/budget_test.go   RED or GREEN by construction
//
// Everything is committed, so `git diff <base> --name-only` is empty until the
// caller dirties a path — which is exactly how a lane's changed-path set is
// derived at build handoff (changedWorktreePathsSince).
func fixtureRepo(t *testing.T, promptsRed bool) (root, baseSHA string) {
	t.Helper()
	root = t.TempDir()

	body := "func " + personaBudgetTestName + "(t *testing.T) {\n"
	if promptsRed {
		body += "\tt.Errorf(\"combined persona line count = 812, want < 751 (pre-dedupe baseline)\")\n"
	}
	body += "}\n"

	files := map[string]string{
		"agents/evolve-scout.md":             "# Evolve Scout\n\n## STOP CRITERION\n\nbaseline persona doc.\n",
		"docs/notes.md":                      "# Notes\n\nnon-persona documentation.\n",
		"go/go.mod":                          "module fixture\n\ngo 1.22\n",
		"go/internal/prompts/budget_test.go": "package prompts\n\nimport \"testing\"\n\n" + body,
	}
	for rel, content := range files {
		abs := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", rel, err)
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	git(t, root, "init", "-q")
	git(t, root, "add", "-A")
	git(t, root, "-c", "user.email=acs@evolve.local", "-c", "user.name=acs", "commit", "-q", "-m", "fixture base")
	baseSHA = strings.TrimSpace(gitOut(t, root, "rev-parse", "HEAD"))
	if baseSHA == "" {
		t.Fatalf("fixture base SHA is empty")
	}
	return root, baseSHA
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
	return string(out)
}

// dirty appends a line to a tracked file so it shows up in
// `git diff <base> --name-only` — the exact surface the build floor reads.
func dirty(t *testing.T, root, rel, extra string) {
	t.Helper()
	abs := filepath.Join(root, rel)
	data, err := os.ReadFile(abs)
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	if err := os.WriteFile(abs, append(data, []byte(extra)...), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

// runFloor invokes the production build-floor engine over the fixture exactly
// as the build-phase DeliverableReviewer does.
func runFloor(t *testing.T, root, baseSHA string) []string {
	t.Helper()
	return core.DefaultBuildFloorChecks(context.Background(), core.ReviewInput{
		Phase:           "build",
		Worktree:        root,
		WorktreeBaseSHA: baseSHA,
	})
}

// TestC1101_001_PersonaBudgetBreachRejectedAtHandoff is the crux predicate: a
// lane that edits agents/evolve-scout.md while go/internal/prompts is RED must
// be REJECTED by the build handoff floor, with a reason naming internal/prompts.
//
// Today this fails with an empty failure list: changedGoTestPackages sees only
// `agents/evolve-scout.md` (not `go/**.go`), yields zero packages, and
// changedPackageFloorChecks early-returns nil.
func TestC1101_001_PersonaBudgetBreachRejectedAtHandoff(t *testing.T) {
	root, base := fixtureRepo(t, true /* promptsRed */)
	dirty(t, root, "agents/evolve-scout.md", "\nextra persona lines that blow the budget.\n")

	failures := runFloor(t, root, base)
	if len(failures) == 0 {
		t.Fatalf("DefaultBuildFloorChecks returned NO failures for a lane touching agents/evolve-scout.md with go/internal/prompts RED — the persona line budget breach reaches main's CI unblocked (the cycle-1101 in-lane gate gap)")
	}
	joined := strings.Join(failures, "\n")
	if !strings.Contains(joined, promptsPkgMarker) {
		t.Errorf("build-floor rejected but no failure names %q — the operator cannot tell which gate fired; got:\n%s", promptsPkgMarker, joined)
	}
}

// TestC1101_002_NonPersonaLaneUnaffected is the regression-safety guard: a lane
// whose diff touches no agents/evolve-*.md file must see ZERO behaviour change,
// even when go/internal/prompts happens to be RED. The persona budget is not
// that lane's responsibility, and firing here would false-block every unrelated
// lane in the fleet.
func TestC1101_002_NonPersonaLaneUnaffected(t *testing.T) {
	root, base := fixtureRepo(t, true /* promptsRed */)
	dirty(t, root, "docs/notes.md", "\nan unrelated documentation edit.\n")

	failures := runFloor(t, root, base)
	if len(failures) != 0 {
		t.Errorf("DefaultBuildFloorChecks returned %d failure(s) for a lane touching only docs/notes.md — the persona gate must not fire for lanes that never touch agents/evolve-*.md (fail-open floor policy); got:\n%s", len(failures), strings.Join(failures, "\n"))
	}
}

// TestC1101_003_PersonaTouchWithGreenBudgetApproves is the anti-false-positive
// guard: touching a persona doc is NOT itself a violation. The gate must run
// the internal/prompts budget test and approve when it is GREEN. An
// implementation that rejects on path-match alone fails here.
func TestC1101_003_PersonaTouchWithGreenBudgetApproves(t *testing.T) {
	root, base := fixtureRepo(t, false /* promptsRed */)
	dirty(t, root, "agents/evolve-scout.md", "\na small, within-budget persona edit.\n")

	failures := runFloor(t, root, base)
	if len(failures) != 0 {
		t.Errorf("DefaultBuildFloorChecks returned %d failure(s) for a persona edit whose go/internal/prompts budget test is GREEN — the gate must assert on the TEST OUTCOME, not on the mere presence of an agents/evolve-*.md path; got:\n%s", len(failures), strings.Join(failures, "\n"))
	}
}
