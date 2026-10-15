package opscmd

import (
	"errors"
	"fmt"
	"github.com/mickeyyaya/evolve-loop/go/cmd/evolve/cmdutil"
	"io"

	"github.com/mickeyyaya/evolve-loop/go/internal/releaseconsistency"
)

var releaseConsistencyRun = releaseconsistency.Run

func RunReleaseConsistency(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	var target string
	for _, a := range args {
		switch {
		case a == "--help" || a == "-h":
			fmt.Fprintln(stdout, "Usage: evolve release-consistency [target-version]")
			fmt.Fprintln(stdout, "Verifies plugin.json + marketplace.json + SKILL.md + README.md + CHANGELOG.md")
			fmt.Fprintln(stdout, "all match the target version (or current plugin.json version if omitted).")
			return exitOK
		case len(a) >= 2 && a[:2] == "--":
			fmt.Fprintf(stderr, "[release-consistency] unknown flag: %s\n", a)
			return exitFail
		default:
			if target == "" {
				target = a
			} else {
				fmt.Fprintf(stderr, "[release-consistency] extra positional arg: %s\n", a)
				return exitFail
			}
		}
	}
	projectRoot := cmdutil.EnvOrCwd("EVOLVE_PROJECT_ROOT")
	_, err := releaseConsistencyRun(releaseconsistency.Options{
		ProjectRoot: projectRoot,
		Target:      target,
		Stderr:      stderr,
	})
	if err == nil {
		return exitOK
	}
	if errors.Is(err, releaseconsistency.ErrInconsistent) {
		return exitFail
	}
	fmt.Fprintf(stderr, "[release-consistency] FAIL: %v\n", err)
	return exitFail
}
