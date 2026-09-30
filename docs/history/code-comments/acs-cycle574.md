# Comment history: `acs/cycle574`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle574/predicates_test.go:3` — above `package cycle574`

```text
// Package cycle574 materialises the cycle-574 acceptance criteria for the single
// triage-committed top_n task (see scout-report.md / triage-report.md):
//
//   - fix-memo-phase-tier-envelope (inbox 0.95 critical) — the memo phase turns
//     a healthy PASS cycle abnormal post-ship. This has TWO halves:
//     (1) TIER: align the memo pin's model tier with its profile envelope so
//     ValidatePin stops raising "outside envelope". Landed in cycle-573
//     (internal/policy memo_envelope_config_test.go) — GREEN; guarded here
//     as REGRESSION coverage (TestC574_001).
//     (2) NON-FATAL OBSERVER: a memo failure AFTER a healthy ship must degrade
//     to a WARN diagnostic, never a cycle-level failure. This is the
//     still-unimplemented half and the RED work of cycle-574
//     (TestC574_002..004, driving the new internal/core classifier).
//
// Predicate strategy: behavioural-via-subprocess (the cycle-549…573 precedent).
// Each predicate shells `go test -run` over the RED unit tests authored this
// cycle in internal/core (postShipObserverSkip) plus the cycle-573 regression
// tests in internal/policy. None is a source-grep — every one exercises the
// system under test (postShipObserverSkip over a real orchestrator catalog/cfg;
// ValidatePin over the shipped config) and asserts on its result. RED now:
// internal/core fails to compile (postShipObserverSkip undefined). GREEN once
// Builder adds the classifier and wires it into the dispatch abort path.
```

### `go/acs/cycle574/predicates_test.go:57` — above `func TestC574_001_MemoTierEnvelopeAligned(t *testing.T) {`

```text
// TestC574_001_MemoTierEnvelopeAligned — AC-1 (regression, pre-existing GREEN):
// the shipped memo pin's model tier still satisfies (and lands inside the rank
// band of) its profile envelope, and envelope enforcement is not gutted. Drives
// the cycle-573 shipped-config tests in internal/policy. Guards against a
// regression of the tier half while cycle-574 adds the observer half.
```
