package ledger

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// RebaselineKind is the Kind of Rebaseline's in-band seal; it carries resetSealKindPrefix and marks a bulk seal.
const RebaselineKind = resetSealKindPrefix + "rebaseline"

// Rebaseline appends one operator seal at the physical tail and deep-verifies; it refuses without a note or entries.
func (l *FileLedger) Rebaseline(ctx context.Context, note string) error {
	if strings.TrimSpace(note) == "" {
		return fmt.Errorf("ledger rebaseline: an operator note is required — sealing a damaged prefix is a trust decision and must carry its justification")
	}
	// Minting a sealed genesis where no chain exists would fabricate an audit record, not repair one.
	lines, err := l.gatherAllLines()
	if err != nil {
		return fmt.Errorf("ledger rebaseline: %w", err)
	}
	if len(lines) == 0 {
		return fmt.Errorf("ledger rebaseline: no ledger entries found (%s) — refusing to seal, and mint, a chain that does not exist", l.ledgerPath)
	}
	// Chain from the physical last line: foreign raw appends sit past the tip, and sealChainsFromPrev
	// checks the physical predecessor. The tip then moves to the seal so later appends chain from it.
	entry := core.LedgerEntry{
		TS:      time.Now().UTC().Format(time.RFC3339),
		Role:    operatorRole,
		Kind:    RebaselineKind,
		Message: note,
	}
	if err := l.appendChainedFromTail(func(seq int, prevHash string) any {
		entry.EntrySeq = seq
		entry.PrevHash = prevHash
		return entry
	}); err != nil {
		return fmt.Errorf("ledger rebaseline: append seal: %w", err)
	}
	// A seal that does not green the chain (stale tip, broken segment binding) fails here, not at the next audit.
	if err := l.VerifyDeep(ctx); err != nil {
		return fmt.Errorf("ledger rebaseline: seal appended but the chain still does not verify forward: %w", err)
	}
	return nil
}
