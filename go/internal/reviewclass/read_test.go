package reviewclass

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

const stageFlipDiff = `diff --git a/.evolve/policy.json b/.evolve/policy.json
--- a/.evolve/policy.json
+++ b/.evolve/policy.json
@@ -1 +1 @@
-  "stage": "enforce"
+  "stage": "shadow"
`

type fakeRepo struct {
	out   map[string]string
	fail  string
	files map[string]string
	calls []string
}

func (r *fakeRepo) git(args ...string) (string, error) {
	key := strings.Join(args, " ")
	r.calls = append(r.calls, key)
	if r.fail != "" && strings.Contains(key, r.fail) {
		return "", errors.New("git failed")
	}
	if out, ok := r.out[key]; ok {
		return out, nil
	}
	return "", errors.New("no such object: " + key)
}

func (r *fakeRepo) readFile(path string) ([]byte, error) {
	if body, ok := r.files[path]; ok {
		return []byte(body), nil
	}
	return nil, errors.New("absent: " + path)
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		out: map[string]string{
			"-c core.quotePath=false diff --no-color --no-ext-diff -U0 --no-renames origin/main": stageFlipDiff,
			"-c core.quotePath=false ls-files --others --exclude-standard -z":                    "new.go\x00",
			"-c core.quotePath=false diff --name-only -z --no-renames origin/main":               ".evolve/policy.json\x00go/b.go\x00go/a.go\x00go/testdata/t.go\x00gone.go\x00",
			"show origin/main:go/a.go":          "package a\n\nfunc A() {}\n",
			"show origin/main:go/b.go":          "package b\n",
			"show origin/main:gone.go":          "package gone\n",
			"show origin/main:go/testdata/t.go": "package t\n",
		},
		files: map[string]string{
			"go/a.go":          "package a\n\n// A is a.\nfunc A() {}\n",
			"go/b.go":          "package b\n\nfunc B() {}\n",
			"go/testdata/t.go": "package t\n\n// x\n",
			"new.go":           "package n\n",
		},
	}
}

func TestRead_ListsThePathsTheProvenCommentOnlyFilesAndTheStageFlips(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	var git Git = repo.git
	var readFile ReadFile = repo.readFile
	got, err := Read(git, readFile, "origin/main")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{".evolve/policy.json", "go/a.go", "go/b.go", "go/testdata/t.go", "gone.go", "new.go"}; !slices.Equal(got.Paths, want) {
		t.Errorf("Paths = %v, want %v", got.Paths, want)
	}
	if want := []string{"go/a.go"}; !slices.Equal(got.CommentOnly, want) {
		t.Errorf("CommentOnly = %v, want %v", got.CommentOnly, want)
	}
	if want := []string{".evolve/policy.json"}; !slices.Equal(got.StageFlips, want) {
		t.Errorf("StageFlips = %v, want %v", got.StageFlips, want)
	}
	if slices.Contains(repo.calls, "show origin/main:go/testdata/t.go") {
		t.Error("Read tried to prove a testdata fixture comment-only; tests read fixtures, so no waiver applies")
	}
}

func TestRead_AnUnparsableFileIsNotCommentOnly(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	repo.files["go/a.go"] = "package a\n\nfunc A( {\n"
	got, err := Read(repo.git, repo.readFile, "origin/main")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.CommentOnly) != 0 {
		t.Errorf("CommentOnly = %v, want none", got.CommentOnly)
	}
}

func TestRead_AFailedGitCallIsAnError(t *testing.T) {
	t.Parallel()
	for _, fail := range []string{"-U0", "--name-only"} {
		repo := newFakeRepo()
		repo.fail = fail
		if _, err := Read(repo.git, repo.readFile, "origin/main"); err == nil {
			t.Errorf("git failing on %q: Read returned no error", fail)
		}
	}
}
