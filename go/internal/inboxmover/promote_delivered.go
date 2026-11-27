package inboxmover

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/integration"
)

type Promoter struct {
	Options Options
}

func (p Promoter) Promote(ctx context.Context, rec integration.DeliverableRecord) error {
	opts, err := p.Options.resolved()
	if err != nil {
		return err
	}
	if rec.Cycle <= 0 {
		return nil
	}
	committed, ok := committedset.ReadRecord(opts.ProjectRoot, rec.Cycle)
	if !ok {
		return nil
	}
	var errs []error
	for _, id := range dedupeIDs(committed.ItemIDs) {
		src, delivered := deliveredAt(opts.InboxDir, id)
		if !delivered {
			continue
		}
		if err := promoteDeliveredFile(ctx, opts, rec.Cycle, id, src); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func promoteDeliveredFile(ctx context.Context, opts Options, cycle int, id, src string) error {
	base := filepath.Base(src)
	destDir := inboxbatch.CycleDir(filepath.Join(opts.InboxDir, StateProcessed), strconv.Itoa(cycle))
	dest := filepath.Join(destDir, base)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("%w: %v", ErrMvFailed, err)
	}
	if err := os.Rename(src, dest); err != nil {
		return fmt.Errorf("%w: %v", ErrMvFailed, err)
	}
	releaseContinuationOnRetire(opts, dest, id, "integration-merged")
	opts.logf("", "promoted: %s → %s/", base, StateProcessed)
	if err := opts.Ledger.AppendLifecycle(ctx, ledger.LifecycleRecord{
		TS:      opts.Now().UTC().Format(time.RFC3339),
		Action:  "promote",
		TaskID:  id,
		Cycle:   cycle,
		Message: ".evolve/inbox/delivered/" + base + " → .evolve/inbox/" + StateProcessed + "/cycle-" + strconv.Itoa(cycle) + "/" + base + ": integration-merged",
	}); err != nil {
		opts.logf("WARN: ", "promote %s: ledger append: %v", id, err)
	}
	return nil
}
