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

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

type backupsFakeGit struct {
	heads     map[string]string
	ancestor  map[string]int
	remotes   map[string]string
	applied   map[string]int
	bundleErr error
	catErr    error
	applyErr  error
}

func (f backupsFakeGit) run(_ context.Context, _, _ string, args, _ []string, _ io.Reader, stdout, _ io.Writer) (int, error) {
	switch args[0] {
	case "bundle":
		if f.bundleErr != nil {
			return -1, f.bundleErr
		}
		io.WriteString(stdout, f.heads[filepath.Base(args[2])])
	case "cat-file":
		if f.catErr != nil {
			return -1, f.catErr
		}
	case "merge-base":
		return f.ancestor[args[2]], nil
	case "branch":
		io.WriteString(stdout, f.remotes[args[3]])
	case "apply":
		if f.applyErr != nil {
			return -1, f.applyErr
		}
		return f.applied[filepath.Base(args[len(args)-1])], nil
	}
	return 0, nil
}

func backupsVerifyDir(t *testing.T, names ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, n := range names {
		if strings.HasSuffix(n, "/") {
			if err := os.Mkdir(filepath.Join(dir, n), 0o755); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestVerifyBackups(t *testing.T) {
	t.Parallel()
	safe := backupsFakeGit{
		heads:    map[string]string{"lane.bundle": "aaa refs/heads/lane\nbbb refs/heads/side\n\n"},
		ancestor: map[string]int{"aaa": 0, "bbb": 1},
		remotes:  map[string]string{"bbb": "  origin/HEAD -> origin/main\n  origin/side\n"},
		applied:  map[string]int{"fix.patch": 0},
	}
	unsafe := backupsFakeGit{
		heads:    map[string]string{"lane.bundle": "ccc refs/heads/lost\n"},
		ancestor: map[string]int{"ccc": 1},
		remotes:  map[string]string{"ccc": "  origin/HEAD -> origin/main\n"},
		applied:  map[string]int{"fix.patch": 1},
	}
	cases := []struct {
		name       string
		git        backupsFakeGit
		files      []string
		wantCode   int
		wantStdout []string
		wantStderr []string
	}{
		{"an empty directory is safe", safe, nil, 0,
			[]string{"KIND  FILE  HEAD  STATUS\n", "every head and patch exists elsewhere"}, nil},
		{"heads in main or on a remote branch and an applied patch are safe", safe,
			[]string{"lane.bundle", "fix.patch", "notes.txt", "nested.bundle/"}, 0,
			[]string{"BUNDLE  lane.bundle  aaa   IN_MAIN", "BUNDLE  lane.bundle  bbb   OTHER_BRANCH", "PATCH   fix.patch    -     APPLIED", "every head and patch exists elsewhere"}, nil},
		{"a head only in the backup and an unapplied patch are refused", unsafe,
			[]string{"lane.bundle", "fix.patch"}, 1,
			[]string{"ccc   ONLY_IN_BACKUP", "UNAPPLIED"},
			[]string{"refusing: head ccc (lane.bundle refs/heads/lost) exists only in the backup", "refusing: patch fix.patch is not applied to origin/main"}},
		{"one unapplied patch alone is refused", unsafe, []string{"fix.patch"}, 1,
			[]string{"PATCH  fix.patch  -     UNAPPLIED"}, []string{"refusing: patch fix.patch is not applied to origin/main\n"}},
		{"a bundle that cannot be listed is an error", backupsFakeGit{bundleErr: errors.New("corrupt bundle")},
			[]string{"lane.bundle"}, 2, []string{"KIND  FILE  HEAD  STATUS\n"}, []string{"backups verify: gitexec: bundle list-heads", "corrupt bundle"}},
		{"a head that cannot be classified is an error", backupsFakeGit{heads: map[string]string{"lane.bundle": "ddd refs/heads/x\n"}, catErr: errors.New("no git")},
			[]string{"lane.bundle"}, 2, nil, []string{"backups verify: gitexec: cat-file ddd: "}},
		{"a patch that cannot be checked is an error", backupsFakeGit{applyErr: errors.New("apply crashed")},
			[]string{"fix.patch"}, 2, nil, []string{"backups verify: gitexec: apply --check"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var out, errb bytes.Buffer
			code := verifyBackups(context.Background(), gitexec.Git{Dir: t.TempDir(), Exec: tc.git.run}, backupsVerifyDir(t, tc.files...), &out, &errb)
			if code != tc.wantCode {
				t.Fatalf("code = %d, want %d (stdout=%q stderr=%q)", code, tc.wantCode, out.String(), errb.String())
			}
			for _, w := range tc.wantStdout {
				if !strings.Contains(out.String(), w) {
					t.Errorf("stdout = %q, want it to contain %q", out.String(), w)
				}
			}
			for _, w := range tc.wantStderr {
				if !strings.Contains(errb.String(), w) {
					t.Errorf("stderr = %q, want it to contain %q", errb.String(), w)
				}
			}
			if tc.wantCode == 0 && errb.Len() != 0 {
				t.Errorf("stderr = %q, want silence on a safe set", errb.String())
			}
			if tc.wantCode != 0 && strings.Contains(out.String(), "exists elsewhere") {
				t.Errorf("stdout = %q, must not claim safety on a refusal", out.String())
			}
		})
	}
}

func TestVerifyBackups_AnUnreadableDirectoryIsAnError(t *testing.T) {
	t.Parallel()
	var out, errb bytes.Buffer
	missing := filepath.Join(t.TempDir(), "absent")

	code := verifyBackups(context.Background(), gitexec.Git{Exec: backupsFakeGit{}.run}, missing, &out, &errb)

	if code != 2 || out.Len() != 0 || !strings.HasPrefix(errb.String(), "backups verify: open "+missing) {
		t.Errorf("code=%d stdout=%q stderr=%q, want 2 and the open error", code, out.String(), errb.String())
	}
}

func TestVerifyBundle_ReturnsOnlyTheHeadsThatLiveNowhereElse(t *testing.T) {
	t.Parallel()
	g := backupsFakeGit{
		heads:    map[string]string{"b.bundle": "aaa refs/heads/a\nbbb refs/heads/b\nccc refs/heads/c\n"},
		ancestor: map[string]int{"aaa": 0, "bbb": 1, "ccc": 1},
		remotes:  map[string]string{"bbb": "origin/b\n", "ccc": ""},
	}
	var w bytes.Buffer

	unsafe, err := verifyBundle(context.Background(), gitexec.Git{Exec: g.run}, &w, "b.bundle", "/x/b.bundle")

	if err != nil {
		t.Fatal(err)
	}
	if len(unsafe) != 1 || unsafe[0] != "head ccc (b.bundle refs/heads/c) exists only in the backup" {
		t.Errorf("unsafe = %q, want only ccc", unsafe)
	}
	if want := "BUNDLE\tb.bundle\taaa\tIN_MAIN\nBUNDLE\tb.bundle\tbbb\tOTHER_BRANCH\nBUNDLE\tb.bundle\tccc\tONLY_IN_BACKUP\n"; w.String() != want {
		t.Errorf("rows = %q, want %q", w.String(), want)
	}
}

func TestVerifyPatch(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		code     int
		applyErr error
		wantRow  string
		want     []string
		wantErr  bool
	}{
		{"applied", 0, nil, "PATCH\tp.patch\t-\tAPPLIED\n", nil, false},
		{"unapplied", 1, nil, "PATCH\tp.patch\t-\tUNAPPLIED\n", []string{"patch p.patch is not applied to origin/main"}, false},
		{"git refuses with another exit", 128, nil, "", nil, true},
		{"git cannot run", 0, errors.New("exec failed"), "", nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := backupsFakeGit{applied: map[string]int{"p.patch": tc.code}, applyErr: tc.applyErr}
			var w bytes.Buffer
			got, err := verifyPatch(context.Background(), gitexec.Git{Exec: g.run}, &w, "p.patch", "/x/p.patch")
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if w.String() != tc.wantRow || strings.Join(got, "|") != strings.Join(tc.want, "|") {
				t.Errorf("row=%q unsafe=%q, want row=%q unsafe=%q", w.String(), got, tc.wantRow, tc.want)
			}
		})
	}
}

func TestRunBackups_ArgumentErrors(t *testing.T) {
	t.Parallel()
	cases := []struct {
		args []string
		want string
	}{
		{nil, backupsUsage + "\n"},
		{[]string{"list"}, backupsUsage + "\n"},
		{[]string{"verify", "--dir"}, "backups verify: bad argument \"--dir\"\n" + backupsUsage + "\n"},
		{[]string{"verify", "--json"}, "backups verify: bad argument \"--json\"\n" + backupsUsage + "\n"},
		{[]string{"verify", "--dir", "/x", "extra"}, "backups verify: bad argument \"extra\"\n" + backupsUsage + "\n"},
	}
	for _, tc := range cases {
		var out, errb bytes.Buffer
		code := runBackups(tc.args, nil, &out, &errb)
		if code != 2 || out.Len() != 0 || errb.String() != tc.want {
			t.Errorf("runBackups(%q) = %d stdout=%q stderr=%q, want 2 and %q", tc.args, code, out.String(), errb.String(), tc.want)
		}
	}
}

func TestRunBackups_DefaultsToTheBackupsDirBesideTheProjectRoot(t *testing.T) {
	hub := t.TempDir()
	root := filepath.Join(hub, "console")
	if err := os.MkdirAll(filepath.Join(hub, "backups"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	var out, errb bytes.Buffer

	code := runBackups([]string{"verify"}, nil, &out, &errb)

	if code != 0 || !strings.Contains(out.String(), "every head and patch exists elsewhere") {
		t.Errorf("code=%d stdout=%q stderr=%q, want the empty hub backups dir verified", code, out.String(), errb.String())
	}
}

func TestRunBackups_TheLastDirFlagWins(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", filepath.Join(t.TempDir(), "console"))
	missing := filepath.Join(t.TempDir(), "first")
	last := t.TempDir()
	var out, errb bytes.Buffer

	code := runBackups([]string{"verify", "--dir", missing, "--dir", last}, nil, &out, &errb)

	if code != 0 || errb.Len() != 0 {
		t.Errorf("code=%d stderr=%q, want the last --dir verified", code, errb.String())
	}
	errb.Reset()
	if code := runBackups([]string{"verify", "--dir", last, "--dir", missing}, nil, &out, &errb); code != 2 || !strings.Contains(errb.String(), missing) {
		t.Errorf("code=%d stderr=%q, want the absent last --dir refused", code, errb.String())
	}
}
