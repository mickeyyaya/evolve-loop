package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/mickeyyaya/evolveloop/go/internal/cyclehealth"
	"github.com/mickeyyaya/evolveloop/go/internal/policy"
)

// runCycleHealth implements `evolve cycle-health <N> <workspace>`.
// Exit codes:
//   - 0 healthy (no anomalies, or only warnings)
//   - 1 OverallFatal (at least one fatal anomaly — caller HALTs)
//   - 10 bad args
//   - 1 internal error
func runCycleHealth(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("cycle-health", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return 10
	}
	rest := fs.Args()
	if len(rest) < 2 {
		fmt.Fprintln(stderr, "evolve cycle-health: missing <cycle-N> <workspace>")
		return 10
	}
	cycle, err := strconv.Atoi(rest[0])
	if err != nil || cycle <= 0 {
		fmt.Fprintf(stderr, "evolve cycle-health: invalid cycle %q\n", rest[0])
		return 10
	}
	projectRoot := os.Getenv("EVOLVE_PROJECT_ROOT")
	if projectRoot == "" {
		projectRoot = filepath.Dir(filepath.Dir(filepath.Dir(rest[1])))
	}
	pol, err := policy.Load(filepath.Join(projectRoot, ".evolve", "policy.json"))
	if err != nil {
		fmt.Fprintf(stderr, "evolve cycle-health: %v\n", err)
		return 1
	}
	retryCfg := pol.RetryConfig()
	res, err := cyclehealth.Check(cyclehealth.Options{
		Cycle:                cycle,
		Workspace:            rest[1],
		NowFn:                time.Now,
		PhaseLatencyCeilingS: retryCfg.PhaseLatencyCeilingS,
	})
	if err != nil {
		fmt.Fprintf(stderr, "evolve cycle-health: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "[cycle-health] cycle=%d workspace=%s\n", res.Cycle, res.Workspace)
	for _, a := range res.Anomalies {
		fmt.Fprintf(stdout, "  [%s] %s: %s\n", a.Severity, a.Signal, a.Message)
	}
	if res.OverallFatal {
		fmt.Fprintln(stdout, "[cycle-health] verdict: HALT (fatal anomalies present)")
		return 1
	}
	fmt.Fprintf(stdout, "[cycle-health] verdict: OK (%d signal(s) ran, %d warn(s))\n",
		len(res.SignalsRun), len(res.Anomalies))
	return 0
}
