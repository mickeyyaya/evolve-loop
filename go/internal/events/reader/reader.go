package reader

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/events/channel"
	"github.com/mickeyyaya/evolve-loop/go/internal/events/filter"
	"github.com/mickeyyaya/evolve-loop/go/internal/events/wake"
)

const readBudget = 1 << 20

var (
	ErrHangup   = errors.New("reader: the reader of the output is gone")
	ErrDeadline = errors.New("reader: the deadline passed")
)

type Waiter interface {
	Arm(wake.Targets) error
	Wait(ctx context.Context, deadline time.Time) (wake.Wake, error)
}

type Channel struct {
	Name  string
	Route filter.Filter
}

type Config struct {
	Root      string
	Catalog   []Channel
	Selectors []string
	Since     Since
}

type Ports struct {
	Waiter  Waiter
	StartOf func(pid int) (string, error)
	Now     func() time.Time
}

type Reader struct {
	cfg       Config
	ports     Ports
	read      func(*channel.Log, int64, int64) (channel.Batch, error)
	routes    map[string]filter.Filter
	wildcard  bool
	opened    bool
	dirty     bool
	armed     wake.Targets
	followers []*follower
	will      *lastWill
	window    *window
}

func New(cfg Config, ports Ports) *Reader {
	routes := map[string]filter.Filter{}
	for _, ch := range cfg.Catalog {
		routes[ch.Name] = ch.Route
	}
	return &Reader{
		cfg:      cfg,
		ports:    ports,
		read:     (*channel.Log).ReadN,
		routes:   routes,
		wildcard: hasWildcard(cfg.Selectors),
		window:   newWindow(windowSize),
	}
}

func (r *Reader) Next(ctx context.Context, deadline time.Time) ([]Item, error) {
	if !r.opened {
		if err := r.open(); err != nil {
			return nil, err
		}
	}
	for {
		items, err := r.settle()
		if err != nil || len(items) > 0 {
			return items, err
		}
		w, err := r.ports.Waiter.Wait(ctx, deadline)
		if err != nil {
			return nil, err
		}
		if err := r.onWake(w); err != nil {
			return nil, err
		}
	}
}

func (r *Reader) open() error {
	if err := os.MkdirAll(r.cfg.Root, dirMode); err != nil {
		return fmt.Errorf("reader: make %s: %w", r.cfg.Root, err)
	}
	if err := r.resolve(); err != nil {
		return err
	}
	will, err := r.startWill()
	if err != nil {
		return err
	}
	r.will = will
	t, err := r.targets()
	if err != nil {
		return err
	}
	if err := r.armWith(t); err != nil {
		return err
	}
	if err := will.seed(); err != nil {
		return err
	}
	r.opened = true
	return nil
}

func (r *Reader) settle() ([]Item, error) {
	for first := true; ; first = false {
		t, err := r.targets()
		if err != nil {
			return nil, err
		}
		changed := r.dirty || !sameTargets(t, r.armed)
		if !changed && !first {
			return nil, nil
		}
		if changed {
			if err := r.rearm(t); err != nil {
				return nil, err
			}
		}
		items, err := r.catchUp()
		if err != nil || len(items) > 0 {
			return items, err
		}
	}
}

func (r *Reader) rearm(t wake.Targets) error {
	if err := r.armWith(t); err != nil {
		return err
	}
	return r.will.verify()
}

func (r *Reader) onWake(w wake.Wake) error {
	switch {
	case w.Hangup:
		return ErrHangup
	case w.Deadline:
		return ErrDeadline
	}
	if w.Changed {
		r.dirty = true
	}
	r.will.exited(w.Exited)
	return nil
}

func (r *Reader) catchUp() ([]Item, error) {
	if r.wildcard {
		if err := r.resolve(); err != nil {
			return nil, err
		}
	}
	lost, err := r.will.catchUp()
	if err != nil {
		return nil, err
	}
	merged, err := r.readFollowers()
	if err != nil {
		r.will.requeue(lost)
		return nil, err
	}
	return r.dedupe(append(merged, r.placeWill(lost)...)), nil
}
