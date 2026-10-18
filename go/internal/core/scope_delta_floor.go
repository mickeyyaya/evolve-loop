package core

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/scopedelta"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const CodeScopeDelta signalcenter.Code = "ORCHESTRATOR_SCOPE_DELTA"

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleOrchestrator, CodeScopeDelta, "the build handoff floor found a changed path outside the lane's declared scope (ADR-0087): fields.path, fields.class (closure, opportunistic, boundary or cross-lane) and fields.disposition (keep, carve or refuse) carry scopedelta's classification, and the reason is the classifier's own; WARN for carve and refuse, INFO for a computed closure; the floor reports and never blocks (shadow rollout)")
}

func ScopeDeltaFloorChecks(protected func(string) bool) BuildFloorCheckFn {
	return func(ctx context.Context, in ReviewInput) []string {
		if in.Worktree == "" || in.ProjectRoot == "" || in.Cycle < 1 {
			return nil
		}
		evolveDir := filepath.Join(in.ProjectRoot, ".evolve")
		declared, err := LaneDeclaredFiles(evolveDir, in.Cycle)
		if err != nil {
			fmt.Fprintf(in.errOut(), "[build-floor] WARN scope-delta: %v; the declared scope is unknown, so no delta is reported\n", err)
			return nil
		}
		if len(declared) == 0 {
			return nil
		}
		changed := ChangedWorktreePathsSinceBase(ctx, in.Worktree, in.WorktreeBaseSHA)
		for _, e := range ClassifyLaneChanges(evolveDir, in.Cycle, declared, changed, protected, in.errOut()) {
			if e.Class == scopedelta.ClassInScope {
				continue
			}
			in.Signals.Emit(scopeDeltaEvent(in, e))
			fmt.Fprintf(in.errOut(), "[build-floor] scope-delta %s/%s: %s\n", e.Class, e.Disposition, e.Path)
		}
		return nil
	}
}

func scopeDeltaEvent(in ReviewInput, e scopedelta.Entry) signalcenter.Event {
	severity := signalcenter.SeverityWarn
	if e.Disposition == scopedelta.DispositionKeep {
		severity = signalcenter.SeverityInfo
	}
	return signalcenter.Event{
		Cycle: in.Cycle, RunID: in.RunID, Phase: in.Phase,
		Module: signalcenter.ModuleOrchestrator, Origin: "ScopeDeltaFloorChecks",
		Kind: signalcenter.KindGatePassed, Severity: severity, Code: CodeScopeDelta,
		Reason: e.Reason,
		Fields: map[string]string{"path": e.Path, "class": string(e.Class), "disposition": string(e.Disposition)},
	}
}
