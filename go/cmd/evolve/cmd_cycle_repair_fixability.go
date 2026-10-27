package main

import (
	"fmt"
	"io"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/fixability"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

func repairFixability(protected func(string) bool, warn io.Writer) func(core.RepairCheck) core.RepairFindings {
	return func(in core.RepairCheck) core.RepairFindings {
		d := fixability.Classify(fixability.Input{
			Defects: in.Defects, GateFailures: in.GateFailures,
			OpenConsoleItems: openConsoleIDs(in.InboxDir, protected, warn), Salvage: in.Salvage,
		})
		return core.RepairFindings{
			Stop: d.Stop, Reason: d.Reason, Lane: findingIDs(d.Lane), Handoff: findingIDs(d.Handoff),
			Blockers: findingIDs(d.Blockers), ConsoleItems: d.ConsoleItems, Salvage: d.Salvage,
		}
	}
}

func openConsoleIDs(inbox string, protected func(string) bool, warn io.Writer) []string {
	scan, err := inboxbatch.ScanDir(inbox)
	if err != nil {
		fmt.Fprintf(warn, "[orchestrator] WARN fixability: the inbox %s is unreadable (%v); no console item can make a finding a hand-off finding\n", inbox, err)
		return nil
	}
	var ids []string
	for _, it := range fixability.OpenConsole(scan.Items, protected) {
		ids = append(ids, it.ID)
	}
	return ids
}

func findingIDs(fs []fixability.Finding) []string {
	var ids []string
	for _, f := range fs {
		ids = append(ids, f.ID)
	}
	return ids
}
