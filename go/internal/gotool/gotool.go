package gotool

import (
	"context"
	"os/exec"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

type Call struct {
	Dir  string
	Env  []string
	Args []string
}

func List(ctx context.Context, call Call) *exec.Cmd {
	return command(ctx, call, "list")
}

func Test(ctx context.Context, call Call) *exec.Cmd {
	return command(ctx, call, "test")
}

func command(ctx context.Context, call Call, verb string) *exec.Cmd {
	cmd := sysexec.Command(ctx, "go", append([]string{verb}, call.Args...)...)
	cmd.Dir = call.Dir
	cmd.Env = call.Env
	return cmd
}
