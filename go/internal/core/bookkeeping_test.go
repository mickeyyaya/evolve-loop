package core

import (
	"errors"
	"testing"
)

type recordingFlusher struct {
	got   []BookkeepingEntry
	err   error
	calls int
}

func (f *recordingFlusher) Flush(entries []BookkeepingEntry) error {
	f.calls++
	f.got = append(f.got, entries...)
	return f.err
}

func TestBookkeepingBatch_AddRefusesAnEntryWithNoPaths(t *testing.T) {
	t.Parallel()
	b := NewBookkeepingBatch()
	if err := b.Add(BookkeepingEntry{Kind: BookkeepingDossier, Message: "m"}); err == nil {
		t.Fatal("Add of a pathless entry returned nil")
	}
	if b.Len() != 0 {
		t.Fatalf("Len = %d, want 0", b.Len())
	}
}

func TestBookkeepingBatch_FlushToHandsOverAllAndEmptiesTheBatch(t *testing.T) {
	t.Parallel()
	b := NewBookkeepingBatch()
	_ = b.Add(BookkeepingEntry{Kind: BookkeepingDossier, Paths: []string{"a"}})
	_ = b.Add(BookkeepingEntry{Kind: BookkeepingInboxStamp, Paths: []string{"b"}})
	f := &recordingFlusher{}
	if err := b.FlushTo(f); err != nil {
		t.Fatal(err)
	}
	if len(f.got) != 2 || b.Len() != 0 {
		t.Fatalf("flushed %d, left %d; want 2 and 0", len(f.got), b.Len())
	}
}

func TestBookkeepingBatch_FlushToKeepsEntriesWhenTheFlusherFails(t *testing.T) {
	t.Parallel()
	b := NewBookkeepingBatch()
	_ = b.Add(BookkeepingEntry{Kind: BookkeepingDossier, Paths: []string{"a"}})
	if err := b.FlushTo(&recordingFlusher{err: errors.New("down")}); err == nil {
		t.Fatal("FlushTo swallowed the flusher error")
	}
	if b.Len() != 1 {
		t.Fatalf("Len = %d, want the entry kept", b.Len())
	}
}

func TestBookkeepingBatch_FlushToSkipsAnEmptyBatch(t *testing.T) {
	t.Parallel()
	f := &recordingFlusher{}
	if err := NewBookkeepingBatch().FlushTo(f); err != nil || f.calls != 0 {
		t.Fatalf("err %v, flusher called %d times on an empty batch", err, f.calls)
	}
}

type addingFlusher struct{ b *BookkeepingBatch }

func (f addingFlusher) Flush([]BookkeepingEntry) error {
	_ = f.b.Add(BookkeepingEntry{Kind: BookkeepingDossier, Paths: []string{"second"}})
	return errors.New("down")
}

func TestBookkeepingBatch_AFailedFlushPutsTheDrainedEntriesBeforeTheLateOnes(t *testing.T) {
	t.Parallel()
	b := NewBookkeepingBatch()
	_ = b.Add(BookkeepingEntry{Kind: BookkeepingDossier, Paths: []string{"first"}})
	if err := b.FlushTo(addingFlusher{b}); err == nil {
		t.Fatal("FlushTo swallowed the flusher error")
	}
	got := b.Drain()
	if len(got) != 2 || got[0].Paths[0] != "first" || got[1].Paths[0] != "second" {
		t.Fatalf("drained %+v, want first then second", got)
	}
}
