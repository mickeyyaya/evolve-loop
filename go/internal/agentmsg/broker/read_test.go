package broker_test

import (
	"context"
	"crypto/rand"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/broker"
	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/envelope"
	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/identity"
	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/policy"
)

func bodies(m broker.Mailbox) []string {
	var out []string
	for _, e := range m.Messages {
		out = append(out, e.Body)
	}
	return out
}

func TestRead_OnlyARecipientSeesAMessage(t *testing.T) {
	h := newHarness(t)
	h.mustSend(scout, msg(envelope.KindNote, "for build", "phase:build"))
	h.mustSend(scout, msg(envelope.KindNote, "for audit", "phase:audit"))
	h.roster.members = append(h.roster.members, live(auditor))
	if got := bodies(h.read(build, 0)); len(got) != 1 || got[0] != "for build" {
		t.Fatalf("build mailbox = %v, want [for build]", got)
	}
	if got := bodies(h.read(tdd, 0)); len(got) != 0 {
		t.Fatalf("tdd mailbox = %v, want none", got)
	}
	if got := bodies(h.read(auditor, 0)); len(got) != 1 || got[0] != "for audit" {
		t.Fatalf("the later audit dispatch mailbox = %v, want the message to the planned phase", got)
	}
}

func TestRead_StartsAtTheCursorAndGivesTheNext(t *testing.T) {
	h := newHarness(t)
	h.mustSend(scout, msg(envelope.KindNote, "1", "phase:build"))
	h.mustSend(tdd, msg(envelope.KindNote, "2", "phase:build"))
	first := h.read(build, 0)
	h.mustSend(scout, msg(envelope.KindNote, "3", "phase:build"))
	rest := h.read(build, first.Next)
	if got := bodies(rest); len(got) != 1 || got[0] != "3" || rest.Next != 3 {
		t.Fatalf("Read(from %d) = %v next %d, want [3] next 3", first.Next, got, rest.Next)
	}
}

func TestRead_APrincipalOfAnotherCycleReadsNothing(t *testing.T) {
	h := newHarness(t)
	h.mustSend(scout, msg(envelope.KindNote, "x", "phase:build"))
	_, err := h.svc.Read(context.Background(), h.credential(lane), 0)
	isErr(t, err, identity.ErrUnauthenticated)
}

func TestRead_ADuplicateRecordIsDeliveredOnce(t *testing.T) {
	h := newHarness(t)
	h.mustSend(scout, msg(envelope.KindNote, "once", "phase:build"))
	rec := h.stored()[0]
	if _, err := h.store.Store.Append(context.Background(), cycle, rec); err != nil {
		t.Fatal(err)
	}
	if got := bodies(h.read(build, 0)); len(got) != 1 {
		t.Fatalf("mailbox = %v, want the message once", got)
	}
}

func TestRead_AnExpiredMessageGoesToTheDeadLetterOnce(t *testing.T) {
	h := newHarness(t)
	deadline := start.Add(time.Minute)
	req := msg(envelope.KindRequest, "answer before audit", "phase:build")
	req.CorrelationID, req.ExpiresAt = "q-1", &deadline
	h.mustSend(scout, req)
	h.clock.advance(time.Minute)
	if got := bodies(h.read(build, 0)); len(got) != 0 {
		t.Fatalf("mailbox = %v, want the expired request withheld", got)
	}
	h.read(build, 0)
	dead := h.store.Dead(cycle)
	if len(dead) != 1 || dead[0].Reason != broker.ReasonExpired || dead[0].Recipient.Phase != "build" {
		t.Fatalf("dead letters = %+v, want one expired letter for build", dead)
	}
	events := h.events.of(broker.EventDead)
	if len(events) != 1 || events[0].To[0] != build.Dispatch {
		t.Fatalf("dead events = %+v, want one for build", events)
	}
}

func TestRead_AGapFromTheStoreIsReportedAndNeverSilent(t *testing.T) {
	h := newHarness(t)
	h.mustSend(scout, msg(envelope.KindNote, "x", "phase:build"))
	gap := broker.Gap{From: 7, To: 19, Reason: "retention"}
	h.store.gaps = []broker.Gap{gap}
	box := h.read(build, 0)
	if len(box.Gaps) != 1 || box.Gaps[0] != gap {
		t.Fatalf("Mailbox.Gaps = %+v, want [%+v]", box.Gaps, gap)
	}
	dead := h.events.of(broker.EventDead)
	if len(dead) != 1 || dead[0].Reason != broker.ReasonGap || dead[0].ID != "gap.7.19" {
		t.Fatalf("dead events = %+v, want one gap event gap.7.19", dead)
	}
}

func TestRead_ANegativeCursorIsRefusedBeforeTheStore(t *testing.T) {
	h := newHarness(t)
	_, err := h.svc.Read(context.Background(), h.credential(build), -1)
	isErr(t, err, broker.ErrCursor)
	if h.store.reads != 0 {
		t.Fatalf("store reads = %d, want 0", h.store.reads)
	}
}

