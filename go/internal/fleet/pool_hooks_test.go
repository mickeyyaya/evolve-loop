package fleet

import (
	"context"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type launchLog struct {
	mu  sync.Mutex
	ids []string
}

func (l *launchLog) launch(code int) LaunchFn {
	return func(_ context.Context, spec CycleSpec) (int, error) {
		l.mu.Lock()
		l.ids = append(l.ids, scopeID(spec))
		l.mu.Unlock()
		return code, nil
	}
}

func (l *launchLog) got() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.ids...)
}

func ids(todos []Todo) []string {
	out := make([]string, 0, len(todos))
	for _, td := range todos {
		out = append(out, td.ID)
	}
	return out
}

func TestRunPool_TheNextHookChoosesEachLaunchFromThePendingTodosInOrder(t *testing.T) {
	backlog := []Todo{{ID: "A"}, {ID: "B"}, {ID: "C"}}
	var seen [][]string
	cfg := PoolConfig{Target: 1, Next: func(inFlight, pending []Todo) int {
		if len(inFlight) != 0 {
			t.Errorf("Next saw in-flight %v at width 1, want none", ids(inFlight))
		}
		seen = append(seen, ids(pending))
		return len(pending) - 1
	}}
	var log launchLog
	RunPool(context.Background(), cfg, backlog, log.launch(0), nil)
	if want := []string{"C", "B", "A"}; !reflect.DeepEqual(log.got(), want) {
		t.Errorf("launch order = %v, want %v (the hook takes the last pending todo)", log.got(), want)
	}
	if want := [][]string{{"A", "B", "C"}, {"A", "B"}, {"A"}}; !reflect.DeepEqual(seen, want) {
		t.Errorf("pending lists given to Next = %v, want %v (backlog order, launched todos removed)", seen, want)
	}
}

func TestRunPool_ANextHookThatAdmitsNothingWaitsForALaneExitThenTheTopTodoGoes(t *testing.T) {
	backlog := []Todo{{ID: "A"}, {ID: "B"}, {ID: "C"}}
	var refused [][]string
	cfg := PoolConfig{Target: 2, Next: func(inFlight, pending []Todo) int {
		if len(inFlight) == 0 && pending[0].ID == "A" {
			return 0
		}
		refused = append(refused, ids(inFlight))
		return -1
	}}
	var log launchLog
	res := RunPool(context.Background(), cfg, backlog, log.launch(0), nil)
	if want := []string{"A", "B", "C"}; !reflect.DeepEqual(log.got(), want) {
		t.Errorf("launches = %v, want %v: each refusal with no lane in flight admits the top todo", log.got(), want)
	}
	if len(refused) == 0 || !reflect.DeepEqual(refused[0], []string{"A"}) {
		t.Errorf("refusals = %v, want the first one while A was in flight", refused)
	}
	for i, r := range res {
		if r.Status() != LaneOK {
			t.Errorf("result[%d] = %+v, want ok", i, r)
		}
	}
}

func TestRunPool_WithNoLaneInFlightARefusedTopTodoIsAdmitted(t *testing.T) {
	cfg := PoolConfig{Target: 3, Next: func(_, _ []Todo) int { return -1 }}
	var log launchLog
	RunPool(context.Background(), cfg, []Todo{{ID: "top"}, {ID: "second"}}, log.launch(0), nil)
	if want := []string{"top", "second"}; !reflect.DeepEqual(log.got(), want) {
		t.Errorf("launches = %v, want %v: only the top todo goes when nothing runs, the next after its exit", log.got(), want)
	}
}

func TestRunPool_PrepareRewritesTheSpecAndADroppedTodoNeverLaunches(t *testing.T) {
	backlog := []Todo{{ID: "stale"}, {ID: "fresh"}}
	var got []CycleSpec
	var mu sync.Mutex
	cfg := PoolConfig{Target: 2, Prepare: func(td Todo, spec CycleSpec) (CycleSpec, bool) {
		if td.ID == "stale" {
			return CycleSpec{}, false
		}
		spec.GoalHash = "prepared"
		return spec, true
	}}
	res := RunPool(context.Background(), cfg, backlog, func(_ context.Context, spec CycleSpec) (int, error) {
		mu.Lock()
		got = append(got, spec)
		mu.Unlock()
		return 0, nil
	}, nil)
	if len(got) != 1 || scopeID(got[0]) != "fresh" || got[0].GoalHash != "prepared" {
		t.Fatalf("launched specs = %+v, want only the prepared fresh spec", got)
	}
	if res[0].Status() != LaneFailed || res[0].Err == nil {
		t.Errorf("dropped todo result = %+v, want a failed result with its cause", res[0])
	}
}

func TestRunPool_AnExitHookThatRetriesLaunchesTheTodoOnceMore(t *testing.T) {
	backlog := []Todo{{ID: "A"}}
	tries := 0
	cfg := PoolConfig{Target: 1, Exited: func(td Todo, r Result) bool {
		tries++
		return tries == 1
	}}
	var log launchLog
	res := RunPool(context.Background(), cfg, backlog, log.launch(0), nil)
	if want := []string{"A", "A"}; !reflect.DeepEqual(log.got(), want) {
		t.Errorf("launches = %v, want %v (one retry)", log.got(), want)
	}
	if len(res) != 1 || res[0].Status() != LaneOK {
		t.Errorf("results = %+v, want the one result of the last try", res)
	}
}

