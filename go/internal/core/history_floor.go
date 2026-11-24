package core

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/commentaudit"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const CodeCommentHistoryShadow signalcenter.Code = "BUILD_COMMENT_HISTORY_SHADOW"

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleBuild, CodeCommentHistoryShadow, "at comment_floor.history_stage shadow, the build removed history-carrying comments that no archive page records, and enforce would send the build back; fields.cycle, count, entries")
}

func historyFloorFailures(ctx context.Context, in ReviewInput) []string {
	stage := historyFloorStage(in.ProjectRoot, in.errOut())
	if stage == config.StageOff || in.Worktree == "" {
		return nil
	}
	base := floorBase(in)
	paths, err := worktreePathsSince(ctx, in.Worktree, base, "--no-renames")
	var unrecorded []commentaudit.HistoryEntry
	if err == nil {
		unrecorded, err = commentaudit.UnrecordedRemovals(paths, commentaudit.ReadAtBase(worktreeGit(ctx, in.Worktree), base), worktreeFile(in.Worktree))
	}
	if err != nil {
		fmt.Fprintf(in.errOut(), "[build-floor] WARN history floor could not read the diff (%v); the audit still judges comments\n", err)
		return nil
	}
	failures, warning := historyFloorResult(stage, unrecorded, base)
	if warning != "" {
		fmt.Fprintln(in.errOut(), warning)
		in.emitHistoryShadow(unrecorded)
	}
	return failures
}

func historyFloorResult(stage config.Stage, unrecorded []commentaudit.HistoryEntry, base string) (failures []string, warning string) {
	if stage == config.StageOff || len(unrecorded) == 0 {
		return nil, ""
	}
	listing := historyListing(unrecorded)
	if stage != config.StageEnforce {
		return nil, fmt.Sprintf("[build-floor] WARN history floor (shadow): the build removes %d history-carrying comment group(s) that no page under %s records:\n    %s", len(unrecorded), commentaudit.HistoryArchiveDir, listing)
	}
	return []string{fmt.Sprintf("the build removes %d history-carrying comment group(s) and no page under %s records them. Run `%s` and keep the pages it writes (docs/conventions/code-comments.md):\n    %s", len(unrecorded), commentaudit.HistoryArchiveDir, commentaudit.RecordHistoryCommand(base), listing)}, ""
}

func historyListing(entries []commentaudit.HistoryEntry) string {
	lines := make([]string, 0, len(entries))
	for _, e := range entries {
		lines = append(lines, fmt.Sprintf("%s:%d %s", e.File, e.Line, e.Anchor))
	}
	return strings.Join(lines, "\n    ")
}

func (in ReviewInput) emitHistoryShadow(unrecorded []commentaudit.HistoryEntry) {
	in.Signals.Emit(signalcenter.Event{
		Cycle: in.Cycle, RunID: in.RunID, Phase: string(PhaseBuild),
		Module: signalcenter.ModuleBuild, Origin: "ReviewInput.emitHistoryShadow", Kind: signalcenter.KindGateCorrected,
		Severity: signalcenter.SeverityInfo, Code: CodeCommentHistoryShadow,
		Reason: "shadow: enforce would send the build back to record removed comment history",
		Fields: map[string]string{"cycle": strconv.Itoa(in.Cycle), "count": strconv.Itoa(len(unrecorded)), "entries": strings.ReplaceAll(historyListing(unrecorded), "\n    ", "; ")},
	})
}

func historyFloorStage(projectRoot string, warn io.Writer) config.Stage {
	chosen := policy.Policy{}.CommentFloorConfig().HistoryStage
	if projectRoot != "" {
		chosen = policy.LoadOrDefaults(filepath.Join(projectRoot, ".evolve", "policy.json"), warn).CommentFloorConfig().HistoryStage
	}
	return chosen.Stage()
}
