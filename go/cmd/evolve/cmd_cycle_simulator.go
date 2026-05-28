package main

import (
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclesimulator"
)

// runCycleSimulator is the `evolve cycle-simulator <cycle> <workspace>` subcommand.
// Ports legacy/scripts/dispatch/cycle-simulator.sh.
func runCycleSimulator(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	var pos []string
	for _, a := range args {
		switch a {
		case "--help", "-h":
			fmt.Fprintln(stdout, "Usage: evolve cycle-simulator <cycle> <workspace>")
			fmt.Fprintln(stdout, "Env: EVOLVE_PROJECT_ROOT, EVOLVE_PLUGIN_ROOT")
			return 0
		default:
			pos = append(pos, a)
		}
	}
	if len(pos) < 2 {
		fmt.Fprintln(stderr, "[simulator] usage: cycle-simulator <cycle> <workspace>")
		return cyclesimulator.ExitRuntimeErr
	}
	cycle, err := strconv.Atoi(pos[0])
	if err != nil {
		fmt.Fprintf(stderr, "[simulator] cycle must be integer, got: %s\n", pos[0])
		return cyclesimulator.ExitRuntimeErr
	}
	// envOrCwd absolutizes a relative $EVOLVE_PROJECT_ROOT (cycle-119 class).
	projectRoot := envOrCwd("EVOLVE_PROJECT_ROOT")
	pluginRoot := os.Getenv("EVOLVE_PLUGIN_ROOT")
	if pluginRoot == "" {
		pluginRoot = projectRoot
	}
	return cyclesimulator.Run(cyclesimulator.Inputs{
		Cycle:       cycle,
		Workspace:   pos[1],
		ProjectRoot: projectRoot,
		PluginRoot:  pluginRoot,
	}, stderr)
}
