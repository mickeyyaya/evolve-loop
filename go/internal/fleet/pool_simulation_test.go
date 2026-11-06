package fleet_test

import (
	"context"
	"fmt"
	"math/rand/v2"
	"slices"
	"strconv"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/admission"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/orchestrator"
)

const (
	simSeeds       = 200
	simMaxDuration = 20
)

type simPlan struct {
	seed      uint64
	target    int
	cap       int
	windowed  bool
	conflicts bool
	admitted  bool
	todos     []fleet.Todo
	duration  map[string]int
	arrival   map[string]int
}

func (pl *simPlan) limit() int {
	if pl.cap > 0 && pl.cap < pl.target {
		return pl.cap
	}
	return pl.target
}

func (pl *simPlan) grahamBound() bool {
	return !pl.conflicts && !pl.windowed
}

func simWindowAt(seed uint64, target, now int) int {
	return 1 + (now+int(seed%7))%target
}

type fileGraph struct{}

func (fileGraph) Packages(files []string) []string { return files }
func (fileGraph) Deps(string) []string             { return nil }
func (fileGraph) Hubs() map[string]bool            { return map[string]bool{} }

func newSimPlan(seed uint64) *simPlan {
	rng := rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))
	pl := &simPlan{
		seed:      seed,
		target:    2 + rng.IntN(7),
		windowed:  seed%4 >= 2,
		conflicts: seed%2 == 1,
		admitted:  seed%3 != 0,
		duration:  map[string]int{},
		arrival:   map[string]int{},
	}
	if rng.IntN(3) == 0 {
		pl.cap = 1 + rng.IntN(pl.target)
	}
	files := 3 + rng.IntN(3)
	count := 0
	newTodo := func() fleet.Todo {
		count++
		id := "t" + strconv.Itoa(count)
		td := fleet.Todo{ID: id, Priority: rng.IntN(6)}
		if pl.conflicts {
			td.Files = []string{"f" + strconv.Itoa(rng.IntN(files))}
			if rng.IntN(3) == 0 {
				td.Files = append(td.Files, "f"+strconv.Itoa(rng.IntN(files)))
			}
		} else {
			td.Files = []string{"own-" + id}
		}
		pl.duration[id] = 1 + rng.IntN(simMaxDuration)
		return td
	}
	initial := 1 + rng.IntN(5*pl.target)
	shortest := simMaxDuration
	for range initial {
		td := newTodo()
		pl.todos = append(pl.todos, td)
		shortest = min(shortest, pl.duration[td.ID])
	}
	if rng.IntN(2) == 0 {
		for range 1 + rng.IntN(6) {
			td := newTodo()
			pl.todos = append(pl.todos, td)
			pl.arrival[td.ID] = 1 + rng.IntN(shortest)
		}
	}
	return pl
}

type simRun struct {
	pl         *simPlan
	mu         sync.Mutex
	now        int
	known      map[string]bool
	launched   map[string]bool
	delivered  map[string]bool
	end        map[string]int
	running    map[string]bool
	gates      map[string]chan struct{}
	closed     map[string]bool
	exited     chan string
	failure    string
	checks     int
	deliveries int
	lastDeliv  int
	cmax       int
	byID       map[string]fleet.Todo
}

func newSimRun(pl *simPlan) *simRun {
	s := &simRun{
		pl:        pl,
		known:     map[string]bool{},
		launched:  map[string]bool{},
		delivered: map[string]bool{},
		end:       map[string]int{},
		running:   map[string]bool{},
		gates:     map[string]chan struct{}{},
		closed:    map[string]bool{},
		exited:    make(chan string, 1),
		byID:      map[string]fleet.Todo{},
	}
	for _, td := range pl.todos {
		s.gates[td.ID] = make(chan struct{})
		s.byID[td.ID] = td
		if _, late := pl.arrival[td.ID]; !late {
			s.known[td.ID] = true
		}
	}
	return s
}

func (s *simRun) fail(format string, args ...any) {
	if s.failure == "" {
		s.failure = fmt.Sprintf("seed=%d target=%d cap=%d windowed=%t conflicts=%t admission=%t now=%d: ", s.pl.seed, s.pl.target, s.pl.cap, s.pl.windowed, s.pl.conflicts, s.pl.admitted, s.now) + fmt.Sprintf(format, args...)
	}
}

