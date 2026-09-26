package core

import (
	"context"
	"testing"
)

func TestLatestAuditedTree_BindsTheRowShipBinds(t *testing.T) {
	row := func(run, role, kind, tree string) LedgerEntry {
		return LedgerEntry{Cycle: unwindCycle, RunID: run, Role: role, Kind: kind, WorktreeTreeSHA: tree}
	}
	cases := []struct {
		name string
		rows []LedgerEntry
		run  string
		want string
	}{
		{"the newest auditor row of the run", []LedgerEntry{
			row(unwindRunID, "auditor", "agent_subprocess", "first"),
			row(unwindRunID, "auditor", "agent_subprocess", "newest"),
			row("run-other", "auditor", "agent_subprocess", "other-run"),
			row(unwindRunID, "builder", "agent_subprocess", "builder"),
			row(unwindRunID, "auditor", "phase", "phase-row"),
		}, unwindRunID, "newest"},
		{"a newest row without a tree declines rather than reach back", []LedgerEntry{
			row(unwindRunID, "auditor", "agent_subprocess", "older"),
			row(unwindRunID, "auditor", "agent_subprocess", ""),
		}, unwindRunID, ""},
		{"no row for the run", []LedgerEntry{
			row("run-other", "auditor", "agent_subprocess", "other-run"),
		}, unwindRunID, ""},
		{"an unstamped run takes the newest row of any run", []LedgerEntry{
			row("run-a", "auditor", "agent_subprocess", "a"),
			row("run-b", "auditor", "agent_subprocess", "b"),
		}, "", "b"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := NewOrchestrator(&fakeStorage{}, &fakeLedger{entries: tc.rows}, buildRunners(nil))

			tree, err := o.latestAuditedTree(context.Background(), tc.run)

			if err != nil || tree != tc.want {
				t.Fatalf("latestAuditedTree = (%q, %v), want (%q, nil)", tree, err, tc.want)
			}
		})
	}
}
