package phasecmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
)

// RED contract for cycle-1150 / wire-docsfloor-verify-cli.
//
// `evolve phase verify build` is the exact self-check every phase prompt's
// Deliverable Contract tells the agent to run before declaring done. It calls
// deliverable.VerifyWithStage, which never sees the build's diff — so the
// ADR-0077 blocking-grade classifier (deliverable.VerifyBuildWithChangedPaths,
// added cycle-1144) has zero production callers and an architecture-class build
// with no docs delta passes the agent's own self-check.
//
// These tests drive the REAL CLI entry point (runPhaseVerify) over a REAL git
// worktree, so they assert on exit codes and stderr the operator actually sees
// — not on an internal seam that is already green in isolation.

// docsFloorBuildReport is a well-formed build deliverable: the floor must be
// the ONLY thing that can turn these cases red, never a well-formedness gap.
const docsFloorBuildReport = "## Changes\n- go/internal/policy/policy.go\nVerdict: PASS\n"

// docsFloorGit runs git in dir, failing the test on error.
func docsFloorGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
	}
}

// newDocsFloorWorktree builds a git repo with one base commit and then
// materialises `changed` as new files on top of it — the shape a cycle's build
// worktree has at verify time (new work is untracked or diffs against HEAD).
func newDocsFloorWorktree(t *testing.T, changed ...string) string {
	t.Helper()
	dir := t.TempDir()
	docsFloorGit(t, dir, "init", "-q")
	docsFloorGit(t, dir, "config", "user.email", "t@t.t")
	docsFloorGit(t, dir, "config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(dir, "base.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	docsFloorGit(t, dir, "add", "base.txt")
	docsFloorGit(t, dir, "commit", "-q", "-m", "base")
	for _, rel := range changed {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("// cycle-1150 fixture\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// newDocsFloorWorkspace returns a workspace holding a well-formed build report.
func newDocsFloorWorkspace(t *testing.T) string {
	t.Helper()
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "build-report.md"), []byte(docsFloorBuildReport), 0o644); err != nil {
		t.Fatal(err)
	}
	return ws
}

// TestPhaseVerify_ArchitectureClassDiffWithoutDocs_Exit1 — AC1, the primary
// rejection contract. A worktree diff that touches a trust-kernel surface
// (go/internal/policy/) and carries no docs delta must be a CONFIRMED violation
// on the live CLI path: exit 1, with the stable `missing_architecture_docs`
// code named in stderr so the agent knows what to fix.
func TestPhaseVerify_ArchitectureClassDiffWithoutDocs_Exit1(t *testing.T) {
	wt := newDocsFloorWorktree(t, "go/internal/policy/policy.go")
	ws := newDocsFloorWorkspace(t)

	code, _, errb := runVerify(t, "build", "--workspace="+ws, "--worktree="+wt)
	if code != 1 {
		t.Fatalf("exit=%d want 1 (confirmed docs-floor violation); stderr=%s", code, errb)
	}
	if !strings.Contains(errb, deliverable.CodeMissingArchitectureDocs) {
		t.Errorf("stderr must name %q so the agent can act on it; got %q",
			deliverable.CodeMissingArchitectureDocs, errb)
	}
}

// TestPhaseVerify_ArchitectureClassDiffWithoutDocs_JSONCarriesCode — AC1, the
// machine-readable half. The host gate and the agent read the same verdict, so
// --json must carry the floor code too, not only the human stderr rendering.
func TestPhaseVerify_ArchitectureClassDiffWithoutDocs_JSONCarriesCode(t *testing.T) {
	wt := newDocsFloorWorktree(t, "go/internal/core/phase_bindings.go")
	ws := newDocsFloorWorkspace(t)

	code, out, _ := runVerify(t, "build", "--workspace="+ws, "--worktree="+wt, "--json")
	if code != 1 {
		t.Fatalf("exit=%d want 1; stdout=%s", code, out)
	}
	if !strings.Contains(out, deliverable.CodeMissingArchitectureDocs) {
		t.Errorf("--json payload must carry %q; got %s", deliverable.CodeMissingArchitectureDocs, out)
	}
}

// TestPhaseVerify_ArchitectureClassDiffWithDocs_Exit0 — AC2, the
// anti-false-positive half. The SAME architecture-class diff must pass once a
// qualifying docs delta rides along. A floor that rejects correctly-documented
// work would block every architecture cycle; a fake that always emits the
// violation dies here.
func TestPhaseVerify_ArchitectureClassDiffWithDocs_Exit0(t *testing.T) {
	for _, doc := range []string{
		"docs/architecture/adr/0099-cycle-1150-wiring.md",
		"docs/operations/runtime-reference.md",
	} {
		t.Run(doc, func(t *testing.T) {
			wt := newDocsFloorWorktree(t, "go/internal/policy/policy.go", doc)
			ws := newDocsFloorWorkspace(t)

			code, _, errb := runVerify(t, "build", "--workspace="+ws, "--worktree="+wt)
			if code != 0 {
				t.Errorf("exit=%d want 0 — %s satisfies the docs floor; stderr=%s", code, doc, errb)
			}
		})
	}
}

// TestPhaseVerify_NonArchitectureDiffInWorktree_Exit0 — AC3, fail-open inside a
// worktree. Ordinary cycles (bugfix, test-only, docs-only) must be untouched by
// the wiring: `_test.go` files never label, and nothing outside the declared
// architecture surfaces does either. This is the regression guard that keeps
// the new code path from taxing every non-architecture cycle.
func TestPhaseVerify_NonArchitectureDiffInWorktree_Exit0(t *testing.T) {
	cases := map[string][]string{
		"test-only":      {"go/internal/policy/policy_test.go"},
		"ordinary-code":  {"go/internal/cli/phasecmd/phase_lint.go", "README.md"},
		"nothing-at-all": nil,
	}
	for name, changed := range cases {
		t.Run(name, func(t *testing.T) {
			wt := newDocsFloorWorktree(t, changed...)
			ws := newDocsFloorWorkspace(t)

			code, _, errb := runVerify(t, "build", "--workspace="+ws, "--worktree="+wt)
			if code != 0 {
				t.Errorf("exit=%d want 0 — %s diffs are not architecture-class; stderr=%s", code, name, errb)
			}
		})
	}
}

// TestPhaseVerify_NoWorktree_ByteIdentical — AC3, the fail-open contract when
// there is no diff source at all. With no --worktree there is nothing to
// classify, so behaviour must be byte-identical to today's VerifyWithStage
// path: a well-formed report passes, a missing one still fails naming its path.
// Rejects an implementation that panics or hard-fails on an empty Worktree.
func TestPhaseVerify_NoWorktree_ByteIdentical(t *testing.T) {
	t.Run("well-formed report still passes", func(t *testing.T) {
		ws := newDocsFloorWorkspace(t)
		code, _, errb := runVerify(t, "build", "--workspace="+ws)
		if code != 0 {
			t.Errorf("exit=%d want 0 with no worktree (fail open); stderr=%s", code, errb)
		}
	})

	t.Run("missing artifact still fails naming the path", func(t *testing.T) {
		ws := t.TempDir()
		code, _, errb := runVerify(t, "build", "--workspace="+ws)
		if code != 1 {
			t.Fatalf("exit=%d want 1 for a missing artifact; stderr=%s", code, errb)
		}
		if !strings.Contains(errb, filepath.Join(ws, "build-report.md")) {
			t.Errorf("stderr must still name the expected path; got %q", errb)
		}
		if strings.Contains(errb, deliverable.CodeMissingArchitectureDocs) {
			t.Errorf("no worktree ⇒ no diff to judge ⇒ the floor must stay silent; got %q", errb)
		}
	})
}

// TestPhaseVerify_NonBuildPhase_UnaffectedByDocsFloor — AC3, the scope guard.
// The docs floor is a BUILD-phase contract (ADR-0077). Threading the changed
// path set through verify must not leak the floor into other phases'
// deliverables, even when the same architecture-class worktree is supplied.
func TestPhaseVerify_NonBuildPhase_UnaffectedByDocsFloor(t *testing.T) {
	wt := newDocsFloorWorktree(t, "go/internal/policy/policy.go")
	ws := t.TempDir()

	// An empty workspace fails the tdd contract on well-formedness alone; the
	// point is WHICH violation surfaces — never the build-only floor code.
	code, _, errb := runVerify(t, "tdd", "--workspace="+ws, "--worktree="+wt)
	if code != 1 {
		t.Fatalf("exit=%d want 1 (missing tdd artifact); stderr=%s", code, errb)
	}
	if strings.Contains(errb, deliverable.CodeMissingArchitectureDocs) {
		t.Errorf("the docs floor is build-scoped; it must not surface for tdd. got %q", errb)
	}
}
