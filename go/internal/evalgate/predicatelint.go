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

type predicateLintFinding struct {
	text     string
	blocking bool
}

type predicateLintOutcome struct {
	files    []string
	findings []predicateLintFinding
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
	reason, blocking := g.findingsReason(cycle, outcome)
	return reason, blocking > 0
}

func (g predicateLintGate) findingsReason(cycle int, outcome predicateLintOutcome) (string, int) {
	findings := slices.SortedFunc(slices.Values(outcome.findings), func(a, b predicateLintFinding) int {
		if a.blocking != b.blocking {
			if a.blocking {
				return -1
			}
			return 1
		}
		return strings.Compare(a.text, b.text)
	})
	blocking := 0
	lines := make([]string, 0, len(findings))
	for _, f := range findings {
		if f.blocking {
			blocking++
		}
		lines = append(lines, f.text)
	}
	shown, suffix := lines, ""
	if len(lines) > predicateLintMaxReported {
		shown = lines[:predicateLintMaxReported]
		suffix = fmt.Sprintf(" (+%d more)", len(lines)-predicateLintMaxReported)
	}
	verdict := "ADVISORY: never blocks"
	if blocking > 0 {
		verdict = fmt.Sprintf("BLOCKING at enforce: %d of the %d finding(s), listed first", blocking, len(lines))
	}
	return fmt.Sprintf("%s authored for cycle%d: %d finding(s) across %d linted file(s)%s — %s. %s %s",
		g.headline, cycle, len(lines), len(outcome.files), suffix, strings.Join(shown, "; "), g.advice, verdict), blocking
}
