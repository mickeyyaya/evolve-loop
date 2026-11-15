package main

import (
	"regexp"
	"strings"
	"testing"
)

type pinnedLedgerRoot struct {
	unobserved int
	reason     string
}

func TestUnobservedLedgerRootsArePinned(t *testing.T) {
	t.Parallel()
	pinned := map[string]pinnedLedgerRoot{
		"cmd/evolve/cmd_cycle.go":           {1, "evolve cycle reset seals through its own ledger (an operator repair path; its seal anchor is unobserved) — S4b threads the root's; the root's own construction beside it IS observed"},
		"cmd/evolve/cmd_gc_ledger_seal.go":  {1, "the batch-end gc hook seals the ledger behind gc.ledger_seal (state-growth-caps); the hook has no Signal Center, so the seal anchor is reported on stderr"},
		"cmd/evolve/cmd_cycle_reaudit.go":   {1, "evolve cycle reaudit appends one audit binding through its own ledger (an operator repair verb outside the orchestrator process)"},
		"cmd/evolve/cmd_ledger.go":          {6, "evolve ledger verify/seal/rebaseline/anchor/deep-verify/evidence restore: operator repair roots outside the orchestrator process"},
		"internal/cli/agentcmd/run.go":      {1, "evolve agent run appends one cycle-0 agent_run entry from an operator process that has no Signal Center; a failed append is exit 3 with the error on stderr and in the JSON envelope (ADR-0152)"},
		"internal/cli/guardcmd/guard.go":    {1, "the guard chain reads the ledger (a read-only consumer)"},
		"internal/inboxmover/inboxmover.go": {1, "the mover's fallback when no Ledger is injected — still reached by the ship phase's post-ship mover and the operator inbox commands until S4b threads the root's"},
	}
	callRE := regexp.MustCompile(`\bledger\.New\(`)
	seen := map[string]int{}
	for _, f := range sharedModuleFiles(t) {
		calls := len(callRE.FindAll(f.Src, -1))
		if calls == 0 {
			continue
		}
		seen[f.Rel] = calls - strings.Count(string(f.Src), "ledger.WithSignals(")
	}
	for file, unobserved := range seen {
		if want := pinned[file].unobserved; unobserved != want {
			t.Errorf("%s: %d unobserved ledger construction(s), pinned %d — observe it (ledger.WithSignals) or pin it here with a reason", file, unobserved, want)
		}
	}
	for file, pin := range pinned {
		if _, present := seen[file]; !present {
			t.Errorf("pinned root %s no longer constructs a ledger; drop it (%s)", file, pin.reason)
		}
	}
}
