package evalgate

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/evalqualitycheck"
)

// flakyShapeGate (Gate D) lints this cycle's ACS predicates at the end of tdd,
// the first point they exist. block is always false: a flaky shape is a smell, not proof.
type flakyShapeGate struct{}

func (flakyShapeGate) name() string                { return "flaky-predicate-shape" }
func (flakyShapeGate) appliesTo(phase string) bool { return phase == string(core.PhaseTDD) }

// flakyShapeMaxReported bounds the reason line; the total is always stated, so
// truncation stays visible.
const flakyShapeMaxReported = 5

// check returns a non-empty reason on every path, so a clean cycle's log reads
// differently from a gate that never ran.
func (flakyShapeGate) check(in core.ReviewInput) (string, bool) {
	cycle := cycleNumFromWorkspace(in.Workspace)
	if cycle <= 0 || in.Worktree == "" {
		return fmt.Sprintf(
			"flaky-shape lint stood down: cannot locate this cycle's predicates (cycle=%d from workspace %q, worktree=%q) — NO predicate shape was inspected. ADVISORY: never blocks",
			cycle, in.Workspace, in.Worktree), false
	}
	dir := filepath.Join(in.Worktree, "go", "acs", fmt.Sprintf("cycle%d", cycle))
	report, err := evalqualitycheck.LintFlakyPredicates(dir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return fmt.Sprintf("flaky-shape lint: cycle%d has no Go ACS predicate package (%s absent) — nothing to lint. ADVISORY: never blocks", cycle, dir), false
	case err != nil:
		return fmt.Sprintf("flaky-shape lint stood down for cycle%d predicates: %v — NO predicate shape was inspected. ADVISORY: never blocks", cycle, err), false
	case len(report.Findings) == 0:
		return fmt.Sprintf("flaky-shape lint: cycle%d predicates CLEAN — linted %d file(s) (%s), 0 findings. ADVISORY: never blocks",
			cycle, report.Linted(), strings.Join(report.Files, ", ")), false
	}
	return flakyShapeReason(cycle, report), false
}

// flakyShapeReason states the linted-file count and the finding total, then up to
// flakyShapeMaxReported sorted findings.
func flakyShapeReason(cycle int, report evalqualitycheck.FlakyLintReport) string {
	lines := make([]string, 0, len(report.Findings))
	for _, f := range report.Findings {
		lines = append(lines, fmt.Sprintf("%s:%s [%s] %s", f.File, f.Func, f.Class, f.Reason))
	}
	sort.Strings(lines)
	shown := lines
	suffix := ""
	if len(shown) > flakyShapeMaxReported {
		shown = shown[:flakyShapeMaxReported]
		suffix = fmt.Sprintf(" (+%d more)", len(lines)-flakyShapeMaxReported)
	}
	return fmt.Sprintf(
		"flaky-shaped predicate(s) authored for cycle%d: %d finding(s) across %d linted file(s)%s — %s. These shapes flake under fleet load (Luo FSE'14 async-wait/concurrency classes); rewrite before they enter the ACS corpus. ADVISORY: never blocks",
		cycle, len(report.Findings), report.Linted(), suffix, strings.Join(shown, "; "))
}