func TestAck_ClearsTheQueueWithoutARead(t *testing.T) {
	h := newHarness(t, limits(func(l *broker.Limits) { l.RecipientQueue = 2 }))
	h.mustSend(scout, msg(envelope.KindNote, "1", "phase:build"))
	second := h.mustSend(tdd, msg(envelope.KindNote, "2", "phase:build"))
	_, err := h.send(scout, msg(envelope.KindNote, "3", "phase:build"))
	isErr(t, err, broker.ErrQueueFull)
	if err := h.svc.Ack(context.Background(), h.credential(build), second.Cursor); err != nil {
		t.Fatalf("Ack error = %v", err)
	}
	h.mustSend(scout, msg(envelope.KindNote, "4", "phase:build"))
	_, err = h.send(tdd, msg(envelope.KindNote, "5", "phase:build"))
	isErr(t, err, broker.ErrQueueFull)
}

func TestAck_RefusesACursorOutsideTheLog(t *testing.T) {
	h := newHarness(t, limits(func(l *broker.Limits) { l.RecipientQueue = 1 }))
	h.mustSend(scout, msg(envelope.KindNote, "1", "phase:build"))
	for _, cursor := range []int64{-1, 1 << 40} {
		err := h.svc.Ack(context.Background(), h.credential(build), cursor)
		isErr(t, err, broker.ErrCursor)
	}
	_, err := h.send(tdd, msg(envelope.KindNote, "2", "phase:build"))
	isErr(t, err, broker.ErrQueueFull)
}

func TestAck_AStoreFailureIsAnError(t *testing.T) {
	h := newHarness(t)
	h.store.readErr = context.Canceled
	err := h.svc.Ack(context.Background(), h.credential(build), 0)
	isErr(t, err, context.Canceled)
}

func TestNew_NamesTheFirstMissingFieldEveryTime(t *testing.T) {
	h := newHarness(t)
	for i := 0; i < 50; i++ {
		_, err := broker.New(broker.Config{Auth: h.auth})
		if err == nil || !strings.Contains(err.Error(), "the cycle is") {
			t.Fatalf("New(empty config) run %d error = %v, want the cycle first", i, err)
		}
	}
}

func TestRead_AStoreFailureIsAnError(t *testing.T) {
	h := newHarness(t)
	h.store.readErr = context.Canceled
	_, err := h.svc.Read(context.Background(), h.credential(build), 0)
	isErr(t, err, context.Canceled)
}

func TestRead_ADeadLetterFailureIsAnError(t *testing.T) {
	h := newHarness(t)
	deadline := start.Add(time.Minute)
	req := msg(envelope.KindRequest, "x", "phase:build")
	req.CorrelationID, req.ExpiresAt = "q-1", &deadline
	h.mustSend(scout, req)
	h.clock.advance(time.Hour)
	h.store.deadErr = context.Canceled
	_, err := h.svc.Read(context.Background(), h.credential(build), 0)
	isErr(t, err, context.Canceled)
}

func TestNew_RefusesAnIncompleteConfig(t *testing.T) {
	h := newHarness(t)
	engine, err := policy.Compile(policy.Defaults())
	if err != nil {
		t.Fatal(err)
	}
	valid := func() broker.Config {
		return broker.Config{
			Cycle: cycle, Auth: h.auth, Policy: engine, Store: h.store, Roster: h.roster, Notifier: h.events,
			IDs: envelope.IDSource{Now: h.clock.Now, Entropy: rand.Reader}, Now: h.clock.Now,
			Envelope: envelope.Limits{BodyBytes: 1, SubjectBytes: 1, Kinds: envelope.DefaultKinds()}, Limits: testLimits,
		}
	}
	cases := map[string]func(*broker.Config){
		"no cycle":         func(c *broker.Config) { c.Cycle = 0 },
		"no authenticator": func(c *broker.Config) { c.Auth = nil },
		"no policy":        func(c *broker.Config) { c.Policy = nil },
		"no entropy":       func(c *broker.Config) { c.IDs.Entropy = nil },
		"no store":         func(c *broker.Config) { c.Store = nil },
		"no roster":        func(c *broker.Config) { c.Roster = nil },
		"no notifier":      func(c *broker.Config) { c.Notifier = nil },
		"no clock":         func(c *broker.Config) { c.Now = nil },
		"no id source":     func(c *broker.Config) { c.IDs.Now = nil },
		"no body cap":      func(c *broker.Config) { c.Envelope.BodyBytes = 0 },
		"no subject cap":   func(c *broker.Config) { c.Envelope.SubjectBytes = 0 },
		"no kinds":         func(c *broker.Config) { c.Envelope.Kinds = nil },
		"no sender rate":   func(c *broker.Config) { c.Limits.SenderPerMinute = 0 },
		"no repeat window": func(c *broker.Config) { c.Limits.RepeatWindow = 0 },
		"no queue cap":     func(c *broker.Config) { c.Limits.RecipientQueue = 0 },
		"a negative hop":   func(c *broker.Config) { c.Limits.MaxHops = -1 },
		"no budget":        func(c *broker.Config) { c.Limits.CycleBudget = 0 },
	}
	for name, breakIt := range cases {
		t.Run(name, func(t *testing.T) {
			c := valid()
			breakIt(&c)
			_, err := broker.New(c)
			isErr(t, err, broker.ErrConfig)
		})
	}
	if _, err := broker.New(valid()); err != nil {
		t.Fatalf("New(valid config) error = %v, want nil", err)
	}
}

