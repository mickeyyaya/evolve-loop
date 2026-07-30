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

// flakyshape.go — Gate D (acs-metapredicate-suite-scope): THE production caller
// of the authoring-time flaky-shape lint.
//
// Why here and nowhere else. The lint reads go/acs/cycle<N>/predicates_test.go,
// which does not exist until the tdd phase has authored it — so the discover-time
// `evolve eval quality-check` invocation in skills/loop/phase2-discover.md can
// never see a predicate source, and a seam wired only into a CLI --help string is
// dead code. Gate D mounts on the SAME per-phase DeliverableReviewer seam as
// Gate B/C (core.WithReviewer, composed in NewReviewer, dispatched by the
// orchestrator at cyclerun_review.go), fires at the END of the tdd phase, and
// therefore inspects the predicates in the same transition the four-layer
// predicate-quality defense already owns (docs/architecture/
// acs-predicate-quality-gate.md) — one phase BEFORE the build tokens that a
// flaky predicate would later waste, and before acssuite's run-time scope-lint
// has to demote anything.
//
// ADVISORY, structurally. check returns block=false as a CONSTANT, not as a
// computed conjunct: a flaky SHAPE is a strong smell, not proof that the
// predicate is wrong, and calibration against the historical corpus (64 of 282
// acs dirs flagged, 2026-07-30) is not yet broken down per class. So a finding is
// surfaced by reviewer.Review's log line and never rejects a deliverable at any
// stage. That makes "this gate cannot fail a cycle" checkable by reading one
// line, which is the property the promotion path (advisory → enforce after a
// per-class false-positive breakdown) will need to deliberately revoke.
//
// KNOWN LIMIT on the surface, not the rule: reviewer.Review writes the reason to
// stderr (the phase log) only. Nothing persists it to an artifact, cycle state,
// or the Auditor's handoff, so the findings are operator-visible but not yet
// agent-consumable — and the per-class audit the promotion path needs cannot be
// fed from live runs until they are. Queued, deliberately out of scope here:
// wiring an advisory-finding channel is a reviewer-seam change, not a lint change.
type flakyShapeGate struct{}

func (flakyShapeGate) name() string                { return "flaky-predicate-shape" }
func (flakyShapeGate) appliesTo(phase string) bool { return phase == string(core.PhaseTDD) }

// flakyShapeMaxReported bounds the reason line: a predicate file with dozens of
// findings must not produce an unreadable multi-KB log entry. The total count is
// always stated, so truncation is visible rather than silent.
const flakyShapeMaxReported = 5

// check ALWAYS returns a non-empty reason, so reviewer.Review logs exactly one
// Gate D line per tdd phase. That is deliberate and it is the H2 fix applied to
// the PRODUCTION path, not just the CLI: with silence reserved for "clean", a
// live cycle log could not distinguish "Gate D ran and the predicates are clean"
// from "Gate D silently no-opped" — which is the H1 dead-code failure mode
// wearing a different hat. Every outcome, including both stand-down branches,
// says what was inspected. block is a constant false, so the extra line costs a
// log entry and nothing else.
func (flakyShapeGate) check(in core.ReviewInput) (string, bool) {
	cycle := cycleNumFromWorkspace(in.Workspace)
	if cycle <= 0 || in.Worktree == "" {
		// Should be unreachable in production: the orchestrator's WorkspacePath is
		// always .evolve/runs/cycle-<N> and the worktree is provisioned at cycle
		// start for every phase. If it DOES happen (a provisioning failure), the
		// gate is inert and the operator must be told, not left with silence.
		return fmt.Sprintf(
			"flaky-shape lint stood down: cannot locate this cycle's predicates (cycle=%d from workspace %q, worktree=%q) — NO predicate shape was inspected. ADVISORY: never blocks",
			cycle, in.Workspace, in.Worktree), false
	}
	dir := filepath.Join(in.Worktree, "go", "acs", fmt.Sprintf("cycle%d", cycle))
	report, err := evalqualitycheck.LintFlakyPredicates(dir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		// Legitimate and common: this cycle authored no Go ACs. Still stated, so
		// "no predicates" reads differently from "gate never ran".
		return fmt.Sprintf("flaky-shape lint: cycle%d has no Go ACS predicate package (%s absent) — nothing to lint. ADVISORY: never blocks", cycle, dir), false
	case err != nil:
		// Unparseable source, or a dir holding no .go files.
		return fmt.Sprintf("flaky-shape lint stood down for cycle%d predicates: %v — NO predicate shape was inspected. ADVISORY: never blocks", cycle, err), false
	case len(report.Findings) == 0:
		return fmt.Sprintf("flaky-shape lint: cycle%d predicates CLEAN — linted %d file(s) (%s), 0 findings. ADVISORY: never blocks",
			cycle, report.Linted(), strings.Join(report.Files, ", ")), false
	}
	return flakyShapeReason(cycle, report), false
}

// flakyShapeReason renders the advisory line: the receipt (how many files were
// actually linted — a finding count with no file count is the silent-clean shape
// this gate exists to avoid), the total, and up to flakyShapeMaxReported
// deduplicated "func [class] reason" entries in deterministic order.
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
