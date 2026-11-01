// Package usageprobe is the proactive, per-cycle CLI-cap detector.
// See docs/architecture/packages/internal-usageprobe.md.
package usageprobe

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"sync"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/clicontrol"
	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/quotastate"
)

const benchReason = clihealth.UsageProbePattern

type Prober struct {
	Families []string
	Probe    func(ctx context.Context, family string) (pane string, err error)
	Classify func(family, pane string) bool
	Windows  func(family, pane string) []quotastate.UsageWindow
	Record   func(cli string, windows []quotastate.UsageWindow) error
	Store    *clihealth.Store
	Log      io.Writer
	Timeout  time.Duration
}

type probeRound struct {
	interrupt context.Context
	mu        sync.Mutex
	pending   map[string]bool
	closed    bool
	judging   sync.WaitGroup
}

func newProbeRound(interrupt context.Context, families []string) *probeRound {
	pending := make(map[string]bool, len(families))
	for _, family := range families {
		pending[family] = true
	}
	return &probeRound{interrupt: interrupt, pending: pending}
}

func (r *probeRound) settle(family string) (judged func(), ok bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed || !r.pending[family] {
		return nil, false
	}
	delete(r.pending, family)
	r.judging.Add(1)
	return r.judging.Done, true
}

func (r *probeRound) close() []string {
	r.mu.Lock()
	r.closed = true
	left := make([]string, 0, len(r.pending))
	for family := range r.pending {
		left = append(left, family)
	}
	r.mu.Unlock()
	r.judging.Wait()
	sort.Strings(left)
	return left
}

func (p *Prober) Run(ctx context.Context) {
	if ctx.Err() != nil {
		fmt.Fprintf(p.Log, "[usage-probe] cancelled before any probe ran (%v) — the loop's interrupt wins\n", ctx.Err())
		return
	}
	probeCtx, cancel := p.bounded(ctx)
	defer cancel()
	families := p.unbenched()
	round := newProbeRound(ctx, families)
	var wg sync.WaitGroup
	for _, family := range families {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p.probeOne(probeCtx, round, family)
		}()
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-probeCtx.Done():
		p.abandon(ctx, round.close())
	}
}

func (p *Prober) unbenched() []string {
	active := p.Store.Active()
	var families []string
	for _, family := range p.Families {
		if _, benched := active[family]; !benched {
			families = append(families, family)
		}
	}
	return families
}

func (p *Prober) bounded(ctx context.Context) (context.Context, context.CancelFunc) {
	if p.Timeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, p.Timeout)
}

func (p *Prober) abandon(ctx context.Context, left []string) {
	if ctx.Err() != nil {
		fmt.Fprintf(p.Log, "[usage-probe] cancelled (%v) — abandoning %d in-flight probe(s); the loop's interrupt wins\n", ctx.Err(), len(left))
		return
	}
	for _, family := range left {
		fmt.Fprintf(p.Log, "[usage-probe] WARN %s: no usage read within %s; nothing benched\n", family, p.Timeout)
	}
}

func (p *Prober) probeOne(probeCtx context.Context, round *probeRound, family string) {
	pane, err := p.Probe(probeCtx, family)
	judged, ok := round.settle(family)
	if !ok {
		return
	}
	defer judged()
	p.judge(round.interrupt, family, pane, err)
}

func (p *Prober) judge(ctx context.Context, family, pane string, err error) {
	switch {
	case ctx.Err() != nil, errors.Is(err, clicontrol.ErrUnsupported):
		return
	case err != nil:
		fmt.Fprintf(p.Log, "[usage-probe] WARN %s: usage probe failed: %v; nothing benched\n", family, err)
		return
	}
	r := p.read(family, pane)
	p.record(family, r)
	p.act(family, r)
}
