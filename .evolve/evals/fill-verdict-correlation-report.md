---
score_cap:
  - criterion: "internal/contextfillcorrelate.Correlate buckets cycles by peak context fill and reports a per-bucket FAIL rate, so high-fill cycles are distinguishable from low-fill ones"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -run TestC1447_001_correlate_buckets_fill_against_verdict -count=1 ./acs/cycle1447"
  - criterion: "A cycle with no usable fill ratio or no verdict is reported as no-data, never fabricated into the zero bucket, and empty buckets yield a finite 0 rate rather than NaN"
    max_if_missing: 9
    evidence: "cd go && go test -tags acs -run TestC1447_002_missing_data_never_silently_zero -count=1 ./acs/cycle1447"
  - criterion: "`evolve context-fill correlate` is reachable from the top-level dispatch table and emits both the --json projection and the --out markdown artifact; a corpus-less root exits non-zero"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -run TestC1447_003_cli_reachable_from_dispatch_emits_report -count=1 ./acs/cycle1447"
  - criterion: "Over the real knowledge-base/cycles corpus, joined cycles plus no-data cycles account for every dossier on disk — no silent drops"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -run TestC1447_004_real_corpus_accounts_for_every_dossier -count=1 ./acs/cycle1447"
  - criterion: "internal/contextfillcorrelate completes ADR-0069 new-package graduation: enrolled in go/.apicover-enforce and every exported symbol named+exercised by apicover_named_test.go"
    max_if_missing: 6
    evidence: "cd go && go test -run TestAPICoverNamedExports -count=1 ./internal/contextfillcorrelate"
  - criterion: "The repo-contract scanner pack (phasespec, profiles, phasecoherence, routingtest) is GREEN in the lane worktree — the precondition that blocked cycle-1402's ship"
    max_if_missing: 9
    evidence: "cd go && go test -tags acs -run TestC1447_006_repo_contract_scanner_pack_green_in_lane -count=1 ./acs/cycle1447"
---

# Eval: Context-fill × verdict correlation report

> Pins part (3) of the P1 inbox item `context-fill-telemetry-and-cap` (weight
> 0.89): the join that turns per-phase context-fill telemetry into evidence.
> Parts (1) and (2) landed in cycle-1271 — `internal/contextfill` derives
> `FillRatio`/`IsHot`, and `phasetiming.Entry.ContextFillRatio` persists it at
> dispatch through `core.recordPhaseOutcome` (the ADR-0044 C1 chokepoint). What
> nothing did was correlate that fill% against the cycle's `final_verdict` in
> `knowledge-base/cycles/cycle-<n>.json`, which is the evidence that promotes or
> demotes the whole tokenopt band. Cycle-1402 builds that join
> (`internal/contextfillcorrelate` plus `evolve context-fill correlate`).
>
> The highest cap belongs to the no-data criterion, not the arithmetic one. The
> corpus is sparse — most historical dossiers have no persisted fill ratio — so
> an implementation that defaults absent fill to 0.0 would load the lowest
> bucket with hundreds of phantom cycles and manufacture exactly the correlation
> the report is supposed to measure. Absent evidence must read as absent.
>
> The CLI cap exists because a pure `Correlate` that no production path reaches
> is dead code: the predicate drives the built `evolve` binary through top-level
> dispatch (registry.go), not the function directly.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| bucket-arithmetic | Peak-fill bucketing with per-bucket FAIL rate; hot bucket bounded by `contextfill.HotThreshold`, not a re-declared literal | 8/10 | `go test -tags acs -run TestC1447_001... ./acs/cycle1447` |
| no-silent-zero | Missing fill / missing verdict → `NoData`; empty bucket rate finite, never NaN | 9/10 | `go test -tags acs -run TestC1447_002... ./acs/cycle1447` |
| cli-wiring-proof | Command reachable from top-level dispatch; `--json` + `--out` both emit; corpus-less root exits non-zero | 7/10 | `go test -tags acs -run TestC1447_003... ./acs/cycle1447` |
| corpus-conservation | joined + no-data == dossier count over the real corpus | 7/10 | `go test -tags acs -run TestC1447_004... ./acs/cycle1447` |
| apicover-graduation | ADR-0069 both halves for the new package | 6/10 | `go test -run TestAPICoverNamedExports ./internal/contextfillcorrelate` |
| scanner-pack-green | The four repo-contract guard suites are GREEN in-lane, each run as its own named package | 9/10 | `go test -tags acs -run TestC1447_006... ./acs/cycle1447` |

> Cycle-1447 addendum: this feature stalled for a full cycle because the
> scanner-pack precondition was only ever discovered AT ship, where the failure
> is terminal for the lane and carries no per-suite attribution. The
> `scanner-pack-green` cap moves that discovery into the cycle's own predicate
> lane and names the red suite, so a future landing that reintroduces a stray
> `.evolve/phases/<name>/phase.json` overlay or an unpaired phase↔agent pair
> fails with the suite named rather than as an opaque gate block. The predicate
> package was renamed `cycle1402` → `cycle1447` in the same move: `evolve acs
> suite --cycle N` binds only the current cycle's package, so a continuation
> that keeps the originating cycle's directory name ships predicates the gate
> never runs.
