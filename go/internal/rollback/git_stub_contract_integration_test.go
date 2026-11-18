//go:build integration

package rollback

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const contractTag = "v9.9.9"

var lsRemoteLine = regexp.MustCompile(`^[0-9a-f]{7,64}\trefs/tags/` + regexp.QuoteMeta(contractTag) + `\n$`)

func lsRemoteShapeErr(out string) error {
	if !lsRemoteLine.MatchString(out) {
		return fmt.Errorf("ls-remote output %q is not <sha> TAB refs/tags/%s", out, contractTag)
	}
	return nil
}

type gitBackend struct {
	name  string
	setup func(t *testing.T) gitRunner
}

type gitRunner struct {
	dir      string
	shaToRev string
	run      func(args ...string) (string, int)
}

func runIn(dir, bin string, args ...string) (string, int) {
	cmd := sysexec.Command(context.Background(), bin, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		code = 1
		if cmd.ProcessState != nil {
			code = cmd.ProcessState.ExitCode()
		}
	}
	return string(out), code
}

func writeCommit(t *testing.T, r *gittest.Repo, content string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(r.Dir, "f.txt"), []byte(content+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r.Git("add", "f.txt")
	r.Git("commit", "-q", "-m", "set "+content)
	return r.Git("rev-parse", "HEAD")
}

func realBackend(tagPublished bool) func(t *testing.T) gitRunner {
	return func(t *testing.T) gitRunner {
		origin := gittest.Bare(t)
		work := gittest.Fixture(t)
		writeCommit(t, work, "a")
		middle := writeCommit(t, work, "b")
		writeCommit(t, work, "c")
		work.Git("remote", "add", "origin", origin.Dir)
		if tagPublished {
			work.Git("tag", contractTag)
			work.Git("push", "-q", "origin", "refs/tags/"+contractTag)
		}
		return gitRunner{dir: work.Dir, shaToRev: middle, run: func(args ...string) (string, int) {
			return runIn(work.Dir, "git", args...)
		}}
	}
}

const stubScript = `#!/bin/sh
case "$1" in
  ls-remote)
    if [ -n "$STUB_TAG_PUBLISHED" ]; then printf '0123456789abcdef0123456789abcdef01234567\trefs/tags/v9.9.9\n'; fi
    exit 0 ;;
  revert)
    echo 'CONFLICT (content): Merge conflict in f.txt'
    echo 'error: could not revert 1234567... set b' >&2
    exit 1 ;;
esac
exit 0
`

func stubBackend(tagPublished bool) func(t *testing.T) gitRunner {
	return func(t *testing.T) gitRunner {
		dir := t.TempDir()
		script := stubScript
		if tagPublished {
			script = strings.Replace(script, `[ -n "$STUB_TAG_PUBLISHED" ]`, `true`, 1)
		}
		fakeclitest.Install(t, filepath.Join(dir, "git"), script)
		t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
		return gitRunner{dir: t.TempDir(), shaToRev: "deadbeef", run: func(args ...string) (string, int) {
			return runIn(dir, filepath.Join(dir, "git"), args...)
		}}
	}
}

type contractCase struct {
	name      string
	published bool
	args      func(g gitRunner) []string
	check     func(t *testing.T, out string, code int)
}

var contractCases = []contractCase{
	{
		name:      "ls_remote_prints_sha_tab_ref",
		published: true,
		args:      func(gitRunner) []string { return []string{"ls-remote", "--tags", "origin", "refs/tags/" + contractTag} },
		check: func(t *testing.T, out string, code int) {
			if code != 0 {
				t.Fatalf("exit = %d, want 0; out=%q", code, out)
			}
			if err := lsRemoteShapeErr(out); err != nil {
				t.Error(err)
			}
		},
	},
	{
		name:      "ls_remote_prints_nothing_for_an_absent_tag",
		published: false,
		args:      func(gitRunner) []string { return []string{"ls-remote", "--tags", "origin", "refs/tags/" + contractTag} },
		check: func(t *testing.T, out string, code int) {
			if code != 0 || strings.TrimSpace(out) != "" {
				t.Errorf("exit = %d out = %q, want exit 0 and empty output", code, out)
			}
		},
	},
	{
		name: "revert_conflict_exits_1_with_CONFLICT",
		args: func(g gitRunner) []string { return []string{"revert", "--no-edit", g.shaToRev} },
		check: func(t *testing.T, out string, code int) {
			if code != 1 || !strings.Contains(out, "CONFLICT") {
				t.Errorf("exit = %d out = %q, want exit 1 with CONFLICT", code, out)
			}
		},
	},
}

func TestGitStubContract(t *testing.T) {
	for _, c := range contractCases {
		t.Run(c.name, func(t *testing.T) {
			for _, b := range []struct {
				name  string
				setup func(bool) func(*testing.T) gitRunner
			}{{"stub", stubBackend}, {"real", realBackend}} {
				t.Run(b.name, func(t *testing.T) {
					g := b.setup(c.published)(t)
					out, code := g.run(c.args(g)...)
					c.check(t, out, code)
				})
			}
		})
	}
}

func TestGitStubContract_LegacyOutputFailsTheShapeCase(t *testing.T) {
	for _, legacy := range []string{"refs/tags/" + contractTag + "\n", contractTag + "\n", ""} {
		if lsRemoteShapeErr(legacy) == nil {
			t.Errorf("legacy output %q passed the shape check", legacy)
		}
	}
	if err := lsRemoteShapeErr("abc1234\trefs/tags/" + contractTag + "\n"); err != nil {
		t.Errorf("real shape rejected: %v", err)
	}
}

func TestGitStubContract_RevertConflictReachesFailedPath(t *testing.T) {
	for _, b := range []struct {
		name  string
		setup func(bool) func(*testing.T) gitRunner
	}{{"stub", stubBackend}, {"real", realBackend}} {
		t.Run(b.name, func(t *testing.T) {
			g := b.setup(false)(t)
			got := revertAndShipWith(gitexec.Default(g.dir), g.dir, g.shaToRev, "conflict", "9.9.9")
			if got != stepFailed {
				t.Errorf("revertAndShipWith = %q, want %q on a revert conflict", got, stepFailed)
			}
		})
	}
}
