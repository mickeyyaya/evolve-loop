// Package gittest owns git-backed test fixtures: their creation and their
// teardown.
package gittest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const (
	teardownAttempts = 8
	teardownBackoff  = 10 * time.Millisecond
)

var maintenanceConfig = [][2]string{
	{"maintenance.auto", "false"},
	{"gc.auto", "0"},
}

func MaintenanceConfig() [][2]string {
	return slices.Clone(maintenanceConfig)
}

var quietConfig = slices.Concat(maintenanceConfig, [][2]string{
	{"user.name", "gittest"},
	{"user.email", "gittest@example.com"},
})

type Repo struct {
	Dir string
	tb  testing.TB
}

func Fixture(tb testing.TB) *Repo {
	tb.Helper()
	r := newRepo(tb)
	r.initWorkTree()
	return r
}

func (r *Repo) initWorkTree() {
	r.tb.Helper()
	r.Git("init", "-q", "-b", "main")
	r.quiet()
}

func Bare(tb testing.TB) *Repo {
	tb.Helper()
	r := newRepo(tb)
	r.initBare()
	return r
}

func (r *Repo) initBare() {
	r.tb.Helper()
	r.Git("init", "-q", "--bare", "-b", "main")
	r.quiet()
}

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

func (r *Repo) CloneBare(src, dst string) *Repo {
	r.tb.Helper()
	args := []string{"clone", "-q", "--bare"}
	for _, kv := range quietConfig {
		args = append(args, "-c", kv[0]+"="+kv[1])
	}
	r.Git(append(args, src, dst)...)
	return &Repo{Dir: dst, tb: r.tb}
}

func ConfigEnv(extra ...[2]string) []string {
	pairs := slices.Concat(maintenanceConfig, extra)
	env := []string{"GIT_CONFIG_COUNT=" + strconv.Itoa(len(pairs))}
	for i, kv := range pairs {
		n := strconv.Itoa(i)
		env = append(env, "GIT_CONFIG_KEY_"+n+"="+kv[0], "GIT_CONFIG_VALUE_"+n+"="+kv[1])
	}
	return env
}

func (r *Repo) Git(args ...string) string {
	r.tb.Helper()
	out, err := CaptureWithEBADFRetry(func() ([]byte, error) {
		cmd := sysexec.Command(context.Background(), "git", append([]string{"-C", r.Dir}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0")
		out, err := combinedOutput(cmd)
		return out, tolerateWaitDelay(cmd, err)
	})
	if err != nil {
		r.tb.Fatalf("gittest: git %s in %s: %v\n%s", strings.Join(args, " "), r.Dir, err, out)
	}
	return strings.TrimSpace(string(out))
}

var combinedOutput = (*exec.Cmd).CombinedOutput

func tolerateWaitDelay(cmd *exec.Cmd, err error) error {
	if errors.Is(err, exec.ErrWaitDelay) && cmd.ProcessState != nil && cmd.ProcessState.Success() {
		return nil
	}
	return err
}

func CaptureWithEBADFRetry(fn func() ([]byte, error)) ([]byte, error) {
	out, err := fn()
	if !IsEBADFLike(err) {
		return out, err
	}
	return fn()
}

func IsEBADFLike(err error) bool {
	return errors.Is(err, syscall.EBADF) || errors.Is(err, io.ErrClosedPipe)
}

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

func holders(dir string) string {
	lsof, err := exec.LookPath("lsof")
	if err != nil {
		return "holding process: diagnostic unavailable (lsof not on PATH)"
	}
	out, err := sysexec.Command(context.Background(), lsof, "+D", dir).Output()
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
