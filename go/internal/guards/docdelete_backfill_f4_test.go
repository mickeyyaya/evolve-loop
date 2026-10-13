package guards

import (
	"context"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestOptionValue_PrefersAttachedThenNextWordThenNothing(t *testing.T) {
	cases := []struct {
		name      string
		attached  string
		args      []string
		wantValue string
		wantWidth int
	}{
		{"attached value", "dst", []string{"-tdst", "a"}, "dst", 1},
		{"separate value", "", []string{"-t", "dst", "a"}, "dst", 2},
		{"option is the last word", "", []string{"-t"}, "", 1},
		{"no words at all", "", nil, "", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			value, width := optionValue(tc.attached, tc.args)
			if value != tc.wantValue || width != tc.wantWidth {
				t.Fatalf("optionValue = (%q,%d), want (%q,%d)", value, width, tc.wantValue, tc.wantWidth)
			}
		})
	}
}

func TestMoveLongOption_RecognisesAbbreviatedTargetAndSuffixOptions(t *testing.T) {
	cases := []struct {
		name       string
		args       []string
		wantTarget string
		wantWidth  int
	}{
		{"full target with equals", []string{"--target-directory=dst", "a"}, "dst", 1},
		{"abbreviated target with equals", []string{"--target=dst", "a"}, "dst", 1},
		{"single letter abbreviation of target", []string{"--t=dst"}, "dst", 1},
		{"target value in the next word", []string{"--target-directory", "dst", "a"}, "dst", 2},
		{"target without any value", []string{"--target-directory"}, "", 1},
		{"unrelated option with a value", []string{"--backup=numbered", "a"}, "", 1},
		{"suffix consumes the next word", []string{"--suffix", "~", "a", "b"}, "", 2},
		{"suffix with equals consumes one word", []string{"--suffix=~", "a", "b"}, "", 1},
		{"suffix as the last word", []string{"--suffix"}, "", 1},
		{"one-letter prefix of suffix is too short", []string{"--s", "x"}, "", 1},
		{"two-letter prefix of suffix counts", []string{"--su", "~"}, "", 2},
		{"unknown flag", []string{"--verbose", "a"}, "", 1},
		{"empty name", []string{"--=x"}, "", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			target, width := moveLongOption(tc.args)
			if target != tc.wantTarget || width != tc.wantWidth {
				t.Fatalf("moveLongOption(%v) = (%q,%d), want (%q,%d)", tc.args, target, width, tc.wantTarget, tc.wantWidth)
			}
		})
	}
}

func TestMoveOperands_SourcesAndDestinationAcrossOptionSpellings(t *testing.T) {
	cases := []struct {
		name     string
		cmd      string
		words    []string
		wantSrcs []string
		wantDst  string
		wantOK   bool
	}{
		{"plain mv", "mv", []string{"a", "b"}, []string{"a"}, "b", true},
		{"several sources", "mv", []string{"a", "b", "dir"}, []string{"a", "b"}, "dir", true},
		{"git mv", "git", []string{"mv", "a", "b"}, []string{"a"}, "b", true},
		{"git with global option before mv", "git", []string{"-C", "repo", "mv", "a", "b"}, []string{"a"}, "b", true},
		{"target option makes every operand a source", "mv", []string{"-t", "dst", "a", "b"}, []string{"a", "b"}, "dst", true},
		{"attached short target", "mv", []string{"-tdst", "a"}, []string{"a"}, "dst", true},
		{"long target", "mv", []string{"--target-directory=dst", "a"}, []string{"a"}, "dst", true},
		{"target option without operands is not a move", "mv", []string{"-t", "dst"}, nil, "dst", false},
		{"suffix option is skipped with its value", "mv", []string{"-S", "~", "a", "b"}, []string{"a"}, "b", true},
		{"double dash ends options", "mv", []string{"--", "-a", "-b"}, []string{"-a"}, "-b", true},
		{"a lone dash is an operand", "mv", []string{"-", "b"}, []string{"-"}, "b", true},
		{"a single operand is not a move", "mv", []string{"only"}, nil, "", false},
		{"no operands", "mv", nil, nil, "", false},
		{"not a move program", "rm", []string{"a", "b"}, nil, "", false},
		{"git subcommand other than mv", "git", []string{"status", "a", "b"}, nil, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srcs, dst, ok := moveOperands(tc.cmd, tc.words)
			if ok != tc.wantOK || dst != tc.wantDst || !reflect.DeepEqual(srcs, tc.wantSrcs) {
				t.Fatalf("moveOperands(%s %v) = (%v,%q,%v), want (%v,%q,%v)", tc.cmd, tc.words, srcs, dst, ok, tc.wantSrcs, tc.wantDst, tc.wantOK)
			}
		})
	}
}

