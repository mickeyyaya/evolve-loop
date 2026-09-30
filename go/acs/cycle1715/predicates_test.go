//go:build acs

package cycle1715

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	c1529Predicate = "TestC1529_004_ClosureStaysDocOnly"
	c1529Ship      = "57e227c1e36f33562c922dcdf2546b160739e45d"
	c1529Base      = "19b427c4214e1ad6f84239cd1781f592b0faec22"
	probeFile      = "go/internal/bridge/zz_c1529_range_probe.go"
)

var redirectingGitVars = map[string]bool{
	"GIT_DIR":                          true,
	"GIT_WORK_TREE":                    true,
	"GIT_INDEX_FILE":                   true,
	"GIT_COMMON_DIR":                   true,
	"GIT_OBJECT_DIRECTORY":             true,
	"GIT_ALTERNATE_OBJECT_DIRECTORIES": true,
	"GIT_NO_REPLACE_OBJECTS":           true,
	"GIT_REPLACE_REF_BASE":             true,
}

func childEnv(extra ...string) []string {
	var env []string
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		if !redirectingGitVars[name] {
			env = append(env, kv)
		}
	}
	return append(env, extra...)
}

func git(t *testing.T, dir string, env []string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = childEnv(env...)
	out, err := cmd.Output()
	if err != nil {
		var stderr []byte
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			stderr = ee.Stderr
		}
		t.Fatalf("fixture: git %s: %v\n%s", strings.Join(args, " "), err, stderr)
	}
	return strings.TrimSpace(string(out))
}

func c1529Binary(t *testing.T, root string) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "cycle1529.test")
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "-C", filepath.Join(root, "go"), "test", "-c", "-tags", "acs", "-o", bin, "./acs/cycle1529")
	if code != 0 {
		t.Fatalf("fixture: compile ./acs/cycle1529 exit=%d err=%v\n%s%s", code, err, stdout, stderr)
	}
	return bin
}

func runC1529(t *testing.T, bin, fx string) (verdict, output string) {
	t.Helper()
	cmd := exec.Command(bin, "-test.run", "^"+c1529Predicate+"$", "-test.v", "-test.count=1")
	cmd.Dir = fx
	cmd.Env = childEnv()
	out, err := cmd.CombinedOutput()
	var ee *exec.ExitError
	if err != nil && !errors.As(err, &ee) {
		t.Fatalf("fixture: launch %s: %v", bin, err)
	}
	output = string(out)
	switch {
	case err == nil && strings.Contains(output, "--- PASS: "+c1529Predicate):
		return "PASS", output
	case err != nil && strings.Contains(output, "--- FAIL: "+c1529Predicate):
		return "FAIL", output
	}
	return "", output
}

func newFixture(t *testing.T, root string) (fx, head string) {
	t.Helper()
	fx = filepath.Join(t.TempDir(), "repo")
	objects := filepath.Join(git(t, root, nil, "rev-parse", "--path-format=absolute", "--git-common-dir"), "objects")
	head = git(t, root, nil, "rev-parse", "HEAD")
	git(t, filepath.Dir(fx), nil, "init", "-q", fx)
	if err := os.WriteFile(filepath.Join(fx, ".git", "objects", "info", "alternates"), []byte(objects+"\n"), 0o644); err != nil {
		t.Fatalf("fixture: alternates: %v", err)
	}
	git(t, fx, nil, "update-ref", "refs/heads/main", head)
	git(t, fx, nil, "symbolic-ref", "HEAD", "refs/heads/main")
	git(t, fx, nil, "sparse-checkout", "set", "--no-cone", "/go/internal/bridge/")
	git(t, fx, nil, "reset", "-q", "--hard", "main")
	git(t, fx, nil, "cat-file", "-e", c1529Ship+"^{commit}")
	if parent := git(t, fx, nil, "rev-parse", c1529Ship+"^"); parent != c1529Base {
		t.Fatalf("fixture: %s^ = %s, want cycle 1529's base %s", c1529Ship, parent, c1529Base)
	}
	return fx, head
}

func bridgeSources(t *testing.T, fx, rev string, n int) []string {
	t.Helper()
	var files []string
	for _, f := range strings.Split(git(t, fx, nil, "ls-tree", "-r", "--name-only", rev, "--", "go/internal/bridge"), "\n") {
		if strings.HasSuffix(f, ".go") && !strings.HasSuffix(f, "_test.go") {
			files = append(files, f)
		}
		if len(files) == n {
			return files
		}
	}
	t.Fatalf("fixture: want %d non-test Go sources under go/internal/bridge at %s, found %d", n, rev, len(files))
	return nil
}

