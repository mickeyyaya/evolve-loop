package changedpkgs

import (
	"reflect"
	"sort"
	"testing"
)

// ChangedFilesChecked reads the WORKING TREE: an unstaged modification, an
// index-added file and an untracked file all count, and only the two the base
// does not have are Added. FromGitChecked is its projection.
func TestChangedFilesChecked_WorkingTreeNotIndex(t *testing.T) {
	root := newRepoWithBaseline(t)
	writeFile(t, root, "go/internal/base/base.go", "package base\n\nconst Changed = true\n") // unstaged
	writeFile(t, root, "go/internal/base/staged_test.go", "package base\n")                  // index-added
	gitCmd(t, root, "add", "go/internal/base/staged_test.go")
	writeFile(t, root, "go/internal/newpkg/new.go", "package newpkg\n") // untracked
	writeFile(t, root, "docs/note.md", "not go\n")                      // untracked, not Go

	files, ok := ChangedFilesChecked(root, "HEAD")
	if !ok {
		t.Fatal("git answered")
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	want := []ChangedFile{
		{Path: "docs/note.md", Added: true},
		{Path: "go/internal/base/base.go", Added: false},
		{Path: "go/internal/base/staged_test.go", Added: true},
		{Path: "go/internal/newpkg/new.go", Added: true},
	}
	if !reflect.DeepEqual(files, want) {
		t.Errorf("files = %+v\nwant   %+v", files, want)
	}
	pkgs, ok := FromGitChecked(root, "HEAD")
	if !ok || !reflect.DeepEqual(pkgs, []string{"./internal/base/...", "./internal/newpkg/..."}) {
		t.Errorf("FromGitChecked = %v ok=%v", pkgs, ok)
	}
}

// A rename is reported once, at its destination, as Added.
func TestChangedFilesChecked_RenameIsAddedAtTheDestination(t *testing.T) {
	root := newRepoWithBaseline(t)
	gitCmd(t, root, "mv", "go/internal/base/base.go", "go/internal/base/moved.go")
	files, ok := ChangedFilesChecked(root, "HEAD")
	if !ok || len(files) != 1 || files[0].Path != "go/internal/base/moved.go" || !files[0].Added {
		t.Errorf("files = %+v ok=%v", files, ok)
	}
}

// Not derivable is never "nothing changed".
func TestChangedFilesChecked_NotDerivable(t *testing.T) {
	for name, args := range map[string][2]string{
		"empty root": {"", "HEAD"},
		"empty base": {newRepoWithBaseline(t), ""},
		"no repo":    {t.TempDir(), "HEAD"},
		"bad ref":    {newRepoWithBaseline(t), "no-such-ref"},
	} {
		if files, ok := ChangedFilesChecked(args[0], args[1]); ok || files != nil {
			t.Errorf("%s: files=%v ok=%v, want nil,false", name, files, ok)
		}
	}
	if files, ok := ChangedFilesChecked(newRepoWithBaseline(t), "HEAD"); !ok || len(files) != 0 {
		t.Errorf("clean tree: files=%v ok=%v, want none,true", files, ok)
	}
}
