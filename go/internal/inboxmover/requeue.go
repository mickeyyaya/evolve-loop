package inboxmover

import (
	"context"
	"encoding/json"
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

type Requeuer struct {
	Options Options
}

func (r Requeuer) Requeue(ctx context.Context, rec integration.DeliverableRecord, briefPath string) error {
	opts, err := r.Options.resolved()
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
		if err := requeueDeliveredFile(ctx, opts, rec.Cycle, id, src, briefPath); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func requeueDeliveredFile(ctx context.Context, opts Options, cycle int, id, src, briefPath string) error {
	base := filepath.Base(src)
	dest := filepath.Join(opts.InboxDir, base)
	if _, err := os.Stat(dest); err == nil {
		return fmt.Errorf("%w: %s already at inbox root", ErrMvFailed, base)
	}
	if err := UpdateItemJSON(src, func(item map[string]json.RawMessage) {
		count := 0
		if raw, ok := item[inboxbatch.FailureCountField]; ok {
			_ = json.Unmarshal(raw, &count)
		}
		item[inboxbatch.FailureCountField] = json.RawMessage(strconv.Itoa(count + 1))
		if briefPath != "" {
			encoded, _ := json.Marshal(briefPath)
			item[inboxbatch.BriefPathField] = encoded
		}
	}); err != nil {
		return err
	}
	if err := os.Rename(src, dest); err != nil {
		return fmt.Errorf("%w: %v", ErrMvFailed, err)
	}
	opts.logf("", "requeued: %s ← delivered/", base)
	if err := opts.Ledger.AppendLifecycle(ctx, ledger.LifecycleRecord{
		TS:      opts.Now().UTC().Format(time.RFC3339),
		Action:  "requeue",
		TaskID:  id,
		Cycle:   cycle,
		Message: ".evolve/inbox/delivered/" + base + " → .evolve/inbox/" + base + ": integration-ejected",
	}); err != nil {
		opts.logf("WARN: ", "requeue %s: ledger append: %v", id, err)
	}
	return nil
}
