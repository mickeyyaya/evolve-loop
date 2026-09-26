package phasecmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
)

// docsFloorBuildReport is well-formed, so only the docs floor can turn a case red.
const docsFloorBuildReport = "## Changes\n- go/internal/policy/policy.go\nVerdict: PASS\n"

func docsFloorGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
	}
}

// newDocsFloorWorktree commits a base, then adds changed as untracked files, as a build worktree looks at verify time.
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

func newDocsFloorWorkspace(t *testing.T) string {
	t.Helper()
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "build-report.md"), []byte(docsFloorBuildReport), 0o644); err != nil {
		t.Fatal(err)
	}
	return ws
}

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

func TestPhaseVerify_NonBuildPhase_UnaffectedByDocsFloor(t *testing.T) {
	wt := newDocsFloorWorktree(t, "go/internal/policy/policy.go")
	ws := t.TempDir()

	// The empty workspace fails tdd on well-formedness; the point is which code surfaces.
	code, _, errb := runVerify(t, "tdd", "--workspace="+ws, "--worktree="+wt)
	if code != 1 {
		t.Fatalf("exit=%d want 1 (missing tdd artifact); stderr=%s", code, errb)
	}
	if strings.Contains(errb, deliverable.CodeMissingArchitectureDocs) {
		t.Errorf("the docs floor is build-scoped; it must not surface for tdd. got %q", errb)
	}
}
