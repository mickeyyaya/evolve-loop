package core

import (
	"context"
	"crypto/rand"
	"sync/atomic"
	"time"
)

// runid.go — CA.5 (concurrency-factory plan, Track C-A): the event-sourced
// run identity. One ULID per RunCycle, threaded into the persisted
// CycleState and stamped onto every ledger entry the run emits (via the
// stampingLedger decorator — zero churn at the ~30 Append call sites), so
// concurrent runs' interleaved entries stay attributable.
//
// The ULID is implemented inline (26-char Crockford base32: 48-bit
// millisecond timestamp + 80-bit crypto/rand) — the repo carries no
// third-party deps and the spec subset needed here is ~30 lines.

// crockford is the ULID alphabet (no I, L, O, U).
const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// MintRunID returns a ULID for the given time: lexicographic order follows
// time order, and the 80 random bits make same-millisecond mints unique.
func MintRunID(t time.Time) string {
	var b [26]byte
	ms := uint64(t.UnixMilli())
	// 48-bit timestamp → 10 base32 chars, most-significant first.
	for i := 9; i >= 0; i-- {
		b[i] = crockford[ms&0x1f]
		ms >>= 5
	}
	var entropy [10]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		// crypto/rand never fails on supported platforms; a zeroed entropy
		// tail still yields a valid (time-ordered) id rather than a panic.
		entropy = [10]byte{}
	}
	// 80 entropy bits → 16 base32 chars (5 bits each).
	var acc uint64
	bits := 0
	pos := 10
	for _, eb := range entropy {
		acc = acc<<8 | uint64(eb)
		bits += 8
		for bits >= 5 {
			bits -= 5
			b[pos] = crockford[(acc>>uint(bits))&0x1f]
			pos++
		}
	}
	return string(b[:])
}

// stampingLedger decorates a Ledger so every appended entry carries the
// CURRENT run's id. Installed ONCE at construction (NewOrchestrator) — the
// per-run identity flows through the atomic value, never by mutating the
// orchestrator's ledger field (an interface store is a two-word write; a
// goroutine-spawning Observer reading o.ledger concurrently would race).
// An empty current id (no run in flight, or a bare test Orchestrator)
// stamps nothing. Verify/Iter pass through.
type stampingLedger struct {
	inner Ledger
	runID *atomic.Value // holds string; "" or unset ⇒ no stamp
}

func (s stampingLedger) Append(ctx context.Context, e LedgerEntry) error {
	if id, _ := s.runID.Load().(string); id != "" && e.RunID == "" {
		e.RunID = id
	}
	return s.inner.Append(ctx, e)
}

func (s stampingLedger) Verify(ctx context.Context) error                 { return s.inner.Verify(ctx) }
func (s stampingLedger) Iter(ctx context.Context) (LedgerIterator, error) { return s.inner.Iter(ctx) }
