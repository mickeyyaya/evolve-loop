package commentaudit

import (
	"bytes"
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

var errGitBoomF2 = errors.New("git boom")

type scriptedGitF2 struct {
	changed        []string
	changedErr     error
	root           string
	rootErr        error
	rootFailsAfter int
	rootCalls      *int
	showErr        error
	base           map[string]string
}

func (g scriptedGitF2) ChangedFiles(string) ([]string, error) { return g.changed, g.changedErr }

func (g scriptedGitF2) Root() (string, error) {
	if g.rootCalls != nil {
		*g.rootCalls++
		if g.rootFailsAfter > 0 && *g.rootCalls > g.rootFailsAfter {
			return "", errGitBoomF2
		}
	}
	return g.root, g.rootErr
}

func (g scriptedGitF2) Show(_, path string) ([]byte, error) {
	if g.showErr != nil {
		return nil, g.showErr
	}
	body, ok := g.base[path]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return []byte(body), nil
}

func TestLoadDiffFrom_ReportsEachFailingGitStepAndExitsOneF2(t *testing.T) {
	t.Parallel()
	missingRoot := filepath.Join(t.TempDir(), "gone")
	cases := []struct {
		name string
		git  scriptedGitF2
		dirs []string
		want string
	}{
		{"changed files fail", scriptedGitF2{changedErr: errGitBoomF2}, nil, "git boom"},
		{"root fails", scriptedGitF2{changed: []string{"a.go"}, rootErr: errGitBoomF2}, nil, "git boom"},
		{"scoping against a root that is gone", scriptedGitF2{changed: []string{"a.go"}, root: missingRoot}, []string{"p"}, "no such file"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var stderr bytes.Buffer

			d, code := loadDiffFrom("HEAD", tc.dirs, &stderr, tc.git)

			if code != 1 || d.files != nil || d.before != nil || d.after != nil || !strings.Contains(stderr.String(), tc.want) {
				t.Errorf("loadDiffFrom = (%+v, %d), stderr %q; want an empty diff, exit 1 and %q", d, code, stderr.String(), tc.want)
			}
		})
	}
}

func TestMain_CommentsFailsWhenABaseFileCannotBeReadF2(t *testing.T) {
	t.Parallel()
	root := writeTree(t, map[string]string{"p.go": "package p\n"})
	git := scriptedGitF2{changed: []string{"p.go"}, root: root, showErr: errGitBoomF2}
	var out, errOut bytes.Buffer

	code := Main([]string{"comments", "-base", "HEAD"}, &out, &errOut, git)

	if code != 1 || out.Len() != 0 || !strings.Contains(errOut.String(), "p.go") || !strings.Contains(errOut.String(), "git boom") {
		t.Errorf("exit %d, stdout %q, stderr %q; want exit 1 naming p.go and the cause", code, out.String(), errOut.String())
	}
}

func TestMain_HistoryRefusesBadArgumentsAndFailuresF2(t *testing.T) {
	t.Parallel()
	root := writeTree(t, map[string]string{"go/internal/p/p.go": stripped})
	changed := []string{"go/internal/p/p.go"}
	base := map[string]string{"go/internal/p/p.go": withHistory}
	cases := []struct {
		name     string
		args     []string
		git      Git
		wantCode int
		wantErr  string
	}{
		{"no base", []string{"history"}, scriptedGitF2{}, 2, "usage: commentaudit"},
		{"unknown flag", []string{"history", "-bogus"}, scriptedGitF2{}, 2, "flag provided but not defined"},
		{"changed files fail", []string{"history", "-base", "HEAD"}, scriptedGitF2{changedErr: errGitBoomF2}, 1, "git boom"},
		{"no changed file under the dir", []string{"history", "-base", "HEAD", "elsewhere"}, scriptedGitF2{changed: changed, root: root}, 1, "no changed Go files under elsewhere"},
		{"a base file cannot be read", []string{"history", "-base", "HEAD"}, scriptedGitF2{changed: changed, root: root, showErr: errGitBoomF2}, 1, "git boom"},
		{"the archive dir cannot be resolved", []string{"history", "-base", "HEAD", "-out", "rel/dir"}, scriptedGitF2{changed: changed, root: root, base: base, rootCalls: new(int), rootFailsAfter: 1}, 1, "git boom"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var out, errOut bytes.Buffer

			code := Main(tc.args, &out, &errOut, tc.git)

			if code != tc.wantCode || !strings.Contains(errOut.String(), tc.wantErr) || out.Len() != 0 {
				t.Errorf("exit %d, stdout %q, stderr %q; want exit %d and stderr containing %q", code, out.String(), errOut.String(), tc.wantCode, tc.wantErr)
			}
		})
	}
}

func TestArchiveDir_ResolvesRelativeToTheRepoRootAndKeepsAbsoluteF2(t *testing.T) {
	t.Parallel()
	abs := filepath.Join(t.TempDir(), "archive")
	calls := 0
	cases := []struct {
		name    string
		out     string
		git     Git
		want    string
		wantErr error
	}{
		{"absolute needs no git", abs, scriptedGitF2{rootErr: errGitBoomF2, rootCalls: &calls}, abs, nil},
		{"relative joins the root", "docs/h", scriptedGitF2{root: "/repo"}, filepath.Join("/repo", "docs/h"), nil},
		{"relative with a failing root", "docs/h", scriptedGitF2{rootErr: errGitBoomF2}, "", errGitBoomF2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := archiveDir(tc.out, tc.git)

			if got != tc.want || !errors.Is(err, tc.wantErr) {
				t.Errorf("archiveDir = (%q, %v), want (%q, %v)", got, err, tc.want, tc.wantErr)
			}
		})
	}
	if calls != 0 {
		t.Errorf("an absolute out asked git for the root %d time(s)", calls)
	}
}

func TestMain_StripReportsAMissingDirAndExitsOneF2(t *testing.T) {
	t.Parallel()
	missing := filepath.Join(t.TempDir(), "gone")
	var out, errOut bytes.Buffer

	code := Main([]string{"strip", missing}, &out, &errOut, nil)

	if code != 1 || out.Len() != 0 || !strings.Contains(errOut.String(), "gone") {
		t.Errorf("exit %d, stdout %q, stderr %q; want exit 1 naming the missing dir and no report", code, out.String(), errOut.String())
	}
}
