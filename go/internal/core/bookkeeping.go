package core

import (
	"errors"
	"fmt"
	"sync"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type BookkeepingKind string

const (
	BookkeepingDossier    BookkeepingKind = "dossier_closeout"
	BookkeepingInboxStamp BookkeepingKind = "inbox_stamp"
)

type BookkeepingEntry struct {
	Kind    BookkeepingKind
	Paths   []string
	Message string
}

type BookkeepingSink interface {
	Add(BookkeepingEntry) error
}

type BookkeepingFlusher interface {
	Flush(entries []BookkeepingEntry) error
}

type BookkeepingBatch struct {
	mu      sync.Mutex
	entries []BookkeepingEntry
}

func NewBookkeepingBatch() *BookkeepingBatch { return &BookkeepingBatch{} }

func (b *BookkeepingBatch) Add(e BookkeepingEntry) error {
	if len(e.Paths) == 0 {
		return errors.New("bookkeeping entry names no path")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.entries = append(b.entries, e)
	return nil
}

func (b *BookkeepingBatch) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.entries)
}

func (b *BookkeepingBatch) Drain() []BookkeepingEntry {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := b.entries
	b.entries = nil
	return out
}

func (b *BookkeepingBatch) FlushTo(f BookkeepingFlusher) error {
	entries := b.Drain()
	if len(entries) == 0 {
		return nil
	}
	if err := f.Flush(entries); err != nil {
		b.mu.Lock()
		b.entries = append(entries, b.entries...)
		b.mu.Unlock()
		return fmt.Errorf("flush the bookkeeping batch: %w", err)
	}
	return nil
}

func WithBookkeeping(stage string, sink BookkeepingSink) Option {
	return func(o *Orchestrator) {
		o.bookkeepingStage = stage
		o.bookkeepingSink = sink
	}
}

func (o *Orchestrator) batchesBookkeeping() bool {
	return o.bookkeepingStage == policy.FleetIntegrationEnforce && o.bookkeepingSink != nil
}
