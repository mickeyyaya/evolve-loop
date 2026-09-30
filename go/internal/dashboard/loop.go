package dashboard

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmcalls"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

type llmCall struct {
	TS        string
	StartedAt string
	EndedAt   string
	CallID    string
	Phase     string
	CLI       string
	Model     string
	Attempt   int
	ExitCode  int
}

func readLoop(root string, now time.Time) (LoopStatus, []string) {
	var ls LoopStatus
	var warnings []string
	evolveDir := paths.EvolveDirOf(root)
	if _, err := os.Stat(paths.LoopStopPath(evolveDir)); err == nil {
		ls.BrakeEngaged = true
	}
	cs, ok, err := readCycleState(core.ResolveCycleStatePath(evolveDir))
	if err != nil {
		warnings = append(warnings, "cycle-state.json: "+err.Error())
	}
	if !ok {
		cs, ok = newestRunState(root)
		ls.Checkpointed = ok
	}
	if !ok {
		return ls, warnings
	}
	ls, warnings = statusForRun(root, now, cs, ls, warnings)
	return enrichLoopStatus(root, ls), warnings
}

func statusForRun(root string, now time.Time, cs cyclestate.CycleState, ls LoopStatus, warnings []string) (LoopStatus, []string) {
	ls.CycleID = cs.CycleID
	ls.Phase = cs.Phase
	ls.PhaseStartedAt = parseTime(cs.PhaseStartedAt)
	ls.ActiveWorktree = cs.ActiveWorktree
	ls.AuditRounds = cs.AuditDispatches
	ws := core.RunWorkspacePath(root, cs.CycleID)
	lease, ok, err := runlease.Read(ws)
	switch {
	case err != nil:
		warnings = append(warnings, fmt.Sprintf("cycle %d lease: %v", cs.CycleID, err))
	case ok:
		ls.LeaseHeartbeat = parseTime(lease.HeartbeatAt)
		if cs.RunID != "" && lease.RunID != cs.RunID {
			warnings = append(warnings, fmt.Sprintf("cycle %d lease identity does not match state", cs.CycleID))
		} else {
			ls.Running = runlease.Fresh(lease, now, runlease.DefaultTTL)
		}
	}
	return ls, warnings
}

func enrichLoopStatus(root string, ls LoopStatus) LoopStatus {
	if ls.CycleID > 0 {
		if call, ok := lastCallForPhase(readLLMCalls(core.RunWorkspacePath(root, ls.CycleID)), ls.Phase); ok {
			ls.CLI, ls.Model = call.CLI, call.Model
		}
	}
	return ls
}

func readRunStatuses(root string, now time.Time, primary LoopStatus) (map[int]LoopStatus, []string) {
	out := map[int]LoopStatus{}
	if primary.CycleID > 0 {
		out[primary.CycleID] = primary
	}
	ids, warning := workspaceCycles(root)
	var warnings []string
	if warning != "" {
		warnings = append(warnings, warning)
	}
	for _, id := range ids {
		ws := core.RunWorkspacePath(root, id)
		cs, ok, err := readCycleState(filepath.Join(ws, core.CycleStateFile))
		if !ok && err == nil {
			if id == primary.CycleID {
				continue
			}
			cs, ok, err = readCycleState(filepath.Join(ws, core.RunStateFile))
		}
		if !ok && err == nil {
			continue
		}
		ls := LoopStatus{CycleID: id, BrakeEngaged: primary.BrakeEngaged}
		if err != nil || cs.CycleID != id {
			warnings = append(warnings, fmt.Sprintf("cycle %d state invalid (declared cycle %d): %v", id, cs.CycleID, err))
			out[id] = ls
			continue
		}
		var w []string
		out[id], w = statusForRun(root, now, cs, ls, nil)
		warnings = append(warnings, w...)
	}
	return out, warnings
}

func newestRunState(root string) (cyclestate.CycleState, bool) {
	ids, _ := workspaceCycles(root)
	if len(ids) == 0 {
		return cyclestate.CycleState{}, false
	}
	newest := ids[0]
	for _, id := range ids[1:] {
		if id > newest {
			newest = id
		}
	}
	var cs cyclestate.CycleState
	ok, _ := readJSON(filepath.Join(core.RunWorkspacePath(root, newest), core.RunStateFile), &cs)
	return cs, ok && cs.CycleID > 0
}

func readCycleState(path string) (cs cyclestate.CycleState, ok bool, err error) {
	buf, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cs, false, nil
	}
	if err != nil {
		return cs, false, err
	}
	if err := json.Unmarshal(buf, &cs); err != nil {
		return cs, false, err
	}
	return cs, true, nil
}

func readLLMCalls(ws string) []llmCall {
	result, _ := llmcalls.ReadWorkspace(ws)
	out := make([]llmCall, 0, len(result.Records))
	for _, rec := range result.Records {
		if rec.Phase == "" {
			continue
		}
		model := rec.Model
		if rec.SchemaVersion >= llmcalls.SchemaVersion || rec.CallID != "" {
			model = rec.DispatchedModel
		}
		exitCode := 0
		if rec.ExitCode != nil {
			exitCode = *rec.ExitCode
		}
		out = append(out, llmCall{
			TS: rec.TS, StartedAt: rec.StartedAt, EndedAt: rec.EndedAt, CallID: rec.CallID,
			Phase: rec.Phase, CLI: rec.CLI, Model: model, Attempt: rec.Attempt, ExitCode: exitCode,
		})
	}
	return out
}

func lastCallForPhase(calls []llmCall, phase string) (llmCall, bool) {
	for i := len(calls) - 1; i >= 0; i-- {
		if calls[i].Phase == phase {
			return calls[i], true
		}
	}
	if len(calls) == 0 {
		return llmCall{}, false
	}
	return calls[len(calls)-1], true
}

func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	return time.Time{}
}
