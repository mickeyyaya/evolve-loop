// Package shiptest holds the real-git fixtures that the ship phase tests share and the black-box ship scenarios that run as their own test package.
// The fixtures never import the ship phase, so the in-package ship tests can import them.
// See docs/architecture/packages/internal-phases-ship-shiptest.md.
package shiptest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/shipwindow"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func check(tb testing.TB, err error, format string, args ...any) {
	tb.Helper()
	if err != nil {
		tb.Fatalf(format+": %v", append(args, err)...)
	}
}

func MustWrite(tb testing.TB, path, content string) {
	tb.Helper()
	check(tb, os.MkdirAll(filepath.Dir(path), 0o755), "mkdir %s", path)
	check(tb, os.WriteFile(path, []byte(content), 0o644), "write %s", path)
}

func MustMkdir(tb testing.TB, path string) {
	tb.Helper()
	check(tb, os.MkdirAll(path, 0o755), "mkdir %s", path)
}

func TempRepoDir(tb testing.TB) string {
	tb.Helper()
	dir, err := os.MkdirTemp("", "shiptest-*")
	check(tb, err, "mkdir temp repo")
	tb.Cleanup(func() {
		_ = filepath.WalkDir(dir, func(p string, _ fs.DirEntry, walkErr error) error {
			if walkErr == nil {
				_ = os.Chmod(p, 0o755)
			}
			return nil
		})
		_ = os.RemoveAll(dir)
	})
	return dir
}

func MakeRepo(tb testing.TB) string {
	tb.Helper()
	dir := gittest.FromTemplate(tb, "ship-make-repo", func(r *gittest.Repo) error {
		r.Write(".gitignore", ".evolve/\n")
		r.Write(".evolve/ledger.jsonl", "")
		r.Write(".evolve/state.json", "{}\n")
		r.Write("fixture.txt", "fixture line 1\n")
		r.Write("ship-binary-fixture", "ship-binary-v1\n")
		r.Git("config", "user.email", "test@evolve-loop.test")
		r.Git("config", "user.name", "Test User")
		r.Git("config", "core.hooksPath", "/dev/null")
		r.Git("add", "-A")
		r.Git("-c", "commit.gpgsign=false", "commit", "-q", "-m", "initial test repo")
		return nil
	}).Dir
	MustMkdir(tb, filepath.Join(dir, ".evolve", "runs", "cycle-1"))
	return dir
}

func AddRemote(tb testing.TB, repo string) string {
	tb.Helper()
	bare := gittest.BareFromTemplate(tb).Dir
	RunGit(tb, repo, "remote", "add", "origin", bare)
	return bare
}

func MakeWorktree(tb testing.TB, repo, branch string) string {
	tb.Helper()
	wt := filepath.Join(tb.TempDir(), "wt")
	RunGit(tb, repo, "worktree", "add", "-b", branch, wt, "main")
	return wt
}

func gitCommand(repo string, args []string) *exec.Cmd {
	cmd := sysexec.Command(context.Background(), "git", append([]string{"-C", repo}, args...)...)
	cmd.Env = FilteredEnv()
	return cmd
}

func RunGit(tb testing.TB, repo string, args ...string) {
	tb.Helper()
	out, err := gittest.CaptureWithEBADFRetry(func() ([]byte, error) {
		return gitCommand(repo, args).CombinedOutput()
	})
	check(tb, err, "git %v (output %q)", args, out)
}

func RunGitOut(tb testing.TB, repo string, args ...string) string {
	tb.Helper()
	out, err := gittest.CaptureWithEBADFRetry(func() ([]byte, error) {
		return gitCommand(repo, args).Output()
	})
	check(tb, err, "git %v", args)
	return string(out)
}

func FilteredEnv() []string {
	var out []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "EVOLVE_") {
			out = append(out, kv)
		}
	}
	return out
}

func HeadSHA(tb testing.TB, repo string) string {
	tb.Helper()
	return strings.TrimSpace(RunGitOut(tb, repo, "rev-parse", "HEAD"))
}

