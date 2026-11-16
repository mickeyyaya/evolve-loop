package landedclaim

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func commitFile(r *gittest.Repo, name string) string {
	r.Write(name, name+"\n")
	r.Git("add", name)
	r.Git("commit", "-q", "-m", "add "+name)
	return strings.TrimSpace(r.Git("rev-parse", "HEAD"))
}

func TestOnMain_FailsClosedUnlessTheCommitIsOnMainAndTouchesADeclaredPath(t *testing.T) {
	r := gittest.Fixture(t)
	commitFile(r, "seed.txt")
	onMain := commitFile(r, "go/pkg/a.go")
	r.Git("checkout", "-q", "-b", "lane")
	offMain := commitFile(r, "go/pkg/b.go")
	r.Git("checkout", "-q", "main")
	check := OnMain(gitexec.Default(r.Dir))

	for name, tc := range map[string]struct {
		sha   string
		paths []string
		want  bool
	}{
		"on main and touches the file":      {onMain, []string{"go/pkg/a.go"}, true},
		"on main and touches the directory": {onMain, []string{"go/pkg"}, true},
		"on main and touches the dir slash": {onMain, []string{"./go/pkg/"}, true},
		"on main, other path":               {onMain, []string{"go/pkg/other.go"}, false},
		"a sibling prefix is not a parent":  {onMain, []string{"go/pk"}, false},
		"not an ancestor of main":           {offMain, []string{"go/pkg/b.go"}, false},
		"no declared path":                  {onMain, nil, false},
		"not a hex id":                      {"--output=x", []string{"go/pkg/a.go"}, false},
		"unknown id":                        {strings.Repeat("a", 40), []string{"go/pkg/a.go"}, false},
		"empty id":                          {"", []string{"go/pkg/a.go"}, false},
	} {
		if got := check(tc.sha, tc.paths); got != tc.want {
			t.Errorf("%s: check = %v, want %v", name, got, tc.want)
		}
	}
}

func TestOnMain_AGitFaultIsNotLanded(t *testing.T) {
	broken := func(call func(args []string) (int, error)) gitexec.Git {
		return gitexec.Git{Exec: func(_ context.Context, _, _ string, args, _ []string, _ io.Reader, _, _ io.Writer) (int, error) {
			return call(args)
		}}
	}
	sha := strings.Repeat("a", 40)
	paths := []string{"go/a.go"}
	cases := map[string]gitexec.Git{
		"the ancestor probe cannot run": broken(func([]string) (int, error) { return 0, errors.New("no git") }),
		"the ancestor probe exits 128":  broken(func([]string) (int, error) { return 128, nil }),
		"the ancestor probe exits 1":    broken(func([]string) (int, error) { return 1, nil }),
		"the path read fails": broken(func(args []string) (int, error) {
			if args[0] == "merge-base" {
				return 0, nil
			}
			return 0, errors.New("log failed")
		}),
	}
	for name, git := range cases {
		if OnMain(git)(sha, paths) {
			t.Errorf("%s: the check answered landed, want fail closed", name)
		}
	}
}

func TestTouchesAny_ABlankLineNeverMatchesTheRootPath(t *testing.T) {
	if touchesAny([]string{"", "  "}, []string{"."}) {
		t.Error("a blank line of git output matched the declared root path")
	}
	if !touchesAny([]string{"", "go/a.go"}, []string{"go"}) {
		t.Error("a changed path under a declared directory must match")
	}
}

func TestOnMain_AMergeIsJudgedOnlyByWhatItBringsToMain(t *testing.T) {
	r := gittest.Fixture(t)
	commitFile(r, "seed.txt")
	r.Git("checkout", "-q", "-b", "lane")
	commitFile(r, "go/lane.go")
	r.Git("checkout", "-q", "main")
	commitFile(r, "go/mainonly.go")
	r.Git("merge", "-q", "--no-ff", "-m", "merge lane", "lane")
	merge := strings.TrimSpace(r.Git("rev-parse", "HEAD"))
	check := OnMain(gitexec.Default(r.Dir))
	if !check(merge, []string{"go/lane.go"}) {
		t.Errorf("a merge that brings go/lane.go to main was refused")
	}
	if check(merge, []string{"go/mainonly.go"}) {
		t.Errorf("a merge was credited with go/mainonly.go, which main already had")
	}
}
