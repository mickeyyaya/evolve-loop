package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
)

type e3FailingWriter struct{}

func (e3FailingWriter) Write([]byte) (int, error) { return 0, errors.New("stdout closed") }

func TestSalvageProjectRoot_FlagThenEnvThenDot(t *testing.T) {
	cases := []struct{ name, flag, env, want string }{
		{"flag wins over env", "/flag", "/env", "/flag"},
		{"env when no flag", "", "/env", "/env"},
		{"dot when neither", "", "", "."},
		{"unicode flag", "/prøject", "", "/prøject"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("EVOLVE_PROJECT_ROOT", tc.env)
			if got := salvageProjectRoot(tc.flag); got != tc.want {
				t.Errorf("salvageProjectRoot(%q) env=%q = %q, want %q", tc.flag, tc.env, got, tc.want)
			}
		})
	}
}

func TestRunSalvage_ReportDispatchesAndReadsTheEnvRoot(t *testing.T) {
	root := salvageCLIFixture(t)
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	var out, errb bytes.Buffer

	code := runSalvage([]string{"report"}, nil, &out, &errb)

	if code != 0 || !strings.HasPrefix(out.String(), "salvage report: "+filepath.Join(root, ".evolve", deliverable.BadVerdictBaselineFile)+"\n") {
		t.Fatalf("code %d stdout %q stderr %q", code, out.String(), errb.String())
	}
}

func TestRunSalvageList_RefusesBadArgs(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"unknown flag", []string{"list", "--nope"}, "flag provided but not defined: -nope"},
		{"positional", []string{"list", "extra"}, `evolve salvage list: unexpected arguments ["extra"]`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out, errb bytes.Buffer
			if code := runSalvage(tc.args, nil, &out, &errb); code != 10 || out.Len() != 0 || !strings.Contains(errb.String(), tc.want) {
				t.Fatalf("code %d stdout %q stderr %q, want 10 with %q", code, out.String(), errb.String(), tc.want)
			}
		})
	}
}

