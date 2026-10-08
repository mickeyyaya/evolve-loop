package proctree

import (
	"context"
	"errors"
	"io"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestNewLister_RunsPSInTheCLocaleAndAttachesArgumentsAndEnvironment(t *testing.T) {
	t.Parallel()
	var argv, env []string
	run := func(_ context.Context, name, _ string, args, e []string, _ io.Reader, stdout, _ io.Writer) (int, error) {
		argv, env = append([]string{name}, args...), e
		_, _ = io.WriteString(stdout, "  100     1   100   501 Thu Oct  8 14:00:00 2026     node\n  101     1   101   501 Thu Oct  8 14:00:00 2026     zsh\n")
		return 0, nil
	}
	var read ArgsReader = func(pid int) ([]string, map[string]string, error) {
		if pid == 101 {
			return nil, nil, errors.New("operation not permitted")
		}
		return []string{"node", "mcp.js"}, map[string]string{"EVOLVE_DISPATCH_ID": "r/1/a/p1n1"}, nil
	}

	got, err := NewLister(run, read, 501)(context.Background())

	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if want := "ps -A -o pid=,ppid=,pgid=,uid=,lstart=,comm="; strings.Join(argv, " ") != want {
		t.Errorf("ran %q, want %q", strings.Join(argv, " "), want)
	}
	if !slices.Contains(env, "LC_ALL=C") {
		t.Errorf("ps env lacks LC_ALL=C: lstart must parse in one fixed format")
	}
	if len(got) != 2 || !reflect.DeepEqual(got[0].Args, []string{"node", "mcp.js"}) || got[0].Env["EVOLVE_DISPATCH_ID"] != "r/1/a/p1n1" {
		t.Fatalf("got %+v, want pid 100 with its arguments and environment", got)
	}
	if got[1].Args != nil || len(got[1].Env) != 0 {
		t.Errorf("pid 101 = %+v, want no arguments and no environment: an unreadable process has no proof", got[1])
	}
}

func TestNewLister_AFailedPSIsAnError(t *testing.T) {
	t.Parallel()
	run := func(context.Context, string, string, []string, []string, io.Reader, io.Writer, io.Writer) (int, error) {
		return 1, nil
	}

	_, err := NewLister(run, nil, 501)(context.Background())

	if err == nil {
		t.Errorf("a ps exit 1 must be an error, never an empty table")
	}
}
