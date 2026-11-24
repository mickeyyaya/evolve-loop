package testhost

import (
	"flag"
	"os"
	"os/exec"
)

const coverDirFlag = "test.gocoverdir"

func HelperArgs(run string) []string {
	return helperArgs(run, flag.Lookup)
}

func Command(run string, env ...string) *exec.Cmd {
	cmd := exec.Command(os.Args[0], HelperArgs(run)...)
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
