# Plan: each sealed cycle names its top blocker, and the failure-digest reaction keeps the repeats

- **Status:** built, 2026-11-21, after review round 3. Inbox item `slow-cycle-blocker-attribution` (rank 85, route console).
- **Extends:** item 1 of [cost-aware-repair-2026-10](cost-aware-repair-2026-10.md) (ADR-0158). **Feeds:** the live incident reaction of X12 ([internal-selfheal.md](../architecture/packages/internal-selfheal.md), ADR-0155). **Reads:** [internal-cyclespend.md](../architecture/packages/internal-cyclespend.md) and [internal-paths.md](../architecture/packages/internal-paths.md).
- **Scope of this file:** the design. The code lands with it.
- **Console decision (binding):** the one-home rule gives option (a). The live reaction owns repeated failure causes. This plan adds no incident class and no stage key.

**Terms used throughout:**

- **Blocker:** the one named cause that holds most of the minutes of a sealed cycle.
- **Wall time:** the time from the start of the cycle to its end (`wave.Cycle.StartedAt` to `EndedAt`).
- **LLM time:** the sum of the `duration_ms` of the LLM calls of the cycle (`cyclespend.Verdict.LLMMinutesToVerdict`).
- **Host time:** the gate minutes of the cycle (`cyclespend.Verdict.HostMinutesToVerdict`).
- **Waiting:** wall time that is neither LLM time nor host time.
- **Check tag:** the `[name]` that a failed floor check puts at the start of its line in the raw failure reasons, for example `[ste-lint]`. The line starts after `floorFailureSeparator` (two spaces and a newline before them).
- **Class name:** `class:<phase>|<pre_class>`, the first two segments of the failure-digest fingerprint.
- **Failure-digest home:** the directory `.evolve/failure-digests`, which `paths.FailureDigestsDir` names.

## Table of contents

