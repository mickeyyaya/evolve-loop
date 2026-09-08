package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/research"
)

// CtxKeyRecallMemory carries bounded, evidence-only lesson digests to phase prompts.
const CtxKeyRecallMemory = "recalled_lessons"

// seedTaskRecall retrieves from the existing durable corpus on BOTH dispatch
// paths. It deliberately does not persist a second mutable memory summary.
func (o *Orchestrator) seedTaskRecall(ctx context.Context, base map[string]string, next Phase, cs CycleState, projectRoot string) map[string]string {
	if o.kb == nil || (next != PhaseScout && !taskContractPhase(next)) {
		return base
	}
	query := base["goal"]
	for _, ref := range o.taskItemRefs(base, projectRoot, cs.WorkspacePath) {
		item, _, err := inboxbatch.LoadFile(ref.path)
		if err == nil {
			query += " " + item.Title + " " + strings.Join(item.Acceptance, " ")
		}
	}
	found, err := o.kb.Lookup(ctx, research.Query{Keywords: strings.Fields(query)})
	var b strings.Builder
	if err != nil {
		fmt.Fprintf(&b, "Memory unavailable: %v", err)
	} else {
		for _, lesson := range found {
			// Reuse the existing rune-safe prompt bound; provenance stays visible
			// even when an unusually long lesson needs shortening.
			fmt.Fprintf(&b, "%s [source: %s]\n", truncateRunes(lesson.Digest(), maxGoalTextChars), lesson.Path)
		}
	}
	out := make(map[string]string, len(base)+1)
	for k, v := range base {
		out[k] = v
	}
	// Always replace inherited recall, including no matches, so a scoped lane
	// cannot accidentally reuse an unrelated task's memory on resume.
	out[CtxKeyRecallMemory] = b.String()
	return out
}