func commitWith(t *testing.T, fx, treeish, parent, path, content string) string {
	t.Helper()
	scratch := t.TempDir()
	env := []string{"GIT_INDEX_FILE=" + filepath.Join(scratch, "index")}
	blobSrc := filepath.Join(scratch, "blob")
	if err := os.WriteFile(blobSrc, []byte(content), 0o644); err != nil {
		t.Fatalf("fixture: blob: %v", err)
	}
	blob := git(t, fx, nil, "hash-object", "-w", blobSrc)
	git(t, fx, env, "read-tree", treeish)
	git(t, fx, env, "update-index", "--add", "--cacheinfo", "100644,"+blob+","+path)
	tree := git(t, fx, env, "write-tree")
	return git(t, fx, nil, "-c", "user.name=acs", "-c", "user.email=acs@example.invalid",
		"commit-tree", "--no-gpg-sign", tree, "-p", parent, "-m", "fixture: "+path)
}

func appendTo(t *testing.T, fx, rel, line string) {
	t.Helper()
	f, err := os.OpenFile(filepath.Join(fx, rel), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("fixture: open %s: %v", rel, err)
	}
	defer f.Close()
	if _, err := f.WriteString("\n" + line + "\n"); err != nil {
		t.Fatalf("fixture: append %s: %v", rel, err)
	}
}

func TestC1715_001_LaterBridgeChangeDoesNotFailC1529Closure(t *testing.T) {
	root := acsassert.RepoRoot(t)
	bin := c1529Binary(t, root)

	t.Run("branch_ahead_of_main", func(t *testing.T) {
		fx, head := newFixture(t, root)
		files := bridgeSources(t, fx, head, 2)
		orig := git(t, fx, nil, "cat-file", "blob", head+":"+files[0])
		later := commitWith(t, fx, head, head, files[0], orig+"\n// later-branch change\n")
		git(t, fx, nil, "update-ref", "refs/heads/later", later)
		git(t, fx, nil, "checkout", "-q", "later")
		appendTo(t, fx, files[1], "// later-branch uncommitted change")

		drift := git(t, fx, nil, "diff", "--name-only", "main", "--", "go/internal/bridge")
		if !strings.Contains(drift, files[0]) || !strings.Contains(drift, files[1]) {
			t.Fatalf("fixture: branch should differ from main in %v, git diff shows %q", files, drift)
		}
		if verdict, out := runC1529(t, bin, fx); verdict != "PASS" {
			t.Errorf("RED: %s reported %q on a later branch whose only bridge changes (%s) are not cycle 1529's — it must diff %s..%s, not the live main ref:\n%s",
				c1529Predicate, verdict, drift, c1529Base[:8], c1529Ship[:8], out)
		}
	})

	t.Run("main_ahead_of_checkout", func(t *testing.T) {
		fx, head := newFixture(t, root)
		file := bridgeSources(t, fx, head, 1)[0]
		orig := git(t, fx, nil, "cat-file", "blob", head+":"+file)
		git(t, fx, nil, "checkout", "-q", "--detach")
		git(t, fx, nil, "update-ref", "refs/heads/main", commitWith(t, fx, head, head, file, orig+"\n// landed on main later\n"))

		drift := git(t, fx, nil, "diff", "--name-only", "main", "--", "go/internal/bridge")
		if drift != file {
			t.Fatalf("fixture: checkout should trail main by %s, git diff shows %q", file, drift)
		}
		if verdict, out := runC1529(t, bin, fx); verdict != "PASS" {
			t.Errorf("RED: %s reported %q on a checkout that trails a later bridge change on main (%s) — cycle 1529's closure does not depend on where main is:\n%s",
				c1529Predicate, verdict, file, out)
		}
	})
}

func TestC1715_002_BridgeChangeInsideC1529RangeStillFails(t *testing.T) {
	root := acsassert.RepoRoot(t)
	bin := c1529Binary(t, root)
	fx, _ := newFixture(t, root)

	if verdict, out := runC1529(t, bin, fx); verdict != "PASS" {
		t.Fatalf("RED: control — on cycle 1529's real, doc-only range with no drift from main, %s reported %q (want PASS):\n%s",
			c1529Predicate, verdict, out)
	}

	fake := commitWith(t, fx, c1529Ship, c1529Base, probeFile, "package bridge\n")
	git(t, fx, nil, "replace", c1529Ship, fake)
	if got := git(t, fx, nil, "diff", "--name-only", c1529Base, c1529Ship, "--", "go/internal/bridge"); got != probeFile {
		t.Fatalf("fixture: replaced range should add %s, git diff shows %q", probeFile, got)
	}
	if drift := git(t, fx, nil, "diff", "--name-only", "main", "--", "go/internal/bridge"); drift != "" {
		t.Fatalf("fixture: checkout must match main so only the range can fail the predicate, git diff shows %q", drift)
	}

	verdict, out := runC1529(t, bin, fx)
	if verdict != "FAIL" {
		t.Errorf("RED: %s reported %q although cycle 1529's own range (%s..%s) now adds %s — it must read that range:\n%s",
			c1529Predicate, verdict, c1529Base[:8], c1529Ship[:8], probeFile, out)
	}
	if !strings.Contains(out, filepath.Base(probeFile)) {
		t.Errorf("RED: %s output does not name %s, the bridge file inside cycle 1529's range:\n%s",
			c1529Predicate, probeFile, out)
	}
}
