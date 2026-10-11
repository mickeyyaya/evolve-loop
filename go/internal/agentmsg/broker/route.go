package broker

import (
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/address"
	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/envelope"
	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/identity"
	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/policy"
)

type routed struct {
	delivered []identity.Principal
	refused   []Refusal
}

func (s *Service) route(n *notices, sender identity.Principal, msg envelope.Envelope, members []Member) (routed, error) {
	if err := s.checkScope(n, sender, msg); err != nil {
		return routed{}, err
	}
	candidates, err := resolve(members, sender, msg.To)
	if err != nil {
		return routed{}, s.refuse(n, msg, sender, RuleRoster, err)
	}
	var r routed
	for _, m := range candidates {
		if refusal, ok := s.admit(n, sender, msg, m); !ok {
			r.refused = append(r.refused, refusal)
			continue
		}
		r.delivered = append(r.delivered, m)
	}
	if len(r.delivered) == 0 {
		return routed{}, nobodyAdmitted(r.refused)
	}
	return r, nil
}

func (s *Service) checkScope(n *notices, sender identity.Principal, msg envelope.Envelope) error {
	for _, a := range msg.To {
		target := a.Resolve(sender.Cycle).Cycle
		if target == s.cfg.Cycle {
			continue
		}
		d := s.cfg.Policy.Authorize(policy.Request{Sender: sender, Recipient: identity.Principal{Cycle: target}, Kind: msg.Kind})
		if !d.Allow {
			return s.refuse(n, msg, sender, d.Rule, unknownRecipient(a))
		}
		return s.refuse(n, msg, sender, d.Rule, fmt.Errorf("%w: %s: the service holds only cycle %d", ErrUnroutable, a, s.cfg.Cycle))
	}
	return nil
}

func unknownRecipient(a address.Address) error {
	return fmt.Errorf("%w: no agent in scope matches %s", ErrUnknownRecipient, a)
}

func resolve(members []Member, sender identity.Principal, to []address.Address) ([]identity.Principal, error) {
	var out []identity.Principal
	seen := map[string]bool{}
	for _, a := range to {
		matched := false
		for _, m := range members {
			if !selects(a, m) || m.Dispatch == sender.Dispatch {
				continue
			}
			matched = true
			if !seen[keyOf(m.Principal)] {
				seen[keyOf(m.Principal)] = true
				out = append(out, m.Principal)
			}
		}
		if !matched {
			return nil, unknownRecipient(a)
		}
	}
	return out, nil
}

func selects(a address.Address, m Member) bool {
	if m.State == MemberPast && a.Selector != address.SelectDispatch {
		return false
	}
	switch a.Selector {
	case address.SelectRole:
		return m.Role == a.Value
	case address.SelectPhase:
		return m.Phase == a.Value
	case address.SelectDispatch:
		return m.Dispatch == a.Value
	}
	return true
}

func (s *Service) admit(n *notices, sender identity.Principal, msg envelope.Envelope, m identity.Principal) (Refusal, bool) {
	req := policy.Request{Sender: sender, Recipient: m, Kind: msg.Kind, ReplyGrant: s.state.granted(msg, sender, m, s.cfg.Now())}
	d := s.cfg.Policy.Authorize(req)
	refusal := Refusal{Recipient: m, Rule: d.Rule, Reason: d.Reason}
	if d.Allow {
		if s.state.queueLen(m) < s.cfg.Limits.RecipientQueue {
			return Refusal{}, true
		}
		refusal.Rule, refusal.Reason = RuleQueueCap, fmt.Sprintf("the queue of %s holds %d messages", keyOf(m), s.cfg.Limits.RecipientQueue)
	}
	n.add(Event{
		Kind: EventDenied, Cycle: s.cfg.Cycle, ID: msg.ID, MsgKind: msg.Kind,
		From: sender.Dispatch, To: []string{keyOf(m)}, Hop: msg.Hop, Rule: refusal.Rule, Reason: refusal.Reason,
	})
	return refusal, false
}

func nobodyAdmitted(refused []Refusal) error {
	for _, r := range refused {
		if r.Rule != RuleQueueCap {
			return fmt.Errorf("%w: %s: %s", ErrDenied, keyOf(r.Recipient), r.Reason)
		}
	}
	return fmt.Errorf("%w: %s", ErrQueueFull, refused[0].Reason)
}

func keyOf(p identity.Principal) string {
	if p.Dispatch != "" {
		return p.Dispatch
	}
	return "phase:" + p.Phase
}

func keysOf(ps []identity.Principal) []string {
	out := make([]string, 0, len(ps))
	for _, p := range ps {
		out = append(out, keyOf(p))
	}
	return out
}

func recipientFor(entries []identity.Principal, reader identity.Principal) (identity.Principal, bool) {
	for _, e := range entries {
		if addressedTo(e, reader) {
			return e, true
		}
	}
	return identity.Principal{}, false
}

func addressedTo(entry, p identity.Principal) bool {
	if entry.Dispatch != "" {
		return entry.Dispatch == p.Dispatch
	}
	return entry.Phase == p.Phase
}
