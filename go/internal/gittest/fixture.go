// Package gittest owns git-backed test fixtures: their creation and their
// teardown. Every repo it builds persists a config that keeps git's automatic
// maintenance out of the background (from git 2.47 a commit or fetch otherwise
// detaches a `git maintenance run --auto` child that keeps writing in
// .git/objects after the command returned), so production code that runs a raw
// git inside a fixture inherits the same guarantee. Teardown retries a bounded
// number of times and, if the tree still cannot be removed, fails the test
// naming the processes that hold it.
package gittest

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// teardownAttempts bounds the removal retry; with a doubling backoff from
// teardownBackoff it waits about 1.3s in total before failing the test.
const (
	teardownAttempts = 8
	teardownBackoff  = 10 * time.Millisecond
)

// quietConfig is written into every fixture repo's own config. maintenance.auto
// stops the detached maintenance child (gc.auto=0 alone does not); gc.auto=0
// covers gits older than 2.47, whose commit runs `gc --auto` directly. The
// identity makes commits work on a runner with no global identity.
var quietConfig = [][2]string{
	{"maintenance.auto", "false"},
	{"gc.auto", "0"},
	{"user.name", "gittest"},
	{"user.email", "gittest@example.com"},
}

// Repo is a git repository owned by one test and removed when that test ends.
type Repo struct {
	// Dir is the absolute work-tree root (the repository itself for a bare repo).
	Dir string
	tb  testing.TB
}

// Fixture returns an initialized, committable work tree on branch main.
func Fixture(tb testing.TB) *Repo {
	tb.Helper()
	r := newRepo(tb)
	r.Git("init", "-q", "-b", "main")
	r.quiet()
	return r
}

// Bare returns an initialized bare repository whose default branch is main.
func Bare(tb testing.TB) *Repo {
	tb.Helper()
	r := newRepo(tb)
	r.Git("init", "-q", "--bare", "-b", "main")
	r.quiet()
	return r
}

// Clone returns a clone of src whose own config carries the fixture settings.
func Clone(tb testing.TB, src string) *Repo {
	tb.Helper()
	r := newRepo(tb)
	args := []string{"clone", "-q"}
	for _, kv := range quietConfig {
		args = append(args, "-c", kv[0]+"="+kv[1])
	}
	r.Git(append(args, src, ".")...)
	return r
}

// Git runs git in Dir and returns its trimmed combined output; a failing git
// fails the test with the arguments, the directory and git's output.
func (r *Repo) Git(args ...string) string {
	r.tb.Helper()
	cmd := exec.Command("git", append([]string{"-C", r.Dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		r.tb.Fatalf("gittest: git %s in %s: %v\n%s", strings.Join(args, " "), r.Dir, err, out)
	}
	return strings.TrimSpace(string(out))
}

// newRepo makes an empty directory under tb.TempDir() and registers its
// retried removal. tb.Cleanup runs LIFO, so the retry runs before TempDir's
// own single RemoveAll, which then finds nothing left to race.
func newRepo(tb testing.TB) *Repo {
	tb.Helper()
	dir := filepath.Join(tb.TempDir(), "repo")
	if err := os.Mkdir(dir, 0o755); err != nil {
		tb.Fatalf("gittest: create fixture dir: %v", err)
	}
	tb.Cleanup(func() { removeWithRetry(tb, dir, teardownAttempts) })
	return &Repo{Dir: dir, tb: tb}
}

func (r *Repo) quiet() {
	r.tb.Helper()
	for _, kv := range quietConfig {
		r.Git("config", kv[0], kv[1])
	}
}

// removeWithRetry removes dir, retrying with a doubling backoff so a git child
// that outlives its test body can finish; on exhaustion it fails tb with the
// path and the processes holding it.
func removeWithRetry(tb testing.TB, dir string, attempts int) {
	tb.Helper()
	var err error
	backoff := teardownBackoff
	for i := 0; i < attempts; i++ {
		if i > 0 {
			time.Sleep(backoff)
			backoff *= 2
		}
		if err = os.RemoveAll(dir); err == nil {
			return
		}
	}
	tb.Errorf("gittest: fixture %s not removed after %d attempts: %v\n%s", dir, attempts, err, holders(dir))
}

// holders reports the processes with a file or cwd under dir, looking lsof up
// on PATH at failure time.
func holders(dir string) string {
	lsof, err := exec.LookPath("lsof")
	if err != nil {
		return "holding process: diagnostic unavailable (lsof not on PATH)"
	}
	// lsof exits 1 even when it prints holders: judge its output, not its status.
	out, err := exec.Command(lsof, "+D", dir).Output()
	var exitErr *exec.ExitError
	if err != nil && !errors.As(err, &exitErr) {
		return fmt.Sprintf("holding process: diagnostic unavailable (lsof: %v)", err)
	}
	procs := parseLsof(string(out))
	if len(procs) == 0 {
		return "holding process: none found by lsof +D " + dir
	}
	return "holding processes: " + strings.Join(procs, ", ")
}

// parseLsof turns lsof's table into distinct "COMMAND (PID n)" entries.
func parseLsof(out string) []string {
	var procs []string
	seen := map[string]bool{}
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) < 2 || f[0] == "COMMAND" {
			continue
		}
		p := fmt.Sprintf("%s (PID %s)", f[0], f[1])
		if !seen[p] {
			seen[p] = true
			procs = append(procs, p)
		}
	}
	return procs
}
