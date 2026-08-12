---
score_cap:
  - criterion: "`evolve salvage report -json` exposes a `saved` counter read from salvage-applied.jsonl, numerically distinct from the baseline's `recoverable`"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1441_007 ./acs/cycle1441"
  - criterion: "The `saved` key is present and zero — exit 0, not an error — when no salvage-applied.jsonl exists (the normal state of a fresh project root)"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1441_008 ./acs/cycle1441"
---

# Eval: `saved` counter on `evolve salvage report`, and the landed extraction docs

> Pins the operator-facing distinction cycle-1441 introduced: `recoverable` is
> measured POTENTIAL (how many blocked deliverables the classifier judged
> repairable, folded from `bad-verdict-baseline.jsonl`), while `saved` is ACTUAL
> coercions (how many times the extraction stage fired, folded from
> `salvage-applied.jsonl`). Before the extraction stage landed the two numbers
> were necessarily the same story; afterwards, reading potential as if it were
> actual overstates what the gate did — the false-confidence failure the research
> memo (docs/research/deliverable-alignment-2026-08/README.md §3.3) is written
> against.
>
> The predicate fixture deliberately makes the two counts differ (3 recoverable,
> 2 saved) so a CLI that aliases one to the other cannot pass, and counts only
> `salvage_applied` event types so a foreign emitter sharing the sidecar cannot
> inflate the figure. Source incident: cycle-1441 (this cycle) — the counter did
> not exist; `evolve salvage report` could report only potential.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| saved-counter | `saved` present, sourced from the applied sidecar, ≠ `recoverable` | 8/10 | `go test -tags acs -run TestC1441_007 ./acs/cycle1441` |
| absent-sidecar-is-normal | `saved: 0` + exit 0 with no applied sidecar | 6/10 | `go test -tags acs -run TestC1441_008 ./acs/cycle1441` |
