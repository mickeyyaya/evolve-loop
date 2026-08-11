package core

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// catalog_publisher_boundary_amplify_test.go — Test Amplifier (cycle-1429).
//
// Black-box adversarial tests for WithCatalogPublisher / CatalogPublisherWired
// and verdictInputsFor, authored from the exported-symbol contract only
// (`go doc ./internal/core WithCatalogPublisher`, `go doc ./internal/core
// Orchestrator`, plus the requiredSymbols signature block in this cycle's
// test-report.md), never from orchestrator.go / system_failure.go's control
// flow or the build diff.
//
// WithCatalogPublisher's own doc is explicit: "Nil is ignored, leaving the
// no-publish default (byte-identical legacy behavior)." That is a promise
// about a null input, and the null-input class is exactly what this phase
// is chartered to probe — including the combinatorial case (a nil option
// applied AFTER a valid one) the frozen TDD suite's
// TestRegisterMintedPhases_NoPublisherIsNoop does not exercise (that test
// covers "no option at all", not "nil option after a valid one").
//
// amplifyFakeStorage/amplifyFakeLedger are minimal port doubles satisfying
// this package's exported Storage/Ledger interfaces (per `go doc
// ./internal/core Storage` / `Ledger`) purely so NewOrchestrator can be
// constructed; none of their methods are exercised by these tests, which
// only probe post-construction predicates (never RunCycle).

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

// TestWithCatalogPublisher_NilFuncIsIgnored pins the doc-promised null-input
// behavior directly: constructing with WithCatalogPublisher(nil) alone must
// leave CatalogPublisherWired() false — matching the documented
// "byte-identical legacy behavior" when no publisher is wired at all.
func TestWithCatalogPublisher_NilFuncIsIgnored(t *testing.T) {
	o := newAmplifyTestOrchestrator(WithCatalogPublisher(nil))
	if o.CatalogPublisherWired() {
		t.Fatalf("WithCatalogPublisher(nil) must be ignored (doc: \"Nil is ignored, leaving the no-publish default\"); CatalogPublisherWired() = true, want false")
	}
}

// TestWithCatalogPublisher_NoOptionAtAllIsAlsoUnwired is the null-input
// control: omitting the option entirely must be indistinguishable from
// passing an explicit nil, per the documented "byte-identical legacy
// behavior" — both are the same no-publish default.
func TestWithCatalogPublisher_NoOptionAtAllIsAlsoUnwired(t *testing.T) {
	o := newAmplifyTestOrchestrator()
	if o.CatalogPublisherWired() {
		t.Fatalf("an orchestrator built with no catalog-publisher option must report CatalogPublisherWired() = false")
	}
}

// TestWithCatalogPublisher_TrailingNilDoesNotUnwireAnEarlierValidPublisher
// is the adversarial combinatorial case the doc's "nil is ignored" wording
// implies but the frozen suite does not exercise: applying a VALID
// publisher first, then a nil one, must not retroactively unwire it. If
// "ignored" were instead implemented as "always assign, and a nil fn value
// just looks unwired", this is exactly the input shape that would regress
// it — a later WithCatalogPublisher(nil) clobbering an earlier real one
// (e.g. from option-merging/defaulting call sites that append a trailing
// nil guard).
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

// TestWithCatalogPublisher_SecondValidPublisherOverridesFirst is the
// ordinary (non-nil) functional-options case: every other WithX option in
// this package follows last-applied-wins (a straight field assignment per
// call), and WithCatalogPublisher's doc gives no exception for this
// option — so two valid, distinct publishers applied in sequence must
// still leave the orchestrator wired (not somehow un-wired by the second
// assignment).
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
