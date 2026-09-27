package treedelta

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func run(ctx context.Context, dir string, args ...string) (string, int, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return string(out), exit.ExitCode(), nil
	}
	return string(out), 0, err
}

func repo(t *testing.T) (dir string, git func(args ...string) string) {
	t.Helper()
	dir = t.TempDir()
	git = func(args ...string) string {
		out, code, err := run(context.Background(), dir, args...)
		if err != nil || code != 0 {
			t.Fatalf("git %v: exit %d %v\n%s", args, code, err, out)
		}
		return strings.TrimSpace(out)
	}
	git("init", "-q", "-b", "main")
	git("-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "root")
	return dir, git
}

func write(t *testing.T, dir, rel, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, rel), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestIdentical_HoldsForTheSameBytesOnTwoBasesAndDeclinesOneByte(t *testing.T) {
	dir, git := repo(t)
	base0 := git("rev-parse", "HEAD")
	write(t, dir, "lane.txt", "the lane file\n")
	git("add", "lane.txt")
	tree0 := git("write-tree")
	git("-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "peer")
	git("rm", "-q", "--cached", "lane.txt")
	write(t, dir, "peer.txt", "a peer landing\n")
	git("add", "peer.txt")
	git("-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "peer file")
	base1 := git("rev-parse", "HEAD")
	git("add", "lane.txt")
	tree1 := git("write-tree")

	audited, composed, ok, err := Identical(context.Background(), run, dir, base0, tree0, base1, tree1)

	if err != nil || !ok || len(audited) == 0 || string(audited) != string(composed) {
		t.Fatalf("Identical = (ok %v, err %v, %d/%d bytes); the same bytes on two bases are one change", ok, err, len(audited), len(composed))
	}
	if !strings.Contains(string(audited), "index 0000000000000000000000000000000000000000..") {
		t.Errorf("full blob ids: %s", audited)
	}

	write(t, dir, "lane.txt", "The lane file\n")
	git("add", "lane.txt")
	if _, _, ok, err := Identical(context.Background(), run, dir, base0, tree0, base1, git("write-tree")); err != nil || ok {
		t.Fatalf("one flipped byte of the same length declines: ok=%v err=%v", ok, err)
	}
	if _, _, ok, err := Identical(context.Background(), run, dir, base0, base0+"^{tree}", base1, base1+"^{tree}"); err != nil || ok {
		t.Fatalf("an empty change never holds: ok=%v err=%v", ok, err)
	}
	if _, _, _, err := Identical(context.Background(), run, dir, "no-such-commit", tree0, base1, tree1); err == nil {
		t.Fatal("an unreadable base is a fault, not a decline")
	}
}

func TestDelta_KeepsWhitespaceAndArgsAreTheOneInvocation(t *testing.T) {
	dir, git := repo(t)
	write(t, dir, "a.txt", "x\n")
	git("add", "a.txt")
	git("-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "a")
	write(t, dir, "a.txt", "x \n")
	git("add", "a.txt")

	delta, err := Delta(context.Background(), run, dir, "HEAD", git("write-tree"))

	if err != nil || len(delta) == 0 {
		t.Fatalf("a trailing space is a change: %q %v", delta, err)
	}
	if got := strings.Join(Args("b", "t"), " "); got != "diff --binary --full-index --no-ext-diff --no-textconv --no-renames b t" {
		t.Errorf("Args = %q", got)
	}
}
