package core

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

type amplifyFakeStorage struct{}

func (amplifyFakeStorage) ReadState(ctx context.Context) (State, error)  { return State{}, nil }
func (amplifyFakeStorage) WriteState(ctx context.Context, s State) error { return nil }
func (amplifyFakeStorage) ReadCycleState(ctx context.Context) (CycleState, error) {
	return CycleState{}, nil
}
func (amplifyFakeStorage) WriteCycleState(ctx context.Context, cs CycleState) error { return nil }
func (amplifyFakeStorage) AcquireLock(ctx context.Context) (func() error, error) {
	return func() error { return nil }, nil
}

type amplifyFakeLedger struct{}

func (amplifyFakeLedger) Append(ctx context.Context, entry LedgerEntry) error { return nil }
func (amplifyFakeLedger) Verify(ctx context.Context) error                    { return nil }
func (amplifyFakeLedger) Iter(ctx context.Context) (LedgerIterator, error)    { return nil, nil }

func newAmplifyTestOrchestrator(opts ...Option) *Orchestrator {
	return NewOrchestrator(amplifyFakeStorage{}, amplifyFakeLedger{}, nil, opts...)
}

func TestWithCatalogPublisher_NilFuncIsIgnored(t *testing.T) {
	o := newAmplifyTestOrchestrator(WithCatalogPublisher(nil))
	if o.CatalogPublisherWired() {
		t.Fatalf("WithCatalogPublisher(nil) must be ignored (doc: \"Nil is ignored, leaving the no-publish default\"); CatalogPublisherWired() = true, want false")
	}
}

func TestWithCatalogPublisher_NoOptionAtAllIsAlsoUnwired(t *testing.T) {
	o := newAmplifyTestOrchestrator()
	if o.CatalogPublisherWired() {
		t.Fatalf("an orchestrator built with no catalog-publisher option must report CatalogPublisherWired() = false")
	}
}

func TestWithCatalogPublisher_TrailingNilDoesNotUnwireAnEarlierValidPublisher(t *testing.T) {
	valid := func(phasespec.Catalog) {}
	o := newAmplifyTestOrchestrator(
		WithCatalogPublisher(valid),
		WithCatalogPublisher(nil),
	)
	if !o.CatalogPublisherWired() {
		t.Fatalf("a trailing WithCatalogPublisher(nil) must be a true no-op and must not unwire an earlier valid publisher; CatalogPublisherWired() = false, want true")
	}
}

func TestWithCatalogPublisher_SecondValidPublisherOverridesFirst(t *testing.T) {
	first := func(phasespec.Catalog) {}
	second := func(phasespec.Catalog) {}
	o := newAmplifyTestOrchestrator(
		WithCatalogPublisher(first),
		WithCatalogPublisher(second),
	)
	if !o.CatalogPublisherWired() {
		t.Fatalf("two valid WithCatalogPublisher options in sequence must leave CatalogPublisherWired() = true")
	}
}
