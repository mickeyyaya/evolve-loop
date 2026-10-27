package main

import (
	"flag"
	"fmt"
	"io"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

func runWaveStop(env waveEnv, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve wave stop", flag.ContinueOnError)
	fs.SetOutput(stderr)
	projectRoot := fs.String("project-root", "", "project root (default: $EVOLVE_PROJECT_ROOT or cwd)")
	asJSON := fs.Bool("json", false, "print the result as JSON and no text")
	wait := fs.Duration("wait", 0, "after the brake, block until no run lease is live, for this long at most (exit 1 at the end; the brake stays)")
	if err := fs.Parse(args); err != nil {
		return waveUsageError(stderr, err)
	}
	if fs.NArg() > 0 || *wait < 0 {
		return waveUsageError(stderr, fmt.Errorf("wave stop takes no operand and a --wait of 0 or more, got %q and %s", fs.Args(), *wait))
	}
	plane, err := resolveWavePlane(env, *projectRoot, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "%s%v\n", wavePrefix, err)
		return cliExitIO
	}
	human := humanWriter(stdout, *asJSON)
	result := loopStopResult{Brake: paths.LoopStopPath(plane.evolveDir), Action: "engaged"}
	rc := engageLoopStop(result.Brake, env.now(), human, stderr)
	if rc == cliExitOK && *wait > 0 {
		rc = waitForIdleLoop(filepath.Join(plane.evolveDir, "runs"), *wait, loopStopWaitPoll, env.now, env.sleep, human, stderr)
		result.Idle = rc == cliExitOK
	}
	if *asJSON && rc == cliExitOK {
		return printVerbJSON("wave stop", result, stdout, stderr)
	}
	return rc
}
