package main

import (
	"io"
	"maps"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const systemFailureHaltExitCode = 4

func cycleRunExitCode(res cyclestate.CycleResult) int {
	if sf := res.SystemFailure; sf != nil && sf.Halt {
		return systemFailureHaltExitCode
	}
	if res.FinalVerdict == cyclestate.VerdictFAIL {
		return 2
	}
	return 0
}

func haltOnSystemFailure(evolveDir, projectRoot string, cycle int, workspace string, sf *cyclestate.SystemFailureSignal, w io.Writer, signals *signalcenter.Center, rule loopHaltRule) int {
	wrote := writePipelineEscalation(evolveDir, projectRoot, cycle, workspace, sf, w)
	fields := make(map[string]string, len(rule.fields)+5)
	maps.Copy(fields, rule.fields)
	fields["category"], fields["level"] = sf.Category, sf.Level
	fields["next"], fields["escalation"], fields["inbox_item"] = wrote.NextAction, wrote.DossierPath, wrote.InboxItemPath
	emitLoopHalt(signals, cycle, "haltOnSystemFailure", rule.code, sf.Category+": "+sf.Evidence, fields)
	return systemFailureHaltExitCode
}

func anyLaneHaltedForSystemFailure(results []fleet.Result) bool {
	for _, r := range results {
		if r.ExitCode == systemFailureHaltExitCode {
			return true
		}
	}
	return false
}

func dispatchHaltDecision(results []fleet.Result) (rc int, stopReason string, halt bool) {
	if anyLaneHaltedForSystemFailure(results) {
		return systemFailureHaltExitCode, "system_failure_halt", true
	}
	return 0, "", false
}
