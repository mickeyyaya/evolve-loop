package publisher

import (
	"context"
	"fmt"
	"slices"
	"time"
	"unicode/utf8"

	"github.com/mickeyyaya/evolve-loop/go/internal/events/channel"
	"github.com/mickeyyaya/evolve-loop/go/internal/events/filter"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type QoS string

const (
	QoSLossless   QoS = "lossless"
	QoSBestEffort QoS = "best_effort"
)

const MaxDispatchBytes = 256

type Channel struct {
	Name         string
	Route        string
	QoS          QoS
	SegmentBytes int64
}

type Config struct {
	Root            string
	Role            string
	Dispatch        string
	Channels        []Channel
	Log             channel.Config
	QueueEvents     int
	QueueBytes      int
	EnqueueDeadline time.Duration
}

type Publisher struct {
	stamp  channel.Record
	routes []*route
}

type appender interface {
	Append(records []channel.Record) (int64, error)
}

type opener func(root, name string, cfg channel.Config) (appender, error)

func New(cfg Config) (*Publisher, error) { return newPublisher(cfg, openLog) }

func Roles() []string {
	return []string{"cycle", "loop", "loop-chain", "ship", "subagent", "simulate", "phase-observer", "wave", "inbox", "ci", "subscriber"}
}

func openLog(root, name string, cfg channel.Config) (appender, error) {
	return channel.New(root, name, cfg)
}

func newPublisher(cfg Config, open opener) (*Publisher, error) {
	if err := cfg.check(); err != nil {
		return nil, err
	}
	stamp := channel.Record{Source: cfg.Role, Dispatch: cutDispatch(cfg.Dispatch)}
	catalog := filter.RegisteredCatalog()
	routes := make([]*route, 0, len(cfg.Channels))
	for _, ch := range cfg.Channels {
		r, err := newRoute(cfg, ch, catalog, stamp, open)
		if err != nil {
			return nil, err
		}
		routes = append(routes, r)
	}
	for _, r := range routes {
		r.start()
	}
	return &Publisher{stamp: stamp, routes: routes}, nil
}

func (c Config) check() error {
	if !slices.Contains(Roles(), c.Role) {
		return fmt.Errorf("publisher: role %q is not one of %v", c.Role, Roles())
	}
	if c.QueueEvents < 1 || c.QueueBytes < 1 {
		return fmt.Errorf("publisher: queue_events %d and queue_bytes %d must be 1 or more", c.QueueEvents, c.QueueBytes)
	}
	seen := map[string]bool{}
	for _, ch := range c.Channels {
		if seen[ch.Name] {
			return fmt.Errorf("publisher: channel %s is configured twice", ch.Name)
		}
		seen[ch.Name] = true
	}
	return nil
}

func cutDispatch(id string) string {
	if len(id) <= MaxDispatchBytes {
		return id
	}
	n := MaxDispatchBytes
	for !utf8.RuneStart(id[n]) {
		n--
	}
	return id[:n]
}

func (p *Publisher) Listen(e signalcenter.Event) {
	rec := p.stamp
	rec.Signal = &e
	match := filter.Record{Source: rec.Source, Signal: &e}
	for _, r := range p.routes {
		if r.filter.Match(match) {
			r.publish(rec)
		}
	}
}

func (p *Publisher) Close(deadline time.Duration) int {
	ctx, cancel := context.WithTimeout(context.Background(), deadline)
	defer cancel()
	return p.closeBy(ctx)
}

func (p *Publisher) closeBy(ctx context.Context) int {
	p.stopWriters(ctx)
	lost := 0
	for _, r := range p.routes {
		r.awaitWriter()
		lost += r.flushGaps(ctx)
	}
	return lost
}

func (p *Publisher) stopWriters(ctx context.Context) {
	for _, r := range p.routes {
		r.stop(ctx)
	}
}
