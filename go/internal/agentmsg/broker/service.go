package broker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/envelope"
	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/identity"
	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/policy"
)

type Limits struct {
	SenderPerMinute int
	RepeatWindow    time.Duration
	RecipientQueue  int
	MaxHops         int
	CycleBudget     int
}

type Config struct {
	Cycle    int
	Auth     identity.Authenticator
	Policy   policy.Authorizer
	Store    Store
	Roster   Roster
	Notifier Notifier
	IDs      envelope.IDSource
	Now      func() time.Time
	Envelope envelope.Limits
	Limits   Limits
}

type Refusal struct {
	Recipient identity.Principal
	Rule      string
	Reason    string
}

type Receipt struct {
	ID        string
	Cursor    int64
	Delivered []identity.Principal
	Refused   []Refusal
	Duplicate bool
}

type Mailbox struct {
	Messages []envelope.Envelope
	Gaps     []Gap
	Next     int64
}

type Service struct {
	cfg   Config
	mu    sync.Mutex
	state *ledger
}

func New(cfg Config) (*Service, error) {
	if err := cfg.check(); err != nil {
		return nil, err
	}
	return &Service{cfg: cfg, state: newLedger()}, nil
}

type requirement struct {
	name    string
	missing bool
}

func (c Config) check() error {
	requirements := []requirement{
		{"cycle", c.Cycle <= 0},
		{"authenticator", c.Auth == nil},
		{"policy", c.Policy == nil},
		{"store", c.Store == nil},
		{"roster", c.Roster == nil},
		{"notifier", c.Notifier == nil},
		{"clock", c.Now == nil},
		{"id source", c.IDs.Now == nil || c.IDs.Entropy == nil},
		{"envelope caps", c.Envelope.BodyBytes <= 0 || c.Envelope.SubjectBytes <= 0 || len(c.Envelope.Kinds) == 0},
		{"limits", c.Limits.SenderPerMinute <= 0 || c.Limits.RepeatWindow <= 0 || c.Limits.RecipientQueue <= 0 || c.Limits.MaxHops < 0 || c.Limits.CycleBudget <= 0},
	}
	for _, r := range requirements {
		if r.missing {
			return fmt.Errorf("%w: the %s is missing or not positive", ErrConfig, r.name)
		}
	}
	return nil
}

type notices []Event

func (n *notices) add(e Event) { *n = append(*n, e) }

func (s *Service) publish(ctx context.Context, n notices) {
	for _, e := range n {
		s.cfg.Notifier.Notify(ctx, e)
	}
}

func (s *Service) Send(ctx context.Context, c identity.Credential, e envelope.Envelope) (Receipt, error) {
	var n notices
	receipt, err := s.send(ctx, c, e, &n)
	s.publish(ctx, n)
	return receipt, err
}

