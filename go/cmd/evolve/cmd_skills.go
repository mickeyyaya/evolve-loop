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

	"github.com/mickeyyaya/evolveloop/go/internal/skillcheck"
)

func runSkills(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: evolve skills <generate|check|publish>")
		return 10
	}
	switch args[0] {
	case "generate":
		// SKILL.md is a generated SOURCE doc (ADR-0040 projection), part of a
		// cycle's committed deliverable — resolve it from the worktree under the
		// ACS suite, not main's stale copy (cycle-355 fix; see sourceRoot).
		return skillsRun(sourceRoot(), true, stdout, stderr)
	case "check":
		return skillsRun(sourceRoot(), false, stdout, stderr)
	case "publish":
		cfg, ok := parsePublishFlags(args[1:], stderr)
		if !ok {
			return 10
		}
		// publish is a main/release operation: it reads the shipped skills and
		// stages them into `.evolve/publish/` STATE. Both belong on the STATE
		// root, never a worktree — resolve via EVOLVE_PROJECT_ROOT explicitly so
		// it cannot inherit a worktree root from EVOLVE_WORKTREE_ROOT.
		return runSkillsPublish(envOrCwd("EVOLVE_PROJECT_ROOT"), cfg, stdout, stderr)
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
