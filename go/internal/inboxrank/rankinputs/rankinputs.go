// Package rankinputs loads the inbox rank's inputs for one evolve dir. See docs/architecture/packages/internal-inboxrank-rankinputs.md.
package rankinputs

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxrank"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/recurrence"
)

const ledgerFile = "recurrence-ledger.json"

func Load(evolveDir string, now time.Time) (inboxrank.Inputs, []string) {
	var warnings []string
	pol, err := policy.Load(paths.PolicyPath(evolveDir))
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("policy unreadable (%v); ranking with the compiled inbox_priority default", err))
	}
	in := inboxrank.Inputs{Config: pol.InboxPriorityConfig(), Now: now}
	ledger, err := recurrence.ReadSnapshot(filepath.Join(evolveDir, ledgerFile))
	if err != nil {
		return in, append(warnings, fmt.Sprintf("recurrence ledger unreadable (%v); the recurrence factor is 0 for every item", err))
	}
	in.Recurrence = ledger.ItemCounts()
	return in, warnings
}
