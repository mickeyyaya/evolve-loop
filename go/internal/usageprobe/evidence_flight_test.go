package usageprobe

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

var claudeQuery = Query{CLI: "claude", Family: "claude"}

func (r *evidenceRig) holdTheQuery(t *testing.T) (started <-chan struct{}, release func()) {
	t.Helper()
	gate, called := make(chan struct{}), make(chan struct{}, 1)
	var once sync.Once
	release = func() { once.Do(func() { close(gate) }) }
	r.source.Prober.Probe = func(ctx context.Context, _ string) (string, error) {
		r.probes.Add(1)
		select {
		case called <- struct{}{}:
		default:
		}
		select {
		case <-gate:
			return "the usage screen", nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	t.Cleanup(func() {
		release()
		r.source.Explain(context.Background(), claudeQuery)
	})
	return called, release
}

type waitingCtx struct {
	context.Context
	waiting chan struct{}
	once    sync.Once
}

func newWaitingCtx(parent context.Context) *waitingCtx {
	return &waitingCtx{Context: parent, waiting: make(chan struct{})}
}

func (c *waitingCtx) Done() <-chan struct{} {
	c.once.Do(func() { close(c.waiting) })
	return c.Context.Done()
}

func explainAsync(src *EvidenceSource, ctx context.Context, q Query) <-chan Evidence {
	out := make(chan Evidence, 1)
	go func() { out <- src.Explain(ctx, q) }()
	return out
}

func within(t *testing.T, answer <-chan Evidence) Evidence {
	t.Helper()
	select {
	case ev := <-answer:
		return ev
	case <-time.After(5 * time.Second):
		t.Fatal("the caller is still waiting on the usage query")
		return Evidence{}
	}
}

func await(t *testing.T, signal <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(5 * time.Second):
		t.Fatalf("%s never happened", what)
	}
}

func TestEvidence_ALeaderCancelledMidQueryReturnsAtOnceAndNeitherCachesNorRecordsTheCancellation(t *testing.T) {
	r := newEvidenceRig(t, claudeWindows(7, 0), nil, false)
	started, release := r.holdTheQuery(t)
	ctx, cancel := context.WithCancel(context.Background())
	leader := explainAsync(r.source, ctx, claudeQuery)
	await(t, started, "the usage query")

	cancel()
	first := within(t, leader)

	if first.Verdict != VerdictUnknown || !strings.Contains(first.Detail, "stopped waiting") || strings.Contains(first.Summary(), "auth, install or network") {
		t.Errorf("the cancelled caller got %+v; its own cancellation says nothing about the CLI", first)
	}
	if recorded, _ := LoadObservations(r.dir); recorded["claude"].CLI != "" {
		t.Errorf("the cancelled caller's query was recorded for every lane: %+v", recorded["claude"])
	}
	lane := newEvidenceRig(t, claudeWindows(7, 0), nil, false)
	lane.source.EvolveDir, lane.source.Act = r.dir, false
	if ev := lane.source.Explain(context.Background(), claudeQuery); ev.Verdict != VerdictHealthy || lane.probes.Load() != 1 {
		t.Errorf("another lane on the same evolve dir got %+v after %d probe(s); want its own healthy read", ev, lane.probes.Load())
	}
	release()
	if later := r.source.Explain(context.Background(), claudeQuery); later.Verdict != VerdictHealthy || r.probes.Load() != 1 {
		t.Errorf("a later live caller got %+v after %d probe(s); want the one query's healthy answer", later, r.probes.Load())
	}
}

func TestEvidence_AFollowerWithALiveContextGetsTheQuerysAnswerWhenTheLeaderIsCancelled(t *testing.T) {
	r := newEvidenceRig(t, claudeWindows(7, 0), nil, false)
	started, release := r.holdTheQuery(t)
	ctx, cancel := context.WithCancel(context.Background())
	leader := explainAsync(r.source, ctx, claudeQuery)
	await(t, started, "the usage query")
	live := newWaitingCtx(context.Background())
	follower := explainAsync(r.source, live, claudeQuery)
	await(t, live.waiting, "the follower's wait on the query in flight")

	cancel()
	within(t, leader)
	release()
	got := within(t, follower)

	if got.Verdict != VerdictHealthy || r.probes.Load() != 1 {
		t.Fatalf("the follower got %+v after %d probe(s); its own context was live, so it gets the query's answer, not the leader's cancellation", got, r.probes.Load())
	}
}

func TestEvidence_AFollowerOfAQueryInFlightGetsAFreshAnswerNotACachedOne(t *testing.T) {
	r := newEvidenceRig(t, claudeWindows(7, 0), nil, false)
	started, release := r.holdTheQuery(t)
	leader := explainAsync(r.source, context.Background(), claudeQuery)
	await(t, started, "the usage query")
	live := newWaitingCtx(context.Background())
	follower := explainAsync(r.source, live, claudeQuery)
	await(t, live.waiting, "the follower's wait on the query in flight")

	release()

	for role, answer := range map[string]<-chan Evidence{"leader": leader, "follower": follower} {
		if ev := within(t, answer); ev.Cached || ev.Verdict != VerdictHealthy {
			t.Errorf("the %s got %+v; both callers share one fresh query, so neither answer is cached", role, ev)
		}
	}
}

func TestEvidence_AFollowerWhoseOwnContextEndsStopsWaitingForASlowQuery(t *testing.T) {
	r := newEvidenceRig(t, claudeWindows(7, 0), nil, false)
	started, _ := r.holdTheQuery(t)
	explainAsync(r.source, context.Background(), claudeQuery)
	await(t, started, "the usage query")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	got := within(t, explainAsync(r.source, ctx, claudeQuery))

	if got.Verdict != VerdictUnknown || got.Cached {
		t.Fatalf("the follower got %+v; a caller that stops waiting gets no verdict about the CLI", got)
	}
}

func TestEvidence_TwoFamiliesOfOneBinaryFailingTogetherShareOneUsageQuery(t *testing.T) {
	r := newEvidenceRig(t, agyWindows(100), nil, false)
	gate, started := make(chan struct{}), make(chan struct{}, 2)
	r.source.Prober.Probe = func(ctx context.Context, _ string) (string, error) {
		r.probes.Add(1)
		started <- struct{}{}
		select {
		case <-gate:
			return "the usage screen", nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	gemini := explainAsync(r.source, context.Background(), Query{CLI: "agy", Family: "agy"})
	await(t, started, "the agy usage query")
	live := newWaitingCtx(context.Background())
	claude := explainAsync(r.source, live, Query{CLI: "agy", Family: "agy-claude"})
	await(t, live.waiting, "the agy-claude caller's wait")

	close(gate)
	g, c := within(t, gemini), within(t, claude)

	if r.probes.Load() != 1 || g.Verdict != VerdictHealthy || c.Verdict != VerdictExhausted {
		t.Fatalf("%d probe(s); agy got %s, agy-claude got %s; one binary is queried once and each family reads its own group", r.probes.Load(), g.Verdict, c.Verdict)
	}
}
