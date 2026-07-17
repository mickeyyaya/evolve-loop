package main

import (
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

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

// writePipelineEscalation records the ADR-0072 halt: it writes
// .evolve/pipeline-escalation.json (the diagnostic dossier) and auto-files a P0
// pipeline-repair inbox item. The inbox write honors never_stop_queue — the
// QUEUE is still injected even though the loop halts, so on resume the pipeline
// fix is the first thing worked. Both writes are best-effort + LOUD on error
// (a halt that also fails to leave a breadcrumb must not do so silently).
func writePipelineEscalation(evolveDir, projectRoot string, cycle int, workspace string, sf *cyclestate.SystemFailureSignal, stderr io.Writer) {
	now := time.Now().UTC()
	esc := pipelineEscalation{
		Schema:     1,
		Category:   sf.Category,
		Level:      sf.Level,
		Evidence:   sf.Evidence,
		Cycle:      cycle,
		Workspace:  workspace,
		DetectedAt: now.Format(time.RFC3339),
		NextAction: "Diagnose the PIPELINE (not the task). Read the cycle's audit-report.md + acs-verdict.json (both green) against the recorded verdict; the runner/verdict-surface path forged a negative verdict. Fix the pipeline defect, then resume: evolve loop --resume.",
		ReproHint:  "The verdict-surface path recorded a negative verdict while the phase artifacts are green — compare recorded outcome vs on-disk evolve-verdict (internal/coherence.CheckVerdictCoherence).",
	}
	escPath := filepath.Join(evolveDir, "pipeline-escalation.json")
	if werr := atomicwrite.JSON(escPath, esc); werr != nil {
		fmt.Fprintf(stderr, "[loop] WARN: could not write pipeline-escalation.json: %v\n", werr)
	}

	// Auto-file a P0 pipeline-repair inbox item. Deterministic filename (no
	// clock collision within a cycle) so a repeated halt on the same category
	// overwrites rather than piling duplicates.
	item := map[string]any{
		"id":         fmt.Sprintf("pipeline-defect-%s", sf.Category),
		"created_at": now.Format(time.RFC3339),
		"weight":     0.99,
		"title":      fmt.Sprintf("PIPELINE DEFECT (%s): the loop halted — %s", sf.Category, sf.Evidence),
		"kind":       "pipeline-repair",
		"priority":   "P0",
		"summary":    fmt.Sprintf("ADR-0072 system-failure halt at cycle %d. The pipeline forged a verdict (category=%s): the recorded cycle verdict was negative while the phase artifacts (audit-report.md + acs-verdict.json) are green. This is a PIPELINE defect, not a task failure — retrying the task reproduces it.", cycle, sf.Category),
		"root_cause": "The verdict-surface / cycle-finalization path recorded a negative verdict that contradicts the phases' own on-disk artifacts. See internal/coherence and ADR-0072.",
		"fix":        "Root-cause the verdict-surface path (runner clean-exit deliverable-authority, session-lifecycle verdict clobber, or a new variant). Add a regression test that reproduces the incoherence. Do NOT retry the halted task until the pipeline is fixed.",
		"connects_to": []string{
			"docs/architecture/adr/0072-system-failure-policy-and-halt.md",
			filepath.Join(".evolve", "pipeline-escalation.json"),
			workspace,
		},
		"notes": fmt.Sprintf("Auto-filed by the ADR-0072 halt at %s. Evidence: %s", now.Format(time.RFC3339), sf.Evidence),
	}
	// atomicwrite.JSON creates the inbox dir if needed.
	itemPath := filepath.Join(projectRoot, ".evolve", "inbox", fmt.Sprintf("pipeline-defect-%s.json", sf.Category))
	if werr := atomicwrite.JSON(itemPath, item); werr != nil {
		fmt.Fprintf(stderr, "[loop] WARN: could not auto-file pipeline-repair inbox item: %v\n", werr)
	}
}