func TestGitSubcommand_SkipsGlobalOptionsAndTheirValues(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		want    string
		wantIdx int
	}{
		{"no arguments", nil, "", -1},
		{"bare subcommand", []string{"status"}, "status", 0},
		{"option with value then subcommand", []string{"-C", "dir", "log"}, "log", 2},
		{"option value is not mistaken for the subcommand", []string{"-c", "core.x=1", "mv", "a", "b"}, "mv", 2},
		{"flag without value then subcommand", []string{"--no-pager", "diff"}, "diff", 1},
		{"only options with values", []string{"-C", "dir"}, "", -1},
		{"value option as the final word", []string{"--git-dir"}, "", -1},
		{"only plain flags", []string{"--no-pager", "-p"}, "", -1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := gitSubcommand(tc.args); got != tc.want {
				t.Errorf("gitSubcommand(%v) = %q, want %q", tc.args, got, tc.want)
			}
			if got := gitSubcommandIndex(tc.args); got != tc.wantIdx {
				t.Errorf("gitSubcommandIndex(%v) = %d, want %d", tc.args, got, tc.wantIdx)
			}
		})
	}
}

func TestNeverCommitted_RejectsEveryRepositoryShapeThatCouldHideACommittedDraft(t *testing.T) {
	ctx := context.Background()
	draft := "docs/explain/builds/cycle-2-draft.md"

	t.Run("empty cwd", func(t *testing.T) {
		if neverCommitted(ctx, "", draft) {
			t.Fatal("an unknown cwd proves nothing")
		}
	})
	t.Run("cwd is not a repository", func(t *testing.T) {
		if neverCommitted(ctx, t.TempDir(), draft) {
			t.Fatal("outside a repository nothing is proven")
		}
	})
	t.Run("a repository with no commit has no HEAD tree to compare", func(t *testing.T) {
		dir := gittest.Fixture(t).Dir
		if neverCommitted(ctx, dir, draft) {
			t.Fatal("an unreadable HEAD must not be treated as an empty one")
		}
	})
	t.Run("cwd is a subdirectory of the repository", func(t *testing.T) {
		dir := gitRepo(t, map[string]string{"sub/keep.txt": "x"})
		if neverCommitted(ctx, filepath.Join(dir, "sub"), draft) {
			t.Fatal("only the repository root proves the draft path")
		}
	})
	t.Run("clean repository with the parents present", func(t *testing.T) {
		dir := gitRepo(t, map[string]string{"README.md": "x"})
		if err := os.MkdirAll(filepath.Join(dir, "docs", "explain", "builds"), 0o755); err != nil {
			t.Fatal(err)
		}
		if !neverCommitted(ctx, dir, draft) {
			t.Fatal("an uncommitted draft below real directories is provable")
		}
	})
	t.Run("a committed file of the same name folded to another case", func(t *testing.T) {
		dir := gitRepo(t, map[string]string{"DOCS/EXPLAIN/BUILDS/CYCLE-2-DRAFT.md": "x"})
		if neverCommitted(ctx, dir, draft) {
			t.Fatal("a case-folded match must count as committed")
		}
	})
	t.Run("a committed directory by the draft's name", func(t *testing.T) {
		dir := gitRepo(t, map[string]string{draft + "/inner.txt": "x"})
		if neverCommitted(ctx, dir, draft) {
			t.Fatal("a committed path below the draft name must count as committed")
		}
	})
	t.Run("the draft path is a symlink", func(t *testing.T) {
		dir := gitRepo(t, map[string]string{"README.md": "x"})
		if err := os.MkdirAll(filepath.Join(dir, "docs", "explain", "builds"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("/etc/hosts", filepath.Join(dir, filepath.FromSlash(draft))); err != nil {
			t.Fatal(err)
		}
		if neverCommitted(ctx, dir, draft) {
			t.Fatal("a symlinked draft is not a regular file")
		}
	})
	t.Run("a parent directory is a symlink", func(t *testing.T) {
		dir := gitRepo(t, map[string]string{"README.md": "x"})
		if err := os.MkdirAll(filepath.Join(dir, "docs", "elsewhere"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(dir, "docs", "elsewhere"), filepath.Join(dir, "docs", "explain")); err != nil {
			t.Fatal(err)
		}
		if neverCommitted(ctx, dir, draft) {
			t.Fatal("a symlinked parent resolves the name somewhere else")
		}
	})
	t.Run("a parent directory is missing", func(t *testing.T) {
		dir := gitRepo(t, map[string]string{"README.md": "x"})
		if neverCommitted(ctx, dir, draft) {
			t.Fatal("a missing parent is not a real directory chain")
		}
	})
}
