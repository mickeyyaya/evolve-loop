package phasecmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/cmd/evolve/cmdutil"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

const (
	exitCycleRefused = 1
	exitCycleUsage   = 10
)

type cycleUsageError string

func (e cycleUsageError) Error() string { return string(e) }

func RequestForCycle(projectRoot string, cycle int, stdin io.Reader) (core.PhaseRequest, error) {
	if cycle <= 0 {
		return core.PhaseRequest{}, cycleUsageError(fmt.Sprintf("--cycle %d is not a positive cycle number", cycle))
	}
	given, err := stdinCarriesRequest(stdin)
	if err != nil {
		return core.PhaseRequest{}, fmt.Errorf("read stdin: %w", err)
	}
	if given {
		return core.PhaseRequest{}, cycleUsageError("--cycle derives the request from the cycle's state, so stdin must not carry a request too")
	}
	root := projectRoot
	if root == "" {
		root = cmdutil.EnvOrCwd("EVOLVE_PROJECT_ROOT")
	}
	root = paths.AbsoluteRoot("--project-root", root, nil)
	runDir := paths.RunWorkspace(root, cycle)
	if lease, live := runlease.LiveOwner(runDir, time.Now()); live {
		return core.PhaseRequest{}, fmt.Errorf("cycle %d is held by live run %s (pid %d); wait for it to end or release it", cycle, lease.RunID, lease.OwnerPID)
	}
	state, err := readCycleState(runDir, cycle)
	if err != nil {
		return core.PhaseRequest{}, err
	}
	if err := requireWorktree(state.ActiveWorktree, cycle); err != nil {
		return core.PhaseRequest{}, err
	}
	return requestFromState(state, root, runDir), nil
}

func CycleRequestExitCode(err error) int {
	var usage cycleUsageError
	if errors.As(err, &usage) {
		return exitCycleUsage
	}
	return exitCycleRefused
}

func stdinCarriesRequest(stdin io.Reader) (bool, error) {
	if stdin == nil {
		return false, nil
	}
	if f, ok := stdin.(*os.File); ok {
		if info, err := f.Stat(); err == nil && info.Mode()&os.ModeCharDevice != 0 {
			return false, nil
		}
	}
	body, err := io.ReadAll(stdin)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(body)) != "", nil
}

func readCycleState(runDir string, cycle int) (core.CycleState, error) {
	path := filepath.Join(runDir, core.CycleStateFile)
	raw, err := os.ReadFile(path)
	if err != nil {
		return core.CycleState{}, fmt.Errorf("cycle %d state not found at %s: %w", cycle, path, err)
	}
	var state core.CycleState
	if err := json.Unmarshal(raw, &state); err != nil {
		return core.CycleState{}, fmt.Errorf("cycle %d state at %s is unparseable: %w", cycle, path, err)
	}
	return state, nil
}

func requireWorktree(worktree string, cycle int) error {
	if worktree == "" {
		return fmt.Errorf("cycle %d records no worktree in its cycle-state.json", cycle)
	}
	if _, err := os.Stat(worktree); err != nil {
		return fmt.Errorf("cycle %d worktree %s is gone: %w", cycle, worktree, err)
	}
	return nil
}

func requestFromState(state core.CycleState, root, runDir string) core.PhaseRequest {
	workspace := state.WorkspacePath
	if workspace == "" {
		workspace = runDir
	}
	return core.PhaseRequest{
		Cycle:                           state.CycleID,
		ProjectRoot:                     root,
		Workspace:                       workspace,
		Worktree:                        state.ActiveWorktree,
		WorktreeBaseSHA:                 state.WorktreeBaseSHA,
		RunID:                           state.RunID,
		GoalHash:                        state.GoalHash,
		ExplanationDocumentationVersion: state.ExplanationDocumentationVersion,
	}
}
