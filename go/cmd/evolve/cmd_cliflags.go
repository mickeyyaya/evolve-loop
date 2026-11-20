package main

import "github.com/mickeyyaya/evolve-loop/go/cmd/evolve/cmdutil"

type cliFlags struct {
	bools  map[string]*bool
	values map[string]*string
}

func (f cliFlags) parse(args []string) ([]string, error) {
	return cmdutil.Flags().Bools(f.bools).Values(f.values).Parse(args)
}
