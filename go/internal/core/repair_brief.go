package core

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/reportdoc"
)

// maxBriefFindings bounds how many auditor findings the brief carries; the
// byte budget (maxFindingsBytes) still applies after.
const maxBriefFindings = 8

// composeRepairBrief renders the repair brief for a cycle in an audit-repair
// round: gate reasons (absent a gate record, the audit's failure-block defects),
// then the rejecting round's auditor findings. Empty when there is nothing to tell.
func composeRepairBrief(cs CycleState) string {
	findings := actionableAuditFindings(cs.WorkspacePath)
	var parts []string
	if reasons := auditRejectionReasons(cs.WorkspacePath, findings[:min(len(findings), maxBriefFindings)]); reasons != "" {
		parts = append(parts, reasons)
	}
	if brief := renderAuditorFindings(cs.WorkspacePath, cs.AuditDispatches, findings); brief != "" {
		parts = append(parts, brief)
	}
	if len(parts) == 0 {
		return ""
	}
	return truncateFindings(strings.Join(parts, "\n\n"))
}

func auditRejectionReasons(workspace string, briefed []reportdoc.Finding) string {
	record := floorFailReasonPath(workspace, PhaseAudit)
	if _, err := os.Stat(record); !os.IsNotExist(err) {
		return readContinuationFindings(record)
	}
	fb, ok := phasecontract.ReadFailureBlock(workspace, string(PhaseAudit))
	if !ok {
		return ""
	}
	var lines []string
	for _, defect := range fb.Defects {
		if defect = strings.TrimSpace(defect); defect != "" && !restatesAFinding(defect, briefed) {
			lines = append(lines, "- "+defect)
		}
	}
	if len(lines) == 0 {
		return ""
	}
	return "audit defects (the verdict's failure block, class " + fb.Class + "):\n" + strings.Join(lines, "\n")
}

func restatesAFinding(defect string, briefed []reportdoc.Finding) bool {
	key := alphanumericKey(defect)
	for _, f := range briefed {
		for _, form := range []string{f.Title, f.ID + f.Title, f.ID + f.Severity + f.Title} {
			if key == alphanumericKey(form) {
				return true
			}
		}
	}
	return false
}

func alphanumericKey(s string) string {
	return strings.Map(func(r rune) rune {
		r = unicode.ToLower(r)
		if 'a' <= r && r <= 'z' || '0' <= r && r <= '9' {
			return r
		}
		return -1
	}, s)
}

// auditorFindingsBrief reads the live audit report (the round that just
// rejected) and the previous round's archive, and renders the findings the
// builder must act on. round is the audit dispatch count (the live report's
// round number); the previous archive is round-1.
func auditorFindingsBrief(workspace string, round int) string {
	return renderAuditorFindings(workspace, round, actionableAuditFindings(workspace))
}

func actionableAuditFindings(workspace string) []reportdoc.Finding {
	reportName := phasecontract.ArtifactFilename(string(PhaseAudit))
	current, err := os.ReadFile(filepath.Join(workspace, reportName))
	if err != nil {
		// Absence is legitimate (the audit crashed before writing a report);
		// anything else is the "looked in the wrong place" class the gate reader
		// beside this one (readContinuationFindings) already reports — same posture.
		if !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN audit-repair: %s unreadable (%v) — builder gets no auditor findings\n", reportName, err)
		}
		return nil
	}
	return actionable(reportdoc.Findings(string(current)))
}

func renderAuditorFindings(workspace string, round int, findings []reportdoc.Finding) string {
	if len(findings) == 0 {
		return ""
	}
	reportName := phasecontract.ArtifactFilename(string(PhaseAudit))
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
