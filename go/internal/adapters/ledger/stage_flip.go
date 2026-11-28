package ledger

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/stageevidence"
)

const StageFlipRole = "stage-flip"

func (l *FileLedger) AppendStageFlip(ctx context.Context, f stageevidence.Flip) error {
	return l.appendFlipEntry(ctx, stageevidence.FlipKind, f.Encode())
}

func (l *FileLedger) AppendStageGenesis(ctx context.Context, g stageevidence.Genesis) error {
	return l.appendFlipEntry(ctx, stageevidence.GenesisKind, g.Encode())
}

func (l *FileLedger) appendFlipEntry(ctx context.Context, kind, message string) error {
	entry := core.LedgerEntry{TS: time.Now().UTC().Format(time.RFC3339), Role: StageFlipRole, Kind: kind, Message: message}
	if err := l.Append(ctx, entry); err != nil {
		return fmt.Errorf("ledger: append %s: %w", kind, err)
	}
	return nil
}

func (l *FileLedger) FlipRecords(_ context.Context) (stageevidence.FlipSet, error) {
	var segs []sealedSegment
	var live [][]byte
	var tip tipRead
	if err := l.readSnapshot(func() (err error) {
		segs, live, err = l.readChain()
		tip = l.readTipFile()
		return err
	}); err != nil {
		return stageevidence.FlipSet{}, fmt.Errorf("%w: %v", stageevidence.ErrLedgerBroken, err)
	}
	if _, err := l.verifyChain(segs, live, tip.check); err != nil {
		return stageevidence.FlipSet{}, fmt.Errorf("%w: %v", stageevidence.ErrLedgerBroken, err)
	}
	return flipSetOf(chainOrder(segs, live))
}

func flipSetOf(lines [][]byte) (stageevidence.FlipSet, error) {
	var set stageevidence.FlipSet
	for _, line := range lines {
		if !bytes.Contains(line, []byte(`"`+stageevidence.FlipKind+`"`)) && !bytes.Contains(line, []byte(`"`+stageevidence.GenesisKind+`"`)) {
			continue
		}
		var e core.LedgerEntry
		if err := json.Unmarshal(line, &e); err != nil {
			return stageevidence.FlipSet{}, fmt.Errorf("%w: stage flip line is not an entry: %v", stageevidence.ErrLedgerBroken, err)
		}
		if err := absorbFlipEntry(&set, e); err != nil {
			return stageevidence.FlipSet{}, err
		}
	}
	return set, nil
}

func absorbFlipEntry(set *stageevidence.FlipSet, e core.LedgerEntry) error {
	switch e.Kind {
	case stageevidence.FlipKind:
		flip, err := stageevidence.ParseFlip(e.Message)
		if err != nil {
			return err
		}
		set.Flips = append(set.Flips, flip)
	case stageevidence.GenesisKind:
		genesis, err := stageevidence.ParseGenesis(e.Message)
		if err != nil {
			return err
		}
		if set.Genesis == nil {
			set.Genesis = &genesis
		}
	}
	return nil
}
