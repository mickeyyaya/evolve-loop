package integration

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/prefixqueue"
)

type PrefixResult struct {
	Batch   Batch
	Landed  []int
	Ejected []*Ejection
}

type prefixRun struct {
	ctx       context.Context
	c         Composer
	b         Batch
	recs      map[string]DeliverableRecord
	tips      map[string]string
	ejections map[string]*Ejection
	fatal     error
}

func (c Composer) PrefixCheck(ctx context.Context, b Batch, recs []DeliverableRecord) (PrefixResult, error) {
	run, err := c.newPrefixRun(ctx, b, recs)
	if err != nil || len(recs) == 0 {
		return PrefixResult{Batch: b}, err
	}
	q := prefixqueue.NewPrefixQueue()
	for _, rec := range recs {
		q.Enqueue(prefixqueue.LaneCandidate{ID: strconv.Itoa(rec.Cycle), Tier: prefixqueue.TierRollup})
	}
	landed, ejected := q.ResolveCulprit(run.verify)
	if run.fatal != nil {
		return PrefixResult{Batch: b}, run.fatal
	}
	return run.result(landed, ejected)
}

func (c Composer) newPrefixRun(ctx context.Context, b Batch, recs []DeliverableRecord) (*prefixRun, error) {
	run := &prefixRun{ctx: ctx, c: c, b: b, recs: map[string]DeliverableRecord{}, tips: map[string]string{}, ejections: map[string]*Ejection{}}
	errs := []error{c.ready(b)}
	for _, rec := range recs {
		id := strconv.Itoa(rec.Cycle)
		if _, dup := run.recs[id]; dup {
			errs = append(errs, fmt.Errorf("integration: cycle %d is in the batch twice", rec.Cycle))
		}
		run.recs[id] = rec
		errs = append(errs, checkComposable(rec))
	}
	return run, errors.Join(errs...)
}

func (p *prefixRun) verify(ids []string) bool {
	if p.fatal != nil {
		return false
	}
	_, ok := p.tipOf(ids)
	return ok
}

func (p *prefixRun) tipOf(ids []string) (string, bool) {
	if len(ids) == 0 {
		return p.b.Tip, true
	}
	key := strings.Join(ids, ",")
	if tip, seen := p.tips[key]; seen {
		return tip, tip != ""
	}
	last := len(ids) - 1
	tip := ""
	if met, ok := p.tipOf(ids[:last]); ok {
		tip = p.step(met, ids[last])
	}
	p.tips[key] = tip
	return tip, tip != ""
}

func (p *prefixRun) step(met, id string) string {
	b := p.b
	b.Tip = met
	st, err := p.c.attempt(p.ctx, b, p.recs[id])
	var ej *Ejection
	switch {
	case errors.As(err, &ej):
		p.ejections[id] = ej
	case err != nil:
		p.fatal = err
	}
	return st.Batch.Tip
}

func (p *prefixRun) result(landed, ejected []string) (PrefixResult, error) {
	res := PrefixResult{Batch: p.b}
	for _, id := range landed {
		res.Landed = append(res.Landed, p.recs[id].Cycle)
	}
	for _, id := range ejected {
		res.Ejected = append(res.Ejected, p.ejections[id])
	}
	if len(landed) == 0 {
		return res, nil
	}
	res.Batch.Tip, _ = p.tipOf(landed)
	if err := p.c.Repo.moveBranch(p.ctx, p.b, res.Batch.Tip); err != nil {
		return PrefixResult{Batch: p.b}, err
	}
	return res, nil
}