func TestRunPool_MaxLaunchesEndsTheRunAtTheLimit(t *testing.T) {
	backlog := []Todo{{ID: "A"}, {ID: "B"}, {ID: "C"}}
	var log launchLog
	res := RunPool(context.Background(), PoolConfig{Target: 1, MaxLaunches: 2}, backlog, log.launch(0), nil)
	if want := []string{"A", "B"}; !reflect.DeepEqual(log.got(), want) {
		t.Errorf("launches = %v, want %v", log.got(), want)
	}
	if res[2].Status() != LaneFailed || res[2].Err == nil {
		t.Errorf("result past the limit = %+v, want a failed never-launched result", res[2])
	}
}

func TestRunPool_HaltedStopsNewAdmissionsAndTheLaneInFlightFinishes(t *testing.T) {
	backlog := []Todo{{ID: "A"}, {ID: "B"}}
	halted := false
	var log launchLog
	launch := func(ctx context.Context, spec CycleSpec) (int, error) {
		halted = true
		return log.launch(0)(ctx, spec)
	}
	res := RunPool(context.Background(), PoolConfig{Target: 1, Halted: func() bool { return halted }}, backlog, launch, nil)
	if want := []string{"A"}; !reflect.DeepEqual(log.got(), want) {
		t.Errorf("launches = %v, want %v (the stop admits nothing new)", log.got(), want)
	}
	if res[0].Status() != LaneOK {
		t.Errorf("the lane in flight = %+v, want it to finish ok", res[0])
	}
}

func TestRunPool_EachTransitionNamesItsKindAndItsTodo(t *testing.T) {
	backlog := []Todo{{ID: "A"}, {ID: "B"}}
	var got []PoolTransition
	var mu sync.Mutex
	var log launchLog
	RunPool(context.Background(), PoolConfig{Target: 1}, backlog, log.launch(0), func(pt PoolTransition) {
		mu.Lock()
		got = append(got, pt)
		mu.Unlock()
	})
	kinds := map[PoolTransitionKind]int{}
	for _, pt := range got {
		kinds[pt.Kind]++
	}
	if kinds[PoolLaunched] != 1 || kinds[PoolExited] != 2 || kinds[PoolBackfilled] != 1 || kinds[PoolIdle] != 1 {
		t.Errorf("transition kinds = %v, want one launch, two exits, one backfill and one idle", kinds)
	}
	want := []PoolTransition{
		{Kind: PoolLaunched, Todo: "A", Live: 1, Target: 1},
		{Kind: PoolExited, Todo: "A", Live: 0, Target: 1},
		{Kind: PoolBackfilled, Todo: "B", Live: 1, Target: 1},
		{Kind: PoolExited, Todo: "B", Live: 0, Target: 1},
		{Kind: PoolIdle, Live: 0, Target: 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("transitions =\n%+v\nwant\n%+v", got, want)
	}
}

func TestRunPool_AKickRunsTheFillAgainWhileALaneRunsAndAnAbsentKickChangesNothing(t *testing.T) {
	for name, kick := range map[string]bool{"kicked": true, "no kick channel": false} {
		t.Run(name, func(t *testing.T) {
			var mu sync.Mutex
			refuse := true
			refused := make(chan struct{}, 4)
			kicks := make(chan struct{}, 1)
			release := make(chan struct{})
			started := make(chan string, 4)
			cfg := PoolConfig{Target: 2, Next: func(inFlight, pending []Todo) int {
				mu.Lock()
				defer mu.Unlock()
				if len(inFlight) > 0 && refuse {
					refused <- struct{}{}
					return -1
				}
				return 0
			}}
			if kick {
				cfg.Kick = kicks
			}
			launch := func(_ context.Context, spec CycleSpec) (int, error) {
				started <- scopeID(spec)
				<-release
				return 0, nil
			}
			done := make(chan []Result, 1)
			go func() { done <- RunPool(context.Background(), cfg, []Todo{{ID: "A"}, {ID: "B"}}, launch, nil) }()
			if got := <-started; got != "A" {
				t.Fatalf("first launch = %q, want A", got)
			}
			<-refused
			mu.Lock()
			refuse = false
			mu.Unlock()
			if kick {
				kicks <- struct{}{}
				if got := <-started; got != "B" {
					t.Fatalf("kicked launch = %q, want B while A still runs", got)
				}
			} else {
				select {
				case got := <-started:
					t.Fatalf("launched %q with no exit and no kick, want the pool to wait", got)
				default:
				}
			}
			close(release)
			for i, r := range <-done {
				if r.Status() != LaneOK {
					t.Errorf("result[%d] = %+v, want ok", i, r)
				}
			}
		})
	}
}

func TestRunPool_AClosedKickChannelIsIgnoredAndNeverSpinsTheFill(t *testing.T) {
	kicks := make(chan struct{})
	close(kicks)
	var nexts atomic.Int64
	release := make(chan struct{})
	started := make(chan string, 4)
	cfg := PoolConfig{Target: 2, Kick: kicks, Next: func(inFlight, pending []Todo) int {
		nexts.Add(1)
		if len(inFlight) > 0 {
			return -1
		}
		return 0
	}}
	launch := func(_ context.Context, spec CycleSpec) (int, error) {
		started <- scopeID(spec)
		<-release
		return 0, nil
	}
	done := make(chan []Result, 1)
	go func() { done <- RunPool(context.Background(), cfg, []Todo{{ID: "A"}, {ID: "B"}}, launch, nil) }()
	<-started
	settled := nexts.Load()

	time.Sleep(50 * time.Millisecond)

	if grew := nexts.Load() - settled; grew > 2 {
		t.Errorf("Next ran %d more times while a lane ran behind a closed Kick channel, want no spin: a closed channel is always ready", grew)
	}
	close(release)
	for i, r := range <-done {
		if r.Status() != LaneOK {
			t.Errorf("result[%d] = %+v, want ok", i, r)
		}
	}
}
