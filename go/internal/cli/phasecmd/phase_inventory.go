package phasecmd

import (
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/mickeyyaya/evolveloop/go/cmd/evolve/cmdutil"
	"github.com/mickeyyaya/evolveloop/go/internal/phasespec"

	"github.com/mickeyyaya/evolveloop/go/internal/phaseinventory"
)

// runPhaseInventory implements `evolve phase-inventory <subcommand>` — the
// phase counterpart of skill-inventory (ADR-0038). Exit codes:
//   - 0  success (cache hit or fresh build)
//   - 10 bad args / unknown subcommand
//   - 1  internal error
func RunPhaseInventory(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(stderr, "evolve phase-inventory: missing subcommand (build)")
		return 10
	}
	switch args[0] {
	case "build":
		return runPhaseInventoryBuild(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "evolve phase-inventory: unknown subcommand %q\n", args[0])
		return 10
	}
}

func runPhaseInventoryBuild(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("phase-inventory build", flag.ContinueOnError)
	fs.SetOutput(stderr)
	projectRoot := fs.String("project-root", "", "project root (default EVOLVE_PROJECT_ROOT or cwd)")
	ttl := fs.Duration("ttl", phaseinventory.DefaultTTL, "cache freshness window (e.g., 1h)")
	force := fs.Bool("force", false, "rebuild even when cache is fresh")
	if err := fs.Parse(args); err != nil {
		return 10
	}
	root := *projectRoot
	if root == "" {
		root = cmdutil.EnvOrCwd("EVOLVE_PROJECT_ROOT")
	}
	res, err := phaseinventory.Build(phaseinventory.Options{
		ProjectRoot: root,
		Roots:       phasespec.Roots(root),
		TTL:         *ttl,
		NowFn:       time.Now,
		Force:       *force,
	})
	if err != nil {
		fmt.Fprintf(stderr, "evolve phase-inventory build: %v\n", err)
		return 1
	}
	for _, w := range res.Warnings {
		fmt.Fprintln(stdout, "WARN:", w)
	}
	if res.CacheHit {
		fmt.Fprintf(stdout, "[phase-inventory] cache hit: %s\n", res.OutputPath)
		return 0
	}
	fmt.Fprintf(stdout, "[phase-inventory] built %d phases → %s\n", res.PhaseCount, res.OutputPath)
	return 0
}
