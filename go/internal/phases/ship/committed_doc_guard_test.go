package ship

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
)

const archivedDoc = "docs/private/research/archived-2026-10-20/guide.md"

func docRepo(t *testing.T) string {
	t.Helper()
	root := emptyRepo(t).Dir
	writeDoc(t, root, "docs/guide.md", "# Guide\n\nThe committed guide body that a rename keeps.\n")
	writeDoc(t, root, "knowledge-base/note.md", "# Note\n\nA committed research note.\n")
	writeDoc(t, root, "go/tool.go", "package tool\n")
	gitIn(t, root, "add", "-A")
	gitIn(t, root, "commit", "-q", "-m", "seed")
	return root
}

func writeDoc(t *testing.T, root, rel, body string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return strings.TrimSpace(string(out))
}

func TestCommittedDocDeletions_NamesOnlyDocPathsThatLeaveTheDocRoots(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"empty diff", "", nil},
		{"docs deletion", "D\x00docs/a.md\x00", []string{"docs/a.md"}},
		{"knowledge-base deletion", "D\x00knowledge-base/r.md\x00", []string{"knowledge-base/r.md"}},
		{"upper-case root", "D\x00Docs/a.md\x00", []string{"Docs/a.md"}},
		{"code deletion", "D\x00go/x.go\x00D\x00docsite/a.md\x00", nil},
		{"rename into the archive", "R100\x00docs/a.md\x00" + archivedDoc + "\x00", nil},
		{"rename across doc roots", "R087\x00knowledge-base/r.md\x00docs/r.md\x00", nil},
		{"rename out of the doc roots", "R100\x00docs/a.md\x00tmp/a.md\x00D\x00docs/b.md\x00", []string{"docs/a.md", "docs/b.md"}},
		{"rename into the retired knowledge-base", "R100\x00docs/a.md\x00knowledge-base/a.md\x00", []string{"docs/a.md"}},
		{"rename into an upper-case Docs", "R100\x00docs/a.md\x00Docs/a.md\x00", []string{"docs/a.md"}},
		{"rename within knowledge-base", "R100\x00knowledge-base/research/r.md\x00knowledge-base/notes/r.md\x00", nil},
		{"rename into an upper-case Knowledge-Base", "R100\x00knowledge-base/r.md\x00Knowledge-Base/r.md\x00", []string{"knowledge-base/r.md"}},
		{"rename from an upper-case Knowledge-Base within the root", "R100\x00Knowledge-Base/r.md\x00knowledge-base/r.md\x00", nil},
		{"rename out of knowledge-base", "R100\x00knowledge-base/r.md\x00tmp/r.md\x00", []string{"knowledge-base/r.md"}},
		{"rename into the doc roots", "R100\x00tmp/a.md\x00docs/a.md\x00", nil},
		{"truncated rename", "R100\x00docs/a.md", nil},
	}
	for _, tc := range cases {
		if got := committedDocDeletions(tc.in); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: committedDocDeletions(%q) = %q, want %q", tc.name, tc.in, got, tc.want)
		}
	}
}

func TestRefuseCommittedDocDeletion_RefusesAStagedDeletionByAnyCommand(t *testing.T) {
	t.Parallel()
	root := docRepo(t)
	if err := os.Remove(filepath.Join(root, "docs", "guide.md")); err != nil {
		t.Fatal(err)
	}
	gitIn(t, root, "add", "-A")

	err := refuseCommittedDocDeletion(context.Background(), &Options{ProjectRoot: root}, "--cached")

	se := wantShipErr(t, err, shiperr.CodeCommittedDocDeleted, core.ShipClassPrecondition, "docs/guide.md")
	if se.Stage != core.StageAtomicShip || se.Debug["paths"] != "docs/guide.md" {
		t.Errorf("stage %q debug %v: the refusal is an atomic-ship stop that names the path", se.Stage, se.Debug)
	}
}

func TestRefuseCommittedDocDeletion_ARenameIntoTheArchiveShips(t *testing.T) {
	t.Parallel()
	root := docRepo(t)
	if err := os.MkdirAll(filepath.Join(root, filepath.Dir(filepath.FromSlash(archivedDoc))), 0o755); err != nil {
		t.Fatal(err)
	}
	gitIn(t, root, "mv", "docs/guide.md", archivedDoc)

	if err := refuseCommittedDocDeletion(context.Background(), &Options{ProjectRoot: root}, "--cached"); err != nil {
		t.Fatalf("a rename into the archive home keeps the content, so it must ship: %v", err)
	}
}

