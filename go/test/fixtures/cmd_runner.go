package fixtures

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

// Compile-time proof FakeExec.Run satisfies the one command-execution seam.
// If sysexec.RunFunc changes, this fails at the definition, not a call site.
var _ sysexec.RunFunc = (*FakeExec)(nil).Run

// FakeExec is the canonical in-memory sysexec.RunFunc — the single test double
// for every faked subprocess (git/gh/go/tmux). It supersedes the per-package
// scripted runners (ship's scriptedRunner, commitprefixgate's execGit var, …)
// so command-faking logic lives in exactly one place.
//
// Zero value = every command succeeds (exit 0, empty output, nil error). A
// test sets only what it cares about: script responses per command, inspect
// the recorded Calls, or inject an exit code / error. Safe for concurrent use
// so it can back t.Parallel tests.
type FakeExec struct {
	mu sync.Mutex

	// Scripts maps a command KEY to its scripted response. The key is
	// derived by execKey: the binary name plus its first non-flag argument
	// (the subcommand), e.g. "git rev-parse". A name-only key ("git") is the
	// fallback when no subcommand key matches.
	//
	// Populate Scripts BEFORE the first Run call. Run only reads it (under the
	// mutex), so concurrent Run calls are safe; mutating Scripts while a Run is
	// in flight is a data race (the normal pattern is setup-then-run).
	Scripts map[string]ExecResponse

	// Default is returned when neither a subcommand nor a name-only key
	// matches. Its zero value is a successful empty run.
	Default ExecResponse

	// Calls records every invocation in order for assertions.
	Calls []ExecCall
}

// ExecResponse is one scripted command outcome. Zero value = success, no
// output. A non-zero ExitCode models a process that ran and exited non-zero
// (returned as (code, nil) per the sysexec contract); a non-nil Err models an
// unrecoverable failure (returned as (-1, Err)).
type ExecResponse struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Err      error
}

// ExecCall is a recorded invocation. Stdin is drained and captured so a test
// can assert what was piped (e.g. a release body to `gh release --notes-file -`).
type ExecCall struct {
	Name  string
	Dir   string
	Args  []string
	Env   []string
	Stdin string
	Key   string // the resolved Scripts key, for readable assertions
}

// Run is the sysexec.RunFunc. Pass the method value anywhere a RunFunc is
// wanted: opts.Runner = fake.Run.
func (f *FakeExec) Run(_ context.Context, name, dir string, args, env []string,
	stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	key := execKey(name, args)

	// I/O (stdin drain here, stdout/stderr writes below) runs OUTSIDE f.mu on
	// purpose: holding the mutex across a caller's Reader/Writer would serialize —
	// or deadlock on — blocking I/O. The contract that makes this safe under -race:
	// each Run call owns its stdin Reader and stdout/stderr Writers (every caller
	// passes a fresh strings.NewReader / *strings.Builder per invocation). Do NOT
	// share one Reader/Writer across concurrent Run calls.
	var in string
	var stdinErr error
	if stdin != nil {
		b, err := io.ReadAll(stdin)
		in = string(b)
		stdinErr = err
	}

	f.mu.Lock()
	f.Calls = append(f.Calls, ExecCall{
		Name:  name,
		Dir:   dir,
		Args:  append([]string(nil), args...),
		Env:   append([]string(nil), env...),
		Stdin: in,
		Key:   key,
	})
	resp, ok := f.Scripts[key]
	if !ok {
		resp, ok = f.Scripts[name] // name-only fallback
	}
	if !ok {
		resp = f.Default
	}
	f.mu.Unlock()

	// Fail loudly rather than silently truncating: a stdin reader that errors
	// is a test-setup bug, surfaced here as an unrecoverable run error. The
	// call is still recorded above for debuggability.
	if stdinErr != nil {
		return -1, fmt.Errorf("fakeexec: read stdin for %q: %w", key, stdinErr)
	}

	// Test writers (strings.Builder / bytes.Buffer) do not fail, so a write
	// error here is not actionable — discard it explicitly.
	if stdout != nil && resp.Stdout != "" {
		_, _ = stdout.Write([]byte(resp.Stdout))
	}
	if stderr != nil && resp.Stderr != "" {
		_, _ = stderr.Write([]byte(resp.Stderr))
	}
	if resp.Err != nil {
		return -1, resp.Err
	}
	return resp.ExitCode, nil
}

// CallKeys returns the resolved command keys in call order — the ergonomic
// "assert the right commands ran in the right order" accessor.
func (f *FakeExec) CallKeys() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	keys := make([]string, len(f.Calls))
	for i, c := range f.Calls {
		keys[i] = c.Key
	}
	return keys
}

// execKey derives a command's script key: the binary name plus its first
// non-flag argument (the subcommand), skipping leading flags and their values
// (-C <path>, -c <kv>). "git -C /wt status" => "git status". This mirrors the
// idiom the ship package's scriptedRunner pioneered, now centralized here.
//
// Only -C/-c are treated as value-taking flags — the git invocations in this
// codebase use exactly those. Other value flags that consume a positional
// (e.g. `git --git-dir <path>`) would key off the value, not the subcommand;
// add them here if a caller starts using one.
func execKey(name string, args []string) string {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "-") {
			return name + " " + a
		}
		if a == "-C" || a == "-c" {
			i++ // skip this flag's value
		}
	}
	return name
}
