package core

// repair_brief.go — what a tdd/build re-dispatch inside an audit-repair round
// is TOLD (research proposal R2, docs/research/ship-rate-harness-reliability-
// 2026-09-02.md §4).
//
// Before this file the brief was audit-fail-reason.json alone: the coherence
// floor's deterministic gate strings ("EGPS: ship_eligible=false", "apicover
// -enforce flagged 2 line(s)"). The auditor's actual findings — the HIGH
// entries in audit-report.md with root cause and path:line evidence — never
// reached the builder. Cycle 1605's H1 (a new exported package with zero
// production callers) survived three rounds while the round-2 builder rewrote
// one sentence of the explanation document; cycle 1596's round-4 builder
// received one truncated defect. The literature is unambiguous that repair
// without the specific finding does not converge (Olausson et al.: feedback
// quality is the bottleneck; Self-Debug: execution-grounded feedback beats
// explanation-only by an order of magnitude).
//
// The brief now carries, in this order and inside the existing byte budget:
//  1. the gate reasons (unchanged — deterministic evidence outranks prose);
//  2. the auditor's findings of the round that just rejected, CRITICAL/HIGH
//     first, MEDIUM after, LOW omitted — parsed by the SAME grammar the
//     dashboard renders (reportdoc.Findings), so the operator and the agent
//     read one list;
//  3. the findings that PERSISTED from the previous round (matched by
//     reportdoc.FindingKey against the archived audit-report.round<N-1>.md) —
//     the "you were told this already" line that turns a blind retry into a
//     targeted one.
//
// The previous round's prompt is archived beside the audit archives
// (build-prompt.round<N>.txt) so what each round was actually told is
// forensically recoverable — gap G10.

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/reportdoc"
)

// maxBriefFindings bounds how many auditor findings the brief carries; the
// byte budget (maxFindingsBytes) still applies after.
const maxBriefFindings = 8

// composeRepairBrief renders the repair brief for a cycle in an audit-repair
// round: gate reasons, then the rejecting round's auditor findings, then the
// persisted set. Empty when there is nothing at all to tell.
func composeRepairBrief(cs CycleState) string {
	var parts []string
	if gate := readContinuationFindings(filepath.Join(cs.WorkspacePath, "audit-fail-reason.json")); gate != "" {
		parts = append(parts, gate)
	}
	if findings := auditorFindingsBrief(cs.WorkspacePath, cs.AuditDispatches); findings != "" {
		parts = append(parts, findings)
	}
	if len(parts) == 0 {
		return ""
	}
	return truncateFindings(strings.Join(parts, "\n\n"))
}

// auditorFindingsBrief reads the live audit report (the round that just
// rejected) and the previous round's archive, and renders the findings the
// builder must act on. round is the audit dispatch count (the live report's
// round number); the previous archive is round-1.
func auditorFindingsBrief(workspace string, round int) string {
	reportName := phasecontract.ArtifactFilename(string(PhaseAudit))
	current, err := os.ReadFile(filepath.Join(workspace, reportName))
	if err != nil {
		// Absence is legitimate (the audit crashed before writing a report);
		// anything else is the "looked in the wrong place" class the gate reader
		// beside this one (readContinuationFindings) already reports — same posture.
		if !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN audit-repair: %s unreadable (%v) — builder gets no auditor findings\n", reportName, err)
		}
		return ""
	}
	findings := actionable(reportdoc.Findings(string(current)))
	if len(findings) == 0 {
		return ""
	}
	persisted := map[string]bool{}
	if round > 1 {
		if prev, err := os.ReadFile(filepath.Join(workspace, phasecontract.RoundArchiveFilename(reportName, round-1))); err == nil {
			for _, f := range reportdoc.Findings(string(prev)) {
				persisted[reportdoc.FindingKey(f.Title)] = true
			}
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "auditor findings (audit round %d — fix THESE; the gate reasons above are their symptoms):\n", round)
	for i, f := range findings {
		if i >= maxBriefFindings {
			fmt.Fprintf(&b, "- … %d more finding(s) in %s\n", len(findings)-i, reportName)
			break
		}
		label := f.Severity
		if f.ID != "" {
			label = f.ID + " (" + f.Severity + ")"
		}
		fmt.Fprintf(&b, "- %s — %s", label, f.Title)
		if persisted[reportdoc.FindingKey(f.Title)] {
			b.WriteString("  [PERSISTED from the previous round — your last repair did not address this]")
		}
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n")
}

// actionable orders findings CRITICAL, HIGH, MEDIUM and drops LOW (advisory
// on the ship gate; a builder's budget goes to what blocks the ship). The
// rank is reportdoc's (the dashboard sorts by the same function), so a
// severity added to the grammar reaches both readers or neither.
func actionable(fs []reportdoc.Finding) []reportdoc.Finding {
	low := reportdoc.SeverityRank("LOW")
	var out []reportdoc.Finding
	for _, f := range fs {
		if reportdoc.SeverityRank(f.Severity) < low {
			out = append(out, f)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return reportdoc.SeverityRank(out[i].Severity) < reportdoc.SeverityRank(out[j].Severity)
	})
	return out
}

// archiveRepairPrompts retires the previous attempt's prompt file for a
// tdd/build re-dispatch inside a repair round to <phase>-prompt.round<N>.txt —
// the same rule the audit archives use (phasecontract.RoundArchiveFilename) —
// so every round's brief stays recoverable. Self-guarding on the persisted
// repair state (the one predicate seedAuditRepairContext and repairRoundTier
// key on), so the live loop and the resume path call it identically. An
// existing archive is never clobbered (a counter rollback would otherwise
// erase the first attempt's evidence); a rename failure is reported, never
// fatal.
func archiveRepairPrompts(cs CycleState, phase Phase) {
	workspace, round := cs.WorkspacePath, cs.AuditRepairAttempts
	if !cs.AuditRepairActive || !repairSeededPhase(phase) || workspace == "" || round < 1 {
		return
	}
	// The bridge names the prompt file after the PHASE (runner.go sets Agent
	// to the phase name; the filename rule is phasecontract's), so there is
	// exactly one file to retire.
	name := phasecontract.PromptArtifactFilename(string(phase))
	src := filepath.Join(workspace, name)
	if _, err := os.Stat(src); err != nil {
		return
	}
	dst := filepath.Join(workspace, phasecontract.RoundArchiveFilename(name, round))
	if _, err := os.Stat(dst); err == nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN audit-repair: %s already exists; %s left in place (round %d prompt not archived)\n", filepath.Base(dst), name, round)
		return
	}
	if err := os.Rename(src, dst); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN audit-repair: could not archive %s as %s: %v\n", name, filepath.Base(dst), err)
	}
}
