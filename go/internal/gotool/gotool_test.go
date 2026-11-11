package gotool_test

import (
	"context"
	"os/exec"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gotool"
)

func TestListAndTest_GiveTheVerbBeforeTheArgsWithTheDirAndEnvOfTheCall(t *testing.T) {
	t.Parallel()
	call := gotool.Call{Dir: "/work", Env: []string{"A=1"}, Args: []string{"-json", "./..."}}
	for name, tc := range map[string]struct {
		build func(context.Context, gotool.Call) *exec.Cmd
		verb  string
	}{
		"list": {gotool.List, "list"},
		"test": {gotool.Test, "test"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			cmd := tc.build(context.Background(), call)
			want := []string{"go", tc.verb, "-json", "./..."}
			if !slices.Equal(cmd.Args, want) || cmd.Dir != "/work" || !slices.Equal(cmd.Env, []string{"A=1"}) {
				t.Errorf("args = %v, dir = %q, env = %v; want %v", cmd.Args, cmd.Dir, cmd.Env, want)
			}
			if cmd.Cancel == nil || cmd.WaitDelay == 0 {
				t.Error("the command has no Cancel or WaitDelay: it did not come from sysexec.Command")
			}
		})
	}
}

func TestCall_ArgsOfTheCallerAreNotChanged(t *testing.T) {
	t.Parallel()
	args := make([]string, 1, 8)
	args[0] = "x"

	_ = gotool.List(context.Background(), gotool.Call{Args: args})
	_ = gotool.Test(context.Background(), gotool.Call{Args: args})

	if args[0] != "x" || len(args) != 1 {
		t.Errorf("args = %v", args)
	}
}
