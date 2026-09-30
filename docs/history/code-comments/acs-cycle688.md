# Comment history: `acs/cycle688`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle688/predicates_test.go:3` — above `package cycle688`

```text
// Package cycle688 materializes the cycle-688 acceptance criteria for the sole
// committed top_n task observer-sink-close-race (triage-report.md ## top_n;
// the scout's chronicle-s2-digest-writer pick was DEFERRED by triage as
// out-of-fleet-scope for this lane — R9.3: predicates bind ONLY to committed
// work, so no chronicle predicates appear here).
//
// AC map (1:1):
//
//	AC1 timeout arm never closes the sink (use-after-close race closed) → C688_001
//	AC2 <-done arm still closes exactly once (regression twin)          → C688_002
//	AC3 nil-closer contract preserved (Start's `if sinkCloser != nil`)  → C688_003
//	AC4 package green under -race + go vet clean                        → C688_004/005
//
// NOTE (pre-existing GREEN): the fix and its unit contract already landed on
// main via the cycle-669 commit 1d0e23ac (closeSinkAfterWait closes only on
// the <-done arm; WARN text documents the deliberate fd leak). These
// predicates therefore pin the behavior against regression rather than gate a
// fresh implementation — documented in test-report.md per the RED
// verification rules ("unexpected pass → pre-existing GREEN").
//
// Each predicate shells `go test -race -run '^<name>$' -v` over the unit-test
// contract, which EXERCISES the SUT (closeSinkAfterWait with real channels,
// timeouts, and a counting io.Closer) — behavioral via subprocess, no
// source-grep predicates (cycle-85 rule). The `-v` + "--- PASS:" guard
// rejects a rename/no-tests-matched silent green.
```
