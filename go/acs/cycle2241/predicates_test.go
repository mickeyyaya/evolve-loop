//go:build acs

package cycle2241

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const optionLookingRef = "--output="

type recordingGit struct {
	argvLog, marker string
}

func installRecordingGit(t *testing.T) recordingGit {
	t.Helper()
	bin := t.TempDir()
	rec := recordingGit{argvLog: filepath.Join(t.TempDir(), "argv.log"), marker: filepath.Join(t.TempDir(), "spawned")}
	script := "#!/bin/sh\n: > '" + rec.marker + "'\nfor a in \"$@\"; do printf '%s\\n' \"$a\" >> '" + rec.argvLog + "'; done\nexit 0\n"
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return rec
}

func (r recordingGit) spawned() bool {
	_, err := os.Stat(r.marker)
	return err == nil
}

func (r recordingGit) argv(t *testing.T) []string {
	t.Helper()
	raw, err := os.ReadFile(r.argvLog)
	if err != nil {
		t.Fatalf("the recording git never ran: %v", err)
	}
	return strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
}

func repoWithOneCommit(t *testing.T) (dir, head string) {
	t.Helper()
	repo := gittest.Fixture(t)
	repo.Git("commit", "--allow-empty", "-m", "seed")
	return repo.Dir, repo.Git("rev-parse", "HEAD")
}

func TestC2241_001_ARefThatLooksLikeAnOptionReachesRealGitAsARef(t *testing.T) {
	dir, _ := repoWithOneCommit(t)
	victim := filepath.Join(t.TempDir(), "x")
	stdout, stderr, code, err := acsassert.GitRefOutput(dir, optionLookingRef+victim, "log")
	if _, statErr := os.Stat(victim); statErr == nil {
		t.Fatalf("git read the ref as --output and wrote %s", victim)
	}
	if code == 0 || err == nil {
		t.Errorf("git accepted %q as a revision: code=%d err=%v stdout=%q", optionLookingRef+victim, code, err, stdout)
	}
	if !strings.Contains(stderr, optionLookingRef+victim) {
		t.Errorf("git did not report the option-looking text as a bad revision: stderr=%q", stderr)
	}
}

func TestC2241_002_TheRefIsTheLastArgumentAndFollowsEndOfOptions(t *testing.T) {
	rec := installRecordingGit(t)
	ref := optionLookingRef + "/tmp/x"
	if _, _, code, err := acsassert.GitRefOutput(t.TempDir(), ref, "log", "--oneline"); code != 0 || err != nil {
		t.Fatalf("recording git run failed: code=%d err=%v", code, err)
	}
	argv := rec.argv(t)
	n := len(argv)
	if n < 2 || argv[n-2] != "--end-of-options" || argv[n-1] != ref {
		t.Errorf("argv = %q, want the ref preceded by --end-of-options as the final two arguments", argv)
	}
	if !strings.Contains(strings.Join(argv, " "), "log --oneline") {
		t.Errorf("argv = %q lost the caller's own arguments", argv)
	}
}

func TestC2241_003_ARefThatIsAValidRevisionStillResolves(t *testing.T) {
	dir, head := repoWithOneCommit(t)
	stdout, _, code, err := acsassert.GitRefOutput(dir, "HEAD", "rev-parse", "--verify")
	if err != nil || code != 0 || strings.TrimSpace(stdout) != head {
		t.Errorf("GitRefOutput(HEAD) = %q code=%d err=%v, want %q", stdout, code, err, head)
	}
}

func TestC2241_004_AShaWithAShellSeparatorIsRefusedBeforeGitRuns(t *testing.T) {
	rec := installRecordingGit(t)
	stdout, stderr, code, err := acsassert.GitSHAOutput(t.TempDir(), "HEAD;rm", "show")
	if !errors.Is(err, acsassert.ErrInvalidSHA) {
		t.Errorf("err = %v, want ErrInvalidSHA", err)
	}
	if rec.spawned() {
		t.Error("git was spawned for the refused SHA")
	}
	if stdout != "" || stderr != "" || code != -1 {
		t.Errorf("refused call returned stdout=%q stderr=%q code=%d, want empty output and code -1", stdout, stderr, code)
	}
}

