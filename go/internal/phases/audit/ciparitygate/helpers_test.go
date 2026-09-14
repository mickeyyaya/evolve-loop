package ciparitygate

// helpers_test.go — the leaf's fakes and fixtures: a scripted runner, a fake
// change-set Strategy, a recording Center, a go-module worktree, the goldens
// captured on 8e8f080f (testdata/*.golden.*) and their path templating.

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

// fakeRunFunc emits fixed stdout/stderr and a fixed (code, err).
func fakeRunFunc(code int, stdout, stderr string, runErr error) sysexec.RunFunc {
	return func(_ context.Context, _, _ string, _, _ []string, _ io.Reader, so, se io.Writer) (int, error) {
		_, _ = io.WriteString(so, stdout)
		_, _ = io.WriteString(se, stderr)
		return code, runErr
	}
}

type step struct {
	Code int
	Out  string
}

// seqRunFunc scripts one (code, stdout) per successive call and records the
// env each call received.
func seqRunFunc(t *testing.T, script []step) (sysexec.RunFunc, *int, *[][]string) {
	t.Helper()
	calls := 0
	envs := [][]string{}
	fn := func(_ context.Context, _, _ string, _, env []string, _ io.Reader, so, _ io.Writer) (int, error) {
		if calls >= len(script) {
			t.Fatalf("run func called %d times, script has %d entries", calls+1, len(script))
		}
		s := script[calls]
		calls++
		envs = append(envs, env)
		_, _ = io.WriteString(so, s.Out)
		return s.Code, nil
	}
	return fn, &calls, &envs
}

// goWorktree builds <root>/go/{go.mod,bin/} — a real go module for moduleDir.
func goWorktree(t *testing.T) (root, goDir string) {
	t.Helper()
	root = t.TempDir()
	goDir = filepath.Join(root, "go")
	if err := os.MkdirAll(filepath.Join(goDir, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(goDir, "go.mod"), []byte("module ciparitytest\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, goDir
}

func fixedSet(pkgs ...string) ChangedSetFunc {
	return func(string, int) ([]string, bool) { return pkgs, true }
}

func underivableSet() ChangedSetFunc {
	return func(string, int) ([]string, bool) { return nil, false }
}

// observed builds gates reporting into a recording Center.
func observed(t *testing.T, run sysexec.RunFunc, set ChangedSetFunc, opts ...Option) (*Gates, *[]signalcenter.Event) {
	t.Helper()
	c := signalcenter.New()
	got := &[]signalcenter.Event{}
	c.Subscribe(func(e signalcenter.Event) { *got = append(*got, e) })
	opts = append(opts, WithSignals(func() *signalcenter.Center { return c }))
	return New(run, set, opts...), got
}

func tierRequest(root, ws string) Request { return Request{3, root, root, ws} }

// golden reads one `key<TAB>quoted` golden captured on 8e8f080f.
func golden(t *testing.T, name string) map[string]string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		k, q, ok := strings.Cut(line, "\t")
		if !ok {
			t.Fatalf("golden line %q", line)
		}
		v, err := strconv.Unquote(q)
		if err != nil {
			t.Fatal(err)
		}
		out[k] = v
	}
	return out
}

// fill substitutes the golden's path placeholders for this test's paths.
func fill(s, root, ws string) string {
	s = strings.ReplaceAll(s, "{MOD}", filepath.Join(root, "go"))
	s = strings.ReplaceAll(s, "{ROOT}", root)
	return strings.ReplaceAll(s, "{WS}", ws)
}

func codesOf(events []signalcenter.Event) []string {
	out := make([]string, 0, len(events))
	for _, e := range events {
		out = append(out, string(e.Code))
	}
	return out
}

// only returns the single event carrying code, failing on zero or several.
func only(t *testing.T, events []signalcenter.Event, code signalcenter.Code) signalcenter.Event {
	t.Helper()
	var found []signalcenter.Event
	for _, e := range events {
		if e.Code == code {
			found = append(found, e)
		}
	}
	if len(found) != 1 {
		t.Fatalf("want exactly one %s, got %d in %v", code, len(found), codesOf(events))
	}
	return found[0]
}

// captureStderr runs fn with os.Stderr swapped for a pipe and returns the bytes.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stderr
	os.Stderr = w
	fn()
	os.Stderr = orig
	_ = w.Close()
	b, _ := io.ReadAll(r)
	return string(b)
}

// streamLine renders one event the way the stream golden (G5) records it:
// the code plus its discriminating field.
func streamLine(e signalcenter.Event) string {
	line := string(e.Code)
	for _, k := range []string{"step", "cause", "reason", "attempt", "scope"} {
		if v, ok := e.Fields[k]; ok {
			line += " " + k + "=" + v
		}
	}
	return line
}

// killedAtDeadline makes a scripted runner faithful to a process the ctx
// deadline KILLED: it returns only once ctx is done — a real SIGKILL follows
// the deadline, never precedes it — so a 1 ns budget reaches the deadline arms
// deterministically. Without it, context.WithTimeout(…, 1ns) may arm a timer
// instead of expiring synchronously (two clock reads inside one tick), and an
// instant fake is observed before ctx.Err() is set: deadlineHit=false →
// retake_red instead of the deadline verdict (2/40 runs, 2026-09-14).
func killedAtDeadline(fn sysexec.RunFunc) sysexec.RunFunc {
	return func(ctx context.Context, name, dir string, args, env []string, in io.Reader, so, se io.Writer) (int, error) {
		<-ctx.Done()
		return fn(ctx, name, dir, args, env, in, so, se)
	}
}
