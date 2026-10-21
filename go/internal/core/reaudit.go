package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

type ReauditErrorKind string

const (
	ReauditLiveLease ReauditErrorKind = "live-lease"
	ReauditConflict  ReauditErrorKind = "rebase-conflict"
	ReauditNoCycle   ReauditErrorKind = "no-such-cycle"
	ReauditInfra     ReauditErrorKind = "infra"
)

type ReauditError struct {
	Kind      ReauditErrorKind
	Cycle     int
	Conflicts []string
	Err       error
}

func (e *ReauditError) Error() string {
	msg := fmt.Sprintf("reaudit cycle %d: %s", e.Cycle, e.Kind)
	if len(e.Conflicts) > 0 {
		msg += " on " + strings.Join(e.Conflicts, ", ")
	}
	if e.Err != nil {
		msg += ": " + e.Err.Error()
	}
	return msg
}

func (e *ReauditError) Unwrap() error { return e.Err }

type ReauditInput struct {
	ProjectRoot string
	EvolveDir   string
	Cycle       int
	Ledger      Ledger
	Audit       PhaseRunner
}

type ReauditResult struct {
	Cycle       int
	NewBase     string
	Verdict     string
	VerdictPath string
}

var reauditRebase = rebaseRecordingConflicts

func ReauditCycle(ctx context.Context, in ReauditInput) (ReauditResult, error) {
	fail := func(kind ReauditErrorKind, err error) error {
		return &ReauditError{Kind: kind, Cycle: in.Cycle, Err: err}
	}
	cs, err := loadReauditState(in.EvolveDir, in.Cycle)
	if err != nil {
		return ReauditResult{}, fail(ReauditNoCycle, err)
	}
	if lease, live := runlease.LiveOwner(cs.WorkspacePath, time.Now()); live {
		return ReauditResult{}, fail(ReauditLiveLease, fmt.Errorf("run %s is owned by live pid %d", lease.RunID, lease.OwnerPID))
	}
	ok, conflict := reauditRebase(ctx, in.ProjectRoot, &cs)
	if conflict {
		return ReauditResult{}, &ReauditError{Kind: ReauditConflict, Cycle: in.Cycle, Conflicts: cs.ShipRecoveryConflicts}
	}
	if !ok {
		return ReauditResult{}, fail(ReauditInfra, errors.New("the rebase onto main failed without a conflict"))
	}
	base, _, err := gitCapture(ctx, cs.ActiveWorktree, "rev-parse", "main")
	if err != nil {
		return ReauditResult{}, fail(ReauditInfra, fmt.Errorf("resolve main: %w", err))
	}
	base = strings.TrimSpace(base)
	resp, err := in.Audit.Run(ctx, PhaseRequest{
		Cycle:                           in.Cycle,
		ProjectRoot:                     in.ProjectRoot,
		Workspace:                       cs.WorkspacePath,
		Worktree:                        cs.ActiveWorktree,
		Fence:                           FenceFor(WorktreePhase(PhaseAudit), nil),
		WorktreeBaseSHA:                 base,
		ExplanationDocumentationVersion: cs.ExplanationDocumentationVersion,
		RunID:                           cs.RunID,
		GoalHash:                        cs.GoalHash,
		PreviousPhase:                   string(PhaseBuild),
	})
	if err != nil {
		return ReauditResult{}, fail(ReauditInfra, err)
	}
	entry, err := auditBindingEntry(ctx, in.ProjectRoot, cs.WorkspacePath, cs.ActiveWorktree, base, resp.Verdict)
	if err != nil {
		return ReauditResult{}, fail(ReauditInfra, err)
	}
	entry.TS = time.Now().UTC().Format(time.RFC3339)
	entry.Cycle = in.Cycle
	entry.RunID = cs.RunID
	if err := in.Ledger.Append(ctx, entry); err != nil {
		return ReauditResult{}, fail(ReauditInfra, fmt.Errorf("record the audit binding: %w", err))
	}
	return ReauditResult{Cycle: in.Cycle, NewBase: base, Verdict: resp.Verdict, VerdictPath: entry.ArtifactPath}, nil
}

func loadReauditState(evolveDir string, cycle int) (CycleState, error) {
	raw, err := os.ReadFile(ResolveCycleStatePath(evolveDir))
	if err != nil {
		return CycleState{}, fmt.Errorf("no cycle state: %w", err)
	}
	var cs CycleState
	if err := json.Unmarshal(raw, &cs); err != nil {
		return CycleState{}, fmt.Errorf("parse cycle state: %w", err)
	}
	if cycle <= 0 || cs.CycleID != cycle {
		return CycleState{}, fmt.Errorf("cycle %d is not the cycle in state (%d)", cycle, cs.CycleID)
	}
	if cs.ActiveWorktree == "" || cs.WorkspacePath == "" {
		return CycleState{}, fmt.Errorf("cycle %d has no worktree or workspace", cycle)
	}
	return cs, nil
}
