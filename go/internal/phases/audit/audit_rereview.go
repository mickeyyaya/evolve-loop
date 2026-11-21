package audit

import (
	"context"
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/convergence"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/reportdoc"
)

const roundScopeRules = "\n## Round scope (host-computed, audit round %d)\n\n" +
	"This is a repair round. Review the previous findings and the hunks of the repair. Do not re-hunt the whole tree.\n\n" +
	"Previous findings, each with its host id, severity, location and title:\n%s\n" +
	"Repair hunks, as post-repair lines of each file:\n%s\n" +
	"- **Dispositions:** write `audit-round-dispositions.json` in the workspace. Give one entry for each previous host id, in this shape: " +
	"`{\"dispositions\": [{\"id\": \"<host id>\", \"status\": \"FIXED\", \"evidence\": \"path:line\"}]}`. " +
	"Copy each host id unchanged. A previous finding with no FIXED entry and resolvable evidence stays open. DEFERRED counts as open.\n" +
	"- **Late findings:** a new finding on code outside the repair hunks, below %[4]s, goes under `## Filed (late)` and does not decide the verdict. " +
	"Each entry names `path:line`. A new finding at %[4]s or above, outside the hunks, goes under `## Issues` and blocks.\n" +
	"- **Inside the hunks:** every finding blocks by the normal rules.\n"

func roundScopePromptBlock(req core.PhaseRequest) string {
	scope := req.AuditScope
	if scope == nil || !scope.Enforce {
		return ""
	}
	return fmt.Sprintf(roundScopeRules, scope.Round, previousLines(scope), hunkLines(scope), convergence.LateFileBar(convergence.LoopAuditRepair))
}

func previousLines(scope *core.AuditRoundScope) string {
	if len(scope.Previous) == 0 {
		return "- none\n"
	}
	var b strings.Builder
	for _, p := range scope.Previous {
		fmt.Fprintf(&b, "- `%s` — %s — %s — %s\n", p.HostID, p.Severity, p.Location, p.Title)
	}
	return b.String()
}

func hunkLines(scope *core.AuditRoundScope) string {
	if len(scope.Hunks) == 0 {
		return "- none: the repair changed no line\n"
	}
	var b strings.Builder
	for _, h := range scope.Hunks {
		fmt.Fprintf(&b, "- %s:%d-%d\n", h.File, h.From, h.To)
	}
	return b.String()
}

func defaultAuditTree(req core.PhaseRequest) string {
	return core.AuditWorktreeTree(context.Background(), req)
}

func (a *auditClassification) checkLateFiling() {
	scope := a.req.AuditScope
	if scope == nil || !scope.Enforce {
		return
	}
	entries := reportdoc.FiledLate(a.artifact)
	if len(entries) == 0 {
		return
	}
	if a.hooks.auditTree != nil && a.hooks.auditTree(a.req) != scope.Tree {
		a.fail("audit re-review tree", "audit re-review: the worktree tree changed after the round scope was computed, so the Filed (late) entries cannot be checked against the repair hunks")
		return
	}
	if problems := scope.CheckFiled(entries); len(problems) > 0 {
		a.fail("audit re-review filing", "audit re-review: a Filed (late) entry breaks the filing rule (ADR-0168): "+strings.Join(problems, "; "))
	}
}
