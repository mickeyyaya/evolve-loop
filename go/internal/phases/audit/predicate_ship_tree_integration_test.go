//go:build integration

package audit

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func gitAt(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func writeAt(t *testing.T, root, rel, body string) {
	t.Helper()
	abs := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func shipTreeFixture(t *testing.T, cycle int) (root, ws string, req core.PhaseRequest) {
	t.Helper()
	root = t.TempDir()
	gitAt(t, root, "init", "-q")
	gitAt(t, root, "config", "user.email", "t@t")
	gitAt(t, root, "config", "user.name", "t")
	writeAt(t, root, ".gitignore", ".evolve/*\n!.evolve/evals/\n!.evolve/evals/*.md\n")
	writeAt(t, root, "go/source.go", "package source\n")
	gitAt(t, root, "add", "-A")
	gitAt(t, root, "commit", "-q", "-m", "base")
	ws = filepath.Join(root, ".evolve", "runs", "cycle-"+strconv.Itoa(cycle))
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	return root, ws, core.PhaseRequest{Cycle: cycle, RunID: "run", AuditRound: 1, Worktree: root, Workspace: ws}
}

func stagedTreeOf(t *testing.T, root string, paths ...string) string {
	t.Helper()
	index := filepath.Join(t.TempDir(), "index")
	real, err := os.ReadFile(filepath.Join(root, ".git", "index"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(index, real, 0o644); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) string {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_INDEX_FILE="+index)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run("add", "-u", "--", ".")
	run(append([]string{"add", "-A", "--"}, paths...)...)
	return run("write-tree")
}

func TestPredicateTreeFor_Cycle1735_TheDeclaredExplanationDocumentIsInTheShipTree(t *testing.T) {
	root, ws, req := shipTreeFixture(t, 1735)
	doc := "docs/explain/builds/cycle-1735-01m3k2r1z13znw2sze4j96xx4k.md"
	writeAt(t, root, "go/source.go", "package source\n\nfunc Shrunk() {}\n")
	writeAt(t, root, doc, "# Why\n")
	writeAt(t, ws, "build-report.md", "1. **What files?** `go/source.go` and `"+doc+"`.\n")
	tree, err := predicateTreeFor(req)
	if err != nil {
		t.Fatalf("Ship commits the declared, untracked explanation document, so the audit runs on it: %v", err)
	}
	if want := stagedTreeOf(t, root, doc); tree != want {
		t.Fatalf("the audited tree %s is not the tree Ship stages %s", tree, want)
	}
}

func TestPredicateTreeFor_Cycle1694_TheDeclaredPredicateAndEvalAreInTheShipTree(t *testing.T) {
	root, ws, req := shipTreeFixture(t, 1694)
	pred := "go/acs/cycle1694/predicates_test.go"
	eval := ".evolve/evals/atomicwrite-linked-state-sweep.md"
	writeAt(t, root, pred, "package cycle1694\n")
	writeAt(t, root, eval, "# eval\n")
	writeAt(t, ws, "test-report.md", "| "+pred+" | 11 |\n| "+eval+" | 6 |\n")
	tree, err := predicateTreeFor(req)
	if err != nil {
		t.Fatalf("Ship commits the declared predicate and eval, so the audit runs on them: %v", err)
	}
	if want := stagedTreeOf(t, root, eval, pred); tree != want {
		t.Fatalf("the audited tree %s is not the tree Ship stages %s", tree, want)
	}
}

func TestPredicateTreeFor_AnUndeclaredUntrackedInputIsRefusedByName(t *testing.T) {
	root, ws, req := shipTreeFixture(t, 1)
	writeAt(t, root, "go/helper.go", "package source\n")
	writeAt(t, root, "go/source.go", "package source\n\nfunc Declared() {}\n")
	writeAt(t, ws, "build-report.md", "Changed `go/source.go`.\n")
	before, err := os.ReadFile(filepath.Join(root, ".git", "index"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = predicateTreeFor(req)
	if err == nil || !strings.Contains(err.Error(), "go/helper.go") {
		t.Fatalf("Ship would leave the undeclared untracked helper out of the commit, so the audit refuses it by name: %v", err)
	}
	if strings.Contains(err.Error(), "go/source.go") {
		t.Errorf("the declared change is not an undeclared input: %v", err)
	}
	if after, err := os.ReadFile(filepath.Join(root, ".git", "index")); err != nil || string(after) != string(before) {
		t.Fatalf("the audit never writes the real index (%v)", err)
	}
}

func TestPredicateTreeFor_AnUndeclaredTrackedEditShipsAndIsAudited(t *testing.T) {
	root, ws, req := shipTreeFixture(t, 1)
	writeAt(t, root, "go/source.go", "package source\n\nvar Edited = true\n")
	writeAt(t, ws, "build-report.md", "No paths named.\n")
	tree, err := predicateTreeFor(req)
	if err != nil {
		t.Fatalf("the audit binding's add -u stages every tracked edit, so Ship commits it and the audit runs on it: %v", err)
	}
	if got := gitAt(t, root, "show", tree+":go/source.go"); !strings.Contains(got, "Edited") {
		t.Fatalf("the audited tree carries the tracked edit: %q", got)
	}
}
