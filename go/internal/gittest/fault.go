package gittest

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const (
	faultBypassEnv    = "GITTEST_FAULT_BYPASS"
	faultedVerbCode   = 128
	maxFaultExitCode  = 255
	globalArgOptions  = "-C|-c|--git-dir|--work-tree|--namespace"
	faultStdoutFile   = "stdout"
	faultStderrFile   = "stderr"
	faultCounterFile  = "count"
	faultDataFileMode = 0o600
	faultShimFileMode = 0o700
)

type GitFault struct {
	Verb   string
	Args   []string
	Skip   int
	Code   int
	Stdout string
	Stderr string
}

func FaultOn(tb testing.TB, verb string) {
	tb.Helper()
	InstallFault(tb, GitFault{Verb: verb, Code: faultedVerbCode})
}

func InstallFault(tb testing.TB, f GitFault) {
	tb.Helper()
	if err := f.validate(); err != nil {
		tb.Fatalf("gittest: InstallFault: %v", err)
	}
	realGit, err := exec.LookPath("git")
	if err != nil {
		tb.Fatalf("gittest: InstallFault: git not on PATH: %v", err)
	}
	bin := tb.TempDir()
	for name, body := range map[string]string{faultStdoutFile: f.Stdout, faultStderrFile: f.stderr()} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), faultDataFileMode); err != nil {
			tb.Fatalf("gittest: InstallFault: write the %s of the fault: %v", name, err)
		}
	}
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(f.shim(realGit, bin)), faultShimFileMode); err != nil {
		tb.Fatalf("gittest: InstallFault: write the git shim: %v", err)
	}
	tb.Setenv("PATH", bin+string(os.PathListSeparator)+getenv("PATH"))
}

var getenv = os.Getenv

func (f GitFault) validate() error {
	switch {
	case f.Verb == "" || strings.HasPrefix(f.Verb, "-"):
		return fmt.Errorf("verb %q is not a git subcommand", f.Verb)
	case f.Skip < 0:
		return fmt.Errorf("skip %d is negative", f.Skip)
	case f.Code < 0 || f.Code > maxFaultExitCode:
		return fmt.Errorf("exit code %d is outside 0..%d", f.Code, maxFaultExitCode)
	}
	return nil
}

func (f GitFault) stderr() string {
	if f.Stderr != "" || f.Code == 0 {
		return f.Stderr
	}
	return "fatal: gittest: injected fault on git " + f.Verb + "\n"
}

func (f GitFault) shim(realGit, bin string) string {
	var argSeen, argMatch strings.Builder
	for i, arg := range f.Args {
		n := strconv.Itoa(i)
		fmt.Fprintf(&argSeen, "  [ \"$a\" = %s ] && m%s=1\n", shellQuote(arg), n)
		fmt.Fprintf(&argMatch, " && [ -n \"$m%s\" ]", n)
	}
	counter := shellQuote(filepath.Join(bin, faultCounterFile))
	return fmt.Sprintf(`#!/bin/sh
real=%s
[ -n "${%s:-}" ] && exec "$real" "$@"
verb=
skipnext=
for a in "$@"; do
  if [ -n "$skipnext" ]; then skipnext=; continue; fi
  if [ -z "$verb" ]; then
    case "$a" in
      %s) skipnext=1 ;;
      -*) ;;
      *) verb=$a ;;
    esac
    continue
  fi
%sdone
if [ "$verb" = %s ]%s; then
  n=$(cat %s 2>/dev/null || echo 0); n=$((n+1)); echo "$n" > %s
  if [ "$n" -gt %d ]; then
    cat %s
    cat %s >&2
    exit %d
  fi
fi
exec "$real" "$@"
`, shellQuote(realGit), faultBypassEnv, globalArgOptions, argSeen.String(), shellQuote(f.Verb), argMatch.String(),
		counter, counter, f.Skip,
		shellQuote(filepath.Join(bin, faultStdoutFile)), shellQuote(filepath.Join(bin, faultStderrFile)), f.Code)
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func BranchExists(tb testing.TB, dir, branch string) bool {
	tb.Helper()
	if branch == "" {
		return false
	}
	cmd := sysexec.Command(context.Background(), "git", "-C", dir, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	cmd.Env = append(os.Environ(), faultBypassEnv+"=1", "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0")
	err := tolerateWaitDelay(cmd, cmd.Run())
	var exitErr *exec.ExitError
	switch {
	case err == nil:
		return true
	case errors.As(err, &exitErr):
		return false
	default:
		tb.Fatalf("gittest: BranchExists %s in %s: git did not run: %v", branch, dir, err)
		return false
	}
}
