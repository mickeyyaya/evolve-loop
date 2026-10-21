package proctree

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
	"time"
)

func lsofStub(out string, code int, err error, seen *[]string) func(context.Context, string, string, []string, []string, io.Reader, io.Writer, io.Writer) (int, error) {
	return func(_ context.Context, name, _ string, args, _ []string, _ io.Reader, stdout, _ io.Writer) (int, error) {
		*seen = append([]string{name}, args...)
		_, _ = io.WriteString(stdout, out)
		return code, err
	}
}

func TestListCwds_ReadsTheCwdOfEachOwnProcessAndRefusesAnEmptyFailure(t *testing.T) {
	t.Parallel()
	var seen []string
	got, err := ListCwds(context.Background(), lsofStub("p7\nR1\nn/wt/go\n", 1, nil, &seen), "")
	if err != nil || !reflect.DeepEqual(got, []Cwd{{Pid: 7, Ppid: 1, Cwd: "/wt/go"}}) {
		t.Fatalf("ListCwds = %+v, %v; lsof exits 1 when one process is unreadable, and the listing still counts", got, err)
	}
	if want := []string{"lsof", "-a", "-d", "cwd", "-u", strconv.Itoa(os.Getuid()), "-R", "-F", "pRn"}; !reflect.DeepEqual(seen, want) {
		t.Errorf("argv = %v, want %v", seen, want)
	}
	if _, err := ListCwds(context.Background(), lsofStub("", 1, nil, &seen), ""); err == nil {
		t.Error("an lsof exit with no output is a fault, not an empty table")
	}
	if _, err := ListCwds(context.Background(), lsofStub("", -1, errors.New("no lsof"), &seen), ""); err == nil {
		t.Error("an lsof that cannot start is a fault")
	}
}

func TestExecCwdLister_FindsThisTestProcess(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("lsof"); err != nil {
		t.Skip("lsof is not installed")
	}
	var list CwdLister = ExecCwdLister()
	cwds, err := list(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	here, _ := os.Getwd()
	for _, c := range cwds {
		if c.Pid == os.Getpid() && Under(here, c.Cwd) {
			return
		}
	}
	t.Fatalf("the listing of %d processes does not hold this test (pid %d, cwd %s)", len(cwds), os.Getpid(), here)
}

func TestOrphanUnder_TakesOnlyAPrivateOrphanWhoseCwdIsInsideTheTree(t *testing.T) {
	t.Parallel()
	tree := t.TempDir()
	cwds := []Cwd{{Pid: 10, Cwd: filepath.Join(tree, "go")}, {Pid: 11, Cwd: tree}, {Pid: 12, Cwd: t.TempDir()}, {Pid: 13, Cwd: tree}}
	since := time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)
	proof := OrphanUnder(tree, since, cwds)
	for _, tc := range []struct {
		name string
		p    Process
		want bool
	}{
		{"an orphan busy loop in a subdirectory", Process{Pid: 10, Ppid: 1, Started: since, Comm: "sh"}, true},
		{"an orphan that started before the dispatch", Process{Pid: 10, Ppid: 1, Started: since.Add(-time.Second), Comm: "sh"}, false},
		{"a child of a live parent", Process{Pid: 11, Ppid: 400, Started: since, Comm: "sh"}, false},
		{"an orphan elsewhere", Process{Pid: 12, Ppid: 1, Started: since, Comm: "sh"}, false},
		{"a shared helper", Process{Pid: 13, Ppid: 1, Started: since, Comm: "tmux"}, false},
		{"a process lsof did not list", Process{Pid: 14, Ppid: 1, Started: since, Comm: "sh"}, false},
	} {
		if got := proof(tc.p); got != tc.want {
			t.Errorf("%s: proof = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestUnder_MatchesThroughASymlinkAndRefusesASibling(t *testing.T) {
	t.Parallel()
	real := t.TempDir()
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(real, "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		dir, path string
		want      bool
	}{
		{link, filepath.Join(real, "x"), true},
		{real, real, true},
		{real, real + "-sibling", false},
		{real, filepath.Dir(real), false},
		{"", real, false},
		{real, "", false},
	} {
		if got := Under(tc.dir, tc.path); got != tc.want {
			t.Errorf("Under(%q, %q) = %v, want %v", tc.dir, tc.path, got, tc.want)
		}
	}
}

func TestOrphanUnder_ARootRelativeOrSymlinkedPathMatchesNothing(t *testing.T) {
	t.Parallel()
	target := t.TempDir()
	link := filepath.Join(t.TempDir(), "wt")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	realTarget, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatal(err)
	}
	intoTarget := filepath.Join(t.TempDir(), "into")
	if err := os.Symlink(realTarget, intoTarget); err != nil {
		t.Fatal(err)
	}
	orphan := Process{Pid: 10, Ppid: 1, Comm: "sh"}
	for _, tc := range []struct {
		name, base, cwd string
	}{
		{"a cwd path that reaches the base only through a symlink", realTarget, intoTarget},
		{"the filesystem root", "/", "/"},
		{"the filesystem root, unclean", "//.", "/Users"},
		{"an empty base", "", "/wt"},
		{"a relative base", "wt", "wt"},
		{"a link to the cwd, which is not resolved again", link, target},
	} {
		if OrphanUnder(tc.base, time.Time{}, []Cwd{{Pid: 10, Ppid: 1, Cwd: tc.cwd}})(orphan) {
			t.Errorf("%s: OrphanUnder(%q) took an orphan in %q", tc.name, tc.base, tc.cwd)
		}
	}
}