func (s *simRun) width() int {
	if !s.pl.windowed {
		return s.pl.limit()
	}
	return max(1, min(s.pl.limit(), simWindowAt(s.pl.seed, s.pl.target, s.now)))
}

func (s *simRun) window() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return simWindowAt(s.pl.seed, s.pl.target, s.now)
}

func (s *simRun) claimed() map[string]bool {
	out := map[string]bool{}
	for id := range s.running {
		for _, f := range s.byID[id].Files {
			out[f] = true
		}
	}
	return out
}

func (s *simRun) admissible(id string, claimed map[string]bool) bool {
	for _, f := range s.byID[id].Files {
		if claimed[f] {
			return false
		}
	}
	return true
}

func (s *simRun) waiting() []string {
	claimed := s.claimed()
	var out []string
	for _, td := range s.pl.todos {
		if s.launched[td.ID] || !s.admissible(td.ID, claimed) {
			continue
		}
		if s.known[td.ID] || s.pl.arrival[td.ID] <= s.now && s.pl.arrival[td.ID] > 0 {
			out = append(out, td.ID)
		}
	}
	return out
}

func (s *simRun) checkNoIdleSlot(where string) {
	s.checks++
	if wait := s.waiting(); len(s.running) < s.width() && len(wait) > 0 {
		s.fail("%s: %d of %d slots are busy while %v waits and is admissible", where, len(s.running), s.width(), wait)
	}
}

func (s *simRun) onTransition(tr fleet.PoolTransition) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if tr.Live > tr.Target || len(s.running) > s.pl.target {
		s.fail("%s: live %d, tracked %d, target %d", tr.Kind, tr.Live, len(s.running), tr.Target)
	}
	switch tr.Kind {
	case fleet.PoolLaunched, fleet.PoolBackfilled:
		s.onLaunch(tr)
	case fleet.PoolExited:
		s.cmax = max(s.cmax, s.end[tr.Todo])
		s.now = s.end[tr.Todo]
		delete(s.running, tr.Todo)
		select {
		case s.exited <- tr.Todo:
		default:
		}
	case fleet.PoolIdle:
		s.checkNoIdleSlot("PoolIdle")
	}
}

func (s *simRun) sharedWithRunning(id string) (string, string, bool) {
	mine := map[string]bool{}
	for _, f := range s.byID[id].Files {
		mine[f] = true
	}
	for _, other := range s.pl.todos {
		if !s.running[other.ID] {
			continue
		}
		for _, f := range other.Files {
			if mine[f] {
				return other.ID, f, true
			}
		}
	}
	return "", "", false
}

func (s *simRun) onLaunch(tr fleet.PoolTransition) {
	if !s.known[tr.Todo] || s.launched[tr.Todo] {
		s.fail("%s of %q, which the pool does not hold as pending", tr.Kind, tr.Todo)
		return
	}
	if other, file, shared := s.sharedWithRunning(tr.Todo); shared {
		s.fail("%s of %q while running lane %q holds the same file %q", tr.Kind, tr.Todo, other, file)
	}
	s.launched[tr.Todo] = true
	s.running[tr.Todo] = true
	s.end[tr.Todo] = s.now + s.pl.duration[tr.Todo]
	if tr.Live != len(s.running) {
		s.fail("%s: pool reports live %d, simulation tracks %d", tr.Kind, tr.Live, len(s.running))
	}
}

func (s *simRun) resupply(held []fleet.Todo) []fleet.Todo {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []fleet.Todo
	expected := len(s.known)
	for _, td := range s.pl.todos {
		at, late := s.pl.arrival[td.ID]
		if late && !s.delivered[td.ID] && at <= s.now {
			s.delivered[td.ID] = true
			s.known[td.ID] = true
			s.lastDeliv = max(s.lastDeliv, s.now)
			s.deliveries++
			out = append(out, td)
		}
	}
	if len(held) != expected {
		s.fail("resupply held %d todos, simulation expects %d", len(held), expected)
	}
	return out
}

func (s *simRun) launch(_ context.Context, spec fleet.CycleSpec) (int, error) {
	<-s.gates[spec.Scope[0]]
	return 0, nil
}

func (s *simRun) release(id string) {
	if !s.closed[id] {
		s.closed[id] = true
		close(s.gates[id])
	}
}

func (s *simRun) releaseAll() {
	for id := range s.gates {
		s.release(id)
	}
}

