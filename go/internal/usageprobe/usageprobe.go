// Package usageprobe is the proactive, per-cycle CLI-cap detector.
// See docs/architecture/packages/internal-usageprobe.md.
package usageprobe

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"

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
}

func (p *Prober) Run(ctx context.Context) {
	if ctx.Err() != nil {
		fmt.Fprintf(p.Log, "[usage-probe] cancelled before any probe ran (%v) — the loop's interrupt wins\n", ctx.Err())
		return
	}
	active := p.Store.Active()
	var wg sync.WaitGroup
	var launched, completed atomic.Int32
	for _, family := range p.Families {
		if _, benched := active[family]; benched {
			continue
		}
		family := family
		launched.Add(1)
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer completed.Add(1)
			p.probeOne(ctx, family)
		}()
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-ctx.Done():
		fmt.Fprintf(p.Log, "[usage-probe] cancelled (%v) — abandoning %d in-flight probe(s); the loop's interrupt wins\n", ctx.Err(), launched.Load()-completed.Load())
	}
}

func (p *Prober) probeOne(ctx context.Context, family string) {
	pane, err := p.Probe(ctx, family)
	if ctx.Err() != nil {
		return
	}
	if errors.Is(err, clicontrol.ErrUnsupported) {
		return
	}
	if err != nil {
		fmt.Fprintf(p.Log, "[usage-probe] %s probe error: %v (skip, advisory)\n", family, err)
		return
	}
	if ctx.Err() != nil {
		return
	}
	r := p.read(family, pane)
	p.record(family, r)
	p.act(family, r)
}
