// cmd_skills.go implements `evolve skills` — the CLI front for the ADR-0040
// projection (generate|check|publish). The projection + drift logic lives in
// internal/skillcheck so the autonomous cycle's audit phase can run the SAME
// drift check in-process without importing package main; this file only routes
// the subcommands and keeps the thin skillsRun seam the producer-side drift
// tests (cmd_skills_drift_test.go) call.
package main

import (
	"fmt"
	"io"

	"github.com/mickeyyaya/evolve-loop/go/internal/skillcheck"
)

func runSkills(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: evolve skills <generate|check|publish>")
		return 10
	}
	project := envOrCwd("EVOLVE_PROJECT_ROOT")
	switch args[0] {
	case "generate":
		return skillsRun(project, true, stdout, stderr)
	case "check":
		return skillsRun(project, false, stdout, stderr)
	case "publish":
		cfg, ok := parsePublishFlags(args[1:], stderr)
		if !ok {
			return 10
		}
		return runSkillsPublish(project, cfg, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown subcommand %q (want generate|check|publish)\n", args[0])
		return 10
	}
}

// skillsRun delegates to internal/skillcheck.Run — the single home for the
// projection. Kept as a thin seam so the producer-side drift tests call the
// exact CLI path the audit gate now shares.
func skillsRun(project string, write bool, stdout, stderr io.Writer) int {
	return skillcheck.Run(project, write, stdout, stderr)
}
