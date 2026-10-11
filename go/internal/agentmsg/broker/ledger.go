package broker

import (
	"crypto/sha256"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/envelope"
	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/identity"
)

const ratePeriod = time.Minute

type sent struct {
	receipt Receipt
	from    identity.Principal
	hop     int
	content [sha256.Size]byte
}

type grant struct {
	responder identity.Principal
	requester identity.Principal
	expires   time.Time
}

type ledger struct {
	accepted map[string]sent
	sends    map[string][]time.Time
	recent   map[[sha256.Size]byte]time.Time
	pending  map[string][]int64
	grants   map[string][]grant
	dead     map[string]bool
	total    int
}

func newLedger() *ledger {
	return &ledger{
		accepted: map[string]sent{},
		sends:    map[string][]time.Time{},
		recent:   map[[sha256.Size]byte]time.Time{},
		pending:  map[string][]int64{},
		grants:   map[string][]grant{},
		dead:     map[string]bool{},
	}
}

func (l *ledger) checkLimits(lim Limits, sender identity.Principal, msg envelope.Envelope, now time.Time) error {
	if msg.Hop > lim.MaxHops {
		return fmt.Errorf("%w: hop %d passes the limit %d", ErrHopLimit, msg.Hop, lim.MaxHops)
	}
	if l.total >= lim.CycleBudget {
		return fmt.Errorf("%w: the cycle accepted %d messages", ErrBudget, l.total)
	}
	if n := len(within(l.sends[sender.Dispatch], now, ratePeriod)); n >= lim.SenderPerMinute {
		return fmt.Errorf("%w: %s sent %d messages in the last minute", ErrRateLimited, sender.Dispatch, n)
	}
	if last, found := l.recent[digest(sender, msg)]; found && now.Sub(last) < lim.RepeatWindow {
		return fmt.Errorf("%w: %s sent the same message at %s", ErrRepeat, sender.Dispatch, last.Format(time.RFC3339))
	}
	return nil
}

func (l *ledger) record(sender identity.Principal, msg envelope.Envelope, r Receipt, now time.Time) {
	l.accepted[msg.ID] = sent{receipt: r, from: sender, hop: msg.Hop, content: contentDigest(msg)}
	l.sends[sender.Dispatch] = append(within(l.sends[sender.Dispatch], now, ratePeriod), now)
	l.recent[digest(sender, msg)] = now
	for _, m := range r.Delivered {
		l.pending[keyOf(m)] = append(l.pending[keyOf(m)], r.Cursor)
		if msg.Kind == envelope.KindRequest {
			g := grant{responder: m, requester: sender, expires: *msg.ExpiresAt}
			l.grants[msg.CorrelationID] = append(l.grants[msg.CorrelationID], g)
		}
	}
	l.total++
}

func (l *ledger) granted(msg envelope.Envelope, responder, recipient identity.Principal, now time.Time) bool {
	if msg.Kind != envelope.KindReply {
		return false
	}
	for _, g := range l.grants[msg.CorrelationID] {
		if addressedTo(g.responder, responder) && g.requester.Dispatch == recipient.Dispatch && now.Before(g.expires) {
			return true
		}
	}
	return false
}

func (l *ledger) queueLen(m identity.Principal) int { return len(l.pending[keyOf(m)]) }

func (l *ledger) consumed(reader identity.Principal, from int64) {
	for _, key := range []string{reader.Dispatch, "phase:" + reader.Phase} {
		l.pending[key] = slices.DeleteFunc(l.pending[key], func(c int64) bool { return c < from })
	}
}

func (s sent) involves(p identity.Principal) bool {
	return s.from.Dispatch == p.Dispatch || slices.ContainsFunc(s.receipt.Delivered, func(e identity.Principal) bool { return addressedTo(e, p) })
}

func within(times []time.Time, now time.Time, period time.Duration) []time.Time {
	return slices.DeleteFunc(slices.Clone(times), func(t time.Time) bool { return now.Sub(t) >= period })
}

func digest(sender identity.Principal, msg envelope.Envelope) [sha256.Size]byte {
	parts := []string{sender.Dispatch, string(msg.Kind), strings.Join(addressTexts(msg), ","), msg.Subject, msg.Body}
	return sha256.Sum256([]byte(strings.Join(parts, "\x00")))
}

func contentDigest(e envelope.Envelope) [sha256.Size]byte {
	expires := ""
	if e.ExpiresAt != nil {
		expires = e.ExpiresAt.UTC().Format(time.RFC3339Nano)
	}
	fields := []string{string(e.Kind), e.Subject, e.Body, e.ReplyTo, e.CorrelationID, expires, strconv.Itoa(len(e.To))}
	fields = append(append(fields, addressTexts(e)...), e.Refs...)
	quoted := make([]string, 0, len(fields))
	for _, f := range fields {
		quoted = append(quoted, strconv.Quote(f))
	}
	return sha256.Sum256([]byte(strings.Join(quoted, " ")))
}

func addressTexts(e envelope.Envelope) []string {
	to := make([]string, 0, len(e.To))
	for _, a := range e.To {
		to = append(to, a.String())
	}
	return to
}
