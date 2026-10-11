package broker_test

import (
	"context"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/broker"
	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/envelope"
	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/policy"
)

func limits(change func(*broker.Limits)) option {
	l := testLimits
	change(&l)
	return withLimits(l)
}

func TestSend_TheHopLimitStopsAReplyChain(t *testing.T) {
	h := newHarness(t, limits(func(l *broker.Limits) { l.MaxHops = 2 }))
	replyTo := func(parent broker.Receipt, body, addr string) envelope.Envelope {
		e := msg(envelope.KindNote, body, addr)
		e.ReplyTo = parent.ID
		return e
	}
	hop0 := h.mustSend(scout, msg(envelope.KindNote, "hop 0", "phase:build"))
	hop1 := h.mustSend(build, replyTo(hop0, "hop 1", "phase:scout"))
	hop2 := h.mustSend(scout, replyTo(hop1, "hop 2", "phase:build"))
	if got := h.stored()[2].Envelope.Hop; got != 2 {
		t.Fatalf("third message hop = %d, want 2", got)
	}
	_, err := h.send(build, replyTo(hop2, "hop 3", "phase:scout"))
	isErr(t, err, broker.ErrHopLimit)
}

func TestSend_ASenderCanFollowUpItsOwnMessage(t *testing.T) {
	h := newHarness(t)
	first := h.mustSend(scout, msg(envelope.KindNote, "the flag is in cmd_loop.go", "phase:build"))
	e := msg(envelope.KindNote, "correction: cmd_loop_boot.go", "phase:build")
	e.ReplyTo = first.ID
	h.mustSend(scout, e)
	if got := h.stored()[1].Envelope.Hop; got != 1 {
		t.Fatalf("follow-up hop = %d, want 1", got)
	}
}

func TestSend_AReplyToAMessageOutsideTheSenderScopeIsNotFound(t *testing.T) {
	h := newHarness(t)
	private := h.mustSend(scout, msg(envelope.KindNote, "for build only", "phase:build"))
	for name, replyTo := range map[string]string{
		"a message to another agent": private.ID,
		"a message that never was":   "01JA0000000000000000000000",
	} {
		t.Run(name, func(t *testing.T) {
			e := msg(envelope.KindNote, "x", "phase:scout")
			e.ReplyTo = replyTo
			_, err := h.send(tdd, e)
			isErr(t, err, broker.ErrNotFound)
		})
	}
}

func TestSend_TheCycleBudgetCapsAcceptedMessages(t *testing.T) {
	h := newHarness(t, limits(func(l *broker.Limits) { l.CycleBudget = 2 }))
	h.mustSend(scout, msg(envelope.KindNote, "1", "phase:build"))
	h.mustSend(tdd, msg(envelope.KindNote, "2", "phase:build"))
	_, err := h.send(build, msg(envelope.KindNote, "3", "phase:scout"))
	isErr(t, err, broker.ErrBudget)
	if denied := h.events.of(broker.EventDenied); len(denied) != 1 || denied[0].Rule != broker.RuleLimit {
		t.Fatalf("denied events = %+v, want one by %s", denied, broker.RuleLimit)
	}
}

func TestSend_TheSenderRateIsCappedForEachMinute(t *testing.T) {
	h := newHarness(t, limits(func(l *broker.Limits) { l.SenderPerMinute = 2 }))
	h.mustSend(scout, msg(envelope.KindNote, "1", "phase:build"))
	h.clock.advance(30 * time.Second)
	h.mustSend(scout, msg(envelope.KindNote, "2", "phase:build"))
	_, err := h.send(scout, msg(envelope.KindNote, "3", "phase:build"))
	isErr(t, err, broker.ErrRateLimited)
	h.mustSend(tdd, msg(envelope.KindNote, "other sender", "phase:build"))
	h.clock.advance(30*time.Second - time.Nanosecond)
	_, err = h.send(scout, msg(envelope.KindNote, "4", "phase:build"))
	isErr(t, err, broker.ErrRateLimited)
	h.clock.advance(time.Nanosecond)
	h.mustSend(scout, msg(envelope.KindNote, "5", "phase:build"))
}

