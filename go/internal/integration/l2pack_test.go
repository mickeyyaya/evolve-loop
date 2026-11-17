package integration

import (
	"context"
	"slices"
	"strings"
	"testing"
)

func TestL2Pack_RunsEachStepInTheModuleDirOfItsCheckoutWithItsCommand(t *testing.T) {
	t.Parallel()
	var dirs, cmds []string
	var released int
	checkout := func(context.Context, string) (string, func(), error) {
		return "/tree", func() { released++ }, nil
	}
	var run Command = func(_ context.Context, dir string, argv ...string) (string, int, error) {
		dirs = append(dirs, dir)
		cmds = append(cmds, strings.Join(argv, " "))
		return "", 0, nil
	}
	for _, step := range L2Pack(checkout, run) {
		out, err := step.Run(context.Background(), "tip", step.Name)
		if err != nil || !out.Green {
			t.Fatalf("step %s = %+v, %v, want green", step.Name, out, err)
		}
	}
	if released != 6 || !slices.Equal(dirs, slices.Repeat([]string{"/tree/go"}, 6)) {
		t.Errorf("released %d, dirs %v, want six checkouts released and every command run in /tree/go", released, dirs)
	}
	for i, want := range []string{"go test -count=1 ./internal/", "make test-integration cover-changed", "make test-integration cover-strict", "go build ./...", "go vet ./...", "gofmt -l ."} {
		if !strings.HasPrefix(cmds[i], want) {
			t.Errorf("command %d = %q, want prefix %q", i, cmds[i], want)
		}
	}
}

func TestL2Pack_ANonZeroExitIsRedAndACheckoutFailureIsAnError(t *testing.T) {
	t.Parallel()
	checkout := func(context.Context, string) (string, func(), error) { return "/tree", func() {}, nil }
	failing := func(context.Context, string, ...string) (string, int, error) { return "boom\n", 2, nil }
	step := L2Pack(checkout, failing)[3]
	if out, err := step.Run(context.Background(), "tip", "build"); err != nil || out.Green || out.Output != "boom\n" {
		t.Errorf("build with exit 2 = %+v, %v, want red with its output", out, err)
	}
	noTree := func(context.Context, string) (string, func(), error) { return "", nil, context.Canceled }
	if _, err := L2Pack(noTree, failing)[3].Run(context.Background(), "tip", "build"); err == nil {
		t.Error("a failed checkout must be an error")
	}
}