func TestAck_CrossQueueIndependence(t *testing.T) {
	ctx := context.Background()
	handledByBuild := map[string]func(t *testing.T, h *harness, head int64){
		"an ack to the head": func(t *testing.T, h *harness, head int64) {
			if err := h.svc.Ack(ctx, h.credential(build), head); err != nil {
				t.Fatalf("Ack error = %v", err)
			}
		},
		"a read from the start": func(_ *testing.T, h *harness, _ int64) { h.read(build, 0) },
		"a read from the head":  func(_ *testing.T, h *harness, head int64) { h.read(build, head) },
	}
	for name, act := range handledByBuild {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t, limits(func(l *broker.Limits) { l.RecipientQueue = 1 }))
			h.mustSend(scout, msg(envelope.KindNote, "for build", "phase:build"))
			h.mustSend(scout, msg(envelope.KindNote, "for tdd", "phase:tdd"))
			h.mustSend(scout, msg(envelope.KindNote, "for audit", "phase:audit"))
			act(t, h, h.read(scout, 0).Next)
			for _, other := range []string{"phase:tdd", "phase:audit"} {
				_, err := h.send(scout, msg(envelope.KindNote, "more for "+other, other))
				isErr(t, err, broker.ErrQueueFull)
			}
			if got := bodies(h.read(tdd, 0)); len(got) != 1 || got[0] != "for tdd" {
				t.Fatalf("tdd mailbox after %s by build = %v, want [for tdd]", name, got)
			}
			h.mustSend(scout, msg(envelope.KindNote, "build has room again", "phase:build"))
		})
	}
}

func TestRead_GapEventsAreDeduplicatedOnReRead(t *testing.T) {
	h := newHarness(t)
	h.mustSend(scout, msg(envelope.KindNote, "x", "phase:build"))
	first := broker.Gap{From: 7, To: 19, Reason: "retention"}
	h.store.gaps = []broker.Gap{first}
	for i := 0; i < 3; i++ {
		if box := h.read(build, 0); len(box.Gaps) != 1 || box.Gaps[0] != first {
			t.Fatalf("read %d Mailbox.Gaps = %+v, want [%+v] on every read", i, box.Gaps, first)
		}
	}
	later := broker.Gap{From: 20, To: 25, Reason: "retention"}
	h.store.gaps = []broker.Gap{first, later}
	if box := h.read(build, 0); len(box.Gaps) != 2 {
		t.Fatalf("Mailbox.Gaps = %+v, want both gaps", box.Gaps)
	}
	var ids []string
	for _, e := range h.events.of(broker.EventDead) {
		if e.Reason == broker.ReasonGap {
			ids = append(ids, e.ID)
		}
	}
	if want := []string{"gap.7.19", "gap.20.25"}; !slices.Equal(ids, want) {
		t.Fatalf("gap dead events = %v, want %v: one event for each distinct gap", ids, want)
	}
}

func TestRead_AMessageToAPlannedPhaseReachesARetryAgain(t *testing.T) {
	h := newHarness(t)
	h.mustSend(scout, msg(envelope.KindNote, "for audit", "phase:audit"))
	h.roster.members = append(h.roster.members, live(auditor))
	if got := bodies(h.read(auditor, 0)); len(got) != 1 {
		t.Fatalf("first audit dispatch mailbox = %v, want the message", got)
	}
	retry := identity.Principal{Cycle: cycle, Dispatch: "r/1850/audit/p15n1", Agent: "auditor", Phase: "audit", Role: "audit"}
	h.roster.members = []broker.Member{live(scout), live(tdd), live(build), planned(audit), past(auditor), live(retry)}
	if got := bodies(h.read(retry, 0)); len(got) != 1 || got[0] != "for audit" {
		t.Fatalf("retry audit dispatch mailbox = %v, want the message again (at-least-once)", got)
	}
}

func TestRead_FromTheHeadClearsTheQueueWithoutMessages(t *testing.T) {
	h := newHarness(t, limits(func(l *broker.Limits) { l.RecipientQueue = 1 }))
	h.mustSend(scout, msg(envelope.KindNote, "never returned", "phase:build"))
	head := h.read(scout, 0).Next
	if got := bodies(h.read(build, head)); len(got) != 0 {
		t.Fatalf("Read(from the head %d) = %v, want no messages", head, got)
	}
	h.mustSend(scout, msg(envelope.KindNote, "room after the head read", "phase:build"))
}
