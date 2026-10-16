package ledger

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

const RebaselineKind = resetSealKindPrefix + "rebaseline"

func (l *FileLedger) Rebaseline(ctx context.Context, note string) error {
	if strings.TrimSpace(note) == "" {
		return fmt.Errorf("ledger rebaseline: an operator note is required — sealing a damaged prefix is a trust decision and must carry its justification")
	}
	segs, live, err := l.readChain()
	if err != nil {
		return fmt.Errorf("ledger rebaseline: %w", err)
	}
	if len(segs) == 0 && len(live) == 0 {
		return fmt.Errorf("ledger rebaseline: no ledger entries found (%s) — refusing to seal, and mint, a chain that does not exist", l.ledgerPath)
	}
	if len(live) == 0 {
		return fmt.Errorf("ledger rebaseline: live ledger %s is empty — no physical tail to chain the seal from", l.ledgerPath)
	}
	tipSeq, _, err := l.readTip()
	if err != nil {
		return fmt.Errorf("ledger rebaseline: %w", err)
	}
	entry := core.LedgerEntry{
		TS:       time.Now().UTC().Format(time.RFC3339),
		Role:     operatorRole,
		Kind:     RebaselineKind,
		Message:  note,
		EntrySeq: tipSeq + 1,
		PrevHash: sha256Hex(live[len(live)-1]),
	}
	if err := l.dryRunSeal(segs, live, entry); err != nil {
		return fmt.Errorf("ledger rebaseline: refused, nothing written — the seal would not verify forward: %w", err)
	}
	if err := l.appendChainedFromTail(func(seq int, prevHash string) any {
		entry.EntrySeq = seq
		entry.PrevHash = prevHash
		return entry
	}); err != nil {
		return fmt.Errorf("ledger rebaseline: append seal: %w", err)
	}
	if err := l.VerifyDeep(ctx); err != nil {
		return fmt.Errorf("ledger rebaseline: seal appended but the chain still does not verify forward: %w", err)
	}
	return nil
}

func (l *FileLedger) dryRunSeal(segs []sealedSegment, live [][]byte, seal core.LedgerEntry) error {
	line, err := hooks.marshal(seal)
	if err != nil {
		return fmt.Errorf("ledger marshal: %w", err)
	}
	sealTip := fmt.Sprintf("%d:%s", seal.EntrySeq, sha256Hex(line))
	sealed := append(live[:len(live):len(live)], line)
	_, err = l.verifyChain(segs, sealed, func(lastSeq int, lastSha string) error {
		return compareTip(sealTip, lastSeq, lastSha)
	})
	return err
}
