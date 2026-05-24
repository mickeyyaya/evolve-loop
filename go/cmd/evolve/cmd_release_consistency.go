package main

import (
	"errors"
	"fmt"
	"io"

	"github.com/mickeyyaya/evolve-loop/go/internal/releaseconsistency"
)

// runReleaseConsistency is `evolve release-consistency [target]`.
// Mirrors the version-marker-consistency half of legacy/scripts/utility/release.sh.
//
// Exit codes:
//
//	0 = all markers consistent
//	1 = at least one inconsistency
func runReleaseConsistency(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	var target string
	for _, a := range args {
		switch {
		case a == "--help" || a == "-h":
			fmt.Fprintln(stdout, "Usage: evolve release-consistency [target-version]")
			fmt.Fprintln(stdout, "Verifies plugin.json + marketplace.json + SKILL.md + README.md + CHANGELOG.md")
			fmt.Fprintln(stdout, "all match the target version (or current plugin.json version if omitted).")
			return 0
		case len(a) >= 2 && a[:2] == "--":
			fmt.Fprintf(stderr, "[release-consistency] unknown flag: %s\n", a)
			return 1
		default:
			if target == "" {
				target = a
			} else {
				fmt.Fprintf(stderr, "[release-consistency] extra positional arg: %s\n", a)
				return 1
			}
		}
	}
	projectRoot := envOrCwd("EVOLVE_PROJECT_ROOT")
	_, err := releaseconsistency.Run(releaseconsistency.Options{
		ProjectRoot: projectRoot,
		Target:      target,
		Stderr:      stderr,
	})
	if err == nil {
		return 0
	}
	if errors.Is(err, releaseconsistency.ErrInconsistent) {
		return 1
	}
	fmt.Fprintf(stderr, "[release-consistency] FAIL: %v\n", err)
	return 1
}