func TestC2241_005_AValidShaRunsGitWithTheShaAfterEndOfOptions(t *testing.T) {
	rec := installRecordingGit(t)
	sha := "0123456789abcdef0123456789abcdef01234567"
	if _, _, code, err := acsassert.GitSHAOutput(t.TempDir(), sha, "show"); code != 0 || err != nil {
		t.Fatalf("code=%d err=%v", code, err)
	}
	argv := rec.argv(t)
	n := len(argv)
	if n < 2 || argv[n-2] != "--end-of-options" || argv[n-1] != sha {
		t.Errorf("argv = %q, want the SHA preceded by --end-of-options as the final two arguments", argv)
	}
}

func TestC2241_006_AValidShaResolvesAgainstRealGit(t *testing.T) {
	dir, head := repoWithOneCommit(t)
	stdout, _, code, err := acsassert.GitSHAOutput(dir, head, "rev-parse", "--verify")
	if err != nil || code != 0 || strings.TrimSpace(stdout) != head {
		t.Errorf("GitSHAOutput(%s) = %q code=%d err=%v", head, stdout, code, err)
	}
}

func TestC2241_007_ValidateSHAAcceptsOnlySevenToSixtyFourHexCharacters(t *testing.T) {
	cases := []struct {
		name, sha string
		valid     bool
	}{
		{"seven lower hex", "abcdef1", true},
		{"forty hex", strings.Repeat("a1", 20), true},
		{"sixty-four hex", strings.Repeat("f", 64), true},
		{"upper hex", "ABCDEF1234", true},
		{"empty", "", false},
		{"six hex", "abcdef", false},
		{"sixty-five hex", strings.Repeat("a", 65), false},
		{"shell separator", "HEAD;rm", false},
		{"non-hex letter", "abcdef1g", false},
		{"option looking", "--output=/tmp/x", false},
		{"trailing newline", "abcdef1\n", false},
		{"leading space", " abcdef1", false},
		{"embedded newline", "abcdef1\nabcdef1", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := acsassert.ValidateSHA(tc.sha)
			if tc.valid && err != nil {
				t.Errorf("ValidateSHA(%q) = %v, want nil", tc.sha, err)
			}
			if !tc.valid && !errors.Is(err, acsassert.ErrInvalidSHA) {
				t.Errorf("ValidateSHA(%q) = %v, want ErrInvalidSHA", tc.sha, err)
			}
		})
	}
}

func TestC2241_008_EveryNewFunctionIsFullyCovered(t *testing.T) {
	root := acsassert.RepoRoot(t)
	profile := filepath.Join(t.TempDir(), "cover.out")
	run := func(args ...string) string {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, "go", args...)
		cmd.Dir = filepath.Join(root, "go")
		cmd.WaitDelay = 5 * time.Second
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("go %s: %v\n%s", strings.Join(args, " "), err, acsassert.OutputTail(string(out), 30))
		}
		return string(out)
	}
	run("test", "-count=1", "-coverprofile="+profile, "./pkg/acsassert")
	out := run("tool", "cover", "-func="+profile)
	for _, fn := range []string{"ValidateSHA", "GitRefOutput", "GitSHAOutput"} {
		if pct, err := acsassert.FuncCoverage(out, fn); err != nil || pct != 100 {
			t.Errorf("%s coverage = %.1f%%, err = %v, want 100%%", fn, pct, err)
		}
	}
}

func TestC2241_009_AcsassertSourceCarriesNoComments(t *testing.T) {
	pkgDir := filepath.Join(acsassert.RepoRoot(t), "go", "pkg", "acsassert")
	paths, err := filepath.Glob(filepath.Join(pkgDir, "*.go"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no sources under %s: %v", pkgDir, err)
	}
	fset := token.NewFileSet()
	for _, path := range paths {
		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, group := range file.Comments {
			if group == file.Doc {
				continue
			}
			if isDirectiveGroup(group) {
				continue
			}
			t.Errorf("%s:%d carries a comment: %q", filepath.Base(path), fset.Position(group.Pos()).Line, group.Text())
		}
	}
}

func isDirectiveGroup(group *ast.CommentGroup) bool {
	for _, c := range group.List {
		if !strings.HasPrefix(c.Text, "//go:") {
			return false
		}
	}
	return true
}
