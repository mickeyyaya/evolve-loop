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
	switch args[0] {
	case "generate":
		return skillsRun(sourceRoot(), true, stdout, stderr)
	case "check":
		return skillsRun(sourceRoot(), false, stdout, stderr)
	case "publish":
		cfg, ok := parsePublishFlags(args[1:], stderr)
		if !ok {
			return 10
		}
		return runSkillsPublish(envOrCwd("EVOLVE_PROJECT_ROOT"), cfg, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown subcommand %q (want generate|check|publish)\n", args[0])
		return 10
	}
}

func skillsRun(project string, write bool, stdout, stderr io.Writer) int {
	return skillcheck.Run(project, write, stdout, stderr)
}
