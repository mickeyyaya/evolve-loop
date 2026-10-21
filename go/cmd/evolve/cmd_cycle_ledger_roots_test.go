package main

import (
	"os"
	"path/filepath"
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
	moduleRoot := filepath.Join("..", "..")
	seen := map[string]int{}
	err := filepath.WalkDir(moduleRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := entry.Name()
		if entry.IsDir() {
			if name == "vendor" || name == "bin" || strings.HasPrefix(name, ".") && path != moduleRoot {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		calls := len(callRE.FindAll(src, -1))
		if calls == 0 {
			return nil
		}
		rel, _ := filepath.Rel(moduleRoot, path)
		seen[filepath.ToSlash(rel)] = calls - strings.Count(string(src), "ledger.WithSignals(")
		return nil
	})
	if err != nil {
		t.Fatal(err)
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