- [1. Request](#1-request)
- [2. Facts](#2-facts)
- [3. Goals and non-goals](#3-goals-and-non-goals)
- [4. Candidate designs](#4-candidate-designs)
- [5. Decisions](#5-decisions)
- [6. Design of the chosen approach](#6-design-of-the-chosen-approach)
- [7. Files](#7-files)
- [8. Red tests](#8-red-tests)
- [9. Rollout](#9-rollout)
- [10. Risks](#10-risks)
- [11. Follow-up items](#11-follow-up-items)
- [12. Open questions](#12-open-questions)

## 1. Request

The operator, on 2026-10-27, in the inbox item:

> "make LLM [minutes] the evaluation on the cycle result; it should fail fast and deliver faster; if not, something is blocking the pipeline; use side indexes"

Acceptance of the item, as the console decision changes it:

1. The committed dossiers of cycles 2049 and 2051 each name `check:ste-lint` as the blocker. Both cycles share one failure-class fingerprint (the pin at `go/internal/core/failure_digest_pinnedpairs_test.go:27`). The live reaction, at its floor of 3, keeps the one incident. This plan adds no second incident.
2. A cycle whose wall time is far above its LLM time names `waiting` as the blocker.
3. The threshold is a strict policy key with a range.
4. The doc is updated in STE.

## 2. Facts

**F2.1** `cyclespend.VerdictOf` computes the LLM minutes to verdict (`go/internal/cyclespend/verdict.go:26`). `Verdict.WithHost` adds the gate minutes. `wave.readCycle` calls both and stores the result in `Cycle.LLMVerdict` (`go/internal/wave/facts.go:137`, `:45`).
*Implication:* the minutes of a cycle exist in one reader. The new code needs no new writer for them.

**F2.2** Nothing names a cause. A search over `go/internal` for "blocker attribution" and "top blocker" finds only the verdict fields and their tests. `FailFastRatio` has two readers: the median in `wavemeasure/cost.go` and the cycle text of `cmd/evolve/cmd_wave_status.go`.
*Implication:* a slow cycle shows a number, and the operator reads the logs by hand for the cause.

**F2.3** The raw failure reasons hold the check tag at the start of a line. The build handoff floor joins its failures with `floorFailureSeparator` (`go/internal/core/build_handoff_floor.go:28`), so each tag line starts after a newline and two spaces. `AssembleFailureDigest` reads these raw reasons from `audit-fail-reason.json` (`go/internal/core/failure_digest.go:73`).
The dossier copy is not usable.

`go/internal/dossier/failure.go:66-72` collapses each reason with `strings.Fields` and cuts it at 200 bytes. In `knowledge-base/cycles/cycle-2049.json` and `cycle-2051.json` the one reason is a single line, and `[ste-lint]` sits at about byte 150, not at the start of a line. Across the 451 committed dossiers that have a failure record, 0 reasons start with a tag, and 427 (95%) have no tag at all. 

Some bracket tokens are not checks: `[bridge]`, `[claude-p]`, `[agy-claude-tmux]` and a defect id (`[dd74abcdfcb2201032e8ecb889a2e5c36]`, cycle 1379).
*Implication:* the check tag is a real name only in the raw reasons, and only for a minority of FAIL cycles. The parse must read the raw reasons at the seal. A cycle with no floor tag needs a second name, the class name (D4).

**F2.4** The digest field `Checks` does not give this name. `failingChecks` returns only `explanation-documentation` or `composed-floor` (`go/internal/core/failure_class.go:31`, `go/internal/core/build_handoff_floor.go:21`, `:22`).
*Implication:* the attribution must parse the tag from the reasons. It cannot reuse `Checks`.

**F2.5** The cost of cycles 2049 and 2051 was a failed cycle, not the minutes of a correction. The sum of `phases[].duration_ms` in the two dossiers is 13.5 and 8.7 minutes. The build archetype alone is 82.6 s and 50.0 s (`timing.by_archetype_ms.build`). The run workspaces of both cycles are gone, so the dossiers are the only record.
*Implication:* the blocker of a failed cycle holds all of its LLM minutes, because all of them were waste. The fixtures come from the dossiers.

**F2.6** A repeated failure cause has a home. The live reaction reads the digest record (`go/internal/selfheal/live.go:21`, `LoadFailureClass`). At `RecurrenceConsoleFloor` of 3 (`go/internal/dispositionrouter/dispositionrouter.go:32`) it opens `pipeline-defect:<fingerprint>` (`live.go:27`). The C2 normalizer has landed and pins 2049 and 2051 as one fingerprint (`failure_digest_pinnedpairs_test.go:27`).
*Implication:* a second repeat rule for FAIL cycles keeps two records of one fault. This plan does not add one.

**F2.7** The failure-digest home survives the run workspaces. `paths.FailureDigestRecordPath` gives `.evolve/failure-digests/cycle-<N>.json` (`go/internal/paths/paths.go:109`). The files of 2049 and 2051 exist there today. The run directory is not durable: gc collects it by modification time (`go/internal/gc/discover.go:42`).
*Implication:* the record of the blocker goes to the failure-digest home, next to the digest.

**F2.8** No code reads that directory by glob. `paths.FailureDigestsDir` has no reader outside `paths.go` and its tests, so a sibling file does not change any existing read.
*Implication:* the new record is a new file in an existing home, and not a change of the digest schema.

**F2.9** The protected list covers `go/internal/selfheal/` (`go/internal/guards/integrity_surface.go:261`), `cmd_selfheal.go` (`:262`), `policy/healing.go` (`:263`) and `cmd_selfheal_live.go` (`:265`). It does not list `cycleblocker`, `wave`, `paths` (except `paths/seat`), `cmd_boundary.go` or `policy/workflow*.go`.
*Implication:* the scoring package, the reader, the record path and the policy key are lane-eligible. The wiring into the reaction is console work.

**F2.10** The strict policy refuses an unknown key by reflection (`go/internal/policy/strict_keys.go`). `workflow.cycle_spend` is the model of a keyed range: pointer fields (`workflow_cycle_spend.go:13`), `keys()` with `min` and `max` (`:31`), and the validator list (`go/internal/policy/patch.go:56`).
*Implication:* a new keyed range copies that shape.

**F2.11** The boundary runs `selfheal incidents` at `go/cmd/evolve/cmd_boundary.go:234`, inside `boundaryHaltSteps` (`:224`). `refuseBoundaryWrite` guards boundary writes (`go/cmd/evolve/boundary_write.go:13`).
*Implication:* this step does not read the digest home.

`gatherIncidentEvidence` reads signals, CI runs, `.evolve/runs` cycles, usage and wave records (`go/cmd/evolve/cmd_selfheal.go:147-185`). The reader of the digest home is the live reaction, `liveReaction.react` on a `KindCycleSealed` FAIL event (`cmd_selfheal_live.go:45-55`). That is the only caller of `selfheal.LoadFailureClass`, and it runs at each seal. So the record must exist at the seal (D9).

**F2.12** No online research is used. The question is internal: which of our own records name a cause.

## 3. Goals and non-goals

| Goal | Measure | Target |
|---|---|---|
| G1. A sealed FAIL names one blocker | sealed FAILs named by a check or by a class name | 100% of all sealed FAILs |
| G2. The name comes from records that exist | new writers of LLM or gate data | 0 |
| G3. A repeat has one home | new incident classes or repeat thresholds | 0 |
| G4. Waiting is visible | a cycle with a high wall time against its LLM time names `waiting` | yes |
| G5. The threshold has a range | out-of-range value in `policy.json` | refused at load |

Non-goals:

- The design does not stop or repair a cycle. It reads, names and records.
- The design does not open an incident. The live reaction does, at its floor.
- The design adds no stage key and no feature flag. The stage of the reader is `healing.live_reaction.stage` (`go/internal/policy/healing.go:180-202`, `go/cmd/evolve/cmd_selfheal_live.go:33-38`) and stays there.
- The design does not score the other side indexes. Section 11 lists them.

## 4. Candidate designs

Scores run 1 to 5. A higher score is better. "Build cost" 5 means cheap.

| Id | Design | Meets acceptance (x2) | One home for each rule (x2) | Off the fence | Build cost | Sum |
|---|---|---|---|---|---|---|
| A1 | Extend `cyclespend.Verdict` with the cause | 2 | 2 | 5 | 3 | 14 |
| A2 | One pure package `cycleblocker` names the blocker of one cycle. The seal-time digest writer writes a sibling record in the failure-digest home | 5 | 5 | 5 | 4 | 28 |
| A3 | Score inside `selfheal` | 4 | 2 | 1 | 4 | 17 |
| A4 | An LLM seat reads the logs | 2 | 1 | 5 | 2 | 13 |
| A5 | Ten side indexes, a new `slow-blocker` class at repeat 2 (the round-2 design) | 3 | 1 | 3 | 1 | 11 |

- **A1.** `VerdictOf` takes LLM records only. The reasons come from the dossier. The function then holds two rules.
- **A2.** The name of a FAIL cycle is a parse of its reasons. The name of a slow shipped cycle is one subtraction. Both are pure and need no clock.
- **A3.** The detector then holds a second copy of the minute arithmetic, and the path is protected (F2.9).
- **A4.** Rule 5 of the core rules keeps LLM work for judgment. A parse and a subtraction are deterministic.
- **A5.** F2.6 shows that a `slow-blocker` incident for FAIL cycles duplicates the live reaction, and it adds a second threshold. Round 2 also showed that its ten minute definitions overlap.

**Why A2 wins.** It names the cause that acceptance 1 and 2 need, from records that exist. The record sits in a durable home. It adds one package and one key.

**What changes our mind.** The shadow reading of section 9 can find a blocker of `none` in more than 20% of the opened cases. Then add the indexes of section 11 one at a time. Give each its own minute definition.

## 5. Decisions

| Id | Decision | Because (findings) | Rejected options and why |
|---|---|---|---|
| D1 | One pure package `go/internal/cycleblocker` names the blocker of one cycle | F2.1, F2.9 | A1 mixes two rules. A3 sits on the fence. |
| D2 | A blocker has two sources only: the check tag of a failed cycle, and `waiting` | F2.3, F2.5 | A5: the other indexes have no minute definition yet (section 11). |
| D3 | Every unshipped cycle is a candidate. A shipped cycle is a candidate when its waiting share reaches `wait_share_pct` | F2.5 | A trigger on a median or on `FailFastRatio` does not fire for 2049 and 2051. `FailFastRatio` measures the waste after the fail point, not the lateness. |
| D4 | The name order is: the first check tag of the raw reasons. Then, for an unshipped cycle with no tag, the class name. Then, for a shipped cycle, `waiting`. Else `none`. An unshipped cycle never names `waiting` | F2.3 | A score race between unrelated sources needs minute definitions that overlap. A name of `waiting` for an untagged FAIL hides the real cause, for example an EGPS red test. |
| D5 | The minutes of a check blocker are all the LLM minutes to verdict. The minutes of `waiting` are wall minus LLM minus host minutes, with a floor of 0 | F2.1, F2.5 | A count of gate time as waiting blames the gate twice. |
| D6 | The fingerprint is `check:<tag>` or `waiting`. It holds no cycle number and no time | F2.6 | A free-text cause makes each cycle a new fingerprint. |
| D7 | The record is `.evolve/failure-digests/cycle-<N>.blocker.json`, from `paths.BlockerRecordPath` beside `FailureDigestRecordPath` | F2.7, F2.8 | The run workspace is collected by gc. A change of the digest schema is core work, and it is protected. |
| D8 | The repeat of a cause stays with the live reaction and its floor of 3. This plan adds no class, no stage and no repeat key | F2.6 | A5 and option (b) keep two thresholds for one fault. |
| D9 | The seal-time writer of the failure digest (`persistFailureDigest`, `go/internal/core/failure_digest.go:92-127`) also writes `cycle-<N>.blocker.json`. It runs before `KindCycleSealed`, so the live reaction sees the record. No boundary step and no verb | F2.11 | A boundary write runs after every seal of the wave, so the live reaction never reads the record of the cycle it reacts to. |
| D10 | One policy key, `workflow.blocker_attribution.wait_share_pct`, with a range and a default | F2.10 | An environment value and a flag are forbidden. |
| D11 | `wave status` loads `wait_share_pct` once in `cmd_wave_status`. For a FAIL cycle it decodes the seal-time record with `cycleblocker.Record`. For a shipped cycle it computes `waiting` from `c.LLMVerdict`, `StartedAt`/`EndedAt` and `c.QuotaPauses`. `readCycle` stays unchanged | F2.2 | `wave.Cycle` has no reasons, and `readCycle` (`go/internal/wave/facts.go:137`) carries no policy. It also serves `range.go` and `read_wave.go`. |

## 6. Design of the chosen approach

### 6.1 The worked example

Take cycle 2049. Its dossier has the verdict `FAIL`. Its raw reason in `audit-fail-reason.json` has a floor line that starts with `[ste-lint]`. Its LLM ledger gives `LLMMinutesToVerdict`.

| Step | Value |
|---|---|
| Unshipped, so a candidate | yes |
| First check tag in the reasons | `ste-lint` |
| Blocker | `check:ste-lint` |
| Minutes | `Verdict.LLMMinutesToVerdict` |

Cycle 2051 gives `check:ste-lint` with its own `LLMMinutesToVerdict`. The two failure digests carry different raw fingerprints (`build|gate-block|3c0001def9d6` and `build|gate-block|8da988239ba3`). The C2 normalizer maps both to one class fingerprint, and the live reaction counts one class. At its floor of 3 it opens one `pipeline-defect` incident. The blocker record adds the name of the cause to that class.

An unshipped cycle with no floor tag, for example an EGPS red test, names `class:<phase>|<pre_class>` and never `waiting`.

A shipped cycle with wall 90, LLM 20 and host 5 has 65 waiting minutes and a share of 72%. It names `waiting`.

### 6.2 The model

```go
type Input struct {
    Shipped     bool
    CheckTags   []string
    Class       string
    Verdict     cyclespend.Verdict
    WallMinutes float64
    Paused      bool
}

type Attribution struct {
    Blocker string
    Minutes float64
}

func Attribute(in Input, waitSharePct int) Attribution
```

- **One home for the tag parse (console work).** `core` holds `floorCheckTags` next to `floorReasonHead` and `floorFailureSeparator` (which `failingChecks` already uses). It returns the check tags that follow the build handoff floor head, in the raw reasons. `core` passes them as `Input.CheckTags`, so `cycleblocker` imports only `cyclespend` and no import cycle forms. The parse reads the full reasons that `AssembleFailureDigest` reads, never the 200-byte dossier copy. A bracket token that is not a floor tag (`[bridge]`, `[claude-p]`, a defect id) is never a tag.
- `Class` is the first two segments of the digest fingerprint, `<phase>|<pre_class>`. It is present in 445 of 451 records.
- **Pause sources.** `Paused` is true when `cyclestate.QuotaWall` is not empty at the seal (`go/internal/cyclestate/state.go:86`). In `wave status`, it is true when `c.QuotaPauses > 0`. Test each one.
- **Check path.** An unshipped cycle takes the first of `CheckTags`. The blocker is `check:<tag>` and the minutes are `Verdict.LLMMinutesToVerdict`.
- **Class path.** An unshipped cycle with no tag gets `class:<Class>` with the same minutes. It never gets `waiting`.
- **Waiting path.** A shipped cycle: `waiting` is `WallMinutes - LLMMinutesToVerdict - HostMinutesToVerdict`, with a floor of 0. It is the blocker when it is at least `waitSharePct` percent of `WallMinutes`.
- **Pause.** A paused cycle never names `waiting`, because the clock of a quota pause runs. It gets `none`. Section 11 gives the quota index.
- **None.** All other cycles get `Blocker` of `none` and 0 minutes.

The package imports `cyclespend` and nothing else of ours. `core` imports `cycleblocker`. `cyclespend` imports only `cyclestate` and `llmcalls`. It has no I/O and no clock.

### 6.3 The record and the seam

`cycle-<N>.blocker.json` in the failure-digest home:

```json
{"schema": "evolve.blocker/1", "cycle": 2049, "blocker": "check:ste-lint", "minutes": 13.5}
```

`cycleblocker` exports `Record` and the constant `SchemaV1`. `persistFailureDigest` writes the record for each sealed FAIL cycle, beside `cycle-<N>.json`, in the same seal-time step. Its inputs are all known at the seal: the full reasons, the LLM ledger of the workspace (`cyclespend.VerdictOf` with `ReadGateSpans`) and `cyclestate.QuotaWall`. That file is not on the protected list, and it runs before `KindCycleSealed`, an order that the live reaction already needs. A shipped cycle gets no record in the failure-digest home. `wave status` computes `waiting` for it from memory (D11).

The follow-up of section 11 makes `selfheal` decode the record with the exported type, so the schema has one home. The dependency path `selfheal` to `cycleblocker` to `cyclespend` has no cycle.

### 6.4 The policy key (D10)

```json
"workflow": {"blocker_attribution": {"wait_share_pct": 50}}
```

| Key | Range | Default | Meaning |
|---|---|---|---|
| `wait_share_pct` | 20 to 95 | 50 | the share of wall time at which `waiting` is the blocker |

The reader is a new file `go/internal/policy/workflow_blocker_attribution.go` with the shape of `workflow_cycle_spend.go`. `WorkflowPolicy` gets one pointer field. The validator joins the list at `patch.go:56`. A value outside the range makes `policy.Parse` return an error that names the key.

### 6.5 CI parity

The package is pure Go and uses the standard library only. It reads no path, so Linux and macOS give one answer. The code needs no feature newer than Go 1.23. The tests use `t.TempDir` and fixed numbers, with no `time.Now`.

## 7. Files

Lane-eligible files (off the fence, F2.9), built together with the console work in this change:

| File | Change |
|---|---|
| `go/internal/cycleblocker/attribute.go`, `record.go`, `doc.go` | new: `Input`, `Attribution`, `Attribute`, `Record`, `SchemaV1` |
| `go/internal/cycleblocker/attribute_test.go`, `apicover_named_test.go`, `testdata/` | new: the red tests of section 8 and the two dossier fixtures |
| `go/internal/paths/paths.go` and its test | `BlockerRecordPath` beside `FailureDigestRecordPath` (`:109`) |
| `go/internal/policy/workflow_blocker_attribution.go` and its test | new: the key, the range, the default |
| `go/internal/policy/workflow.go`, `patch.go` | one pointer field (beside `:37`); the validator in the list at `:56` |
| `go/internal/core/failure_digest.go`, `failure_blocker.go` | `AssembleFailureDigest` writes the blocker record after `persistFailureDigest` |
| `go/internal/wave/facts.go` | no change. `readCycle` (`:137`) stays as it is |
| `go/cmd/evolve/cmd_wave_status.go` | loads `wait_share_pct` once and decodes the record of each FAIL cycle. It computes `waiting` for shipped cycles, prints the blocker (near `:255`) and adds per-wave counts to `--json` |
| `go/.apicover-enforce`, `go/.cover-strict` | one line for `./internal/cycleblocker` at 100 |
| `docs/architecture/packages/internal-cycleblocker.md`, `docs/architecture/packages/README.md` | new package note and its index row |
| `docs/architecture/policy-config.md`, `internal-policy.md`, `internal-paths.md` | one row each |

Console work:

| File | Change | Reason |
|---|---|---|
| `go/internal/core/build_handoff_floor.go` | `floorCheckTags`, the tag parse beside `floorReasonHead` | the file is protected |
| `go/internal/selfheal/live.go`, `sources.go` | held to a later step: read the blocker record and add the name to the summary of the `pipeline-defect` incident (section 11) | `integrity_surface.go:261` |

The item lists `go/internal/selfheal/incidents.go`. That file does not exist. The detector files are `incident.go`, `detect.go` and `observation.go`.

## 8. Red tests

Each test is written first and must fail on `main`.

| Test | Package | What it proves | Acceptance |
|---|---|---|---|
| `TestAttribute_Wave143Cycles2049And2051NameTheSTECheck` | `core` | the raw 2049 and 2051 strings pinned at `core/failure_digest_pinnedpairs_test.go:27`, and the two 200-byte dossier strings, go through the tag parse and `Attribute`. The raw strings give `check:ste-lint`. The dossier strings are the negative case that shows why the raw reasons are read. The minutes equal `Input.Verdict.LLMMinutesToVerdict` | 1 |
| `TestFloorCheckTags_NonCheckBracketsAreNotTags` | `core` | `[bridge]` and `[dd74abcdfcb2201032e8ecb889a2e5c36]` give no tag | 1 |
| `TestAttribute_AnUntaggedFailNamesItsClassNeverWaiting` | `cycleblocker` | an unshipped cycle with no tag and a high waiting share gives `class:<Class>` | 1 |
| `TestAssembleFailureDigest_WritesTheBlockerRecordBesideTheDigest` | `core` | through the real entry: a FAIL workspace whose `audit-fail-reason.json` holds the 2049 floor line makes `AssembleFailureDigest` write `cycle-N.blocker.json` with `check:ste-lint` beside `cycle-N.json` | 1 |
| `TestAttribute_TheFingerprintHoldsNoCycleNumber` | `cycleblocker` | cycles 2049 and 2051 give an equal `Blocker` string | 1 |
| `TestAttribute_WallFarAboveLLMNamesWaiting` | `cycleblocker` | a shipped cycle with wall 90, LLM 20, host 5 gives `waiting` with 65 minutes | 2 |
| `TestAttribute_GateMinutesAreNotWaiting` | `cycleblocker` | wall 60, LLM 20, host 40 gives `none` | 2 |
| `TestAttribute_APausedCycleNeverNamesWaiting` | `cycleblocker` | the 90, 20, 5 shipped cycle with `Paused` gives `none` | 2 |
| `TestWaveStatus_QuotaPausesMakeAShippedCycleNone` | `cmd/evolve` | a shipped cycle with `QuotaPauses > 0` gives `none` in `wave status` | 2 |
| `TestAttribute_ACheckTagBeatsWaiting` | `cycleblocker` | an unshipped cycle with a tag and a high waiting share names the tag | 1 |
| `TestParse_BlockerAttributionRange` | `policy` | `wait_share_pct` at 19 and 96 fails with the key name, at 20 and 95 passes, and `waitshare` fails with `ErrUnknownKey` | 3 |
| `TestBlockerRecordPath_SitsBesideTheDigest` | `paths` | the path has the directory of `FailureDigestRecordPath` and the suffix `.blocker.json` | 1 |
| `TestWaveStatus_TextPrintsTheBlockerOfACycle` | `cmd/evolve` | a cycle with a blocker prints it; a `none` cycle prints no blocker text | 2 |
| `TestWaveStatus_JSONCountsBlockersByKind` | `cmd/evolve` | `--json` gives per-wave counts of FAILs named by check, named by class and named `none`, and of shipped cycles named `waiting` | 1 |

The acceptance 4 proof is the CI check `ste-lint --changed --added` on this file and on the new package note. It is not a Go test.

Each test proves intent. The first fails if the model reads `Checks` (F2.4), the 200-byte dossier copy (F2.3) or a median trigger (F2.5), because none names `ste-lint`. The pause test fails if a quota pause counts as waiting. Rule 12 applies: a `grep` for each test name proves that it exists before the build report claims it.

## 9. Rollout

- **No stage key.** The attribution reads and reports. It opens nothing. The stage of the reaction, `healing.live_reaction.stage`, controls the incidents (F2.6).
- **Shadow reading:** `wave status --json` gives the counts for each wave: sealed FAILs named by check, named by class and named `none`, and shipped cycles named `waiting`. The criterion uses these fields. A `none` share above 20% of the sealed FAILs stops the follow-up of section 11 until the cause is known. Measure the share against the day-one baseline of the counts. A class name is a valid name, so day one does not trip the rule.
- **Enforce path:** the follow-up that adds the name to the incident summary has its own test through `liveReaction.react` on a `KindCycleSealed` FAIL event. It runs with the stage of the reaction as it is. This plan changes no stage.
- **X13 cost:** this plan opens no incident, so it adds no seat run of `evolve selfheal propose`.

## 10. Risks

| Risk | Control |
|---|---|
| The first tag is not the main cause when two checks fail | The record keeps one name. The reasons stay in the dossier. The shadow reading of section 9 checks the name. |
| A deadline-stopped shipped cycle names `waiting` | Its wall time is mostly LLM time. The deadline index is in section 11. The shadow reading counts such cycles. |
| A cycle has no reasons text | The blocker is `none`. The record is still written, so the gap is visible. |
| The seal writes a record that a later read uses with a new schema | The schema constant `evolve.blocker/1` names the version. A reader refuses another value. |
| The size ratchet blocks `workflow.go` | The key sits in its own reader file (F2.10). |

## 11. Follow-up items

The console files one inbox item for each line at the boundary, each with a dependency on this item. A mid-wave edit of the inbox is refused.

| Follow-up | Owner | Part of the item that it closes |
|---|---|---|
| F-A. `selfheal.LoadFailureClass` reads the blocker record with `cycleblocker.Record`, and `FromFailureClass` adds the name to the incident summary. Test through `liveReaction.react` on a `KindCycleSealed` FAIL event with two records. | console | "give one incident" with a named cause |
| F-C. A pointer from the cycle result to the record (open question Q1) | console | "record it in the cycle result" |
| F-D. The other side indexes. Each gets a minute definition, a source record and the index that it is subtracted from. The list: corrections, audit rounds beyond the first, quota minutes, context fill, continuation, ship rebase, deadline, cache-read growth and the split for each phase. Quota minutes come from the `quota.paused` and resume signals | lane, after the shadow reading | "use side indexes" |

Option (b) of round 2 is rejected, so no change of the live reaction is needed beyond F-A.

## 12. Open questions

- **Q1.** The item asks for the record "in the cycle result". A new field in `cyclestate.CycleResult` changes the seal path in `core`, which is protected. The console decides if the sibling record of D7 is enough.
- **Q2.** `wait_share_pct` has no absolute floor, so a short shipped cycle can name `waiting`. The design adds a floor key only if the shadow reading shows noise.
