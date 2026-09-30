# Comment history: `acs/cycle767`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle767/predicates_test.go:3` — above `package cycle767`

```text
// Package cycle767 materializes the cycle-767 acceptance criteria for the sole
// committed top_n task dispatch-freshness-gate (triage-report.md ## top_n;
// fleet_scope pins this lane to exactly that id — the scout's own proposals
// were left for their owning cycle, so per R9.3 no predicates bind to them).
//
// Task source: inbox id dispatch-freshness-gate (weight 0.95, width-3 batch
// 2026-07-13 postmortem): ~3 of 8 failed lane-slots were doomed at dispatch —
// a task dispatched after it shipped, a task re-picked after landing
// (consumption raced), and a task dispatched 3x against an unmet dependency.
// The gate re-resolves every spec's scope ids against current inbox/consumed
// state + deps immediately before lane launch: stale ids are skipped with a
// logged reason, freed slots are refilled from the pending backlog, and an
// honest empty-scope build after the gate verdicts SKIPPED, never FAIL.
//
// AC map (1:1), from the inbox item's acceptance[] list:
//
//	AC1 skip consumed task + refill slot        → C767_001
//	AC2 skip deps-unmet task with named reason  → C767_002
//	AC3 empty-scope build after gate → SKIPPED  → C767_003
//	AC4 go test -race PASS                      → every predicate shells the
//	    unit contract under -race (apicover runs in the repo-wide gate);
//	    C767_004 pins the adversarial negative/edge axes (anti-no-op).
//
// Each predicate shells `go test -race -count=1 -v -run '^<name>$'` over the
// unit contract in internal/fleet, which EXERCISES FreshenSpecs /
// ClassifyEmptyScopeBuild through injected probe/refill fakes — behavioral
// via subprocess, no source-grep predicates (cycle-85 rule). The `-v` +
// "--- PASS:" guard rejects a rename/no-tests-matched silent green. The unit
// contract embeds the adversarial axes: negative (all-fresh wave untouched;
// real FAIL never masked; empty scope never PASSes), edge (empty backlog
// leaves the slot unfilled; partially-stale merged scope filters ids without
// burning the slot), semantic (skip-consumed vs skip-deps-unmet vs verdict
// classification are separate behaviors).
```
