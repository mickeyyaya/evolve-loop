package core

// worktree_inplace_mutators_test.go — the invariant "a cycle never mutates the
// operator's tree" is enforced INSIDE each worktree mutator, not at its
// callers: a resume path, a composition seam or the next in-place root cannot
// forget it. Each helper, handed the project root as its worktree, must leave
// the tree byte-identical (an unformatted tracked file, an uncommitted edit,
// the index, the commit count) and say so.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type inPlaceRepo struct {
	root        string
	unformatted string // go/seed.go's exact bytes, gofmt would rewrite them
	edit        string // README.md's exact bytes, an uncommitted edit
}

func newInPlaceRepo(t *testing.T) inPlaceRepo {
	t.Helper()
	root := t.TempDir()
	initDossierRepo(t, root)
	r := inPlaceRepo{root: root, unformatted: "package seed\n\nfunc   Ugly( ) int {\n\treturn   1\n}\n", edit: "seed\noperator's uncommitted edit\n"}
	for rel, content := range map[string]string{"go/go.mod": "module example.com/seed\n\ngo 1.24\n", "go/seed.go": r.unformatted, "README.md": "seed\n"} {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	gitOutput(t, root, "add", "-A")
	gitOutput(t, root, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "seed module")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte(r.edit), 0o644); err != nil {
		t.Fatal(err)
	}
	return r
}

// fingerprint is what a mutator could change: file bytes, porcelain status
// (index included), commit count.
func (r inPlaceRepo) fingerprint(t *testing.T) string {
	t.Helper()
	seed, _ := os.ReadFile(filepath.Join(r.root, "go", "seed.go"))
	readme, _ := os.ReadFile(filepath.Join(r.root, "README.md"))
	status := gitOutput(t, r.root, "status", "--porcelain")
	return string(seed) + "|" + string(readme) + "|" + status + "|" + gitOutput(t, r.root, "rev-list", "--all", "--count")
}

func gitOutput(t *testing.T, root string, args ...string) string {
	t.Helper()
	out, _, err := gitCapture(context.Background(), root, args...)
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return strings.TrimSpace(out)
}

func TestWorktreeMutators_RefuseTheProjectRoot(t *testing.T) {
	ctx := context.Background()
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	for _, tc := range []struct {
		name string
		run  func(t *testing.T, r inPlaceRepo) string // returns what the helper reported
	}{
		{"normalizeBuildWorktree (gofmt -w, reset --soft, projection regen)", func(t *testing.T, r inPlaceRepo) string {
			return captureStderr(t, func() {
				o.normalizeBuildWorktree(ctx, PhaseBuild, CycleState{ActiveWorktree: r.root, WorktreeBaseSHA: gitOutput(t, r.root, "rev-parse", "HEAD")}, r.root)
			})
		}},
		{"recoverBuildLeak (git checkout -- / add -f)", func(t *testing.T, r inPlaceRepo) string {
			if !recoverBuildLeak(ctx, r.root, r.root, map[string]bool{}, true) {
				t.Error("an in-place root has nothing to relocate — recovery reports done, never failed")
			}
			return "recovered"
		}},
		{"worktreeContentSHA (git add -u)", func(t *testing.T, r inPlaceRepo) string {
			if sha := worktreeContentSHA(ctx, r.root, r.root); sha != "" {
				t.Errorf("no content SHA is written from the operator's index, got %q", sha)
			}
			return "no sha"
		}},
		{"snapshotPreservedWorktree (add -A + commit)", func(t *testing.T, r inPlaceRepo) string {
			sha, err := snapshotPreservedWorktree(ctx, r.root, r.root)
			if err == nil || sha != "" {
				t.Errorf("a salvage snapshot of the project root is refused, got sha=%q err=%v", sha, err)
			}
			if err == nil || !strings.Contains(err.Error(), "the active worktree is the project root — no salvage snapshot") {
				t.Errorf("the refusal says why: %v", err)
			}
			return "refused"
		}},
		{"rebaseCycleBranchOntoMain", func(t *testing.T, r inPlaceRepo) string {
			ok, conflict := rebaseCycleBranchOntoMain(ctx, r.root, r.root)
			if ok || conflict {
				t.Errorf("the operator's tree is never rebased, got ok=%v conflict=%v", ok, conflict)
			}
			return "refused"
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newInPlaceRepo(t)
			before := r.fingerprint(t)
			tc.run(t, r)
			if after := r.fingerprint(t); after != before {
				t.Errorf("the operator's tree changed:\nbefore %q\nafter  %q", before, after)
			}
		})
	}
}

// The audit binding degrades, on an in-place root, to an empty worktree tree
// SHA (ship then falls back to the auditor's value) — a behavioural contract
// with ship, pinned here rather than only through the e2e porcelain compare.
func TestRecordAuditBinding_InPlaceRootDegradesToNoTreeSHA(t *testing.T) {
	r := newInPlaceRepo(t)
	ws := filepath.Join(r.root, ".evolve", "runs", "cycle-7")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "audit-report.md"), []byte("## Verdict\n**PASS**\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	led := &fakeLedger{}
	o := NewOrchestrator(nil, led, nil)
	before := r.fingerprint(t)
	o.emitPhaseBindings(context.Background(), 7, r.root, CycleState{CycleID: 7, WorkspacePath: ws, ActiveWorktree: r.root}, PhaseAudit, VerdictPASS)
	if len(led.entries) != 1 || led.entries[0].WorktreeTreeSHA != "" {
		t.Fatalf("binding on an in-place root carries no worktree tree SHA: %+v", led.entries)
	}
	if after := r.fingerprint(t); after != before {
		t.Errorf("the binding staged the operator's index:\nbefore %q\nafter  %q", before, after)
	}
}
