package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gcpolicy"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

func e3StatusGit(out string, code int, err error) gitexec.Git {
	return gitexec.Git{Dir: "/wt", Exec: func(_ context.Context, _, _ string, _, _ []string, _ io.Reader, stdout, stderr io.Writer) (int, error) {
		_, _ = io.WriteString(stdout, out)
		if code != 0 {
			_, _ = io.WriteString(stderr, "fatal: not a git repository\n")
		}
		return code, err
	}}
}

func TestChangedPaths(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		out     string
		code    int
		err     error
		want    []string
		wantErr string
	}{
		{"clean", "", 0, nil, nil, ""},
		{"modified and untracked", " M a.go\x00?? ünïcode file.txt\x00", 0, nil, []string{"a.go", "ünïcode file.txt"}, ""},
		{"rename carries its origin", "R  new.go\x00old.go\x00 M b.go\x00", 0, nil, []string{"new.go", "old.go", "b.go"}, ""},
		{"copy carries its origin", "C  copy.go\x00src.go\x00", 0, nil, []string{"copy.go", "src.go"}, ""},
		{"rename as the last entry without an origin", "R  new.go\x00", 0, nil, []string{"new.go"}, ""},
		{"minimal entry", "?? x\x00", 0, nil, []string{"x"}, ""},
		{"short entry", "?? \x00", 0, nil, nil, `git status -z: unexpected entry "?? "`},
		{"git refused", "", 128, nil, nil, "git status -z rc=128 err=<nil>: fatal: not a git repository"},
		{"runner error", "", -1, errors.New("exec: git not found"), nil, "git status -z rc=-1 err=exec: git not found"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := changedPaths(context.Background(), e3StatusGit(tc.out, tc.code, tc.err))
			if tc.wantErr != "" {
				if err == nil || !strings.HasPrefix(err.Error(), tc.wantErr) || got != nil {
					t.Fatalf("got %q err %v, want error %q", got, err, tc.wantErr)
				}
				return
			}
			if err != nil || !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %q err %v, want %q", got, err, tc.want)
			}
		})
	}
}

func FuzzChangedPaths(f *testing.F) {
	for _, seed := range []string{"", " M a\x00", "R  b\x00a\x00", "?? \x00", "XY\x00", "C  c"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, out string) {
		paths, err := changedPaths(context.Background(), e3StatusGit(out, 0, nil))
		if err != nil {
			return
		}
		for _, p := range paths {
			if strings.Contains(p, "\x00") {
				t.Fatalf("path %q carries a NUL from %q", p, out)
			}
		}
		if out == "" && paths != nil {
			t.Fatalf("clean status gave %q", paths)
		}
	})
}

func TestDevQuietPeriod(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, policy string
		want         time.Duration
		wantWarn     bool
	}{
		{"no policy", "", gcpolicy.WorktreesPolicy{}.DevQuietPeriod(), false},
		{"malformed policy warns and uses the default", "{", gcpolicy.WorktreesPolicy{}.DevQuietPeriod(), true},
		{"declared minutes", `{"gc":{"worktrees":{"dev_quiet_minutes":95}}}`, 95 * time.Minute, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if tc.policy != "" {
				writeFileForPublishTest(t, filepath.Join(root, ".evolve", "policy.json"), tc.policy)
			}
			var errb bytes.Buffer

			got := devQuietPeriod(root, &errb)

			if got != tc.want {
				t.Errorf("quiet = %v, want %v", got, tc.want)
			}
			if warned := strings.Contains(errb.String(), "evolve worktree cleanup: WARN: policy load failed: "); warned != tc.wantWarn {
				t.Errorf("warned = %v (stderr %q), want %v", warned, errb.String(), tc.wantWarn)
			}
		})
	}
}

func TestDevFail_KeepsTheErrorTextVerbatim(t *testing.T) {
	t.Parallel()
	o := devFail(errors.New("git: rc=128 ünïcode"))
	if o != (devOutcome{code: devFailed, detail: "git: rc=128 ünïcode"}) {
		t.Errorf("devFail = %+v", o)
	}
}

func TestDevCleanerReport_RoutesEachOutcome(t *testing.T) {
	t.Parallel()
	c := devCleaner{effects: devEffectsFor(false)}
	var out, errb bytes.Buffer

	c.report(&out, &errb, "dev/a", devOutcome{code: devKept, detail: "dirty"})
	c.report(&out, &errb, "dev/b", devOutcome{code: devFailed, detail: "rc=128"})
	c.report(&out, &errb, "dev/c", devOutcome{code: devRemovable, branch: "lane/c", detail: "head is in origin/main"})

	if out.String() != "kept dev/a: dirty\nremoved dev/c and branch lane/c (head is in origin/main)\n" || errb.String() != "failed dev/b: rc=128\n" {
		t.Errorf("stdout %q stderr %q", out.String(), errb.String())
	}
}
