package broker

import (
	"context"

	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/envelope"
	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/identity"
)

type Record struct {
	Cursor     int64                `json:"-"`
	Envelope   envelope.Envelope    `json:"envelope"`
	Recipients []identity.Principal `json:"recipients"`
}

type Gap struct {
	From   int64  `json:"from"`
	To     int64  `json:"to"`
	Reason string `json:"reason"`
}

type Batch struct {
	Records []Record
	Gaps    []Gap
	Next    int64
}

type MemberState string

const (
	MemberPlanned MemberState = "planned"
	MemberLive    MemberState = "live"
	MemberPast    MemberState = "past"
)

type Member struct {
	identity.Principal
	State MemberState
}

type Dead struct {
	Envelope  envelope.Envelope  `json:"envelope"`
	Recipient identity.Principal `json:"recipient"`
	Reason    string             `json:"reason"`
}

type Store interface {
	Append(ctx context.Context, cycle int, r Record) (int64, error)
	Read(ctx context.Context, cycle int, from int64) (Batch, error)
	DeadLetter(ctx context.Context, cycle int, d Dead) error
}

type Roster interface {
	Members(ctx context.Context, cycle int) ([]Member, error)
}

type EventKind string

const (
	EventSent   EventKind = "msg.sent"
	EventDenied EventKind = "msg.denied"
	EventDead   EventKind = "msg.dead"
)

type Event struct {
	Kind    EventKind
	Cycle   int
	ID      string
	MsgKind envelope.Kind
	From    string
	To      []string
	Hop     int
	Rule    string
	Reason  string
}

type Notifier interface {
	Notify(ctx context.Context, e Event)
}
