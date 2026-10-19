package fleet

import (
	"context"
	"errors"
	"runtime"
	"slices"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

var ErrNeverLaunched = errors.New("fleet: the pool never launched this todo")

type PoolConfig struct {
	Target      int
	Concurrency int
	MaxLaunches int
	Next        func(inFlight, pending []Todo) int
	Prepare     func(td Todo, spec CycleSpec) (CycleSpec, bool)
	Resupply    func(held []Todo) []Todo
	Halted      func() bool
	Kick        <-chan struct{}
	Window      func() int
}

type PoolTransitionKind string

const (
	PoolLaunched   PoolTransitionKind = "launched"
	PoolExited     PoolTransitionKind = "exited"
	PoolBackfilled PoolTransitionKind = "backfilled"
	PoolResupplied PoolTransitionKind = "resupplied"
	PoolIdle       PoolTransitionKind = "idle"
)

type PoolTransition struct {
	Kind    PoolTransitionKind
	Todo    string
	Live    int
	Target  int
	Pending int
}

type pool struct {
	ctx          context.Context
	cfg          PoolConfig
	backlog      []Todo
	launch       LaunchFn
	onTransition func(PoolTransition)
	limit        int
	results      []Result
	pending      []int
	running      map[int]bool
	claimed      map[string]bool
	known        map[string]bool
	launches     int
	completions  chan completion
}

type completion struct {
	idx    int
	result Result
}

func RunPool(ctx context.Context, cfg PoolConfig, backlog []Todo, launch LaunchFn, onTransition func(PoolTransition)) []Result {
	if launch == nil {
		return noLaunchResults(len(backlog))
	}
	p := newPool(ctx, cfg, backlog, launch, onTransition)
	p.fill(PoolLaunched, true)
	kick := cfg.Kick
	for len(p.running) > 0 {
		select {
		case done := <-p.completions:
			p.results[done.idx] = done.result
			p.settle(done.idx)
		case _, open := <-kick:
			if !open {
				kick = nil
				continue
			}
			p.fill(PoolBackfilled, false)
		}
	}
	return p.results
}

func newPool(ctx context.Context, cfg PoolConfig, backlog []Todo, launch LaunchFn, onTransition func(PoolTransition)) *pool {
	limit := cfg.Target
	if cfg.Concurrency > 0 && cfg.Concurrency < limit {
		limit = cfg.Concurrency
	}
	p := &pool{ctx: ctx, cfg: cfg, launch: launch, onTransition: onTransition,
		limit: max(limit, 1), running: map[int]bool{}, claimed: map[string]bool{}, known: map[string]bool{}}
	p.completions = make(chan completion, p.limit)
	for _, td := range backlog {
		p.add(td)
	}
	return p
}

func (p *pool) add(td Todo) {
	idx := len(p.backlog)
	p.known[td.ID] = true
	p.backlog = append(p.backlog, td)
	p.pending = append(p.pending, idx)
	p.results = append(p.results, Result{Index: idx, ExitCode: -1, Err: ErrNeverLaunched})
}

func (p *pool) resupply() bool {
	if p.cfg.Resupply == nil || !p.open() {
		return false
	}
	added := false
	for _, td := range p.cfg.Resupply(slices.Clone(p.backlog)) {
		if p.known[td.ID] {
			continue
		}
		p.add(td)
		p.emit(PoolResupplied, td.ID)
		added = true
	}
	return added
}

func (p *pool) emit(kind PoolTransitionKind, todo string) {
	if p.onTransition != nil {
		p.onTransition(PoolTransition{Kind: kind, Todo: todo, Live: len(p.running), Target: p.cfg.Target, Pending: len(p.pending)})
	}
}

func (p *pool) settle(idx int) {
	for f := range normalizeFiles(p.backlog[idx].Files) {
		delete(p.claimed, f)
	}
	delete(p.running, idx)
	p.emit(PoolExited, p.backlog[idx].ID)
	if p.fill(PoolBackfilled, false) {
		return
	}
	if !p.resupply() || !p.fill(PoolBackfilled, false) {
		p.emit(PoolIdle, "")
	}
}

func (p *pool) width() int {
	if p.cfg.Window == nil {
		return p.limit
	}
	return max(1, min(p.limit, p.cfg.Window()))
}

func (p *pool) fill(kind PoolTransitionKind, initial bool) (full bool) {
	width := p.width()
	for len(p.running) < width && len(p.pending) > 0 && p.open() {
		at := p.next(initial)
		if at < 0 && len(p.running) > 0 {
			break
		}
		at = max(at, 0)
		idx := p.pending[at]
		p.pending = append(p.pending[:at:at], p.pending[at+1:]...)
		p.dispatch(idx, kind)
	}
	return len(p.running) >= width
}

func (p *pool) open() bool {
	if p.cfg.MaxLaunches > 0 && p.launches >= p.cfg.MaxLaunches {
		return false
	}
	return p.cfg.Halted == nil || !p.cfg.Halted()
}

func (p *pool) next(initial bool) int {
	if p.cfg.Next != nil {
		return p.cfg.Next(p.todos(p.inFlight()), p.todos(p.pending))
	}
	if initial {
		for at, idx := range p.pending {
			if p.disjoint(idx) {
				return at
			}
		}
		return -1
	}
	return p.selectDisjoint()
}

func (p *pool) inFlight() []int {
	out := make([]int, 0, len(p.running))
	for i := range p.backlog {
		if p.running[i] {
			out = append(out, i)
		}
	}
	return out
}

func (p *pool) todos(idxs []int) []Todo {
	out := make([]Todo, len(idxs))
	for i, idx := range idxs {
		out[i] = p.backlog[idx]
	}
	return out
}

func (p *pool) disjoint(idx int) bool {
	for f := range normalizeFiles(p.backlog[idx].Files) {
		if p.claimed[f] {
			return false
		}
	}
	return true
}

func (p *pool) selectDisjoint() int {
	best := -1
	for at, idx := range p.pending {
		if !p.disjoint(idx) {
			continue
		}
		if best < 0 || p.backlog[idx].Priority > p.backlog[p.pending[best]].Priority {
			best = at
		}
	}
	return best
}

func (p *pool) dispatch(idx int, kind PoolTransitionKind) {
	td := p.backlog[idx]
	spec := poolSpec(td, p.limit)
	if p.cfg.Prepare != nil {
		prepared, ok := p.cfg.Prepare(td, spec)
		if !ok {
			return
		}
		spec = prepared
	}
	for f := range normalizeFiles(td.Files) {
		p.claimed[f] = true
	}
	p.running[idx] = true
	p.launches++
	p.emit(kind, td.ID)
	started := make(chan struct{})
	go func() {
		started <- struct{}{}
		code, err := p.launch(p.ctx, spec)
		p.completions <- completion{idx: idx, result: Result{Index: idx, ExitCode: code, Err: err}}
	}()
	<-started
	runtime.Gosched()
}

func noLaunchResults(n int) []Result {
	results := make([]Result, n)
	for i := range results {
		results[i] = Result{Index: i, ExitCode: -1, Err: errNoLaunch}
	}
	return results
}

func poolSpec(td Todo, width int) CycleSpec {
	env := map[string]string{
		ipcenv.FleetScopeKey: td.ID,
		ipcenv.FleetKey:      "1",
	}
	if width > 0 {
		env[ipcenv.FleetWidthKey] = strconv.Itoa(width)
	}
	return CycleSpec{
		Scope:          []string{td.ID},
		OutputContract: td.OutputContract,
		Optional:       td.Optional,
		Env:            env,
	}
}
