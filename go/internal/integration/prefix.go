package integration

import (
	"context"
	"errors"
	"fmt"
)

type PrefixResult struct {
	Batch   Batch
	Landed  []int
	Ejected []*Ejection
}

func (c Composer) PrefixCheck(ctx context.Context, b Batch, recs []DeliverableRecord) (PrefixResult, error) {
	if err := c.checkPrefix(b, recs); err != nil {
		return PrefixResult{Batch: b}, err
	}
	res := PrefixResult{Batch: b}
	for _, rec := range recs {
		st, err := c.attempt(ctx, res.Batch, rec)
		var ej *Ejection
		switch {
		case errors.As(err, &ej):
			res.Ejected = append(res.Ejected, ej)
		case err != nil:
			return PrefixResult{Batch: b}, err
		default:
			res.Batch = st.Batch
			res.Landed = append(res.Landed, rec.Cycle)
		}
	}
	if len(res.Landed) == 0 {
		return res, nil
	}
	if err := c.Repo.moveBranch(ctx, b, res.Batch.Tip); err != nil {
		return PrefixResult{Batch: b}, err
	}
	return res, nil
}

func (c Composer) checkPrefix(b Batch, recs []DeliverableRecord) error {
	errs := []error{c.ready(b)}
	seen := map[int]bool{}
	for _, rec := range recs {
		if seen[rec.Cycle] {
			errs = append(errs, fmt.Errorf("integration: cycle %d is in the batch twice", rec.Cycle))
		}
		seen[rec.Cycle] = true
		errs = append(errs, checkComposable(rec))
	}
	return errors.Join(errs...)
}
