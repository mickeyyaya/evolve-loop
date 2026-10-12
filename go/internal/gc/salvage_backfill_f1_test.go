package gc

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

type gitReplyF1 struct {
	out  string
	code int
	err  error
}

func gitScriptF1(replies map[string]gitReplyF1) sysexec.RunFunc {
	return func(_ context.Context, _, _ string, args, _ []string, _ io.Reader, stdout, _ io.Writer) (int, error) {
		r, ok := replies[args[0]]
		if !ok {
			return 0, nil
		}
		_, _ = io.WriteString(stdout, r.out)
		return r.code, r.err
	}
}

const salvageShaF1 = "0123456789abcdef0123456789abcdef01234567"

func salvageOptsF1(t *testing.T, replies map[string]gitReplyF1) (WorktreeOptions, string) {
	t.Helper()
	evolve := t.TempDir()
	return WorktreeOptions{ProjectRoot: t.TempDir(), EvolveDir: evolve, WorktreeBase: t.TempDir(), Exec: gitScriptF1(replies)}, evolve
}

func TestSalvageWorktreeBackfillF1_EachStepFailureAbortsWithItsOwnReason(t *testing.T) {
	boom := errors.New("git exploded")
	cases := []struct {
		name     string
		replies  map[string]gitReplyF1
		preBlock string
		want     string
	}{
		{"head cannot be read", map[string]gitReplyF1{"rev-parse": {err: boom}}, "", "git rev-parse HEAD"},
		{"head file cannot be written", map[string]gitReplyF1{"rev-parse": {out: salvageShaF1 + "\n"}}, salvageHeadFile, "write salvage HEAD"},
		{"diff fails", map[string]gitReplyF1{"rev-parse": {out: salvageShaF1}, "diff": {err: boom}}, "", "git diff HEAD --binary"},
		{"patch file cannot be written", map[string]gitReplyF1{"rev-parse": {out: salvageShaF1}, "diff": {out: "diff --git a/x b/x\n"}}, salvagePatchFile, "write salvage patch"},
		{"untracked listing fails", map[string]gitReplyF1{"rev-parse": {out: salvageShaF1}, "ls-files": {err: boom}}, "", "git ls-files --others"},
		{"an untracked name that is not on disk", map[string]gitReplyF1{"rev-parse": {out: salvageShaF1}, "ls-files": {out: "ghost.txt\x00"}}, "", "archive ghost.txt"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o, evolve := salvageOptsF1(t, tc.replies)
			wt := t.TempDir()
			leafDir := filepath.Join(OperatorSalvageDir(evolve), filepath.Base(wt))
			if tc.preBlock != "" {
				if err := os.MkdirAll(filepath.Join(leafDir, tc.preBlock), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			err := o.salvageWorktree(wt, "cycle-b-9")
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}

func TestSalvageWorktreeBackfillF1_ACleanTreeLeavesOnlyItsHeadRecord(t *testing.T) {
	o, evolve := salvageOptsF1(t, map[string]gitReplyF1{"rev-parse": {out: " " + salvageShaF1 + " \n"}})
	wt := t.TempDir()
	if err := o.salvageWorktree(wt, "cycle-b-9"); err != nil {
		t.Fatal(err)
	}
	leaf := filepath.Join(OperatorSalvageDir(evolve), filepath.Base(wt))
	head, err := os.ReadFile(filepath.Join(leaf, salvageHeadFile))
	if err != nil || string(head) != salvageShaF1+" cycle-b-9\n" {
		t.Fatalf("HEAD record = %q, %v; want the trimmed sha and the branch", head, err)
	}
	entries, _ := os.ReadDir(leaf)
	if len(entries) != 1 {
		t.Fatalf("a clean tree has no patch and no archive, got %d entries", len(entries))
	}
}

func TestSalvageRemoveWorktreesBackfillF1(t *testing.T) {
	t.Run("a removal git refuses is reported and the tree does not count as removed", func(t *testing.T) {
		boom := errors.New("worktree locked")
		o, _ := salvageOptsF1(t, map[string]gitReplyF1{"rev-parse": {out: salvageShaF1}, "worktree": {err: boom}})
		wt := t.TempDir()
		did, errs := o.salvageRemoveWorktrees([]WorktreeItem{{Path: wt, Branch: "b", Action: WorktreeActionSalvageRemove}}, nil)
		if did || len(errs) != 1 || !errors.Is(errs[0], boom) {
			t.Fatalf("did=%v errs=%v", did, errs)
		}
	})
	t.Run("refused and non-salvage items are never touched", func(t *testing.T) {
		calls := 0
		o, _ := salvageOptsF1(t, nil)
		o.Exec = func(context.Context, string, string, []string, []string, io.Reader, io.Writer, io.Writer) (int, error) {
			calls++
			return 0, nil
		}
		refusedPath := t.TempDir()
		did, errs := o.salvageRemoveWorktrees([]WorktreeItem{
			{Path: refusedPath, Action: WorktreeActionSalvageRemove},
			{Path: t.TempDir(), Action: WorktreeActionRemove},
			{Path: t.TempDir(), Action: WorktreeActionFlagDirty},
		}, map[string]bool{refusedPath: true})
		if did || len(errs) != 0 || calls != 0 {
			t.Fatalf("did=%v errs=%v git calls=%d", did, errs, calls)
		}
	})
}

func TestArchiveUntrackedBackfillF1(t *testing.T) {
	t.Run("nothing untracked writes no archive", func(t *testing.T) {
		dest := filepath.Join(t.TempDir(), "untracked.tgz")
		for _, list := range []string{"", "\x00", "\x00\x00"} {
			if err := archiveUntracked(t.TempDir(), list, dest); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := os.Stat(dest); !os.IsNotExist(err) {
			t.Fatalf("an empty list must not create %s: %v", dest, err)
		}
	})
	t.Run("the destination cannot be created", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "a.txt"), "a")
		err := archiveUntracked(root, "a.txt\x00", filepath.Join(root, "no", "such", "dir", "untracked.tgz"))
		if err == nil || !strings.Contains(err.Error(), "create untracked archive") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("a file that cannot be read fails the whole archive naming it", func(t *testing.T) {
		requireNonRootF1(t)
		root := t.TempDir()
		secret := filepath.Join(root, "secret.txt")
		writeFile(t, secret, "s")
		if err := os.Chmod(secret, 0); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(secret, 0o644) })
		err := archiveUntracked(root, "secret.txt\x00", filepath.Join(t.TempDir(), "untracked.tgz"))
		if err == nil || !strings.Contains(err.Error(), "archive secret.txt") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("files and a symlink round-trip into the archive", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "dir", "a.txt"), "a")
		writeFile(t, filepath.Join(root, "b.txt"), "b")
		if err := os.Symlink("b.txt", filepath.Join(root, "link")); err != nil {
			t.Fatal(err)
		}
		dest := filepath.Join(t.TempDir(), "untracked.tgz")
		if err := archiveUntracked(root, "dir/a.txt\x00b.txt\x00link\x00", dest); err != nil {
			t.Fatal(err)
		}
		if n, err := countUntrackedArchive(dest); err != nil || n != 3 {
			t.Fatalf("archive holds %d entries (%v), want 3", n, err)
		}
		if got := untarLink(t, dest, "link"); got != "b.txt" {
			t.Fatalf("symlink target = %q", got)
		}
		if got := untarFile(t, dest, "dir/a.txt"); got != "a" {
			t.Fatalf("file content = %q", got)
		}
	})
}

func TestAddTarEntryBackfillF1(t *testing.T) {
	newWriter := func() (*tar.Writer, *bytes.Buffer) {
		var buf bytes.Buffer
		return tar.NewWriter(&buf), &buf
	}
	t.Run("a missing file", func(t *testing.T) {
		tw, _ := newWriter()
		if err := addTarEntry(tw, t.TempDir(), "absent"); !os.IsNotExist(err) {
			t.Fatalf("err = %v, want not-exist", err)
		}
	})
	t.Run("a symlink whose target cannot be read", func(t *testing.T) {
		root := t.TempDir()
		if err := os.Symlink("x", filepath.Join(root, "l")); err != nil {
			t.Fatal(err)
		}
		boom := errors.New("readlink boom")
		orig := salvageReadlink
		t.Cleanup(func() { salvageReadlink = orig })
		salvageReadlink = func(string) (string, error) { return "", boom }
		tw, _ := newWriter()
		if err := addTarEntry(tw, root, "l"); !errors.Is(err, boom) {
			t.Fatalf("err = %v, want the readlink failure", err)
		}
	})
	t.Run("a socket cannot be archived", func(t *testing.T) {
		short, err := os.MkdirTemp("", "gcsock")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.RemoveAll(short) })
		ln, err := net.Listen("unix", filepath.Join(short, "s"))
		if err != nil {
			t.Skipf("unix sockets unavailable: %v", err)
		}
		t.Cleanup(func() { _ = ln.Close() })
		tw, _ := newWriter()
		if err := addTarEntry(tw, short, "s"); err == nil || !strings.Contains(err.Error(), "sockets not supported") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("a writer that was already closed refuses the header", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "a"), "a")
		tw, _ := newWriter()
		if err := tw.Close(); err != nil {
			t.Fatal(err)
		}
		if err := addTarEntry(tw, root, "a"); !errors.Is(err, tar.ErrWriteAfterClose) {
			t.Fatalf("err = %v, want ErrWriteAfterClose", err)
		}
	})
	t.Run("a directory entry carries only its header", func(t *testing.T) {
		root := t.TempDir()
		if err := os.Mkdir(filepath.Join(root, "d"), 0o755); err != nil {
			t.Fatal(err)
		}
		tw, buf := newWriter()
		if err := addTarEntry(tw, root, "d"); err != nil {
			t.Fatal(err)
		}
		_ = tw.Close()
		hdr, err := tar.NewReader(buf).Next()
		if err != nil || hdr.Name != "d" || hdr.Typeflag != tar.TypeDir || hdr.Size != 0 {
			t.Fatalf("header = %+v, %v", hdr, err)
		}
	})
}
