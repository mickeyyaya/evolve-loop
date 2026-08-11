// rebaseline.go — seal a densely damaged chain prefix in ONE operator call.
//
// `evolve ledger anchor` is the remedy for ONE accepted break: the operator names
// the post-damage line and the chain verifies forward from it. That does not
// scale to the shape the console-plane ledger actually has — ~180 dense breaks
// from pre-CA.1 fleet-concurrency interleaving — because each anchor call binds a
// single line and the LAST one wins, so repairing N breaks would mean N sequential
// invocations with only the final one in effect. That is why that ledger was left
// broken rather than repaired.
//
// Rebaseline uses the IN-BAND seal instead (ADR-0081; see resetSealKindPrefix in
// anchor.go): it APPENDS one operator-role `reset-seal-*` entry at the tip. The
// entry chains from the current tip like any other line, so effectiveAnchorSHA
// moves the epoch anchor to it and walkChain validates strictly forward from
// there — however many breaks lie behind it. The damaged prefix is preserved
// byte-for-byte (this only ever appends; nothing is rewritten or truncated), it
// is simply no longer chain-validated, which is exactly the preservation remedy
// ADR-0048 chose over a destructive rebuild.
//
// It is gated on an operator note for the same reason `anchor` is an explicit
// command: declaring a prefix trusted-but-unvalidated is a TRUST decision, and an
// unattributable one is worse than a red chain.
package ledger

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// RebaselineKind is the Kind stamped on the in-band seal Rebaseline appends. It
// carries resetSealKindPrefix so the existing seal resolver recognises it, and
// says "rebaseline" so a forensics sweep can tell a bulk prefix seal from an
// ordinary per-line epoch anchor.
const RebaselineKind = resetSealKindPrefix + "rebaseline"

// Rebaseline appends one operator epoch seal at the tip, making the whole
// preceding prefix trusted-as-preserved in a single call, then proves the result
// by deep-verifying. Refuses — writing nothing — without an operator note or
// against a chain with no entries.
func (l *FileLedger) Rebaseline(ctx context.Context, note string) error {
	if strings.TrimSpace(note) == "" {
		return fmt.Errorf("ledger rebaseline: an operator note is required — sealing a damaged prefix is a trust decision and must carry its justification")
	}
	// Refuse an empty chain rather than appending a genesis line: minting a
	// one-line "sealed" ledger where none existed would fabricate an audit
	// record instead of repairing one.
	lines, err := l.gatherAllLines()
	if err != nil {
		return fmt.Errorf("ledger rebaseline: %w", err)
	}
	if len(lines) == 0 {
		return fmt.Errorf("ledger rebaseline: no ledger entries found (%s) — refusing to seal, and mint, a chain that does not exist", l.ledgerPath)
	}
	// Append chained from the PHYSICAL last line, not the tip: the damage
	// class this command exists for (out-of-band interleaved appends) leaves
	// foreign lines PAST the tip, and sealChainsFromPrev tests the physical
	// predecessor — a tip-chained seal binds the wrong line and is rejected
	// (console-plane live failure 2026-08-11). Same flock as the normal path,
	// so a concurrent chained writer cannot interleave; the tip moves to the
	// seal so subsequent normal appends chain green from it.
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
	// A seal that does not actually green the chain (a stale ledger.tip, damage
	// in a sealed SEGMENT's own binding) must fail loudly here rather than look
	// repaired until the next audit.
	if err := l.VerifyDeep(ctx); err != nil {
		return fmt.Errorf("ledger rebaseline: seal appended but the chain still does not verify forward: %w", err)
	}
	return nil
}
