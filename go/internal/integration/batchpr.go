package integration

import (
	"context"
	"errors"
	"fmt"
)

type Verdict string

const (
	Green Verdict = "green"
	Red   Verdict = "red"
)

const MaxMoves = 3

var ErrCIRed = errors.New("integration: the CI of the batch is red")

type PR struct {
	Number int
	Tip    string
}

type CI interface {
	Open(ctx context.Context, b Batch) (PR, error)
	Watch(ctx context.Context, pr PR) (Verdict, error)
	Rerun(ctx context.Context, pr PR) error
}

type Main interface {
	Head(ctx context.Context) (string, error)
	FastForward(ctx context.Context, tip string) error
}

type Composition struct {
	Batch   Batch
	Kept    []DeliverableRecord
	Ejected []*Ejection
}

type Merger struct {
	CI      CI
	Main    Main
	Compose func(ctx context.Context, recs []DeliverableRecord) (Composition, error)
	Heads   func(ctx context.Context, recs []DeliverableRecord) ([]*Ejection, error)
	Eject   func(ctx context.Context, rec DeliverableRecord, cause error) error
}

type Outcome struct {
	Landed       []int
	Ejected      []int
	Flaked       bool
	ConfirmedRed bool
	Via          map[int]int
	Inbox        []string
}

func (m Merger) Land(ctx context.Context, recs []DeliverableRecord) (Outcome, error) {
	var out Outcome
	err := m.land(ctx, recs, &out)
	return out, err
}

func (m Merger) land(ctx context.Context, recs []DeliverableRecord, out *Outcome) error {
	if len(recs) == 0 {
		return errors.New("integration: a batch needs at least one deliverable")
	}
	for range MaxMoves + 1 {
		comp, err := m.Compose(ctx, recs)
		if err != nil {
			return err
		}
		if err := m.eject(ctx, recs, comp.Ejected, out); err != nil {
			return err
		}
		if len(comp.Kept) == 0 {
			return nil
		}
		recs = comp.Kept
		b := comp.Batch
		verdict, pr, err := m.test(ctx, b, out)
		if err != nil {
			return err
		}
		if verdict == Red {
			out.ConfirmedRed = true
			return m.split(ctx, recs, out)
		}
		rest, changed, err := m.recheck(ctx, recs, out)
		if err != nil {
			return err
		}
		if changed {
			if len(rest) == 0 {
				return nil
			}
			recs = rest
			continue
		}
		landed, err := m.fastForward(ctx, b)
		if landed {
			out.record(recs, pr)
		}
		if err != nil || landed {
			return err
		}
	}
	return fmt.Errorf("integration: main moved %d times while the batch was tested", MaxMoves+1)
}

func (m Merger) recheck(ctx context.Context, recs []DeliverableRecord, out *Outcome) ([]DeliverableRecord, bool, error) {
	if m.Heads == nil {
		return recs, false, nil
	}
	ejected, err := m.Heads(ctx, recs)
	if err != nil || len(ejected) == 0 {
		return recs, false, err
	}
	rest := withoutEjected(recs, ejected)
	return rest, true, m.eject(ctx, recs, ejected, out)
}

func (m Merger) eject(ctx context.Context, recs []DeliverableRecord, ejected []*Ejection, out *Outcome) error {
	byRef := make(map[int]DeliverableRecord, len(recs))
	for _, r := range recs {
		byRef[r.Ref()] = r
	}
	var errs []error
	for _, ej := range ejected {
		out.Ejected = append(out.Ejected, ej.Ref())
		errs = append(errs, m.Eject(ctx, byRef[ej.Ref()], ej))
	}
	return errors.Join(errs...)
}

func (o *Outcome) record(recs []DeliverableRecord, pr PR) {
	if o.Via == nil {
		o.Via = map[int]int{}
	}
	for _, ref := range refsOf(recs) {
		o.Landed = append(o.Landed, ref)
		o.Via[ref] = pr.Number
	}
}

func (m Merger) test(ctx context.Context, b Batch, out *Outcome) (Verdict, PR, error) {
	pr, err := m.CI.Open(ctx, b)
	if err != nil {
		return "", PR{}, err
	}
	verdict, err := m.CI.Watch(ctx, pr)
	if err != nil || verdict == Green {
		return verdict, pr, err
	}
	if err := m.CI.Rerun(ctx, pr); err != nil {
		return "", pr, err
	}
	if verdict, err = m.CI.Watch(ctx, pr); err == nil && verdict == Green {
		out.Flaked = true
	}
	return verdict, pr, err
}

func (m Merger) fastForward(ctx context.Context, b Batch) (bool, error) {
	head, err := m.Main.Head(ctx)
	if err != nil || head != b.Base {
		return false, err
	}
	if err := m.Main.FastForward(ctx, b.Tip); err != nil {
		return false, err
	}
	return true, nil
}

func (m Merger) split(ctx context.Context, recs []DeliverableRecord, out *Outcome) error {
	if len(recs) == 1 {
		out.Ejected = append(out.Ejected, recs[0].Ref())
		return m.Eject(ctx, recs[0], ErrCIRed)
	}
	half := len(recs) / 2
	if err := m.land(ctx, recs[:half], out); err != nil {
		return err
	}
	return m.land(ctx, recs[half:], out)
}
