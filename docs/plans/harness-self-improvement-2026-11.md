# Plan: the loop measures its lesson reads first and adopts three Harness² refinements in shadow

- **Status:** proposed, 2026-11-26.
- **Research:** [harness2-self-evolution-2026-11.md](../research/harness2-self-evolution-2026-11.md). **Decision:** [ADR-0169](../architecture/adr/0169-lesson-yield-is-measured-before-a-lesson-store-is-built.md). **Design:** [harness-self-improvement.md](../architecture/harness-self-improvement.md).
- The ADR and the design are written from this plan. Each of the four files links to the other three.
- **Terms used throughout:**
  - **lesson:** one YAML file in `.evolve/instincts/lessons/` that the retrospective writes.
  - **cell index:** the list of entries that a phase seat can open ([cell-context-index.md](../architecture/cell-context-index.md)).
  - **MDE:** the smallest change that a sample can detect, from [llm-quality-indexes.md](../architecture/llm-quality-indexes.md) §4.6.
  - **stage key:** a policy key with the values `off`, `shadow` and `enforce` ([shadow-stage-contract.md](../architecture/shadow-stage-contract.md)).
  - **R1 to R8:** the eight refinements of research section 4.

## Table of contents

1. [Request](#1-request)
2. [Facts](#2-facts)
3. [Goals and non-goals](#3-goals-and-non-goals)
4. [Candidate designs](#4-candidate-designs)
5. [Decisions](#5-decisions)
6. [Components](#6-components)
7. [Order](#7-order)
8. [Verification](#8-verification)
9. [Risks](#9-risks)
10. [Open questions](#10-open-questions)

## 1. Request

The operator asked on 2026-11-26:

> "Research on the open source project : google-research/ harness2 and write the report recording the core logics and how it drives harness self improvement and self evolving."

The operator then added: "Check the docs/ folder and writer skill". Then the operator asked: "Let's go through the full investigation and research pipeline for adr, report, plan, design".

The report exists. It is the research dossier above. This plan is the second step. It decides which of the eight refinements the loop adopts, in which order, and how it proves each one.

## 2. Facts

The research ids are in the dossier. The investigation ids I1 to I15 are new. Each one comes from a command or a path that this plan names. All values are from 2026-11-26. The plane is `/Users/danleemh/ai/claude/evolve-loop/runtime`.

| Id | Fact | Source | Research id |
|---|---|---|---|
| I1 | The plane holds 877 lesson files. They total 3,294,606 bytes, or 1,098,202 tokens at `ceil(bytes / 3)`. The L1 budget of one cell is 8,000 tokens. | `ls`, `cat`, `wc -c` on `.evolve/instincts/lessons/`; `docs/architecture/cell-context-index.md:66` | F1.3 |
| I2 | 871 of 877 lessons carry `confidence`. The values are authors' ratings: 0.5 in 193 files, 0.9 in 181 and 0.85 in 122. No lesson carries `cue` or `confounded`. 11 carry `evidence` and 10 carry `rootCause`. | `grep -E "^\s*(- )?<key>:"` on the same directory | F1.2, P2.16 |
| I3 | A lesson has these fields: `id`, `pattern`, `description`, `confidence`, `source`, `type`, `category`, `failureContext`, `preventiveAction`, `relatedInstincts` and `contradicts`. | `inst-L2646b-lane-item-omits-protected-inboxmover-surface-and-tdd-skips-manifest-check.yaml` | F1.2 |
| I4 | The cell index lists a lesson only when its text names the item or a lineage cycle. At least two personas also tell a seat to `Grep` the lesson directory: `evolve-triage` and `evolve-builder`. | `go/internal/inputindex/lineage.go:87-109`; `agents/evolve-triage.md:13`; `agents/evolve-builder.md:13` | F1.3, P2.17 |
| I5 | 569 access logs cover cycles 2029 to 2649. They list 1,350 lesson entries and record 20 opens: 1.5%. 291 of the 4,188 cell records have `source: none`, and they hold 103 of the 1,350 listings and no open. In the cells that report a source, 20 of 1,247 listings were opened: 1.6%. So the open rate is at least 1.5% of the listings in cells that report a source. The build seat had 328 listed and 3 opened. | a count over `.evolve/context-access/cycle-*.jsonl`, by the `source` field of each `cell` record; `docs/architecture/cell-context-index.md:278,282` | F1.3, P2.35 |
| I6 | Wave 189 ran cycles 2630 to 2649. Two lesson files name one of these cycles, and both name cycle 2646. Five of the 20 cycles did not ship. Waves 187 and 188 have no per-cycle list in `wave status`, so their yield has no count. | `evolve wave status --json`; a match of the cycle numbers against the lesson files | F1.1, P2.35 |
| I7 | Wave 189 cost 833.0 LLM minutes for 20 cycles, and 15 shipped: 55.5 minutes per shipped cycle. Wave 188 had 129.4 (11 of 20). Wave 187 had 73.3 (1 of 2). | `evolve wave status --json`, `measures.cost` and `runs` | F1.11 |
| I8 | The persona diet plan is proposed. Its before and after table has no reading yet. | `docs/plans/persona-diet-2026-11.md:96-101` | F1.11, P2.30 |
| I9 | At a ship rate of 0.75 (15 of 20) and 20 cycles for each arm, the MDE is 0.383. A 10-point change needs 294 cycles for each arm, which is about 16,317 LLM minutes for each arm. This is an estimate: 294 × 55.5. | MDE formula in `docs/architecture/llm-quality-indexes.md` §4.6; I7 | P2.35 |
| I10 | `Promotion` has five integers. The `ready` rule tests decisions, escapes, false stops and conflicts. Condition 2 of the flip cites the MDE and paired rule. `evolve stage` has `verify` and `report`, and no `flip`. ADR-0167 is Proposed. | `go/internal/gatecontract/shadow.go:40-46`; `go/internal/stageevidence/score.go:327-333`; `docs/architecture/shadow-stage-contract.md:376-382`; `evolve stage` usage | F1.9, P2.35 |
| I11 | The sample rule for a move to a gate has three parts. The MDE is below the claimed effect. At most 1 breach in 10 windows has no real defect. A paired comparison supports the effect. | `docs/architecture/llm-quality-indexes.md` §4.6 | F1.9 |
| I12 | The search for a grader-verdict scan found none. Build cells list `audit-report` 523 times of 1,026 cells and open it 53 times. They list `acs-verdict` 325 times and open it 7 times. A retry reads the audit report by design. The audit seat also gets the verdict facts by design. | `Grep` for the marker names in `go/` and `docs/`; access logs; `docs/architecture/llm-isolation.md:703`; `go/internal/phases/audit/facts_seed_test.go:92-99` | F1.10, P2.22 |
| I13 | The ledger has 263,209 lines. It holds 2,218 `phase_plan`, 19,910 `phase`, 18,588 `agent_subprocess` and 174,518 `phase_skipped` entries. No source file names a seat count or a seat identity. The `seats` field of `wave status` is pane state. | `.evolve/ledger.jsonl`; `grep` on `go/` and `docs/architecture/`; `go/cmd/evolve/cmd_wave_status.go:48` | F1.7, P2.28 |
| I14 | Wave 189 had 3 no-work cycles: 2630, 2642 and 2649. Each ended in `SKIPPED_UNKNOWN` with `triage-empty-commitment`. They cost 2.3, 2.4 and 2.1 LLM minutes: 6.8 of 833.0 (0.8%). Two aborted cycles cost 49.3 (2633) and 11.9 (2646): 61.2 (7.3%). | `evolve wave status --json`, `cycles[]` | P2.14 |
| I15 | A lesson comes from the retrospective on a FAIL or WARN verdict only. `retro.go`, the retrospective persona, `gatecontract/`, `gateregistry/` and `inputindex/` are on the protected surface. `wavemeasure` is not on the manifest. | `docs/architecture/learn-phase.md:11-15`; `go/internal/guards/integrity_surface.go:166,172,192,269-270` | F1.2, F1.5 |

### 2.1 State of each refinement

| Item | State | Key fact |
|---|---|---|
| R1 confidence and confounded causes | Partly built | `confidence` exists in 871 of 877 files as an author rating. `cue` and `confounded` do not exist (I2). |
| R2 lesson index | Partly built | The cell index lists a lesson only by name match. No per-domain index exists. The 1,098,202 tokens do not fit the 8,000-token budget (I1, I4). |
| R3 lesson yield | Partly measurable | The access logs give listed and opened. The count of lessons written has a count for wave 189 only (I5, I6). |
| R4 removal test | Not built | The diet table has no reading. A paired test is underpowered at the cost of one cycle (I8, I9). |
| R5 bar for shadow to enforce | Partly built | ADR-0167 and the §4.6 rule hold a count bar and a paired rule. The `flip` verb does not exist (I10, I11). |
| R6 grader-verdict scan | Not built | No scan exists. A seat reads the audit report by design (I12). |
| R7 seat-count identity | Not built | The ledger has the plan and the run records. Nothing compares them (I13). |
| R8 no-work exit | Built | The no-work cycle ends in triage at about 2.3 LLM minutes (I14). |

## 3. Goals and non-goals

**Goals:**

- G1: the loop reports how many lessons it writes, and how many a seat reads, for each wave.
- G2: a new lesson records a cue, evidence and a confounded marker, so a seat can find it and trust it.
- G3: the loop detects hidden-oracle text in the context of a build seat, at `shadow`.
- G4: each new behavior starts at `shadow`, and a `ready` verdict on chained evidence moves it.

**Non-goals:**

- a claim of a gain in the ship rate. The sample cannot show one (I9);
- a lesson store or a lesson index. D5 sets the condition for one;
- a change to the strictness of any gate;
- a feature flag. Each behavior comes from configuration data;
- a change to an old lesson file.

## 4. Candidate designs

Each criterion has a score from 1 (bad) to 5 (good). Cost is scored so that a cheap design scores high. Risk is scored so that a safe design scores high. Overlap is scored so that a design with no overlap scores high.

- **Value** is the gain to the ship rate that the design can show: 5 is a proven gain and 1 is none. **Reversibility** is how cheap it is to undo the design: 5 is a policy change and 1 is a new store.
- **A. One by one:** adopt each of R1 to R8 into the component that owns the subject.
- **B. One subsystem:** build one evidence-and-lesson store in the shape of the Harness² library, with a per-domain index, the fields and the scans.
- **C. Measure first:** adopt nothing until a removal test proves the value of a part.
- **D. Filtered one by one:** measure the lesson read path first. Adopt R1, R3 and R6 in shadow. Defer R2, R4 and R7. Reject R5 and R8 as new work.

| Design | Value to the ship rate | Cost in LLM minutes | Risk to gates | Overlap with built work | Reversibility | Score |
|---|---|---|---|---|---|---|
| A | 3 | 2 | 3 | 2 | 4 | 14 |
| B | 3 | 1 | 2 | 2 | 2 | 10 |
| C | 1 | 1 | 5 | 4 | 5 | 16 |
| D | 3 | 4 | 4 | 4 | 5 | 20 |

**Why D wins.** D and A have the same value, because the items that D drops add no value. A stays a fair option, because it is the full set of refinements and only the overlap and cost scores separate it from D. C scores above A on the sum, but it scores 1 on value, so it adds nothing that a seat can use.

R5 is built by ADR-0167 (I10, I11). R8 costs 0.8% of the wave minutes (I14). D saves their cost and removes their overlap. B scores low on cost and risk. It adds a protected surface, and the read path that it needs is 1.5% used (I5). 

C scores low on cost. Two arms of a removal test cost about 32,634 LLM minutes (I9), and the plan can use the persona diet readings instead (I8).

**A worked example.** The lesson `inst-L2646b` has `confidence: 0.90`, a `source` with paths and a `preventiveAction`. It has no `cue` and no `evidence` field (I2, I3). Under D3, a lesson of the new form must carry both. The shadow stage records `would_exclude` for a new lesson with no cue. The stage does not touch `inst-L2646b`, because D3 applies to new files only.

## 5. Decisions

Each decision has 20 words or fewer.

| Id | Decision | Because | Rejected |
|---|---|---|---|
| D1 | Measure the lesson read path before the loop builds any lesson store. | I5, I6, P2.35 | B: it builds on a path that has a use of about 1.5%. |
| D2 | R3 is adopted. `wave status` reports lessons written, listed and opened, for each wave. | I5, I6, R3 | A count by file time: the time changes on a copy. |
| D3 | R1 is adopted at `shadow`. A new lesson carries `cue`, `evidence` and `confounded`. | I2, I3, P2.16, P2.19 | A rewrite of the 877 old files: it costs LLM minutes and has no reader. |
| D4 | `confidence` keeps its name and meaning. C2 records `would_exclude` only for a new lesson that is marked `confounded`. No component excludes a lesson from any reading in this plan. | I2 | A rename: it breaks 871 files. An exclusion now: it needs the shadow evidence of C2. |
| D5 | R2 is deferred. The loop designs an index when two waves of D2 data exist and 10 new lessons carry a cue. | I1, I4, I5 | An index now: the index has no cue to key on (I2). |
| D6 | R4 is deferred. The persona diet readings serve as the removal readings. A removal test states its MDE. | I8, I9, P2.30 | A new ablation harness: 16,317 LLM minutes for each arm. |
| D7 | R5 is rejected as new work. It follows ADR-0167. Each new stage names its `Promotion` data from the MDE. | I10, I11, P2.35 | A second bar: it gives the bar two homes. |
| D8 | R6 is adopted at `shadow`. The scan covers hidden-oracle text in the context of build and TDD cells. | I12, F1.10, P2.22 | A scan of `audit-report`: a retry reads it by design. |
| D9 | R7 is deferred. The loop starts it when a cycle ships with a planned seat missing. | I13, P2.28 | An identity now: 174,518 skip entries make a plain count noisy. |
| D10 | R8 is rejected. The no-work exit exists and costs 0.8% of the wave minutes. | I14, P2.14 | A new exit: it saves under 6.8 minutes for each wave. |
| D11 | Each new behavior is a stage key at `shadow` with a signal, a scorer and `Promotion` data. No gate changes strictness. | I10, F1.9 | A feature flag: the hard rules ban it. |
| D12 | The decision record is ADR-0169. Its name is `0169-lesson-yield-is-measured-before-a-lesson-store-is-built.md`. | D1 | none |

**What changes our mind.** We revisit D1 and D5 if the opened share of listed lessons rises above 20% with the cue fields present. Then the read path is live, and an index pays. We revisit D9 if one cycle ships with a missing planned seat.

## 6. Components

| Id | Component | Red test first | Owner |
|---|---|---|---|
| C1 | Lesson yield in `go/internal/wavemeasure` and `wave status`. It counts lessons written, by the cycle that each file names. It counts listed and opened lessons from the access logs. | A fixture plane with 2 lessons that name a wave cycle, 3 listed entries and 1 open must report 2, 3 and 1. | lane |
| C2 | Lesson record fields. The retrospective persona asks for `cue`, `evidence` and `confounded`. A validator checks a new lesson. Stage key `lesson_record.stage` and signal `LESSON_RECORD_SHADOW`. See the notes of C2 below the table. | A new lesson with no cue must give a decision with `would_exclude: true`. A lesson with only a confounded cause must give the same. A lesson whose `failureContext.cycle` is below `since_cycle` must give no decision. | console |
| C3 | The registry entry of `lesson_record`: `Decide`, `Enforce`, `Score` and `Promotion`, with a wiring test of the `enforce` branch (S3 of ADR-0167). The new keys must satisfy `TestEveryStageKeyHasRegistryEntry`, `TestEnforceBranchActsAtEnforce` and `TestStageGateRefusesEnforceWithoutActor` (`docs/architecture/shadow-stage-contract.md:311,317,538`). | The component adds `TestLessonRecordEntryHasScorer`. It must fail when the `lesson_record` entry has no scorer. | console |
| C4 | Hidden-oracle scan of the context of build and TDD cells. Stage key `seat_context_scan.stage` and signal `SEAT_CONTEXT_SCAN_SHADOW`. The marker list is policy data. | A build cell whose entry body holds a marker must give `would_stop`. A clean cell must give none. | console |
| C5 | ADR-0169, the design `harness-self-improvement.md`, the research-index entries and the `docs/research-index.md` regeneration. | `evolve docs ste-lint --strict` must report 0 findings. | console |

**Notes of C2:**

- The validator is in `go/internal/phases/retro`, beside the lesson existence check at `retro.go:256`.
- The retro phase is on the protected surface (I15), so the console owns C2.
- The stage key follows the pattern of `docs_floor.stage` (`go/internal/policy/stage_keys.go:184`).
- A lesson is new when `failureContext.cycle` is at or above `lesson_record.since_cycle`.
- `lesson_record.since_cycle` is a `policy.json` value. It is set when `lesson_record.stage` turns to `shadow`.
- File time is not used (D2).

Protected-surface parts are console work. `retro.go`, the retrospective persona, `gatecontract/`, `gateregistry/` and `inputindex/` are on the manifest (I15, I12). C1 is lane work, because `wavemeasure` is not on the manifest.

## 7. Order

1. **C5 first.** The ADR and the design fix the names and the fields before code.
2. **C1.** It is read-only and gives the baseline. A later reading of C2 needs the baseline.
3. **C2 and C3 together.** Q1 must have an answer first. The stage needs its entry at the same time as its emitter (S1 and S3 of ADR-0167).
4. **C4.** Q3 must have an answer first. It does not depend on C2. It can run at the same time as step 3 when the console has the capacity.
5. **Read after two waves.** The reading decides D5 and the `Promotion` numbers. No code step follows until the reading exists.

## 8. Verification

| Measure | Before (2026-11-26) | After and method |
|---|---|---|
| Lessons listed to seats, and opened | 1,350 listed, 20 opened (1.5%) (I5) | C1 reports both for each wave. A move of the share is a reading, not a goal. |
| Lessons with a cue | 0 of 877 (I2) | The share of new lessons with a cue and evidence, from the `lesson_record` signal. |
| Lessons with `evidence` | 11 of 877 (I2) | The same signal. |
| Lessons written for wave 189 | 2 for 20 cycles (I6) | C1 reports the count for each later wave. |
| Hidden-oracle hits in build cells | no scan, no count | The `seat_context_scan` signal reports `would_stop` for each wave. |
| LLM minutes per shipped cycle | 55.5 (I7) | `wave status`. This is a guard on cost, not a goal. The sample sees no change under 38 points (I9). |

Each stage follows ADR-0167. The scorer reads a hash chain of decisions. `evolve stage verify <key>` must return `ready` before a flip, and the `Promotion` numbers use the MDE rule of §4.6 (D7). This plan sets no numeric target for a lesson share, because no baseline exists. The two waves of step 5 give the baseline.

**Limits.** A test cannot prove that a lesson helps a later seat. The plan measures the read path and the record, not the effect on the ship rate.

## 9. Risks

| Risk | Control |
|---|---|
| The retrospective writes a cue and evidence of low quality, only to pass the check. | The stage is at `shadow`. A reviewer reads a sample of 10 new lessons before a flip. |
| The count of lessons written by file name misses a lesson with no cycle. | Q2 asks for the count of such files. C1 reports the unmatched count beside the matched count. |
| The scan marker list leaks the thresholds of the oracle into the seat context. | The list is policy data on the protected surface. No persona links to it. |
| Wave 189 is not typical, so the baseline misleads. | The reading needs two waves. The report shows each wave apart. |
| The `lesson_record` stage never reaches `MinDecisions`, because FAIL and WARN cycles are few. | Wave 189 had 2 failed cycles. The scorer says `not ready` and the stage stays at `shadow`. |
| D5 delays an index that can help. | D5 sets a measurable condition, so the delay has an end. |

## 10. Open questions

1. How does the retrospective seat judge that a cause is `confounded`? The rule needs a short definition in the persona. The research gives the idea (P2.19) and no test.
2. How many lesson files name no cycle? The answer sets the quality of the C1 count (I6).
3. Which strings are the hidden-oracle markers for C4? The first list comes from the quality panel names and the predicate file paths. The console decides it.
4. Does the log record a `Grep` read of a lesson? The access logs record opens of index entries (I5). No record of a `Grep` read was found.
5. Does the loop need a count for waves 187 and 188? Their per-cycle data is not in `wave status` (I6).
