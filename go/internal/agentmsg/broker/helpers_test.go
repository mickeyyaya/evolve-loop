package broker_test

import (
	"context"
	"crypto/rand"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/address"
	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/broker"
	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/envelope"
	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/identity"
	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/memstore"
	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/policy"
)

const cycle = 1850

var (
	key   = []byte("0123456789abcdef0123456789abcdef")
	start = time.Date(2026, 10, 9, 21, 0, 0, 0, time.UTC)

	scout   = identity.Principal{Cycle: cycle, Dispatch: "r/1850/scout/p11n1", Agent: "scout", Phase: "scout", Role: "scout"}
	tdd     = identity.Principal{Cycle: cycle, Dispatch: "r/1850/tdd/p12n1", Agent: "tdd-engineer", Phase: "tdd", Role: "tdd"}
	build   = identity.Principal{Cycle: cycle, Dispatch: "r/1850/build/p13n1", Agent: "builder", Phase: "build", Role: "build"}
	audit   = identity.Principal{Cycle: cycle, Phase: "audit", Role: "audit"}
	auditor = identity.Principal{Cycle: cycle, Dispatch: "r/1850/audit/p14n1", Agent: "auditor", Phase: "audit", Role: "audit"}
	lane    = identity.Principal{Cycle: 1851, Dispatch: "r/1851/build/p21n1", Agent: "builder", Phase: "build", Role: "build"}

	testLimits = broker.Limits{SenderPerMinute: 30, RepeatWindow: time.Minute, RecipientQueue: 50, MaxHops: 4, CycleBudget: 200}
)

type clock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *clock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func member(p identity.Principal, state broker.MemberState) broker.Member {
	return broker.Member{Principal: p, State: state}
}

func live(p identity.Principal) broker.Member { return member(p, broker.MemberLive) }

func planned(p identity.Principal) broker.Member { return member(p, broker.MemberPlanned) }

func past(p identity.Principal) broker.Member { return member(p, broker.MemberPast) }

type roster struct {
	members []broker.Member
	calls   []int
	err     error
}

func (r *roster) Members(_ context.Context, cycle int) ([]broker.Member, error) {
	r.calls = append(r.calls, cycle)
	return slices.Clone(r.members), r.err
}

type recorder struct {
	events []broker.Event
	onSent func()
}

func (r *recorder) Notify(_ context.Context, e broker.Event) {
	r.events = append(r.events, e)
	if e.Kind == broker.EventSent && r.onSent != nil {
		r.onSent()
	}
}

func (r *recorder) of(kind broker.EventKind) []broker.Event {
	var out []broker.Event
	for _, e := range r.events {
		if e.Kind == kind {
			out = append(out, e)
		}
	}
	return out
}

type failingStore struct {
	*memstore.Store
	appendErr, readErr, deadErr error
	gaps                        []broker.Gap
	reads                       int
}

func (f *failingStore) Append(ctx context.Context, cycle int, r broker.Record) (int64, error) {
	if f.appendErr != nil {
		return 0, f.appendErr
	}
	return f.Store.Append(ctx, cycle, r)
}

func (f *failingStore) Read(ctx context.Context, cycle int, from int64) (broker.Batch, error) {
	f.reads++
	if f.readErr != nil {
		return broker.Batch{}, f.readErr
	}
	b, err := f.Store.Read(ctx, cycle, from)
	b.Gaps = f.gaps
	return b, err
}

func (f *failingStore) DeadLetter(ctx context.Context, cycle int, d broker.Dead) error {
	if f.deadErr != nil {
		return f.deadErr
	}
	return f.Store.DeadLetter(ctx, cycle, d)
}

type harness struct {
	t      *testing.T
	svc    *broker.Service
	auth   *identity.Authority
	clock  *clock
	store  *failingStore
	roster *roster
	events *recorder
}

type option func(*broker.Config)

func withRules(rules []policy.Rule) option {
	return func(c *broker.Config) {
		e, err := policy.Compile(rules)
		if err != nil {
			panic(err)
		}
		c.Policy = e
	}
}

func withLimits(l broker.Limits) option { return func(c *broker.Config) { c.Limits = l } }

func newHarness(t *testing.T, opts ...option) *harness {
	t.Helper()
	clk := &clock{now: start}
	auth, err := identity.NewAuthority(key, clk.Now, 48*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	engine, err := policy.Compile(policy.Defaults())
	if err != nil {
		t.Fatal(err)
	}
	h := &harness{
		t: t, auth: auth, clock: clk,
		store:  &failingStore{Store: memstore.New()},
		roster: &roster{members: []broker.Member{live(scout), live(tdd), live(build), planned(audit)}},
		events: &recorder{},
	}
	cfg := broker.Config{
		Cycle: cycle, Auth: auth, Policy: engine, Store: h.store, Roster: h.roster, Notifier: h.events,
		IDs:      envelope.IDSource{Now: clk.Now, Entropy: rand.Reader},
		Now:      clk.Now,
		Envelope: envelope.Limits{BodyBytes: 8192, SubjectBytes: 256, Kinds: envelope.DefaultKinds()},
		Limits:   testLimits,
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	svc, err := broker.New(cfg)
	if err != nil {
		t.Fatalf("New error = %v", err)
	}
	h.svc = svc
	return h
}

func (h *harness) credential(p identity.Principal) identity.Credential {
	h.t.Helper()
	tok, err := h.auth.Mint(p, start.Add(24*time.Hour))
	if err != nil {
		h.t.Fatalf("Mint(%+v) error = %v", p, err)
	}
	return identity.Credential{Token: tok, Location: p.Dispatch}
}

func (h *harness) send(from identity.Principal, e envelope.Envelope) (broker.Receipt, error) {
	h.t.Helper()
	return h.svc.Send(context.Background(), h.credential(from), e)
}

func (h *harness) mustSend(from identity.Principal, e envelope.Envelope) broker.Receipt {
	h.t.Helper()
	r, err := h.send(from, e)
	if err != nil {
		h.t.Fatalf("Send from %s error = %v", from.Phase, err)
	}
	return r
}

func (h *harness) read(p identity.Principal, from int64) broker.Mailbox {
	h.t.Helper()
	m, err := h.svc.Read(context.Background(), h.credential(p), from)
	if err != nil {
		h.t.Fatalf("Read for %s error = %v", p.Phase, err)
	}
	return m
}

func (h *harness) stored() []broker.Record {
	h.t.Helper()
	b, err := h.store.Store.Read(context.Background(), cycle, 0)
	if err != nil {
		h.t.Fatal(err)
	}
	return b.Records
}

func to(addrs ...string) []address.Address {
	out := make([]address.Address, 0, len(addrs))
	for _, s := range addrs {
		a, err := address.Parse(s)
		if err != nil {
			panic(err)
		}
		out = append(out, a)
	}
	return out
}

func msg(kind envelope.Kind, body string, addrs ...string) envelope.Envelope {
	return envelope.Envelope{Kind: kind, To: to(addrs...), Body: body}
}

func phases(ps []identity.Principal) []string {
	var out []string
	for _, p := range ps {
		out = append(out, p.Phase)
	}
	return out
}

func isErr(t *testing.T, err, want error) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}
