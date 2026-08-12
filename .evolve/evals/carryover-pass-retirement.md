---
score_cap:
  - criterion: "core.RetireCarryoverTodos retires committed ids and their cross-cycle fingerprint variants, leaves everything else in order, and never mutates its input"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestRetireCarryoverTodos_' ./internal/core"
  - criterion: "the PRODUCTION PASS closeout (promoteInbox) reaches the retirement seam and removes the committed id from state.json, gated on the same landing check promotion rides"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run '^TestPromoteInbox_(LandedPassRetiresCommittedCarryover|UnlandedPassKeepsCarryover|NoStateFileIsNoOp)$' ./internal/phases/ship"
---

# Eval: carryover todos retire at PASS closeout

> Pins the removal half of the carryover lifecycle. `mergeCarryoverTodos`
> (go/internal/core/failure_learning.go) unions and dedupes but has no deletion
> path, so a todo whose work actually shipped persists forever and saturates the
> router prompt's 20-slot carryover window with already-done work — the
> 2026-08-10 investigation found 124 of 254 live entries were stale re-mints of a
> few classes. PR #439 closed the FAIL-side identity half and left this gap open.
> Source incident: cycle-1440 (inbox item `carryover-pass-retirement`).
>
> The wiring criterion carries the higher cap on purpose: a green pure seam with
> no production caller is dead code, which is the exact failure mode this eval
> exists to make permanent.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| retirement-semantics | committed id + fingerprint variant retire; unmatched survive in order; input not mutated | 6/10 | `go test -run '^TestRetireCarryoverTodos_' ./internal/core` |
| production-wiring | landed PASS closeout mutates state.json; unlanded ship retires nothing; missing state.json is a no-op | 8/10 | `go test -run '^TestPromoteInbox_(LandedPassRetiresCommittedCarryover\|UnlandedPassKeepsCarryover\|NoStateFileIsNoOp)$' ./internal/phases/ship` |
