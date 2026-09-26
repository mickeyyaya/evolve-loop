package phasecmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/cmd/evolve/cmdutil"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// runPhaseLint implements `evolve phase lint <name>`. It is fail-open by contract:
// every finding is a warning and only a missing name exits non-zero.
// See ADR-0035.
func runPhaseLint(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
		fmt.Fprintln(stderr, "usage: evolve phase lint <name>")
		return 10
	}
	name := strings.ToLower(strings.TrimSpace(args[0]))
	project := cmdutil.EnvOrCwd("EVOLVE_PROJECT_ROOT")

	user, _, discWarns := phasespec.DiscoverUserSpecsFromRoots(phasespec.Roots(project))
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
		fmt.Fprintf(stdout, "WARN: no user phase named %q in any phase root of %s\n", name, project)
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
	return 0
}

func lintSpec(s phasespec.PhaseSpec) []string {
	warnings := append([]string(nil), phasespec.ValidateUserSpec(s)...)
	return append(warnings, softLintWarnings(s)...)
}

func kindLabel(k phasecontract.Kind) string {
	if k == phasecontract.KindJSON {
		return "json"
	}
	return "markdown"
}
