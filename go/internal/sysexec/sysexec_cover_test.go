package sysexec_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func TestOutput_AnUnrecoverableRunErrorIsWrappedWithTheCommand(t *testing.T) {
	t.Parallel()
	cause := errors.New("binary not found")

	got, err := sysexec.Output(context.Background(), stubRun("partial\n", "", -1, cause), "", "git", "rev-parse", "HEAD")

	if !errors.Is(err, cause) {
		t.Fatalf("Output error = %v, want it to wrap %v", err, cause)
	}
	if want := "sysexec: git [rev-parse HEAD]: binary not found"; err.Error() != want {
		t.Errorf("Output error = %q, want %q", err.Error(), want)
	}
	if got != "" {
		t.Errorf("Output = %q, want no stdout on a run error", got)
	}
}

func TestCombinedOutput_AGivenStdinReachesTheCommand(t *testing.T) {
	t.Parallel()
	var seen string
	run := func(_ context.Context, _, _ string, _, _ []string, stdin io.Reader, stdout, _ io.Writer) (int, error) {
		if stdin == nil {
			return -1, errors.New("no stdin")
		}
		b, err := io.ReadAll(stdin)
		seen = string(b)
		_, _ = io.WriteString(stdout, strings.ToUpper(seen))
		return 0, err
	}

	got, err := sysexec.CombinedOutput(context.Background(), run, "", "codex", "classify this")

	if err != nil || seen != "classify this" || got != "CLASSIFY THIS" {
		t.Errorf("CombinedOutput = %q, %v (stdin seen %q), want the stdin piped to the command and its reply returned", got, err, seen)
	}
}
