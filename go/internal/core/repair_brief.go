package core

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/reportdoc"
)

const maxBriefFindings = 8

const (
	findingsLead          = "fix THESE"
	findingsLeadAfterGate = "fix THESE; the gate reasons above are their symptoms"
)

func composeRepairBrief(cs CycleState, warn io.Writer) string {
	findings := actionableAuditFindings(cs.WorkspacePath, warn)
	lead := findingsLead
	if runnerDiagnosedAudit(cs) {
		lead = findingsLeadAfterGate
	}
	var parts []string
	if reasons := auditRejectionReasons(cs, briefedFindings(findings)); reasons != "" {
		parts = append(parts, reasons)
	}
	if brief := renderAuditorFindings(cs.WorkspacePath, cs.AuditDispatches, findings, lead); brief != "" {
		parts = append(parts, brief)
	}
	if len(parts) == 0 {
		return ""
	}
	return truncateFindings(strings.Join(parts, "\n\n"))
}

func auditRejectionReasons(cs CycleState, briefed []reportdoc.Finding) string {
	declared, isDeclared := phasecontract.ReadFailureBlock(cs.WorkspacePath, string(PhaseAudit))
	if derived, ok := gateFailureBlock(cs.AuditFailReasons); ok && !isDeclared {
		return renderFailureBlock("the deterministic gates' failure block", derived, briefed)
	}
	if runnerDiagnosedAudit(cs) {
		return renderFailReasons(string(PhaseAudit), cs.AuditFailReasons)
	}
	if !isDeclared {
		return ""
	}
	return renderFailureBlock("the verdict's failure block", declared, briefed)
}

func renderFailureBlock(source string, fb *phasecontract.FailureBlock, briefed []reportdoc.Finding) string {
	var lines []string
	for _, defect := range fb.Defects {
		if defect = strings.TrimSpace(defect); defect != "" && !restatesAFinding(defect, briefed) {
			lines = append(lines, "- "+defect)
		}
	}
	if len(lines) == 0 {
		return ""
	}
	return "audit defects (" + source + ", class " + fb.Class + "):\n" + strings.Join(lines, "\n")
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

func auditorFindingsBrief(workspace string, round int, warn io.Writer) string {
	return renderAuditorFindings(workspace, round, actionableAuditFindings(workspace, warn), findingsLead)
}

func actionableAuditFindings(workspace string, warn io.Writer) []reportdoc.Finding {
	reportName := phasecontract.ArtifactFilename(string(PhaseAudit))
	current, err := os.ReadFile(filepath.Join(workspace, reportName))
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Fprintf(warn, "[orchestrator] WARN audit-repair: %s unreadable (%v) — builder gets no auditor findings\n", reportName, err)
		}
		return nil
	}
	return actionable(reportdoc.Findings(string(current)))
}

func briefedFindings(findings []reportdoc.Finding) []reportdoc.Finding {
	return findings[:min(len(findings), maxBriefFindings)]
}

func renderAuditorFindings(workspace string, round int, findings []reportdoc.Finding, lead string) string {
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
	shown := briefedFindings(findings)
	var b strings.Builder
	fmt.Fprintf(&b, "auditor findings (audit round %d — %s):\n", round, lead)
	for _, f := range shown {
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
	if more := len(findings) - len(shown); more > 0 {
		fmt.Fprintf(&b, "- … %d more finding(s) in %s\n", more, reportName)
	}
	return strings.TrimRight(b.String(), "\n")
}

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

func archiveRepairPrompts(cs CycleState, phase Phase) {
	workspace, round := cs.WorkspacePath, cs.AuditRepairAttempts
	if !cs.AuditRepairActive || !repairSeededPhase(phase) || workspace == "" || round < 1 {
		return
	}
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
