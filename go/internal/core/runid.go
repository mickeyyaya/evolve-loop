package core

import (
	"context"
	"sync/atomic"
	"time"
)

const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

func MintRunID(t time.Time) string {
	var b [26]byte
	ms := uint64(t.UnixMilli())
	for i := 9; i >= 0; i-- {
		b[i] = crockford[ms&0x1f]
		ms >>= 5
	}
	var entropy [10]byte
	if _, err := randRead(entropy[:]); err != nil {
		entropy = [10]byte{}
	}
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

type stampingLedger struct {
	inner Ledger
	runID *atomic.Value
}

func (s stampingLedger) Append(ctx context.Context, e LedgerEntry) error {
	if id, _ := s.runID.Load().(string); id != "" && e.RunID == "" {
		e.RunID = id
	}
	return s.inner.Append(ctx, e)
}

func (s stampingLedger) Verify(ctx context.Context) error                 { return s.inner.Verify(ctx) }
func (s stampingLedger) Iter(ctx context.Context) (LedgerIterator, error) { return s.inner.Iter(ctx) }
