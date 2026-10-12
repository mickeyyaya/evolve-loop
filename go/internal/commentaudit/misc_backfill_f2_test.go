package commentaudit

import (
	"errors"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func TestAcrossDiff_ReturnsTheReadErrorNamingTheFileForEachRuleF2(t *testing.T) {
	t.Parallel()
	boom := errors.New("disk boom")
	okRead := func(string) ([]byte, error) { return []byte("package p\n"), nil }
	failRead := func(string) ([]byte, error) { return nil, boom }
	rules := map[string]rule{"comments": commentsRule, "narrative": narrativeRule}
	sides := map[string][2]func(string) ([]byte, error){
		"before fails": {failRead, okRead},
		"after fails":  {okRead, failRead},
	}
	for ruleName, r := range rules {
		for sideName, side := range sides {
			t.Run(ruleName+"/"+sideName, func(t *testing.T) {
				t.Parallel()

				got, err := r.acrossDiff([]string{"skipped.txt", "a.go"}, side[0], side[1])

				if got != nil || !errors.Is(err, boom) || !strings.Contains(err.Error(), "a.go") {
					t.Errorf("acrossDiff = (%v, %v), want nil and the read error naming a.go", got, err)
				}
			})
		}
	}
}

func TestEquivalent_ReportsWhichSideDoesNotParseF2(t *testing.T) {
	t.Parallel()
	valid := []byte("package p\n")
	broken := []byte("package p\nfunc (")
	cases := []struct {
		name          string
		before, after []byte
		wantPrefix    string
	}{
		{"before", broken, valid, "before: "},
		{"after", valid, broken, "after: "},
		{"both name the before side first", broken, broken, "before: "},
		{"empty before", nil, valid, "before: "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			same, why, err := Equivalent("p.go", tc.before, tc.after)

			if same || why != "" || err == nil || !strings.HasPrefix(err.Error(), tc.wantPrefix) {
				t.Errorf("Equivalent = (%v, %q, %v), want false, no reason and an error starting %q", same, why, err, tc.wantPrefix)
			}
		})
	}
}

func TestExecGit_ChangedFilesReportsAFailingUntrackedListingF2(t *testing.T) {
	repo := gitRepoWithCommitF2(t)
	chdir(t, repo)
	t.Setenv("GIT_LITERAL_PATHSPECS", "1")
	t.Setenv("GIT_GLOB_PATHSPECS", "1")

	got, err := ExecGit{}.ChangedFiles("HEAD")

	if got != nil || err == nil || !strings.Contains(err.Error(), "ls-files") {
		t.Errorf("ChangedFiles = (%v, %v), want nil and an error naming ls-files", got, err)
	}
}

func TestExecGit_ChangedFilesListsTrackedEditsAndUntrackedFilesF2(t *testing.T) {
	repo := gitRepoWithCommitF2(t)
	chdir(t, repo)
	writeRepoFile(t, filepath.Join(repo, "tracked.go"), "package p\n\nfunc a() {}\n")
	writeRepoFile(t, filepath.Join(repo, "sub", "new.go"), "package p\n")

	got, err := ExecGit{}.ChangedFiles("HEAD")

	slices.Sort(got)
	if err != nil || !slices.Equal(got, []string{"sub/new.go", "tracked.go"}) {
		t.Errorf("ChangedFiles = (%v, %v), want the edit and the untracked file", got, err)
	}
}

func TestExecGit_ChangedFilesRejectsAnUnknownBaseF2(t *testing.T) {
	repo := gitRepoWithCommitF2(t)
	chdir(t, repo)

	got, err := ExecGit{}.ChangedFiles("no-such-ref")

	if got != nil || err == nil || !strings.Contains(err.Error(), "no-such-ref") {
		t.Errorf("ChangedFiles = (%v, %v), want nil and an error naming the base", got, err)
	}
}

type walkFailsFSF2 struct{}

func (walkFailsFSF2) Open(string) (fs.File, error) { return nil, errors.New("open boom") }

type readFailsFSF2 struct{ fstest.MapFS }

func (f readFailsFSF2) ReadFile(name string) ([]byte, error) {
	if strings.HasSuffix(name, "bad.go") {
		return nil, errors.New("read boom")
	}
	return f.MapFS.ReadFile(name)
}

func TestRank_ReturnsAWalkOrReadFailureF2(t *testing.T) {
	t.Parallel()
	cases := map[string]fs.FS{
		"the root cannot be opened": walkFailsFSF2{},
		"a file cannot be read":     readFailsFSF2{fstest.MapFS{"p/ok.go": {Data: []byte("package p\n")}, "p/bad.go": {Data: []byte("package p\n")}}},
	}
	for name, fsys := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := Rank(fsys)

			if got != nil || err == nil || !strings.Contains(err.Error(), "boom") {
				t.Errorf("Rank = (%v, %v), want nil and the failure", got, err)
			}
		})
	}
}

func TestRank_SkipsVendoredHiddenAndTestdataDirsF2(t *testing.T) {
	t.Parallel()
	src := []byte("package p\n\n// note\nfunc f() {}\n")
	fsys := fstest.MapFS{
		"keep/k.go":      {Data: src},
		"vendor/v.go":    {Data: src},
		"testdata/t.go":  {Data: src},
		".hidden/h.go":   {Data: src},
		"keep/readme.md": {Data: []byte("# not go")},
	}

	got, err := Rank(fsys)

	if err != nil || len(got) != 1 || got[0].Dir != "keep" || got[0].Files != 1 {
		t.Errorf("Rank = (%+v, %v), want only keep with one file", got, err)
	}
}

func TestRank_OrdersByNarrativeThenCommentThenDirF2(t *testing.T) {
	t.Parallel()
	plain := []byte("package p\n\n// plain\nfunc f() {}\n")
	narrative := []byte("package p\n\n// cycle-9 note\nfunc f() {}\n")
	twoPlain := []byte("package p\n\n// plain\n// again\nfunc f() {}\n")
	none := []byte("package p\n\nfunc f() {}\n")
	fsys := fstest.MapFS{
		"d/z.go": {Data: none}, "c/z.go": {Data: none},
		"b/z.go": {Data: plain}, "a/z.go": {Data: twoPlain},
		"n/z.go": {Data: narrative},
	}

	got, err := Rank(fsys)

	var dirs []string
	for _, p := range got {
		dirs = append(dirs, p.Dir)
	}
	if err != nil || !slices.Equal(dirs, []string{"n", "a", "b", "c", "d"}) {
		t.Errorf("Rank order = %v (%v), want narrative first, then more comments, then the dir name", dirs, err)
	}
}

func gitRepoWithCommitF2(t *testing.T) string {
	t.Helper()
	repo := gittest.Fixture(t)
	writeRepoFile(t, filepath.Join(repo.Dir, "tracked.go"), "package p\n")
	repo.Git("add", ".")
	repo.Git("commit", "-q", "-m", "base")
	return repo.Dir
}
