package memstore

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/address"
	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/broker"
	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/envelope"
	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/identity"
)

var _ broker.Store = (*Store)(nil)

func record(id string) broker.Record {
	return broker.Record{
		Envelope: envelope.Envelope{
			ID:   id,
			Kind: envelope.KindNote,
			To:   []address.Address{{Selector: address.SelectPhase, Value: "audit"}},
			Refs: []string{"build-report.md"},
		},
		Recipients: []identity.Principal{{Cycle: 5, Phase: "audit"}},
	}
}

func appendAll(t *testing.T, s *Store, cycle int, ids ...string) []int64 {
	t.Helper()
	var cursors []int64
	for _, id := range ids {
		c, err := s.Append(context.Background(), cycle, record(id))
		if err != nil {
			t.Fatalf("Append(%s) error = %v", id, err)
		}
		cursors = append(cursors, c)
	}
	return cursors
}

func ids(b broker.Batch) []string {
	var out []string
	for _, r := range b.Records {
		out = append(out, r.Envelope.ID)
	}
	return out
}

func TestStore_EachCycleHasItsOwnIncreasingCursors(t *testing.T) {
	s := New()
	five := appendAll(t, s, 5, "a", "b", "c")
	six := appendAll(t, s, 6, "x")
	if five[0] != 0 || five[1] != 1 || five[2] != 2 || six[0] != 0 {
		t.Fatalf("cursors = %v and %v, want 0,1,2 and 0", five, six)
	}
	b, err := s.Read(context.Background(), 6, 0)
	if err != nil || len(b.Records) != 1 || b.Records[0].Envelope.ID != "x" {
		t.Fatalf("Read(cycle 6) = %v, %v; want only x", ids(b), err)
	}
}

func TestStore_ReadFromACursorReturnsTheRestAndTheNextCursor(t *testing.T) {
	s := New()
	appendAll(t, s, 5, "a", "b", "c")
	b, err := s.Read(context.Background(), 5, 1)
	if err != nil {
		t.Fatalf("Read error = %v", err)
	}
	if got := ids(b); len(got) != 2 || got[0] != "b" || got[1] != "c" || b.Next != 3 {
		t.Fatalf("Read(5, 1) = %v next %d, want [b c] next 3", got, b.Next)
	}
	if b.Records[0].Cursor != 1 || b.Records[1].Cursor != 2 {
		t.Fatalf("cursors = %d, %d; want 1, 2", b.Records[0].Cursor, b.Records[1].Cursor)
	}
}

func TestStore_ReadAtOrPastTheEndReturnsNothingAndTheEnd(t *testing.T) {
	s := New()
	appendAll(t, s, 5, "a")
	for _, from := range []int64{1, 9} {
		b, err := s.Read(context.Background(), 5, from)
		if err != nil || len(b.Records) != 0 || b.Next != 1 {
			t.Fatalf("Read(5, %d) = %v next %d, %v; want nothing, next 1", from, ids(b), b.Next, err)
		}
	}
}

func TestStore_RefusesANegativeCursor(t *testing.T) {
	if _, err := New().Read(context.Background(), 5, -1); !errors.Is(err, ErrCursor) {
		t.Fatalf("Read(-1) error = %v, want ErrCursor", err)
	}
}

func TestStore_KeepsItsOwnCopyOfEachRecord(t *testing.T) {
	s := New()
	in := record("a")
	deadline := time.Date(2026, 10, 9, 22, 0, 0, 0, time.UTC)
	in.Envelope.ExpiresAt = &deadline
	if _, err := s.Append(context.Background(), 5, in); err != nil {
		t.Fatal(err)
	}
	in.Envelope.Refs[0] = "changed"
	in.Envelope.To[0].Value = "changed"
	in.Recipients[0].Phase = "changed"
	deadline = deadline.Add(time.Hour)
	out, _ := s.Read(context.Background(), 5, 0)
	out.Records[0].Envelope.Refs[0] = "changed too"
	again, _ := s.Read(context.Background(), 5, 0)
	r := again.Records[0]
	if r.Envelope.Refs[0] != "build-report.md" || r.Envelope.To[0].Value != "audit" || r.Recipients[0].Phase != "audit" || r.Envelope.ExpiresAt.Hour() != 22 {
		t.Fatalf("stored record = %+v, want the record as appended", r)
	}
}

func TestStore_DeadLettersArePerCycle(t *testing.T) {
	s := New()
	d := broker.Dead{Envelope: record("a").Envelope, Reason: "expired"}
	if err := s.DeadLetter(context.Background(), 5, d); err != nil {
		t.Fatal(err)
	}
	if got := s.Dead(5); len(got) != 1 || got[0].Reason != "expired" || got[0].Envelope.ID != "a" {
		t.Fatalf("Dead(5) = %+v, want the one dead letter", got)
	}
	if got := s.Dead(6); len(got) != 0 {
		t.Fatalf("Dead(6) = %+v, want none", got)
	}
}

func TestStore_ACanceledContextFailsEachCall(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s := New()
	if _, err := s.Append(ctx, 5, record("a")); !errors.Is(err, context.Canceled) {
		t.Fatalf("Append error = %v, want context.Canceled", err)
	}
	if _, err := s.Read(ctx, 5, 0); !errors.Is(err, context.Canceled) {
		t.Fatalf("Read error = %v, want context.Canceled", err)
	}
	if err := s.DeadLetter(ctx, 5, broker.Dead{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("DeadLetter error = %v, want context.Canceled", err)
	}
	if b, _ := s.Read(context.Background(), 5, 0); len(b.Records) != 0 {
		t.Fatalf("a canceled Append stored %v", ids(b))
	}
}
