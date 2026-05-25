package main

import (
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/skillinventory"
)

// runSkillInventory implements `evolve skill-inventory <subcommand>`.
// Currently supports the `build` subcommand. Exit codes:
//   - 0  success (cache hit or fresh build)
//   - 10 bad args / unknown subcommand
//   - 1  internal error
func runSkillInventory(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(stderr, "evolve skill-inventory: missing subcommand (build)")
		return 10
	}
	switch args[0] {
	case "build":
		return runSkillInventoryBuild(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "evolve skill-inventory: unknown subcommand %q\n", args[0])
		return 10
	}
}

func runSkillInventoryBuild(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("skill-inventory build", flag.ContinueOnError)
	fs.SetOutput(stderr)
	projectRoot := fs.String("project-root", ".", "project root containing skills/")
	ttl := fs.Duration("ttl", skillinventory.DefaultTTL, "cache freshness window (e.g., 1h)")
	force := fs.Bool("force", false, "rebuild even when cache is fresh")
	if err := fs.Parse(args); err != nil {
		return 10
	}
	res, err := skillinventory.Build(skillinventory.Options{
		ProjectRoot: *projectRoot,
		TTL:         *ttl,
		NowFn:       time.Now,
		Force:       *force,
	})
	if err != nil {
		fmt.Fprintf(stderr, "evolve skill-inventory build: %v\n", err)
		return 1
	}
	if res.CacheHit {
		fmt.Fprintf(stdout, "[skill-inventory] cache hit: %s\n", res.OutputPath)
		return 0
	}
	fmt.Fprintf(stdout, "[skill-inventory] built %d skills → %s\n", res.SkillCount, res.OutputPath)
	return 0
}
