package cmdutil

import (
	"slices"

	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

func TakeFlagValue(args []string, name string) (string, []string, bool, bool) {
	i := slices.Index(args, name)
	if i < 0 {
		return "", args, false, true
	}
	if i+1 >= len(args) {
		return "", nil, true, false
	}
	return args[i+1], slices.Concat(args[:i], args[i+2:]), true, true
}

func TakeProjectRoot(args []string) (string, []string, bool) {
	root, rest, found, ok := TakeFlagValue(args, "--project-root")
	if !found {
		return EnvOrCwd("EVOLVE_PROJECT_ROOT"), args, true
	}
	return paths.AbsoluteRoot("--project-root", root, nil), rest, ok
}