func e3SalvageLeaf(t *testing.T, root, name, headFile string) {
	t.Helper()
	leaf := filepath.Join(gc.OperatorSalvageDir(filepath.Join(root, ".evolve")), name)
	if err := os.MkdirAll(leaf, 0o755); err != nil {
		t.Fatal(err)
	}
	if headFile != "" {
		if err := os.WriteFile(filepath.Join(leaf, "HEAD"), []byte(headFile), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func e3StubSalvageGit(t *testing.T, code int) {
	t.Helper()
	prev := salvageGitRunner
	t.Cleanup(func() { salvageGitRunner = prev })
	salvageGitRunner = func(context.Context, string, string, []string, []string, io.Reader, io.Writer, io.Writer) (int, error) {
		return code, nil
	}
}

func TestRunSalvageList_UnreadableLeavesExitTwoAfterPrintingTheReadableOnes(t *testing.T) {
	root := t.TempDir()
	e3SalvageLeaf(t, root, "cycle-x-3", "0123456789abcdef0123456789abcdef01234567\n")
	e3SalvageLeaf(t, root, "cycle-y-4", "")
	e3SalvageLeaf(t, root, "cycle-z-5", "")
	e3StubSalvageGit(t, 1)
	var out, errb bytes.Buffer

	code := runSalvage([]string{"list", "--project-root", root}, nil, &out, &errb)

	if code != 2 {
		t.Fatalf("code = %d, want 2 when a leaf cannot be read", code)
	}
	if !strings.Contains(out.String(), "cycle-x-3  cycle=3  branch=-  ") || !strings.Contains(out.String(), "landed=no") {
		t.Errorf("the readable leaf row is missing or wrong: %q", out.String())
	}
	lines := strings.Split(strings.TrimRight(errb.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("stderr lines = %q, want one per unreadable leaf", lines)
	}
	for i, leaf := range []string{"cycle-y-4", "cycle-z-5"} {
		if !strings.HasPrefix(lines[i], "evolve salvage list: gc: salvage leaf "+leaf+": ") {
			t.Errorf("stderr line %d = %q, want it prefixed and naming %s", i, lines[i], leaf)
		}
	}
}

func TestPrintSalvageRows(t *testing.T) {
	yes, no := true, false
	salvaged := time.Now().Add(-49 * time.Hour)
	rows := []salvageListRow{
		{SalvageLeaf: gc.SalvageLeaf{Leaf: "cycle-a-1", Cycle: 1, Branch: "lane/a", Head: "abc", ChangedFiles: 2, PatchBytes: 10, UntrackedFiles: 1, SalvagedAt: salvaged}, Landed: &yes},
		{SalvageLeaf: gc.SalvageLeaf{Leaf: "cycle-b-2", Cycle: 2, Head: "def", SalvagedAt: salvaged}, Landed: &no},
		{SalvageLeaf: gc.SalvageLeaf{Leaf: "cycle-c-3", Cycle: 3, Head: "0ff", SalvagedAt: salvaged}},
	}
	t.Run("no rows print nothing in either mode", func(t *testing.T) {
		for _, asJSON := range []bool{false, true} {
			var out, errb bytes.Buffer
			if code := printSalvageRows(nil, asJSON, &out, &errb); code != 0 || out.Len() != 0 || errb.Len() != 0 {
				t.Errorf("json=%v: code %d out %q err %q", asJSON, code, out.String(), errb.String())
			}
		}
	})
	t.Run("text rows", func(t *testing.T) {
		var out bytes.Buffer
		if code := printSalvageRows(rows, false, &out, io.Discard); code != 0 {
			t.Fatalf("code %d", code)
		}
		want := "cycle-a-1  cycle=1  branch=lane/a  head=abc  changed=2  patch=10B  untracked=1  age=49h  landed=yes\n" +
			"cycle-b-2  cycle=2  branch=-  head=def  changed=0  patch=0B  untracked=0  age=49h  landed=no\n" +
			"cycle-c-3  cycle=3  branch=-  head=0ff  changed=0  patch=0B  untracked=0  age=49h  landed=unknown\n"
		if out.String() != want {
			t.Errorf("text =\n%s\nwant\n%s", out.String(), want)
		}
	})
	t.Run("a failed JSON write exits two", func(t *testing.T) {
		var errb bytes.Buffer
		if code := printSalvageRows(rows, true, e3FailingWriter{}, &errb); code != 2 || !strings.Contains(errb.String(), "evolve salvage list: encode: stdout closed") {
			t.Errorf("code %d stderr %q", code, errb.String())
		}
	})
}

func TestRunSalvageReport_FailurePaths(t *testing.T) {
	writeEvolve := func(t *testing.T, root, name string, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, ".evolve", name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	dirAt := func(t *testing.T, root, name string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Join(root, ".evolve", name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	loopAt := func(t *testing.T, root, name string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
			t.Fatal(err)
		}
		p := filepath.Join(root, ".evolve", name)
		if err := os.Symlink(p, p); err != nil {
			t.Fatal(err)
		}
	}
	cases := []struct {
		name     string
		setup    func(t *testing.T, root string)
		args     []string
		wantCode int
		want     string
	}{
		{"unknown flag", func(*testing.T, string) {}, []string{"--nope"}, 10, "flag provided but not defined"},
		{"baseline unreadable as a stream", func(t *testing.T, root string) { dirAt(t, root, deliverable.BadVerdictBaselineFile) }, nil, 1, "salvage report: read "},
		{"baseline cannot be opened", func(t *testing.T, root string) { loopAt(t, root, deliverable.BadVerdictBaselineFile) }, nil, 1, "salvage report: open " + deliverable.BadVerdictBaselineFile},
		{"applied unreadable as a stream", func(t *testing.T, root string) { dirAt(t, root, deliverable.SalvageAppliedFile) }, nil, 1, "salvage report: read "},
		{"applied cannot be opened", func(t *testing.T, root string) {
			writeEvolve(t, root, deliverable.BadVerdictBaselineFile, "")
			loopAt(t, root, deliverable.SalvageAppliedFile)
		}, nil, 1, "salvage report: open " + deliverable.SalvageAppliedFile},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			tc.setup(t, root)
			var out, errb bytes.Buffer

			code := runSalvageReport(append([]string{"-project-root", root}, tc.args...), &out, &errb)

			if code != tc.wantCode || out.Len() != 0 {
				t.Fatalf("code %d stdout %q stderr %q, want %d", code, out.String(), errb.String(), tc.wantCode)
			}
			if !strings.Contains(strings.ReplaceAll(errb.String(), filepath.Join(root, ".evolve")+string(filepath.Separator), ""), tc.want) {
				t.Errorf("stderr = %q, want %q", errb.String(), tc.want)
			}
		})
	}
}

func TestRunSalvageReport_AFailedJSONWriteExitsOne(t *testing.T) {
	root := salvageCLIFixture(t)
	var errb bytes.Buffer

	code := runSalvageReport([]string{"-json", "-project-root", root}, e3FailingWriter{}, &errb)

	if code != 1 || !strings.Contains(errb.String(), "salvage report: encode: stdout closed") {
		t.Fatalf("code %d stderr %q", code, errb.String())
	}
}