func TestSend_AnIdenticalRepeatInsideTheWindowIsDropped(t *testing.T) {
	h := newHarness(t, limits(func(l *broker.Limits) { l.RepeatWindow = 45 * time.Second }))
	h.mustSend(scout, msg(envelope.KindNote, "same", "phase:build"))
	_, err := h.send(scout, msg(envelope.KindNote, "same", "phase:build"))
	isErr(t, err, broker.ErrRepeat)
	h.mustSend(scout, msg(envelope.KindNote, "same", "phase:tdd"))
	h.mustSend(tdd, msg(envelope.KindNote, "same", "phase:build"))
	h.clock.advance(45 * time.Second)
	h.mustSend(scout, msg(envelope.KindNote, "same", "phase:build"))
}

func TestSend_AFullRecipientQueueRefusesTheRecipientUntilItReads(t *testing.T) {
	h := newHarness(t, limits(func(l *broker.Limits) { l.RecipientQueue = 2 }))
	h.mustSend(scout, msg(envelope.KindNote, "1", "phase:build"))
	h.mustSend(tdd, msg(envelope.KindNote, "2", "phase:build"))
	_, err := h.send(scout, msg(envelope.KindNote, "3", "phase:build"))
	isErr(t, err, broker.ErrQueueFull)
	r := h.mustSend(scout, msg(envelope.KindBroadcast, "4", "*"))
	if len(r.Refused) != 1 || r.Refused[0].Recipient.Phase != "build" || r.Refused[0].Rule != broker.RuleQueueCap {
		t.Fatalf("Refused = %+v, want build by %s", r.Refused, broker.RuleQueueCap)
	}
	_, err = h.svc.Read(context.Background(), h.credential(build), 1<<40)
	isErr(t, err, broker.ErrCursor)
	_, err = h.send(scout, msg(envelope.KindNote, "5", "phase:build"))
	isErr(t, err, broker.ErrQueueFull)
	h.read(build, 0)
	h.mustSend(scout, msg(envelope.KindNote, "6", "phase:build"))
	h.mustSend(tdd, msg(envelope.KindNote, "7", "phase:build"))
	_, err = h.send(scout, msg(envelope.KindNote, "8", "phase:build"))
	isErr(t, err, broker.ErrQueueFull)
}

func TestSend_ARequestGrantsItsRecipientsARightToReplyUntilItExpires(t *testing.T) {
	rules := []policy.Rule{
		{Name: "scout_asks", Effect: policy.EffectAllow, When: policy.Match{FromPhase: []string{"scout"}}},
		{Name: "reply_right", Effect: policy.EffectAllow, When: policy.Match{ReplyGrant: true}},
	}
	h := newHarness(t, withRules(rules))
	deadline := start.Add(10 * time.Minute)
	req := msg(envelope.KindRequest, "where is the flag parsed?", "phase:build")
	req.CorrelationID, req.ExpiresAt = "q-1", &deadline
	h.mustSend(scout, req)
	reply := func(corr string) envelope.Envelope {
		e := msg(envelope.KindReply, "cmd_loop.go:88", "phase:scout")
		e.CorrelationID = corr
		return e
	}
	_, err := h.send(build, msg(envelope.KindNote, "a free note", "phase:scout"))
	isErr(t, err, broker.ErrDenied)
	_, err = h.send(build, reply("q-2"))
	isErr(t, err, broker.ErrDenied)
	_, err = h.send(tdd, reply("q-1"))
	isErr(t, err, broker.ErrDenied)
	toTDD := reply("q-1")
	toTDD.To = to("phase:tdd")
	_, err = h.send(build, toTDD)
	isErr(t, err, broker.ErrDenied)
	h.mustSend(build, reply("q-1"))
	h.clock.advance(10 * time.Minute)
	second := reply("q-1")
	second.Body = "a second answer"
	_, err = h.send(build, second)
	isErr(t, err, broker.ErrDenied)
}

func TestSend_ARequestToAPlannedPhaseGrantsItsLaterDispatch(t *testing.T) {
	rules := []policy.Rule{
		{Name: "scout_asks", Effect: policy.EffectAllow, When: policy.Match{FromPhase: []string{"scout"}}},
		{Name: "reply_right", Effect: policy.EffectAllow, When: policy.Match{ReplyGrant: true}},
	}
	h := newHarness(t, withRules(rules))
	deadline := start.Add(time.Hour)
	req := msg(envelope.KindRequest, "check the flag", "phase:audit")
	req.CorrelationID, req.ExpiresAt = "q-9", &deadline
	h.mustSend(scout, req)
	h.roster.members = append(h.roster.members, live(auditor))
	e := msg(envelope.KindReply, "checked", "phase:scout")
	e.CorrelationID = "q-9"
	h.mustSend(auditor, e)
}