func (s *Service) send(ctx context.Context, c identity.Credential, e envelope.Envelope, n *notices) (Receipt, error) {
	sender, members, err := s.authenticate(ctx, c)
	if err != nil {
		return Receipt{}, s.refuse(n, e, sender, RuleAuthenticate, err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if prior, found := s.state.accepted[e.ID]; found {
		return repeatOf(prior, sender, e)
	}
	msg, err := s.stamp(sender, e)
	if err != nil {
		return Receipt{}, s.refuse(n, e, sender, RuleValidate, err)
	}
	if err := s.state.checkLimits(s.cfg.Limits, sender, msg, s.cfg.Now()); err != nil {
		return Receipt{}, s.refuse(n, msg, sender, RuleLimit, err)
	}
	route, err := s.route(n, sender, msg, members)
	if err != nil {
		return Receipt{}, err
	}
	return s.accept(ctx, n, sender, msg, route)
}

func (s *Service) authenticate(ctx context.Context, c identity.Credential) (identity.Principal, []Member, error) {
	p, err := s.cfg.Auth.Authenticate(ctx, c)
	if err != nil {
		return identity.Principal{}, nil, err
	}
	if p.Cycle != s.cfg.Cycle {
		return identity.Principal{}, nil, fmt.Errorf("%w: a principal of cycle %d at the service of cycle %d", identity.ErrUnauthenticated, p.Cycle, s.cfg.Cycle)
	}
	members, err := s.cfg.Roster.Members(ctx, s.cfg.Cycle)
	if err != nil {
		return identity.Principal{}, nil, fmt.Errorf("broker: roster: %w", err)
	}
	for _, m := range members {
		if m.State == MemberLive && m.Principal == p {
			return m.Principal, members, nil
		}
	}
	return identity.Principal{}, nil, fmt.Errorf("%w: the dispatch %s is not a live roster entry of cycle %d with the identity of its token", identity.ErrUnauthenticated, p.Dispatch, s.cfg.Cycle)
}

func repeatOf(prior sent, sender identity.Principal, e envelope.Envelope) (Receipt, error) {
	if prior.from.Dispatch != sender.Dispatch {
		return Receipt{}, fmt.Errorf("%w: another sender used the id %s", ErrIDInUse, prior.receipt.ID)
	}
	if prior.content != contentDigest(e) {
		return Receipt{}, fmt.Errorf("%w: the id %s was accepted with other content", ErrIDConflict, prior.receipt.ID)
	}
	r := prior.receipt
	r.Duplicate = true
	return r, nil
}

func (s *Service) stamp(sender identity.Principal, e envelope.Envelope) (envelope.Envelope, error) {
	e.Schema, e.From, e.ContextID, e.TS, e.Hop = envelope.SchemaVersion, sender, s.cfg.Cycle, s.cfg.Now(), 0
	if e.ReplyTo != "" {
		parent, found := s.state.accepted[e.ReplyTo]
		if !found || !parent.involves(sender) {
			return e, fmt.Errorf("%w: reply_to %s", ErrNotFound, e.ReplyTo)
		}
		e.Hop = parent.hop + 1
	}
	if e.ID == "" {
		id, err := s.cfg.IDs.New()
		if err != nil {
			return e, err
		}
		e.ID = id
	}
	return e, e.Validate(s.cfg.Envelope)
}

func (s *Service) accept(ctx context.Context, n *notices, sender identity.Principal, msg envelope.Envelope, r routed) (Receipt, error) {
	cursor, err := s.cfg.Store.Append(ctx, s.cfg.Cycle, Record{Envelope: msg, Recipients: r.delivered})
	if err != nil {
		return Receipt{}, fmt.Errorf("broker: store: %w", err)
	}
	receipt := Receipt{ID: msg.ID, Cursor: cursor, Delivered: r.delivered, Refused: r.refused}
	s.state.record(sender, msg, receipt, s.cfg.Now())
	n.add(Event{
		Kind: EventSent, Cycle: s.cfg.Cycle, ID: msg.ID, MsgKind: msg.Kind,
		From: sender.Dispatch, To: keysOf(r.delivered), Hop: msg.Hop,
	})
	return receipt, nil
}

func (s *Service) refuse(n *notices, e envelope.Envelope, sender identity.Principal, rule string, err error) error {
	n.add(Event{
		Kind: EventDenied, Cycle: s.cfg.Cycle, ID: e.ID, MsgKind: e.Kind,
		From: sender.Dispatch, Rule: rule, Reason: err.Error(),
	})
	return err
}

func (s *Service) Read(ctx context.Context, c identity.Credential, from int64) (Mailbox, error) {
	var n notices
	box, err := s.read(ctx, c, from, &n)
	s.publish(ctx, n)
	return box, err
}

func (s *Service) read(ctx context.Context, c identity.Credential, from int64, n *notices) (Mailbox, error) {
	reader, _, err := s.authenticate(ctx, c)
	if err != nil {
		return Mailbox{}, err
	}
	if from < 0 {
		return Mailbox{}, fmt.Errorf("%w: %d is below 0", ErrCursor, from)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	batch, err := s.cfg.Store.Read(ctx, s.cfg.Cycle, from)
	if err != nil {
		return Mailbox{}, fmt.Errorf("broker: store: %w", err)
	}
	if from > batch.Next {
		return Mailbox{}, fmt.Errorf("%w: %d is past the head %d", ErrCursor, from, batch.Next)
	}
	box := Mailbox{Next: batch.Next, Gaps: batch.Gaps}
	for _, g := range batch.Gaps {
		n.add(Event{Kind: EventDead, Cycle: s.cfg.Cycle, ID: fmt.Sprintf("gap.%d.%d", g.From, g.To), Reason: ReasonGap})
	}
	seen := map[string]bool{}
	for _, r := range batch.Records {
		entry, addressed := recipientFor(r.Recipients, reader)
		if !addressed || seen[r.Envelope.ID] {
			continue
		}
		seen[r.Envelope.ID] = true
		if expired(r.Envelope, s.cfg.Now()) {
			if err := s.deadLetter(ctx, n, r.Envelope, entry); err != nil {
				return Mailbox{}, err
			}
			continue
		}
		box.Messages = append(box.Messages, r.Envelope)
	}
	s.state.consumed(reader, batch.Next)
	return box, nil
}

func (s *Service) deadLetter(ctx context.Context, n *notices, e envelope.Envelope, entry identity.Principal) error {
	key := e.ID + "|" + keyOf(entry)
	if s.state.dead[key] {
		return nil
	}
	if err := s.cfg.Store.DeadLetter(ctx, s.cfg.Cycle, Dead{Envelope: e, Recipient: entry, Reason: ReasonExpired}); err != nil {
		return fmt.Errorf("broker: dead letter: %w", err)
	}
	s.state.dead[key] = true
	n.add(Event{
		Kind: EventDead, Cycle: s.cfg.Cycle, ID: e.ID, MsgKind: e.Kind,
		From: e.From.Dispatch, To: []string{keyOf(entry)}, Hop: e.Hop, Reason: ReasonExpired,
	})
	return nil
}

func expired(e envelope.Envelope, now time.Time) bool {
	return e.ExpiresAt != nil && !now.Before(*e.ExpiresAt)
}

func (s *Service) Ack(ctx context.Context, c identity.Credential, cursor int64) error {
	reader, _, err := s.authenticate(ctx, c)
	if err != nil {
		return err
	}
	if cursor < 0 {
		return fmt.Errorf("%w: %d is below 0", ErrCursor, cursor)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	batch, err := s.cfg.Store.Read(ctx, s.cfg.Cycle, cursor)
	if err != nil {
		return fmt.Errorf("broker: store: %w", err)
	}
	if cursor > batch.Next {
		return fmt.Errorf("%w: %d is past the head %d", ErrCursor, cursor, batch.Next)
	}
	s.state.consumed(reader, cursor)
	return nil
}
