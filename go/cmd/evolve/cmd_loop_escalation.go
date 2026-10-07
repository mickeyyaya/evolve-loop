package main

import (
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

// escalationInjectedBy stamps halt-autofiled inbox items with their producer.
const escalationInjectedBy = "loop-escalation"

// pipelineEscalation is the diagnostic dossier written on an ADR-0072
// system-failure halt. It is the operator's (and the resumed loop's) starting
// point for the pipeline diagnosis the halt demands.
type pipelineEscalation struct {
	Schema     int    `json:"schema_version"`
	Category   string `json:"category"`
	Level      string `json:"level"`
	Evidence   string `json:"evidence"`
	Cycle      int    `json:"cycle"`
	Workspace  string `json:"workspace"`
	DetectedAt string `json:"detected_at"`
	NextAction string `json:"next_action"`
	ReproHint  string `json:"repro_hint"`
}

// pipelineEscalationRecord is what writePipelineEscalation alone knows and
// the halt INCIDENT reports: the dossier's next_action (the ONE home of what
// the operator does next) and where the dossier and the P0 item landed.
type pipelineEscalationRecord struct {
	NextAction    string
	DossierPath   string
	InboxItemPath string
}

// writePipelineEscalation records the ADR-0072 halt: it writes
// .evolve/pipeline-escalation.json (the diagnostic dossier) and auto-files a P0
// pipeline-repair inbox item. The inbox write honors never_stop_queue — the
// QUEUE is still injected even though the loop halts, so on resume the pipeline
// fix is the first thing worked. Both writes are best-effort + LOUD on error
// (a halt that also fails to leave a breadcrumb must not do so silently).
// The returned record names what it wrote, for the halt INCIDENT.
func writePipelineEscalation(evolveDir, projectRoot string, cycle int, workspace string, sf *cyclestate.SystemFailureSignal, stderr io.Writer) pipelineEscalationRecord {
	halt := haltRecord{cycle: cycle, workspace: workspace, sf: sf, now: time.Now().UTC()}
	story := halt.narrative()
	escPath := filepath.Join(evolveDir, "pipeline-escalation.json")
	if werr := atomicwrite.JSON(escPath, halt.dossier(story)); werr != nil {
		fmt.Fprintf(stderr, "[loop] WARN: could not write pipeline-escalation.json: %v\n", werr)
	}
	itemID, item := halt.inboxItem(story)
	itemPath := filepath.Join(projectRoot, ".evolve", "inbox", itemID+".json")
	if werr := atomicwrite.JSON(itemPath, item); werr != nil {
		fmt.Fprintf(stderr, "[loop] WARN: could not auto-file pipeline-repair inbox item: %v\n", werr)
	}
	return pipelineEscalationRecord{NextAction: story.nextAction, DossierPath: escPath, InboxItemPath: itemPath}
}

type haltRecord struct {
	cycle     int
	workspace string
	sf        *cyclestate.SystemFailureSignal
	now       time.Time
}

type haltNarrative struct {
	nextAction, reproHint, summary, rootCause, fix string
}

func (h haltRecord) narrative() haltNarrative {
	sf := h.sf
	if sf.Category == "verdict-incoherence" {
		return haltNarrative{
			nextAction: "Diagnose the PIPELINE (not the task). Read the cycle's audit-report.md + acs-verdict.json (both green) against the recorded verdict; the runner/verdict-surface path forged a negative verdict. Fix the pipeline defect, then resume: evolve loop --resume.",
			reproHint:  "The verdict-surface path recorded a negative verdict while the phase artifacts are green — compare recorded outcome vs on-disk evolve-verdict (internal/coherence.CheckVerdictCoherence).",
			summary:    fmt.Sprintf("ADR-0072 system-failure halt at cycle %d. The pipeline forged a verdict (category=%s): the recorded cycle verdict was negative while the phase artifacts (audit-report.md + acs-verdict.json) are green. This is a PIPELINE defect, not a task failure — retrying the task reproduces it.", h.cycle, sf.Category),
			rootCause:  "The verdict-surface / cycle-finalization path recorded a negative verdict that contradicts the phases' own on-disk artifacts. See internal/coherence and ADR-0072.",
			fix:        "Root-cause the verdict-surface path (runner clean-exit deliverable-authority, session-lifecycle verdict clobber, or a new variant). Add a regression test that reproduces the incoherence. Do NOT retry the halted task until the pipeline is fixed.",
		}
	}
	return haltNarrative{
		nextAction: fmt.Sprintf("Diagnose the PIPELINE (not the task) from the system-failure evidence: %s. Fix the pipeline defect, then resume: evolve loop --resume.", sf.Evidence),
		reproHint:  fmt.Sprintf("Reproduce and root-cause the reported system failure: %s.", sf.Evidence),
		summary:    fmt.Sprintf("ADR-0072 system-failure halt at cycle %d (category=%s). Evidence: %s. This is a PIPELINE defect, not a task failure — diagnose the reported failure before retrying the task.", h.cycle, sf.Category, sf.Evidence),
		rootCause:  fmt.Sprintf("The system-failure signal reported: %s. Preserve this evidence while identifying the pipeline cause.", sf.Evidence),
		fix:        fmt.Sprintf("Root-cause the pipeline failure described by the recorded evidence: %s. Add a regression test that reproduces it. Do NOT retry the halted task until the pipeline is fixed.", sf.Evidence),
	}
}

func (h haltRecord) dossier(story haltNarrative) pipelineEscalation {
	return pipelineEscalation{
		Schema:     1,
		Category:   h.sf.Category,
		Level:      h.sf.Level,
		Evidence:   h.sf.Evidence,
		Cycle:      h.cycle,
		Workspace:  h.workspace,
		DetectedAt: h.now.Format(time.RFC3339),
		NextAction: story.nextAction,
		ReproHint:  story.reproHint,
	}
}

func (h haltRecord) inboxItem(story haltNarrative) (string, map[string]any) {
	itemID := fmt.Sprintf("pipeline-defect-%s-cycle%d", h.sf.Category, h.cycle)
	return itemID, map[string]any{
		"id":             itemID,
		"created_at":     h.now.Format(time.RFC3339),
		"weight":         0.99,
		"title":          fmt.Sprintf("PIPELINE DEFECT (%s): the loop halted — %s", h.sf.Category, h.sf.Evidence),
		"kind":           inboxbatch.KindPipelineRepair,
		"priority":       "P0",
		"priority_class": inboxbatch.ClassStability,
		"injected_by":    escalationInjectedBy,
		"summary":        story.summary,
		"root_cause":     story.rootCause,
		"fix":            story.fix,
		"connects_to": []string{
			"docs/architecture/adr/0072-system-failure-policy-and-halt.md",
			filepath.Join(".evolve", "pipeline-escalation.json"),
			h.workspace,
		},
		"notes": fmt.Sprintf("Auto-filed by the ADR-0072 halt at %s. Evidence: %s", h.now.Format(time.RFC3339), h.sf.Evidence),
	}
}
