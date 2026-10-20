package runner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/log"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/treefence"
)

type retraction struct {
	snap    treefence.Snapshot
	takeErr error
}

func (b *BaseRunner) RetractsOnFail() bool { return b.retractOnFail }

func (b *BaseRunner) takeRetraction(ctx context.Context, req core.PhaseRequest) *retraction {
	if !b.retractOnFail || req.Worktree == "" || req.Workspace == "" {
		return nil
	}
	snap, err := treefence.Take(ctx, req.Worktree)
	return &retraction{snap: snap, takeErr: err}
}

func (r *retraction) apply(ctx context.Context, phase, artifactPath, workspace string, resp core.PhaseResponse) core.PhaseResponse {
	if r == nil || !statedFail(artifactPath) {
		return resp
	}
	msg := r.retract(ctx, phase, filepath.Join(workspace, phase+"-retracted"))
	log.Diag().Warnf("[runner] WARN %s\n", msg)
	resp.Diagnostics = append(resp.Diagnostics, core.Diagnostic{Severity: "warning", Message: msg})
	return resp
}

func (r *retraction) retract(ctx context.Context, phase, keepDir string) string {
	if r.takeErr != nil {
		return fmt.Sprintf("retraction: snapshot unavailable for phase %s (%v) — its writes stay in the worktree for the audit", phase, r.takeErr)
	}
	res, err := r.snap.Retract(ctx, keepDir)
	if err != nil {
		return fmt.Sprintf("retraction: phase %s stated FAIL but the retraction failed (%v) — restored %d path(s) [%s]; its other writes stay in the worktree for the audit", phase, err, len(res.Restored), strings.Join(res.Restored, ", "))
	}
	return fmt.Sprintf("retraction: phase %s stated FAIL on its own work; restored %d path(s) it wrote to the dispatch tree, kept in %s: %s",
		phase, len(res.Restored), keepDir, strings.Join(res.Restored, ", "))
}

func statedFail(artifactPath string) bool {
	content, err := os.ReadFile(artifactPath)
	if err != nil {
		return false
	}
	stated, ok := phasecontract.ParseVerdictSentinel(string(content))
	return ok && stated == core.VerdictFAIL
}
