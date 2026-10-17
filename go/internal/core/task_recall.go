package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/research"
)

const CtxKeyRecallMemory = "recalled_lessons"

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
			fmt.Fprintf(&b, "%s [source: %s]\n", truncateRunes(lesson.Digest(), maxGoalTextChars), lesson.Path)
		}
	}
	if err == nil && cs.WorkspacePath != "" {
		if werr := dossier.AppendRecalledLessons(cs.WorkspacePath, lessonIDs(found)); werr != nil {
			fmt.Fprintf(o.errOut(), "[orchestrator] WARN recalled-lessons record: %v\n", werr)
		}
	}
	out := make(map[string]string, len(base)+1)
	for k, v := range base {
		out[k] = v
	}
	out[CtxKeyRecallMemory] = b.String()
	return out
}

func lessonIDs(lessons []research.Lesson) []string {
	ids := make([]string, 0, len(lessons))
	for _, l := range lessons {
		ids = append(ids, l.ID)
	}
	return ids
}
