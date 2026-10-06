package cliupdate_test

import (
	"context"
	"errors"
	"io"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliupdate"
)

type recordedRun struct {
	name        string
	args        []string
	env         []string
	hasDeadline bool
	hasStdin    bool
}

func fakeRunner(code int, stdout, stderr string, err error, got *recordedRun) func(context.Context, string, string, []string, []string, io.Reader, io.Writer, io.Writer) (int, error) {
	return func(ctx context.Context, name, _ string, args, env []string, stdin io.Reader, out, errOut io.Writer) (int, error) {
		_, hasDeadline := ctx.Deadline()
		*got = recordedRun{name: name, args: args, env: env, hasDeadline: hasDeadline, hasStdin: stdin != nil}
		_, _ = io.WriteString(out, stdout)
		_, _ = io.WriteString(errOut, stderr)
		return code, err
	}
}

func TestExec_RunsTheUpdaterArgvUnderADeadlineWithoutStdin(t *testing.T) {
	var got recordedRun
	run := cliupdate.Exec(fakeRunner(0, "Successfully updated", "", nil, &got))

	if err := run(context.Background(), claude()); err != nil {
		t.Fatalf("exit 0 is success, got %v", err)
	}
	want := recordedRun{name: "claude", args: []string{"update"}, hasDeadline: true}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ran %+v, want %+v (a family with no off switch inherits the environment)", got, want)
	}
}

func TestExec_TheUpdaterRunsWithoutItsFamilysAutoUpdateOffSwitch(t *testing.T) {
	t.Setenv("AGY_CLI_DISABLE_AUTO_UPDATE", "1")
	t.Setenv("CLIUPDATE_TEST_KEPT", "yes")
	var got recordedRun

	if err := cliupdate.Exec(fakeRunner(0, "", "", nil, &got))(context.Background(), agy()); err != nil {
		t.Fatalf("Exec: %v", err)
	}

	if got.env == nil || !slices.Contains(got.env, "CLIUPDATE_TEST_KEPT=yes") {
		t.Fatalf("the updater keeps the rest of the operator's environment; env=%v", got.env)
	}
	for _, kv := range got.env {
		if strings.HasPrefix(kv, "AGY_CLI_DISABLE_AUTO_UPDATE=") {
			t.Fatalf("an operator who exports agy's off switch must not silently block `agy update`; env carries %q", kv)
		}
	}
}

func TestExec_ANonZeroExitIsAnErrorCarryingTheUpdatersOutput(t *testing.T) {
	var got recordedRun
	run := cliupdate.Exec(fakeRunner(3, "checking for updates\n", "error: managed by Homebrew\n", nil, &got))

	err := run(context.Background(), agy())

	if err == nil || !strings.Contains(err.Error(), "agy update") || !strings.Contains(err.Error(), "exit 3") || !strings.Contains(err.Error(), "managed by Homebrew") {
		t.Fatalf("want an error naming the argv, the exit code and the output, got %v", err)
	}
}

func TestExec_ALongUpdaterOutputKeepsOnlyItsTail(t *testing.T) {
	var got recordedRun
	noise := strings.Repeat("downloading chunk\n", 100)
	run := cliupdate.Exec(fakeRunner(1, noise, "FINAL: checksum mismatch", nil, &got))

	err := run(context.Background(), claude())

	if err == nil || !strings.HasSuffix(err.Error(), "FINAL: checksum mismatch") {
		t.Fatalf("the updater's last stderr line must survive the cut, got %v", err)
	}
	if !strings.Contains(err.Error(), "…") || len(err.Error()) > 500 {
		t.Errorf("a long output is cut to its tail (%d bytes): %q", len(err.Error()), err.Error())
	}
}

func TestExec_ALaunchFailureIsAnError(t *testing.T) {
	var got recordedRun
	run := cliupdate.Exec(fakeRunner(-1, "", "", errors.New("executable file not found"), &got))

	if err := run(context.Background(), agy()); err == nil || !strings.Contains(err.Error(), "executable file not found") {
		t.Fatalf("want the launch error, got %v", err)
	}
}

func TestExec_AnEmptyArgvRunsNothing(t *testing.T) {
	var got recordedRun
	run := cliupdate.Exec(fakeRunner(0, "", "", nil, &got))

	if err := run(context.Background(), cliupdate.Family{Name: "ollama"}); err == nil || got.name != "" {
		t.Fatalf("an empty argv is an error and launches nothing: err=%v ran=%+v", err, got)
	}
}
