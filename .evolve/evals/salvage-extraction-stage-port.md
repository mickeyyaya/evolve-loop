---
score_cap:
  - criterion: "The contract gate itself salvages a sole, unambiguous, recoverable bad_verdict — the extraction stage is reached from Reviewer.Review, not only from tests"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1441_001 ./acs/cycle1441"
  - criterion: "Salvage never acts when bad_verdict co-occurs with another violation (report-forgery bypass, cycle-1392 CRITICAL-1)"
    max_if_missing: 9
    evidence: "cd go && go test -tags acs -count=1 -run TestC1441_002 ./acs/cycle1441"
  - criterion: "Salvage refuses genuine ambiguity rather than picking one of several disagreeing verdict candidates (cycle-1406 CRITICAL-1)"
    max_if_missing: 9
    evidence: "cd go && go test -tags acs -count=1 -run TestC1441_003 ./acs/cycle1441"
  - criterion: "Salvage fails CLOSED and returns a byte-identical Result when the phase contract cannot be resolved (never flips OK from the classification alone)"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1441_004 ./acs/cycle1441"
  - criterion: "Every coercion is surfaced to the operator from the same sidecar the gate wrote — one counter, never two that can drift"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1441_005 ./acs/cycle1441"
  - criterion: "The ported salvage sources are git-TRACKED and their own package tests stay green"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run Salvage ./internal/deliverable"
---

# Eval: Land the schema-aligned salvage layer's extraction/coercion stage

> Pins the behavioural contract of the extraction stage landed in cycle-1441 by
> porting the stranded snapshot `a2d65920`
> (`.evolve/worktrees/cycle-42824668-1434`) onto main. The instrumentation half
> (`ClassifyBadVerdict`, `SummarizeBadVerdictBaseline`, `evolve salvage report`)
> had been on main since cycle-1389; the extraction half was built, green, and
> never landed across ten+ continuation worktrees with no PR ever opened.
> Source incidents: cycle-1296 (contract-gate CIRCUIT OPEN on repeated
> `bad_verdict` — the failure mode this stage exists to absorb), cycle-1392
> audit CRITICAL-1 / MEDIUM-3, cycle-1406 audit CRITICAL-1.
>
> The load-bearing caps are the two REFUSALS, not the salvage itself. A salvage
> layer that approves too eagerly is strictly worse than none: acting on a
> multi-violation result erases the anti-forgery proof-of-read check wholesale,
> and acting on multiple candidates manufactures a verdict the report never
> gave. Both are gate-decision bypasses, so both cap harder (9/10) than a
> missing salvage (8/10).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| production-wiring | Gate reaches the stage from `Reviewer.Review`, not from a test | 8/10 | `go test -tags acs -run TestC1441_001 ./acs/cycle1441` |
| sole-violation-only | Multi-violation deliverables are never salvaged | 9/10 | `go test -tags acs -run TestC1441_002 ./acs/cycle1441` |
| ambiguity-refusal | Multi-candidate deliverables are refused, not resolved | 9/10 | `go test -tags acs -run TestC1441_003 ./acs/cycle1441` |
| fail-closed | Unresolvable phase ⇒ refusal + byte-identical Result | 8/10 | `go test -tags acs -run TestC1441_004 ./acs/cycle1441` |
| surfaced-coercion | Summary line single-sourced from the applied sidecar | 6/10 | `go test -tags acs -run TestC1441_005 ./acs/cycle1441` |
| tracked-and-green | Ported files tracked; package salvage tests green | 7/10 | `go test -run Salvage ./internal/deliverable` |
