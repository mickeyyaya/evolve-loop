---
score_cap:
  - criterion: "TestC1529_004_ClosureStaysDocOnly passes on a later branch that changes go/internal/bridge relative to main (committed and uncommitted), and on a checkout that trails a later bridge change landed on main"
    max_if_missing: 9
    evidence: "cd go && go test -count=1 -tags acs -run '^TestC1715_001_LaterBridgeChangeDoesNotFailC1529Closure$' ./acs/cycle1715/"
  - criterion: "TestC1529_004_ClosureStaysDocOnly passes on cycle 1529's real doc-only range and FAILs, naming the file, when cycle 1529's own ship commit (57e227c1, base 19b427c4) is made to carry a go/internal/bridge source"
    max_if_missing: 9
    evidence: "cd go && go test -count=1 -tags acs -run '^TestC1715_002_BridgeChangeInsideC1529RangeStillFails$' ./acs/cycle1715/"
  - criterion: "The cycle 1529 predicate package still compiles and its closure predicate is green in the real checkout"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -tags acs -run '^TestC1529_004_ClosureStaysDocOnly$' ./acs/cycle1529/"
---

# Eval: cycle 1529's closure predicate judges cycle 1529's own diff, not the live `main` ref

> Pins the fix for inbox item `acs-cycle1529-doc-only-predicate-diffs-against-local-main`
> (cycle 1715). `TestC1529_004_ClosureStaysDocOnly` proves cycle 1529's
> closure of the stale `completion-contract-cancel-parity` item stayed
> doc-only, but it ran `git diff --name-only main -- go/internal/bridge`
> against the checkout. That compares the tree with whatever `main` is
> today, so any later bridge change on any branch failed it. ADR-0103 unit
> 10, a bridge launch-outcome leaf, tripped it in a whole-module floor on
> 2026-09-14. The cycle-1715 bug reproduction counted 269 bridge files
> between cycle 1529's ship commit and live main, none written by cycle 1529.
> Cycle 1529's own range, `19b427c4..57e227c1`, touches four files, none of
> them under `go/internal/bridge`.
>
> Both cycle-1715 predicates compile the real cycle1529 package and run
> `TestC1529_004` inside a throwaway repository. That repository borrows
> this one's objects through alternates and checks out only
> `go/internal/bridge`. The negative axis uses `git replace` to give the ship
> commit a bridge source, with no drift from `main`. A predicate that is
> skipped, deleted, or that stops reading cycle 1529's range fails it.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| later-drift-green | bridge drift from `main` in either direction, committed or uncommitted, does not fail the closure predicate | 9/10 | `TestC1715_001_LaterBridgeChangeDoesNotFailC1529Closure` |
| own-range-still-bites | a bridge source inside cycle 1529's own base..ship range fails the predicate, and the failure names that file | 9/10 | `TestC1715_002_BridgeChangeInsideC1529RangeStillFails` |
| real-checkout-green | the cycle1529 package compiles and `TestC1529_004` passes in the real checkout | 6/10 | `TestC1529_004_ClosureStaysDocOnly` |
