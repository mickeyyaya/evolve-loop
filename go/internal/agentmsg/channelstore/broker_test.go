package channelstore

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/address"
	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/broker"
	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/envelope"
	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/identity"
	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/policy"
)

var (
	scout = identity.Principal{Cycle: cycle, Dispatch: "r/5/scout/p1n1", Agent: "scout", Phase: "scout", Role: "scout"}
	build = identity.Principal{Cycle: cycle, Dispatch: "r/5/build/p3n1", Agent: "builder", Phase: "build", Role: "build"}
)

type clock struct{ now time.Time }

func (c *clock) Now() time.Time { return c.now }

type fixedRoster []broker.Member

func (r fixedRoster) Members(context.Context, int) ([]broker.Member, error) { return r, nil }

type silentNotifier struct{}

func (silentNotifier) Notify(context.Context, broker.Event) {}

type mailService struct {
	svc   *broker.Service
	auth  *identity.Authority
	clock *clock
}

func (m mailService) credential(t *testing.T, p identity.Principal) identity.Credential {
	t.Helper()
	token, err := m.auth.Mint(p, sentAt.Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	return identity.Credential{Token: token, Location: p.Dispatch}
}

func newMailService(t *testing.T, store broker.Store) mailService {
	t.Helper()
	clk := &clock{now: sentAt}
	auth, err := identity.NewAuthority([]byte("0123456789abcdef0123456789abcdef"), clk.Now, 48*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	engine, err := policy.Compile(policy.Defaults())
	if err != nil {
		t.Fatal(err)
	}
	svc, err := broker.New(broker.Config{
		Cycle: cycle, Auth: auth, Policy: engine, Store: store,
		Roster:   fixedRoster{{Principal: scout, State: broker.MemberLive}, {Principal: build, State: broker.MemberLive}},
		Notifier: silentNotifier{},
		IDs:      envelope.IDSource{Now: clk.Now, Entropy: rand.Reader},
		Now:      clk.Now,
		Envelope: envelope.Limits{BodyBytes: 8192, SubjectBytes: 256, Kinds: envelope.DefaultKinds()},
		Limits:   broker.Limits{SenderPerMinute: 30, RepeatWindow: time.Minute, RecipientQueue: 50, MaxHops: 4, CycleBudget: 200},
	})
	if err != nil {
		t.Fatal(err)
	}
	return mailService{svc: svc, auth: auth, clock: clk}
}

func toBuild() []address.Address {
	return []address.Address{{Selector: address.SelectPhase, Value: "build"}}
}

func TestChannelStore_ServesTheBrokerSendAndRead(t *testing.T) {
	t.Parallel()
	s, _ := newStore(t, 1<<20)
	m := newMailService(t, s)
	ctx := context.Background()
	body := strings.Repeat("the handoff names the flag. ", 220)

	receipt, err := m.svc.Send(ctx, m.credential(t, scout), envelope.Envelope{Kind: envelope.KindHandoff, To: toBuild(), Body: body})
	if err != nil || receipt.Cursor != 0 || len(receipt.Delivered) != 1 {
		t.Fatalf("Send = %+v, %v, want one delivery at cursor 0, also for a body of %d bytes", receipt, err, len(body))
	}
	box, err := m.svc.Read(ctx, m.credential(t, build), 0)
	if err != nil || len(box.Messages) != 1 || box.Messages[0].Body != body || box.Next <= receipt.Cursor || len(box.Gaps) != 0 {
		t.Fatalf("Read = %+v, %v, want the handoff and a next cursor above %d", box, err, receipt.Cursor)
	}
	again, err := m.svc.Read(ctx, m.credential(t, build), box.Next)
	if err != nil || len(again.Messages) != 0 || again.Next != box.Next {
		t.Fatalf("Read(next) = %+v, %v, want nothing and the same next cursor", again, err)
	}
	if _, err := m.svc.Read(ctx, m.credential(t, build), box.Next+100); !errors.Is(err, broker.ErrCursor) {
		t.Fatalf("Read past the head error = %v, want broker.ErrCursor: the reset gap of the log does not hide it", err)
	}
	if err := m.svc.Ack(ctx, m.credential(t, build), box.Next); err != nil {
		t.Fatalf("Ack(next) error = %v", err)
	}
}

func TestChannelStore_AnExpiredMessageGoesToMsgDeadOnce(t *testing.T) {
	t.Parallel()
	s, root := newStore(t, 1<<20)
	m := newMailService(t, s)
	ctx := context.Background()
	deadline := sentAt.Add(time.Minute)
	request := envelope.Envelope{Kind: envelope.KindRequest, To: toBuild(), Body: "which flag?", CorrelationID: "c1", ExpiresAt: &deadline}
	receipt, err := m.svc.Send(ctx, m.credential(t, scout), request)
	if err != nil {
		t.Fatal(err)
	}
	m.clock.now = sentAt.Add(2 * time.Minute)

	box, err := m.svc.Read(ctx, m.credential(t, build), 0)
	if err != nil || len(box.Messages) != 0 {
		t.Fatalf("Read of an expired request = %+v, %v, want no message", box, err)
	}
	if _, err := m.svc.Read(ctx, m.credential(t, build), 0); err != nil {
		t.Fatalf("second Read error = %v", err)
	}
	dead, err := rawLog(t, root, "msg.dead.5").Read(0)
	if err != nil || len(dead.Records) != 1 {
		t.Fatalf("msg.dead.5 = %+v, %v, want one dead letter, also after a second read", dead, err)
	}
	var letter broker.Dead
	if err := json.Unmarshal(dead.Records[0].Message, &letter); err != nil || letter.Envelope.ID != receipt.ID || letter.Reason != broker.ReasonExpired || letter.Recipient.Phase != "build" {
		t.Fatalf("dead letter = %+v, %v, want the expired request for build", letter, err)
	}
}

func TestChannelStore_ACursorFromAppendIsTheStartOfItsRecord(t *testing.T) {
	t.Parallel()
	s, _ := newStore(t, 1<<20)
	m := newMailService(t, s)
	ctx := context.Background()
	first, err := m.svc.Send(ctx, m.credential(t, scout), envelope.Envelope{Kind: envelope.KindNote, To: toBuild(), Body: "first"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := m.svc.Send(ctx, m.credential(t, scout), envelope.Envelope{Kind: envelope.KindNote, To: toBuild(), Body: "second"})
	if err != nil {
		t.Fatal(err)
	}

	if err := m.svc.Ack(ctx, m.credential(t, build), second.Cursor); err != nil {
		t.Fatalf("Ack(cursor of the second record) error = %v", err)
	}
	fromSecond, err := m.svc.Read(ctx, m.credential(t, build), second.Cursor)
	if err != nil {
		t.Fatal(err)
	}
	afterSecond, err := m.svc.Read(ctx, m.credential(t, build), fromSecond.Next)
	if err != nil {
		t.Fatal(err)
	}

	if first.Cursor != 0 || second.Cursor <= first.Cursor {
		t.Fatalf("cursors = %d, %d, want 0 and a larger start", first.Cursor, second.Cursor)
	}
	if len(fromSecond.Messages) != 1 || fromSecond.Messages[0].Body != "second" {
		t.Fatalf("Read(cursor of the second record) = %+v, want only the second: it is not skipped and the first is not delivered again", fromSecond.Messages)
	}
	if len(afterSecond.Messages) != 0 || fromSecond.Next <= second.Cursor {
		t.Fatalf("Read(next) = %+v with next %d, want nothing: the record is not delivered twice", afterSecond.Messages, fromSecond.Next)
	}
}
