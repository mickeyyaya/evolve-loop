package plane

// plane_test.go — ADR-0080 S2: the loop must KNOW which worktree plane it
// occupies. Batch-15 lost four lanes to operator activity in the shared
// primary checkout (1149-1152); the classification below is what lets the
// loop boot-WARN when launched from the primary and lets `evolve doctor
// plane` report the layout. Filesystem-only on purpose: .git is a directory
// in the primary checkout and a `gitdir:` pointer FILE in a linked worktree —
// no git subprocess, hermetic under t.TempDir.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func primaryFixture(t *testing.T, branch string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	head := "ref: refs/heads/" + branch + "\n"
	if err := os.WriteFile(filepath.Join(root, ".git", "HEAD"), []byte(head), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func linkedFixture(t *testing.T, branch string) (root, gitdir string) {
	t.Helper()
	root = t.TempDir()
	hub := t.TempDir()
	gitdir = filepath.Join(hub, "worktrees", "runtime")
	if err := os.MkdirAll(gitdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git"), []byte("gitdir: "+gitdir+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitdir, "HEAD"), []byte("ref: refs/heads/"+branch+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, gitdir
}

func TestClassify_PrimaryCheckout(t *testing.T) {
	root := primaryFixture(t, "console-plane")
	got, err := Classify(root)
	if err != nil {
		t.Fatal(err)
	}
	if got.IsLinkedWorktree {
		t.Errorf("primary checkout classified as linked worktree: %+v", got)
	}
	if got.Branch != "console-plane" {
		t.Errorf("Branch = %q, want console-plane", got.Branch)
	}
}

func TestClassify_LinkedWorktree(t *testing.T) {
	root, _ := linkedFixture(t, "main")
	got, err := Classify(root)
	if err != nil {
		t.Fatal(err)
	}
	if !got.IsLinkedWorktree {
		t.Errorf("linked worktree classified as primary: %+v", got)
	}
	if got.Branch != "main" {
		t.Errorf("Branch = %q, want main", got.Branch)
	}
}

func TestClassify_DetachedHEADIsNotABranch(t *testing.T) {
	root := primaryFixture(t, "x")
	if err := os.WriteFile(filepath.Join(root, ".git", "HEAD"), []byte("0123456789abcdef0123456789abcdef01234567\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Classify(root)
	if err != nil {
		t.Fatal(err)
	}
	if got.Branch != "" {
		t.Errorf("detached HEAD must report an empty Branch, got %q", got.Branch)
	}
}

func TestClassify_NotARepoErrors(t *testing.T) {
	if _, err := Classify(t.TempDir()); err == nil {
		t.Fatal("a directory with no .git must error, not misclassify")
	}
}

// TestBootLine_WarnsOnlyOnPrimary pins the operator-facing contract: the boot
// line for a linked worktree is informational; for the PRIMARY checkout it
// carries the ADR-0080 warning that operator activity here kills lanes.
func TestBootLine_WarnsOnlyOnPrimary(t *testing.T) {
	primary := Info{IsLinkedWorktree: false, Branch: "main"}
	linked := Info{IsLinkedWorktree: true, Branch: "main"}
	if w := BootLine(primary); !containsAll(w, "PRIMARY", "ADR-0080") {
		t.Errorf("primary boot line must warn with the ADR reference, got %q", w)
	}
	if w := BootLine(linked); containsAll(w, "PRIMARY") {
		t.Errorf("linked-worktree boot line must not warn, got %q", w)
	}
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}

// TestClassify_RelativeGitdirPointerResolvesAgainstRoot (review MEDIUM): git
// >=2.48 relative pointers must resolve against the checkout, or Branch
// blanks and the S3 sync becomes a silent permanent no-op.
func TestClassify_RelativeGitdirPointerResolvesAgainstRoot(t *testing.T) {
	root := t.TempDir()
	gitdir := filepath.Join(root, "hub", "worktrees", "rt")
	if err := os.MkdirAll(gitdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git"), []byte("gitdir: hub/worktrees/rt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitdir, "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Classify(root)
	if err != nil {
		t.Fatal(err)
	}
	if got.Branch != "main" {
		t.Fatalf("relative gitdir pointer lost the branch: %+v", got)
	}
}

// TestClassify_ForeignDotGitFileErrors (review MEDIUM): a non-gitdir .git
// file must ERROR — silently classifying it as a linked worktree turns the
// PRIMARY-checkout tripwire dark in the direction that matters.
func TestClassify_ForeignDotGitFileErrors(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".git"), []byte("not a pointer\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Classify(root); err == nil {
		t.Fatal("a foreign .git file must error, not read as the runtime plane")
	}
}

// TestCommonGitDir_WalksCommondir: the hub resolution the console lease
// depends on — linked worktrees resolve through their commondir file;
// primaries are their own hub.
func TestCommonGitDir_WalksCommondir(t *testing.T) {
	root, gitdir := linkedFixture(t, "main")
	if err := os.WriteFile(filepath.Join(gitdir, "commondir"), []byte("../..\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := Classify(root)
	if err != nil {
		t.Fatal(err)
	}
	common, err := CommonGitDir(info)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Clean(filepath.Join(gitdir, "..", ".."))
	if common != want {
		t.Fatalf("common dir = %q, want %q", common, want)
	}
	primary := primaryFixture(t, "main")
	pinfo, err := Classify(primary)
	if err != nil {
		t.Fatal(err)
	}
	pcommon, err := CommonGitDir(pinfo)
	if err != nil {
		t.Fatal(err)
	}
	if pcommon != pinfo.GitDir {
		t.Fatalf("primary common dir = %q, want its own .git %q", pcommon, pinfo.GitDir)
	}
}
