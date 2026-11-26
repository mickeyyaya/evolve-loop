# internal/cycleblocker

> Plan: [slow-cycle-blocker-attribution-2026-11](../../plans/slow-cycle-blocker-attribution-2026-11.md). Extends item 1 of [cost-aware-repair-2026-10](../../plans/cost-aware-repair-2026-10.md). Callers: `internal/core` (the seal-time writer) and `cmd/evolve` (`wave status`).

## Purpose

`internal/cycleblocker` names the one cause that holds most of the minutes of a sealed cycle. It is pure. It reads no file and no clock, and it imports only `internal/cyclespend`.

## Design

- **`Attribute`.** `Attribute(in, waitSharePct)` gives an `Attribution` of a `Blocker` name and its `Minutes`.
- **Failed cycle, with a check tag.** The blocker is `check:<tag>`, the first tag of `Input.CheckTags`. The minutes are `Verdict.LLMMinutesToVerdict`, because all LLM minutes of a failed cycle were waste.
- **Failed cycle, with no tag.** The blocker is `class:<phase>|<pre_class>` from `Input.Class`. It is never `waiting`, because that name hides the real cause, for example a red EGPS test. A class that is empty gives `none`.
- **Shipped cycle.** `waiting` is wall minutes minus LLM minutes minus host minutes, with a floor of 0. It is the blocker when it is at least `waitSharePct` percent of the wall minutes. Gate minutes are never waiting.
- **Paused cycle.** A cycle with `Paused` set never names `waiting`, because the clock of a quota pause runs. It gets `none`.
- **Names hold no cycle number.** `CheckPrefix` and `ClassPrefix` start the two failed-cycle names. The name of cycles 2049 and 2051 is one string, so the live reaction counts one class.

## The record

`Record` is the file `.evolve/failure-digests/cycle-<N>.blocker.json` (`paths.BlockerRecordPath`). Its schema string is `SchemaV1`. `DecodeRecord` refuses another schema and names it in the error, so a later reader cannot read a new schema as the old one.

`core.AssembleFailureDigest` writes the record in the seal-time step that writes the digest record. It runs before `KindCycleSealed`, so the live reaction can read the record of the cycle it reacts to. The tags come from `core.floorCheckTags`, which parses the raw reasons of `audit-fail-reason.json` and never the 200-byte copy of the dossier. The LLM minutes come from `llm-calls.ndjson`. A missing ledger gives 0 minutes. An unreadable ledger is an error.

A shipped cycle has no record. `evolve wave status` computes its blocker from memory with `Attribute`.

## Where `wave status` reads it

`cmd/evolve/cmd_wave_blocker.go` loads `workflow.blocker_attribution.wait_share_pct` once. For each cycle above the floor of the wave, it does one of three things:

- A shipped cycle: it calls `Attribute` with `wave.Cycle.LLMVerdict`, the wall time from `StartedAt` to `EndedAt` and `QuotaPauses > 0`.
- A failed cycle: it decodes the record. A missing record gives `none` with no warning, because cycles from before the writer have none. A record that cannot be decoded gives `none` and a warning.
- An open cycle: no blocker.

The text of `wave status` adds `blocker <name> (<minutes> min)` to the row of a cycle. A `none` blocker prints nothing. `--json` has `blockers` and `blocker_counts`. The counts are `check`, `class` and `none` for failed cycles, and `waiting` for shipped cycles. A shipped cycle with `none` adds no count.

## Invariants

- Cycles 2049 and 2051 name `check:ste-lint` from their raw reasons, and the dossier copy names no tag (`core` `TestAttribute_Wave143Cycles2049And2051NameTheSTECheck`).
- An untagged failed cycle never names `waiting` (`TestAttribute_AnUntaggedFailNamesItsClassNeverWaiting`).
- A paused cycle never names `waiting` (`TestAttribute_APausedCycleNeverNamesWaiting`, `cmd/evolve` `TestWaveStatus_QuotaPausesMakeAShippedCycleNone`).
- The seal writes the record beside the digest (`core` `TestAssembleFailureDigest_WritesTheBlockerRecordBesideTheDigest`).

## Limits

- The record keeps one name. When two checks fail, the first tag wins. The reasons stay in the dossier.
- `Paused` has one effect, on shipped cycles. The seal-time writer sees only failed cycles, so it does not read `cyclestate.QuotaWall`.
- The live reaction does not read the record yet. The follow-up F-A of the plan adds the name to the summary of the `pipeline-defect` incident.
