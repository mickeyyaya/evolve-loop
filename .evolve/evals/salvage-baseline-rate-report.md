---
score_cap:
  - criterion: "SummarizeBadVerdictBaseline computes the recoverable-malformed rate and per-pattern counts, ignoring foreign events in the shared sidecar"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1407_001 ./acs/cycle1407"
  - criterion: "The summarizer rejects a torn JSONL line loudly and returns Rate 0 (never NaN) on an empty baseline"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1407_002 ./acs/cycle1407"
  - criterion: "The rate is reachable from the real `evolve` binary, not only from tests — the summarizer has a production caller"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1407_003 ./acs/cycle1407"
  - criterion: "The README documents the surface with an invocation that actually executes and prints the rate"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC1407_004 ./acs/cycle1407"
  - criterion: "New exported symbols in the apicover-enrolled internal/deliverable package are named and executed, so the repo-wide ADR-0069 gate stays green"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1407_009 ./acs/cycle1407"
---

# Eval: salvage-baseline-rate-report

> Pins the first *consumer* of `.evolve/bad-verdict-baseline.jsonl`. Cycle-1389
> landed the instrumentation half of the `schema-aligned-salvage-layer`
> portfolio item — `ClassifyBadVerdict` plus a sidecar writer — under an
> explicit "measurement before extraction" mandate. Cycle-1407's scout then
> found the mandate half-executed: the file has been written for 18 cycles and
> **nothing has ever read it**. A grep for `bad-verdict-baseline` outside the
> writer and its own tests returns only the writer. The item's extraction stage
> is gated on a measured recoverable-malformed *rate*, so the gate was blocked
> on a number no code computed, and the instrumentation was accumulating data
> toward a decision it could not inform. This eval keeps that gap closed.
>
> The load-bearing cap is the **reachability** one (8/10). A pure summarizer
> whose only caller is a test is dead code, and dead code satisfies a unit test
> forever while leaving the operator exactly as blind as before the cycle. The
> criterion is therefore stated against the real `evolve` binary's exit code and
> stdout, not against the function.
>
> Source incidents: cycle-1389 (instrumentation landed without a consumer);
> cycle-644 (a frozen structural pin that was never reachability-probed burned a
> whole cycle — hence the CI-shaped, profile-supplied apicover criterion rather
> than a flagless one that would have demanded 22 unrelated fixes).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| rate-correctness | Hand-computed 3/5 = 0.6 with per-pattern breakdown; foreign events skipped | 7/10 | `go test -tags acs -run TestC1407_001 ./acs/cycle1407` |
| loud-on-torn-input | Truncated line errors; empty baseline yields 0, never NaN | 6/10 | `go test -tags acs -run TestC1407_002 ./acs/cycle1407` |
| production-reachability | `evolve salvage report -json` surfaces the rate end to end | 8/10 | `go test -tags acs -run TestC1407_003 ./acs/cycle1407` |
| executable-docs | The README-published invocation runs and prints the rate | 5/10 | `go test -tags acs -run TestC1407_004 ./acs/cycle1407` |
| apicover-graduation | New exported symbols named + executed in the enrolled package | 7/10 | `go test -tags acs -run TestC1407_009 ./acs/cycle1407` |
