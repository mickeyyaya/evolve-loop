package plane

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

func TestClassify_ForeignDotGitFileErrors(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".git"), []byte("not a pointer\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Classify(root); err == nil {
		t.Fatal("a foreign .git file must error, not read as the runtime plane")
	}
}

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

func TestConsoleLeaseFileName_IsTheSingleSharedName(t *testing.T) {
	if ConsoleLeaseFileName != "evolve-console-lease.json" {
		t.Fatalf("ConsoleLeaseFileName = %q — renaming it orphans existing hub leases; coordinate reader+writer+docs", ConsoleLeaseFileName)
	}
}
