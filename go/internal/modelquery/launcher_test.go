package modelquery

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type launchCall struct {
	name string
	args []string
}

func useFakeLauncher(t *testing.T, out string) *[]launchCall {
	t.Helper()
	var calls []launchCall
	UseLauncher(func(_ context.Context, name string, args []string, _ string) (string, error) {
		calls = append(calls, launchCall{name: name, args: args})
		return out, nil
	})
	t.Cleanup(func() { UseLauncher(nil) })
	return &calls
}

func TestHelpEffortLister_TheDefaultRunnerStartsAgyHelpThroughTheInjectedLauncher(t *testing.T) {
	calls := useFakeLauncher(t, "  --effort   Reasoning effort for the current CLI session (low|medium|high)\n")

	efforts, err := HelpEffortLister{}.ListEfforts(context.Background(), "agy")

	if err != nil || !reflect.DeepEqual(efforts, []string{"low", "medium", "high"}) {
		t.Fatalf("got %q, %v", efforts, err)
	}
	if !reflect.DeepEqual(*calls, []launchCall{{name: "agy", args: []string{"--help"}}}) {
		t.Errorf("launcher calls = %+v; want one `agy --help`", *calls)
	}
}

func TestUseLauncher_WithoutALauncherTheDefaultRunnerStartsNothing(t *testing.T) {
	UseLauncher(nil)

	_, err := defaultRunner(context.Background(), "agy", []string{"models"}, "")

	if !errors.Is(err, errNoLauncher) {
		t.Fatalf("err = %v; want errNoLauncher, never a direct exec", err)
	}
}

func TestRunner_TheDefaultRunnerPassesTheCallAndTheStdinToTheLauncher(t *testing.T) {
	var got string
	UseLauncher(func(_ context.Context, name string, args []string, stdin string) (string, error) {
		got = name + " " + args[0] + " " + stdin
		return "out", nil
	})
	t.Cleanup(func() { UseLauncher(nil) })
	var run Runner = defaultRunner

	out, err := run(context.Background(), "ollama", []string{"list"}, "in")

	if err != nil || out != "out" || got != "ollama list in" {
		t.Fatalf("out=%q err=%v call=%q", out, err, got)
	}
}
