package main

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/orchestrator"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func watchOptimisticAdmission(signals *signalcenter.Center, getenv func(string) string) {
	todo, holder := getenv(ipcenv.FleetScopeKey), getenv(ipcenv.FleetReachHolderKey)
	if todo == "" || holder == "" {
		return
	}
	signals.Subscribe(orchestrator.RecordOptimisticFailure(todo, holder, optimisticGateOf, signals.Emit))
}

func optimisticGateOf(e signalcenter.Event) (string, bool) {
	switch e.Code {
	case core.CodeComposedGateDeclined:
		return e.Fields["gates"], true
	case core.CodeFleetRebaseConflict:
		return "rebase", true
	default:
		return "", false
	}
}
