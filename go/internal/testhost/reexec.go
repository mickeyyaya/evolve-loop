package testhost

import (
	"context"
	"flag"
	"os"
	"os/exec"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const coverDirFlag = "test.gocoverdir"

func HelperArgs(run string) []string {
	return helperArgs(run, flag.Lookup)
}

func Command(run string, env ...string) *exec.Cmd {
	cmd := sysexec.Command(context.Background(), os.Args[0], HelperArgs(run)...)
	cmd.Env = append(os.Environ(), env...)
	return cmd
}

func helperArgs(run string, lookup func(string) *flag.Flag) []string {
	args := []string{"-test.run=" + run}
	if f := lookup(coverDirFlag); f != nil && f.Value.String() != "" {
		args = append(args, "-"+coverDirFlag+"="+f.Value.String())
	}
	return args
}
