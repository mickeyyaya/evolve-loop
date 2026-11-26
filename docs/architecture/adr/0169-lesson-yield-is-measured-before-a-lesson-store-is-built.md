# ADR-0169: Lesson yield is measured before a lesson store is built

- **Status:** Proposed (2026-11-26). The operator asked for the research, the plan, the design and the decision record: "Let's go through the full investigation and research pipeline for adr, report, plan, design".
- **Plan:** [harness-self-improvement-2026-11.md](../../plans/harness-self-improvement-2026-11.md). **Research:** [harness2-self-evolution-2026-11.md](../../research/harness2-self-evolution-2026-11.md). **Design:** [harness-self-improvement](../harness-self-improvement.md) (the spec of record).
- **Relates to:**
  - [ADR-0167](0167-shadow-stages-prove-themselves-on-chained-evidence.md) governs each new stage key. This record adds no second bar for a move to `enforce`.
  - [ADR-0154](0154-the-pipeline-is-the-only-controller-and-each-agent-is-an-isolated-replaceable-worker.md) owns the cell index and its access log. This record reads them and does not change them.
  - [ADR-0153](0153-quality-is-a-panel-of-separate-indexes-with-intervals-that-the-signal-center-publishes.md) owns the sample rule of [llm-quality-indexes](../llm-quality-indexes.md) section 4.6. The `Promotion` data of each new stage uses it.
- **Evidence:** the facts of the plan section 2, with the research ids.
  - I5 (F1.3, P2.35): 569 access logs list 1,350 lesson entries and record 20 opens. 291 of 4,188 cell records have `source: none`, and they hold 103 of the listings. In the cells that report a source, 20 of 1,247 listings were opened. The open rate is at least 1.5% of the listings in cells that report a source.
  - I1 (F1.3): the plane holds 877 lessons, about 1,098,202 tokens. The L1 budget of one cell is 8,000 tokens.
  - I2 (F1.2, P2.16): no lesson carries `cue` or `confounded`. 11 of 877 carry `evidence`.
  - I9 (P2.35): at a ship rate of 0.75 and 20 cycles for each arm, the MDE is 0.383. A 10-point change needs about 16,317 LLM minutes for each arm.
  - I12 (F1.10, P2.22): no scan of seat context for hidden-oracle text exists. A seat reads the audit report by design.

## Context

Harness² keeps a library of lessons with a cue, evidence and a confidence. Its own issue tracker shows that the lesson gate often returns no entry, and that a paired test needs many tasks (P2.35). Research section 4 lists eight refinements, R1 to R8.

Our loop writes a lesson on a FAIL or WARN verdict only. It has 877 lesson files. A seat opens at least 1.5% of the lessons that the index lists: 20 of 1,247 in the cells that report a source. A store or an index of lessons builds on a read path that a seat barely uses. No lesson has a cue, so an index has nothing to key on.

The sample is small. A test of the ship rate cannot show a gain below 38 points at the cost of one wave. So the loop first measures the read path and the record, and adds only the parts that its own gates can prove at `shadow`.

## Decision

1. The loop measures the lesson read path before it builds any lesson store or lesson index.
2. `wave status` reports lessons written, listed and opened for each wave (R3).
3. A new lesson carries `cue`, `evidence` and `confounded`, checked at `shadow` (R1). No old file changes.
4. `confidence` keeps its name and meaning. C2 records `would_exclude` only for a new lesson that is marked `confounded`. No component excludes a lesson from any reading in this plan. An exclusion is a later decision that needs the shadow evidence of C2.
   - A lesson is new when `failureContext.cycle` is at or above `lesson_record.since_cycle`, a `policy.json` value set when `lesson_record.stage` turns to `shadow`. File time is not used.
5. The lesson index (R2) waits for two waves of data from rule 2 and 10 new lessons with a cue.
6. The removal test (R4) waits. The persona diet readings serve as the removal readings, and a test states its MDE.
7. The loop adds no second bar for `enforce` (R5). Each new stage follows ADR-0167 and names its `Promotion` data from the MDE.
8. A scan of build and TDD cell context for hidden-oracle text runs at `shadow` (R6).
9. The seat-count identity (R7) waits until a cycle ships with a planned seat missing.
10. The loop builds no new no-work exit (R8). The exit exists in triage.
11. Each new behavior is a stage key at `shadow` with a signal, a scorer and `Promotion` data. No gate changes strictness.
12. This record is ADR-0169. The design is `harness-self-improvement.md`.

The loop revisits rules 1 and 5 if the opened share of listed lessons rises above 20% with the cue fields present. It revisits rule 9 if one cycle ships with a missing planned seat.

## Consequences

**Good:**

- The loop gets its first count of lesson yield. Until now, no number said if a lesson helps a later seat.
- Each new behavior starts at `shadow`, so no gate gets stricter and no cycle is blocked by this work.
- The work reuses the stage contract, the access log and the policy registry. It adds no store, no flag and no second bar.
- A deferred item has a written condition, so the delay has an end.

**Bad:**

- The record cannot prove that a lesson helps a later seat. It measures the read path and the record, not the effect on the ship rate.
- The `lesson_record` stage can stay at `shadow` for a long time. FAIL and WARN cycles are few: wave 189 had 2 failed cycles.
- Wave 189 is one baseline. The reading needs two waves before it decides rule 5.
- The 877 old lessons stay without a cue. A seat that needs one finds it only by name match or by `Grep`.
- The open rate of 1.5% is a lower bound. 103 of the 1,350 listings are in cells with `source: none`, and those cells count for no open.
- The work touches the protected surface (`retro.go`, the retrospective persona, `gatecontract/`, `gateregistry/` and `inputindex/`), so the console builds it and not a lane.

## Rejected

| Option | Fact that rules it out |
|---|---|
| Design A: adopt R1 to R8 one by one | R5 is built by ADR-0167 (I10, I11). R8 costs 0.8% of the wave minutes (I14). The plan scores A at 14 and D at 20. |
| Design B: one evidence-and-lesson store with a per-domain index | A seat opens at least 1.5% of the lessons listed in cells that report a source (I5). The store adds a protected surface. The plan scores B at 10. |
| Design C: adopt nothing until a removal test proves each part | Two arms cost about 32,634 LLM minutes (I9). The plan scores C at 16. |
| R5: a second bar for `shadow` to `enforce` | ADR-0167 and section 4.6 hold the bar (I10, I11). A second bar gives the bar two homes. |
| R8: a new no-work exit | The exit exists. Three no-work cycles cost 6.8 of 833.0 LLM minutes (I14). |
| An index now (R2) | No lesson has a cue to key on (I2). |
| A rewrite of the 877 old lessons | It costs LLM minutes and no reader needs it (I2, I5). |
| A rename of `confidence` | 871 files carry the field (I2). |
| A scan of `audit-report` for verdict text (R6) | A retry reads the audit report by design (I12). |
| A count of lessons by file time | The time changes on a copy. |
| A feature flag | The hard rules ban it. Configuration data selects each behavior. |