func RemoteHeadSHA(tb testing.TB, repo string) string {
	tb.Helper()
	fields := strings.Fields(RunGitOut(tb, repo, "ls-remote", "origin", "refs/heads/main"))
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func MustHashFile(tb testing.TB, path string) string {
	tb.Helper()
	f, err := os.Open(path)
	check(tb, err, "open %s", path)
	h := sha256.New()
	_, err = io.Copy(h, f)
	closeErr := f.Close()
	check(tb, err, "hash %s", path)
	check(tb, closeErr, "close %s", path)
	return hex.EncodeToString(h.Sum(nil))
}

func TreeStateSHA(tb testing.TB, repo string) string {
	tb.Helper()
	sum := sha256.Sum256([]byte(RunGitOut(tb, repo, "diff", "HEAD")))
	return hex.EncodeToString(sum[:])
}

func commitAndPush(tb testing.TB, clone, file, content, message string) {
	tb.Helper()
	MustWrite(tb, filepath.Join(clone, file), content)
	RunGit(tb, clone, "add", "-A")
	RunGit(tb, clone, "-c", "commit.gpgsign=false", "commit", "-q", "-m", message)
	RunGit(tb, clone, "push", "-q", "origin", "main")
}

func cloneAs(tb testing.TB, bare, start, name, email string) string {
	tb.Helper()
	clone := filepath.Join(TempRepoDir(tb), "clone")
	RunGit(tb, filepath.Dir(clone), "clone", "-q", bare, clone)
	RunGit(tb, clone, "checkout", "-q", "-B", "main", start)
	RunGit(tb, clone, "config", "user.email", email)
	RunGit(tb, clone, "config", "user.name", name)
	return clone
}

func PushDivergentCommit(tb testing.TB, bare string) {
	tb.Helper()
	clone := cloneAs(tb, bare, "origin/main", "Second Writer", "second@evolve-loop.test")
	commitAndPush(tb, clone, "divergent.txt", "independent change\n", "divergent")
}

func PushOnTopOf(tb testing.TB, bare, commit string) string {
	tb.Helper()
	clone := cloneAs(tb, bare, commit, "Peer Lane", "peer@evolve-loop.test")
	commitAndPush(tb, clone, "peer.txt", "a peer landing on top\n", "peer landing")
	return HeadSHA(tb, clone)
}

func SkipWhenRoot(tb testing.TB) {
	tb.Helper()
	skipIf(tb, os.Geteuid() == 0, "root ignores directory permissions, so a read-only directory cannot fail a write")
}

func skipIf(tb testing.TB, skip bool, reason string) {
	tb.Helper()
	if skip {
		tb.Skip(reason)
	}
}

func LeaseHeld(root string) bool {
	_, err := os.Stat(shipwindow.PathIn(filepath.Join(root, ".evolve")))
	return err == nil
}

func WriteStrictAuditPolicy(tb testing.TB, root string) {
	tb.Helper()
	MustWrite(tb, filepath.Join(root, ".evolve", "policy.json"), `{"workflow":{"strict_audit":true}}`)
}

func ContainsLog(logs []string, substr string) bool {
	for _, l := range logs {
		if strings.Contains(l, substr) {
			return true
		}
	}
	return false
}

func MustShipErr(tb testing.TB, err error) *core.ShipError {
	tb.Helper()
	se, ok := core.AsShipError(err)
	if !ok {
		tb.Fatalf("expected a recoverable *core.ShipError; got %T: %v", err, err)
	}
	return se
}

func WantShipErr(tb testing.TB, err error, code core.ShipErrorCode, class core.ShipErrorClass, msgSubstr string) *core.ShipError {
	tb.Helper()
	se := MustShipErr(tb, err)
	if se.Code != code {
		tb.Fatalf("want Code=%s, got %s (%v)", code, se.Code, err)
	}
	if se.Class != class {
		tb.Fatalf("want Class=%s, got %s (%v)", class, se.Class, err)
	}
	if msgSubstr != "" && !strings.Contains(se.Message, msgSubstr) {
		tb.Errorf("message should contain %q; got %q", msgSubstr, se.Message)
	}
	return se
}

func ShipWorktreeFixture(tb testing.TB, cycleBranch string) (repo, wt string) {
	tb.Helper()
	repo = MakeRepo(tb)
	AddRemote(tb, repo)
	wt = MakeWorktree(tb, repo, cycleBranch)
	MustWrite(tb, filepath.Join(wt, "feature.txt"), "real cycle work\n")
	RunGit(tb, wt, "add", "feature.txt")
	boundTree := strings.TrimSpace(RunGitOut(tb, wt, "write-tree"))
	MustWrite(tb, filepath.Join(repo, ".evolve", "cycle-state.json"),
		`{"cycle_id":1,"phase":"ship","active_worktree":"`+wt+`"}`)
	SeedAuditWithBoundTree(tb, repo, "PASS", boundTree)
	return repo, wt
}

func MergedUnpushedFixture(tb testing.TB, cycleBranch string) string {
	tb.Helper()
	repo := MakeRepo(tb)
	AddRemote(tb, repo)
	RunGit(tb, repo, "push", "-q", "origin", "main")
	wt := MakeWorktree(tb, repo, cycleBranch)
	MustWrite(tb, filepath.Join(wt, "feature.txt"), "audited cycle work\n")
	RunGit(tb, wt, "add", "-A")
	RunGit(tb, wt, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "cycle work")
	boundTree := strings.TrimSpace(RunGitOut(tb, wt, "rev-parse", "HEAD^{tree}"))
	SeedAuditWithBoundTree(tb, repo, "PASS", boundTree)
	RunGit(tb, repo, "merge", "--ff-only", "-q", cycleBranch)
	MustWrite(tb, filepath.Join(repo, ".evolve", "cycle-state.json"),
		fmt.Sprintf(`{"cycle_id":1,"run_id":"test-run","audit_dispatches":1,"phase":"ship","active_worktree":%q}`, wt))
	return repo
}
