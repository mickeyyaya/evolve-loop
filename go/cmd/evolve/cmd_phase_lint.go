package main

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// runPhaseLint implements `evolve phase lint <name>` — a developer aid that
// checks an operator-authored phase descriptor against the unified phase
// descriptor (ADR-0035) and reports what the runtime WILL derive from it. It
// reuses the exact runtime path (DiscoverUserSpecs → ValidateUserSpec →
// FromSpec) so the lint reflects production behavior, not a parallel schema.
//
// It is FAIL-OPEN by contract: every finding is a warning and the command
// always exits 0 (except a usage error: missing name → 10). Linting must never
// block a developer — the runtime gates (ValidateUserSpec floor, contract gate)
// are where enforcement lives.
func runPhaseLint(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
		fmt.Fprintln(stderr, "usage: evolve phase lint <name>")
		return 10
	}
	name := strings.ToLower(strings.TrimSpace(args[0]))
	project := envOrCwd("EVOLVE_PROJECT_ROOT")

	user, discWarns := phasespec.DiscoverUserSpecs(filepath.Join(project, ".evolve", "phases"))
	for _, w := range discWarns {
		fmt.Fprintln(stdout, "WARN:", w)
	}

	var spec phasespec.PhaseSpec
	found := false
	for _, s := range user {
		if s.Name == name {
			spec, found = s, true
			break
		}
	}
	if !found {
		// Fail-open: a missing phase is a warning, not an error.
		fmt.Fprintf(stdout, "WARN: no user phase named %q under %s/.evolve/phases/\n", name, project)
		return 0
	}

	warnings := lintSpec(spec)
	if len(warnings) == 0 {
		c := phasecontract.FromSpec(spec)
		fmt.Fprintf(stdout, "OK    %s — derives %s (%s), %d required section(s)\n",
			name, c.ArtifactName, kindLabel(c.Kind), len(c.Sections))
		return 0
	}
	fmt.Fprintf(stdout, "WARN  %s — %d issue(s):\n", name, len(warnings))
	for _, w := range warnings {
		fmt.Fprintf(stdout, "        - %s\n", w)
	}
	return 0 // fail-open
}

// lintSpec collects descriptor warnings: the hard ValidateUserSpec floor plus
// soft best-practice checks (an evaluate phase with no required sections produces
// a contract that verifies nothing; an undefined artifact name).
func lintSpec(s phasespec.PhaseSpec) []string {
	warnings := append([]string(nil), phasespec.ValidateUserSpec(s)...)

	if s.RoleOrDefault() == phasespec.RoleEvaluate {
		if s.Classify == nil || len(s.Classify.RequireSections) == 0 {
			warnings = append(warnings, "evaluate phase declares no classify.require_sections — its derived contract checks no structure")
		}
	}
	if len(s.Outputs.Files) == 0 {
		warnings = append(warnings, fmt.Sprintf("no outputs.files — the deliverable will default to %q", s.Name+"-report.md"))
	}
	return warnings
}

func kindLabel(k phasecontract.Kind) string {
	if k == phasecontract.KindJSON {
		return "json"
	}
	return "markdown"
}
