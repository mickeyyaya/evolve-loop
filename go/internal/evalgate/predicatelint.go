package evalgate

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/acssuite"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

const predicateLintMaxReported = 5

func cyclePredicateDir(worktree string, cycle int) string {
	return filepath.Join(worktree, "go", filepath.FromSlash(acssuite.CyclePackage(cycle)))
}

type predicateLintOutcome struct {
	files    []string
	findings []string
}

type predicateLintGate struct {
	gateName string
	label    string
	subject  string
	headline string
	advice   string
	lint     func(dir string) (predicateLintOutcome, error)
}

func (g predicateLintGate) name() string { return g.gateName }

func (predicateLintGate) appliesTo(phase string) bool { return phase == string(core.PhaseTDD) }

func (g predicateLintGate) check(in core.ReviewInput) (string, bool) {
	cycle := cycleNumFromWorkspace(in.Workspace)
	if cycle <= 0 || in.Worktree == "" {
		return fmt.Sprintf(
			"%s stood down: cannot locate this cycle's predicates (cycle=%d from workspace %q, worktree=%q) — NO %s was inspected. ADVISORY: never blocks",
			g.label, cycle, in.Workspace, in.Worktree, g.subject), false
	}
	dir := cyclePredicateDir(in.Worktree, cycle)
	outcome, err := g.lint(dir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return fmt.Sprintf("%s: cycle%d has no Go ACS predicate package (%s absent) — nothing to lint. ADVISORY: never blocks", g.label, cycle, dir), false
	case err != nil:
		return fmt.Sprintf("%s stood down for cycle%d predicates: %v — NO %s was inspected. ADVISORY: never blocks", g.label, cycle, err, g.subject), false
	case len(outcome.findings) == 0:
		return fmt.Sprintf("%s: cycle%d predicates CLEAN — linted %d file(s) (%s), 0 findings. ADVISORY: never blocks",
			g.label, cycle, len(outcome.files), strings.Join(outcome.files, ", ")), false
	}
	return g.findingsReason(cycle, outcome), false
}

func (g predicateLintGate) findingsReason(cycle int, outcome predicateLintOutcome) string {
	lines := slices.Sorted(slices.Values(outcome.findings))
	shown, suffix := lines, ""
	if len(lines) > predicateLintMaxReported {
		shown = lines[:predicateLintMaxReported]
		suffix = fmt.Sprintf(" (+%d more)", len(lines)-predicateLintMaxReported)
	}
	return fmt.Sprintf("%s authored for cycle%d: %d finding(s) across %d linted file(s)%s — %s. %s ADVISORY: never blocks",
		g.headline, cycle, len(lines), len(outcome.files), suffix, strings.Join(shown, "; "), g.advice)
}