func (s *simRun) earliest() (string, bool) {
	best, found := "", false
	for _, td := range s.pl.todos {
		if !s.running[td.ID] {
			continue
		}
		if !found || s.end[td.ID] < s.end[best] {
			best, found = td.ID, true
		}
	}
	return best, found
}

func (s *simRun) drive(kick chan<- struct{}, done <-chan struct{}) {
	for {
		select {
		case kick <- struct{}{}:
		case <-done:
			return
		}
		s.mu.Lock()
		s.checkNoIdleSlot("settled")
		id, ok := s.earliest()
		bad := s.failure != ""
		s.mu.Unlock()
		if bad || !ok {
			s.releaseAll()
			<-done
			return
		}
		s.release(id)
		select {
		case <-s.exited:
		case <-done:
			return
		}
	}
}

func (s *simRun) verify(results []fleet.Result) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failure != "" {
		return
	}
	s.checks++
	if len(results) != len(s.pl.todos) {
		s.fail("%d results for %d todos", len(results), len(s.pl.todos))
		return
	}
	for i, r := range results {
		if r.Err != nil || r.ExitCode != 0 {
			s.fail("todo %s never ran: exit %d, err %v", s.pl.todos[i].ID, r.ExitCode, r.Err)
			return
		}
	}
	total := 0
	longest := 0
	for _, td := range s.pl.todos {
		total += s.pl.duration[td.ID]
		longest = max(longest, s.pl.duration[td.ID])
	}
	if s.pl.grahamBound() {
		m := s.pl.limit()
		if s.cmax*m > m*(s.lastDeliv+longest)+total {
			s.fail("Cmax %d passes r_max %d + sum %d / m %d + pmax %d", s.cmax, s.lastDeliv, total, m, longest)
		}
		return
	}
	if s.cmax > total {
		s.fail("Cmax %d passes the sum of lane times %d", s.cmax, total)
	}
}

func (s *simRun) config(kick <-chan struct{}) fleet.PoolConfig {
	cfg := fleet.PoolConfig{Target: s.pl.target, Concurrency: s.pl.cap, Kick: kick}
	if s.pl.admitted {
		cfg.Next = orchestrator.AdmitByCell(admission.Admitter{Graph: fileGraph{}})
	}
	if len(s.pl.arrival) > 0 {
		cfg.Resupply = s.resupply
	}
	if s.pl.windowed {
		cfg.Window = s.window
	}
	return cfg
}

func runWaveSeed(seed uint64) *simRun {
	pl := newSimPlan(seed)
	s := newSimRun(pl)
	var initial []fleet.Todo
	for _, td := range pl.todos {
		if s.known[td.ID] {
			initial = append(initial, td)
		}
	}
	kick := make(chan struct{})
	done := make(chan struct{})
	var results []fleet.Result
	go func() {
		defer close(done)
		results = fleet.RunPool(context.Background(), s.config(kick), slices.Clone(initial), s.launch, s.onTransition)
	}()
	s.drive(kick, done)
	<-done
	s.verify(results)
	return s
}

func TestWaveSimulation(t *testing.T) {
	var graham, serial, arrivals, checks, admitted int
	for seed := uint64(1); seed <= simSeeds; seed++ {
		s := runWaveSeed(seed)
		if s.failure != "" {
			t.Fatalf("wave simulation failed, replay with seed=%d\n%s", seed, s.failure)
		}
		if s.pl.grahamBound() {
			graham++
		} else {
			serial++
		}
		if s.deliveries > 0 {
			arrivals++
		}
		if s.pl.admitted {
			admitted++
		}
		checks += s.checks
	}
	t.Logf("wave-simulation seeds=%d graham=%d serial=%d arrivals=%d checks=%d admission=%d", simSeeds, graham, serial, arrivals, checks, admitted)
}

func TestWaveSimulationReplaysTheSameSeedIdentically(t *testing.T) {
	first, second := runWaveSeed(37), runWaveSeed(37)
	if first.failure != "" || second.failure != "" {
		t.Fatalf("seed=37 failed: %q %q", first.failure, second.failure)
	}
	if first.cmax != second.cmax || first.checks != second.checks || first.deliveries != second.deliveries {
		t.Errorf("seed=37 replayed differently: cmax %d/%d checks %d/%d deliveries %d/%d", first.cmax, second.cmax, first.checks, second.checks, first.deliveries, second.deliveries)
	}
}
