# ADR-0171: Research is an LLM-steered loop inside a code envelope, gated in depth blocks by evidence coverage

- **Status:** Proposed (2026-11-27). The operator asked: "I would like to improve the research skill to have a more diverse views and be able to justify how many layer / depth / level to dive deep before stop". The operator asked next: "When it reach to multiple layers search (e.g. every 3 layer depth) it report the status and update summary to the main session and decide if it has enough confidence to end the research or continue to the next multiple layer search".
- **Plan:** [research-diversity-and-depth-2026-11](../../plans/research-diversity-and-depth-2026-11.md). **Research:** [research-diversity-and-depth-2026-11](../../research/research-diversity-and-depth-2026-11.md). **Design:** [research-loop](../research-loop.md) (the spec of record).
- **Relates to:**
  - [ADR-0154](0154-the-pipeline-is-the-only-controller-and-each-agent-is-an-isolated-replaceable-worker.md) makes the pipeline the only controller. This record keeps that rule. The lead is the console, and no phase agent starts a seat.
  - [ADR-0167](0167-shadow-stages-prove-themselves-on-chained-evidence.md) governs a new stage. This record applies the same idea to the thresholds of the band: shadow first, then a gate.
  - [ADR-0170](0170-the-loop-keeps-the-llm-at-the-leaves-and-hardens-its-host-resume-fleet-and-self-edit-edges.md) came from the first run of the research skill. This record changes the method of that skill.
- **Evidence:**
  - F1.4: the skill before this change ran one fan-out and had no barrier. F1 describes the skill at commit 6dd24b4, before this change.
  - P2.1, P2.3, P2.4: each prompted system and each open source project that the dossier read is an LLM inside a code envelope. The exception is II-Researcher, which has no turn limit (P2.4). The RL products are end-to-end (P2.1, maker claim).
  - P3.2, P3.3: agents stop too early on 51–79% of the questions, and a claim coverage stop beats an LLM sufficiency judge.
  - P3.7: verbal confidence is close to noise.
  - P5.4, P5.5: debate between like agents converges and costs two to three times more tokens.

## Context

The research skill splits a question by topic and runs one parallel fan-out. It has no stance axis, no stop rule and no depth rule. It cannot justify how deep a run went. A decision document cannot ask the research for a missing fact.

The operator wants a loop that an LLM steers. It reports at each block of depth. It has a stop rule with a stated confidence.

The measured studies agree on one point. The stop rules that work check the gathered evidence against the required claims. They do not ask the model how sure it is. The pipeline also has a hard rule: no LLM in the pipeline starts or directs another. The lead must therefore be the console.

## Decision

1. The research skill runs as a loop. The console is the lead and starts each seat.
2. The lead splits the question into decisions, and each decision into required claims.
3. A claim ledger holds each claim with its state, sources, independent count and label.
4. Angles split the topic. Lenses split the stance. The lens floor is builder, skeptic and measurer.
5. The lead finds the perspectives of the subject from three to five overview sources before the fan-out.
6. Seats stay independent until a barrier. The skill has no debate rounds.
7. One contrarian seat runs after block 1 and argues against the leading option.
8. A second model family takes the skeptic or contrarian seat when the console has one.
9. A stakes tier sets the seats, blocks and calls before the first query. A tier only rises.
10. A block is up to three levels of follow-up for each seat. The block size is configuration.
11. Each seat sees its remaining calls after each call.
12. A seat follows a lead one level deeper only when it names the claim that the lead can change.
13. Each seat returns a barrier report. Its status comes from a closed set. Code (C2, planned) sets it. Until then the lead sets it by the written status rules.
14. Verbal confidence is logged and is never an input of a gate.
15. Code (C2, planned) computes the band from coverage, stability and independent verification. Until then the lead computes it in shadow. Progress does not set the band. It detects a stall and blocks DEEPEN. Block 1 has no baseline for progress.
16. The lead chooses STOP, DEEPEN, WIDEN or STOP-INCOMPLETE and cites ledger ids.
17. The lead cannot stop below the tier band: T0 MEDIUM, T1 MEDIUM with all driving claims covered, T2 HIGH.
18. At a cap, code (C2, planned) forces STOP-INCOMPLETE. Until then the lead chooses it in shadow. Each open claim becomes a written gap.
19. The lead re-reads the gathered evidence before it orders more search.
20. The lead reports to the main session at each barrier in a fixed short form.
21. The dossier ends with Decision, Rationale and Alternatives entries that cite finding ids.
22. Each ADR option and driver cites a finding id. The ADR names its Confirmation check.
23. A writer marks a missing fact `NEEDS-EVIDENCE: <claim>`. Each marker is a narrow one-block run.
24. A pass handles at most five markers. A marker still open after two runs goes to the operator.
25. Code (C2, planned) owns counts, caps and status. The LLM owns the judgment of meaning. Until C2 exists, the lead does the work of code by the written rules.
26. Until the `evolve research` verbs exist, the lead applies the rules in shadow. It sets the seat status, computes the band and forces the stop at a cap. It records the result.

The rules map to the plan decisions as follows:
- rules 1 to 3 to D1 and D13, 4 to 8 to D2 and D3;
- 9 to D4, 10 and 11 to D5, 12 to D6, 13 and 14 to D7;
- 15 to D8, 16 to 19 to D9, 20 to D10;
- 21 and 22 to D11, 23 and 24 to D12, 25 to D13, 26 to D14.

## Consequences

Good:

- Each stop has a reason that a reviewer can check against the ledger.
- A stop that is too early, the common failure, is harder than a continuation.
- An error or a cap never reads as "covered".
- A decision document links back to its evidence and can ask for more.

Bad:

- The ledger and the barrier add work to each run. T0 keeps the cost small with one block.
- The thresholds of the band are first values with no measured source. A wrong value stops a run too early or too late.
- Until C2 exists, the lead counts by hand and can make an error.
- All measured stop results come from short-answer benchmarks. A long report can fail in other ways.

## Confirmation

For now, the `docs-review` check of each run confirms this decision. It reads the depth log and the ledger and recomputes the band by hand. It reports a stop below the band of the tier, or an open `NEEDS-EVIDENCE` marker. Later, the table tests of C2 confirm it: the band for each row of §6 of the spec, and the rejection of a lower tier. The marker gate of C3 confirms the markers. The shadow log (C4) confirms the thresholds.

Change this decision when the shadow log shows that the stops at HIGH need rework more often than the stops at MEDIUM. The thresholds are then wrong.

## Rejected

| Option | Fact that rules it out |
|---|---|
| A. Fixed quotas for lenses, depth and sources | A fixed depth overspends on an easy question and stops a hard one too early (P3.8, P3.9). The fixed trees of P2.3 have this shape. |
| B. LLM-only judgment of depth and stop | Agents stop before the evidence is enough on 51–79% of the questions (P3.2). |
| C. A code-only saturation loop | No paper measures a novelty stop for research agents (gap, synthesis; the makers state novelty rules with no measured threshold, P3.1). Code cannot judge if a source supports a claim. |
| Debate rounds between seats | Debate of like agents converges on the majority and costs 2.1–3.4 times the tokens (P5.4, P5.5). |
| A stop on verbal confidence | 96.5% of the self-ratings were the maximum (P3.7). |
| An LLM supervisor that starts the seats | The hard rule forbids it: no LLM in the pipeline starts or directs another. |