func TestRefuseCommittedDocDeletion_ANeverCommittedDraftIsNotACommittedPath(t *testing.T) {
	t.Parallel()
	root := docRepo(t)
	gitIn(t, root, "checkout", "-q", "-b", "lane")
	writeDoc(t, root, "docs/explain/cycle-1830-draft.md", "# Draft\n")
	gitIn(t, root, "add", "-A")
	gitIn(t, root, "commit", "-q", "-m", "lane draft")
	gitIn(t, root, "rm", "-q", "docs/explain/cycle-1830-draft.md")
	gitIn(t, root, "commit", "-q", "-m", "lane retracts its draft")

	if err := refuseCommittedDocDeletion(context.Background(), &Options{ProjectRoot: root}, "main", "lane"); err != nil {
		t.Fatalf("main never held the cycle's own draft, so removing it is not a deletion of committed documentation: %v", err)
	}
}

func TestRefuseCommittedDocDeletion_AGitFailureFailsClosed(t *testing.T) {
	t.Parallel()
	root := docRepo(t)

	err := refuseCommittedDocDeletion(context.Background(), &Options{ProjectRoot: root}, "no-such-rev", "main")

	wantShipErr(t, err, core.CodeGitIO, core.ShipClassTransient, "")
}

func TestShipDirect_RefusesToCommitADocDeletion(t *testing.T) {
	t.Parallel()
	root := docRepo(t)
	gitIn(t, root, "rm", "-q", "knowledge-base/note.md")
	head := gitOut(t, root, "rev-parse", "HEAD")
	opts := &Options{Class: ClassManual, CommitMessage: "chore: drop a note", ProjectRoot: root, Stdout: io.Discard, Stderr: io.Discard}

	err := shipDirect(context.Background(), opts, &RunResult{}, "main")

	wantShipErr(t, err, shiperr.CodeCommittedDocDeleted, core.ShipClassPrecondition, "knowledge-base/note.md")
	if got := gitOut(t, root, "rev-parse", "HEAD"); got != head {
		t.Errorf("HEAD moved to %s: the refusal comes before the commit", got)
	}
}

func TestWorktreeShipLand_RefusesALaneThatDeletesACommittedDoc(t *testing.T) {
	t.Parallel()
	root := docRepo(t)
	wt := filepath.Join(t.TempDir(), "lane")
	gitIn(t, root, "worktree", "add", "-q", "-b", "lane", wt)
	gitIn(t, wt, "rm", "-q", "docs/guide.md")
	tree := gitOut(t, wt, "write-tree")
	pending := gitOut(t, wt, "commit-tree", tree, "-p", "HEAD", "-m", "evolve-cycle 1830")
	main := gitOut(t, root, "rev-parse", "main")
	opts := &Options{ProjectRoot: root, Stdout: io.Discard, Stderr: io.Discard}
	s := newWorktreeShip(context.Background(), opts, &RunResult{}, "main", wt)
	s.cycleBranch = "lane"

	err := s.land(pending)

	wantShipErr(t, err, shiperr.CodeCommittedDocDeleted, core.ShipClassPrecondition, "docs/guide.md")
	if got := gitOut(t, root, "rev-parse", "main"); got != main {
		t.Errorf("main moved to %s: a refused landing leaves main alone", got)
	}
	if got := gitOut(t, wt, "rev-parse", "HEAD"); got != pending {
		t.Errorf("lane HEAD %s, want the refused commit %s kept on the lane for triage", got, pending)
	}
}

func failingGit(fail string, quietExit int) CmdRunner {
	return func(_ context.Context, _, _ string, args, _ []string, _ io.Reader, _, _ io.Writer) (int, error) {
		argv := strings.Join(args, " ")
		if argv == fail {
			return -1, errors.New(fail + " exploded")
		}
		if argv == "diff --cached --quiet" {
			return quietExit, nil
		}
		return 0, nil
	}
}

func TestShipDirect_DryRunStagedProbeFailureIsGitIO(t *testing.T) {
	t.Parallel()
	opts := &Options{Class: ClassManual, DryRun: true, Runner: failingGit("diff --cached --quiet", 0)}

	err := shipDirect(context.Background(), opts, &RunResult{}, "main")

	wantShipErr(t, err, core.CodeGitIO, core.ShipClassTransient, "diff --cached --quiet failed")
}

func TestShipDirect_DryRunFooterFailureStopsBeforeTheCommit(t *testing.T) {
	t.Parallel()
	opts := &Options{Class: ClassCycle, DryRun: true, Runner: failingGit("diff --cached --name-status", 1)}

	err := shipDirect(context.Background(), opts, &RunResult{}, "main")

	if err == nil || !strings.Contains(err.Error(), "diff --name-status") {
		t.Fatalf("shipDirect = %v, want the footer's diff failure", err)
	}
}
