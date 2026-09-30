# Comment history: `acs/cycle669`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle669/predicates_test.go:3` — above `package cycle669`

```text
// Package cycle669 materialises the cycle-669 acceptance criteria for the single
// triage-committed (`## top_n`) task: observer-sink-close-race.
//
// TASK BINDING (R9.3 — predicates bind ONLY to triage `## top_n` work):
//
//	triage-report.md commits exactly ONE task to this lane:
//	  observer-sink-close-race (weight 0.92, priority H) — C669_001..003
//	Nothing is deferred or dropped in this lane (fleet scope pins it to this
//	item); the scout-report's `new-package-graduation-buildentry-gate`
//	narrative belongs to ANOTHER lane and gets ZERO predicates here.
//
// FEATURE CONTEXT
//
//	core_adapter.go's cancel closure waits ≤10s for the watcher goroutine,
//	then (pre-fix) unconditionally closed the events-sink *os.File. A watcher
//	wedged past the bound (e.g. a stuck liveness probe) later calls
//	emit() → sink.Write() — a use-after-close race on the fd. Fix of record:
//	close ONLY on the <-done arm (closeSinkAfterWait); the timeout arm accepts
//	the fd leak (OS reclaims at exit) and WARNs.
//
// PRE-EXISTING GREEN (disclosed, not gamed): the fix and its behavioural
// tests LANDED in commit 22a90595 (2026-07-08) — core_adapter.go's
// closeSinkAfterWait + core_adapter_sinkclose_test.go (+ the _amplify twin).
// The inbox item was stale when this cycle picked it up. These predicates are
// therefore GREEN at authoring time; their value is REGRESSION PINNING: any
// later edit that reverts to an unconditional Close, renames/deletes the
// behavioural tests, or introduces a race in the package flips them RED at
// audit time. The TDD report records the pre-existing-GREEN status per the
// RED-verification rules.
//
// PREDICATE QUALITY (cycle-85): every predicate EXERCISES the SUT — it shells
// `go test` / `go vet` against the real package and asserts the NAMED
// behavioural test emitted a `--- PASS:` marker in -v output. `go test -run X`
// on a missing test exits 0 with "no tests to run", so a bare exit-code check
// would vacuously green; the marker assertion defeats that hole.
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - Negative : C669_001 — the timeout arm must NOT Close (Close count 0
//     asserted by the bound test); run under -race so the exact reported
//     failure mode is exercised.
//   - Positive : C669_002 — the <-done arm still closes EXACTLY once (defeats
//     a degenerate "never close" fix) + nil-closer edge stays panic-free.
//   - Hygiene  : C669_003 — `go vet` clean AND the WHOLE package green under
//     -race (the inbox AC-3), not just the two named tests.
//
// TEST-NAME CONTRACT — the predicates target these exact committed test names
// in internal/adapters/observer (do NOT rename without updating this file):
//
//	TestCoreAdapter_NoSinkCloseRaceOnTimeout
//	TestCoreAdapter_SinkClosedOnNormalDone
//	TestCoreAdapter_CloseSinkAfterWait_NilCloserSafe
```
