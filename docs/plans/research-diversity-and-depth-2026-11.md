# Plan: rebuild the research skill as an LLM-steered loop that has diverse views and a justified stop rule

- **Status:** proposed, 2026-11-27.
- **Design:** [research-loop](../architecture/research-loop.md). **Decision:** [ADR-0171](../architecture/adr/0171-research-is-an-llm-steered-loop-inside-a-code-envelope-gated-in-depth-blocks-by-evidence-coverage.md). **Research:** [research-diversity-and-depth-2026-11](../research/research-diversity-and-depth-2026-11.md).
- This plan builds on [oss-harness-architectures-2026-11](oss-harness-architectures-2026-11.md), the first design set that the research skill made.

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

The operator made three requests on 2026-11-27.

1. "I would like to improve the research skill to have a more diverse views and be able to justify how many layer / depth / level to dive deep before stop"
2. "It should be ai driven research skill and dynamic / adaptively controlled by LLM and be able to integrate into the research, adr, design, and plan flow. Research online to find out how ai do the online deep research"
3. "When it reach to multiple layers search (e.g. every 3 layer depth) it report the status and update summary to the main session and decide if it has enough confidence to end the research or continue to the next multiple layer search"

The operator then decided: "follow your recommendation and full speed to build the features". This change builds C1 only. The scope of C2 to C4 is an open question (Q1).

## 2. Facts

| Fact | Research id |
|---|---|
| The skill before this change split a question by topic only. It had no stance axis. (F1 describes the skill at commit 6dd24b4, before this change.) | F1.1 |
| The skill before this change had no stop rule and no depth rule. | F1.2, F1.3 |
| The skill before this change ran one fan-out. No barrier existed. | F1.4 |
| A decision document cannot ask the research for a missing fact. | F1.5 |
| Each prompted system and each open source project that the research read is an LLM inside a code envelope. The exception is II-Researcher, which has no turn limit. The RL products are end-to-end (maker claim). | P2.1, P2.3, P2.4 |
| A backstop that turns an error into "done" is the worst failure. | P2.5 |
| The agent stops too early on 51–79% of the questions. | P3.2 |
| A claim coverage stop beats an LLM sufficiency judge. | P3.3 |
| Verbal confidence is close to noise. | P3.7 |
| A budget display gives the same accuracy at a tenth of the budget. | P4.2 |
| A router for the effort is right 54.52% of the time. | P4.6 |
| Debate between like agents converges and costs two to three times more. | P5.4, P5.5 |
| Spec Kit markers close the loop from plan back to research. | P6.1, P6.2 |

## 3. Goals and non-goals

Goals, in order:

1. Each stop has a reason that cites ledger ids, a band and the tier.
2. The lead cannot stop below the band of its tier.
3. Each request has at least the builder, skeptic and measurer lenses.
4. The lead reports to the main session at each barrier.
5. Each ADR option and driver cites a finding id.
6. A writer can send a missing fact back to the research.

Non-goals:

- The change does not build the Go verbs, the marker gate or the shadow log (C2 to C4).
- The change does not start a seat from code. The console starts each seat.
- The change does not give the thresholds a measured source. They are first values.

## 4. Candidate designs

| Id | Design |
|---|---|
| A | Fixed quotas. Fixed lenses, a fixed depth and fixed source counts for each request. |
| B | LLM-only judgment. The lead decides the depth and the stop freely, as in prompted systems. |
| C | A code-only saturation loop. A count of new findings stops the run. |
| D | An LLM-steered loop inside a code envelope. Depth blocks are gated by evidence coverage. |

Scores: 1 is poor, 5 is strong. The scores are synthesis from the research ids in the last column.

| Criterion | A | B | C | D | Research id |
|---|---|---|---|---|---|
| Stops before the evidence is enough (low risk) | 2 | 1 | 2 | 4 | P3.2, P3.8 |
| Stops late and wastes budget (low risk) | 2 | 2 | 4 | 4 | P3.9, P4.3 |
| Fits a hard question and an easy one | 1 | 4 | 2 | 4 | P4.6 |
| Diverse views | 3 | 2 | 1 | 4 | P5.2, P5.9 |
| A stop that a reviewer can check | 4 | 1 | 3 | 5 | P3.4, P3.7 |
| Links to the ADR and plan flow | 2 | 2 | 1 | 5 | P6.1, P6.4 |
| Cost to build now (5 is cheap) | 5 | 5 | 2 | 4 | the components below |
| **Total** | 19 | 17 | 15 | **30** | |

Why D wins. A has no answer for a hard question, and a fixed depth stops it too early or overspends on an easy one (P3.8, P3.9). B is the shape of the prompted systems, and the measured studies show that those stop too early. C counts novelty. No paper measures that stop on research agents (gap, synthesis). It also cannot judge if a source supports a claim.

D keeps the judgment with the LLM, which can read the sources, and puts each number in code or in a written rule. 

D costs more than A and B, because the ledger and the barrier add work. The C1 part needs no Go code. The rules run by hand until C2 exists.

### What it takes from each

| Design | What D takes | What D leaves |
|---|---|---|
| A | The tier table gives a fixed envelope before the first query. The lens floor is fixed. | A fixed depth for every question. |
| B | The lead chooses the verb at each barrier. The seats read and extract freely inside a block. | A free stop. The lead cannot stop below the band of the tier. |
| C | Progress and stall detection from a count of new supported claims. The forced stop at a cap. | A stop on novelty alone. |

## 5. Decisions

| Id | Decision | Research id |
|---|---|---|
| D1 | The lead splits the question into decisions and claims. A claim ledger tracks each claim. | R1, P3.3, P3.5 |
| D2 | Two axes: angles split the topic and lenses split the stance. The lens floor is builder, skeptic and measurer. One contrarian seat runs after block 1. | R2, R3, R5, P5.2, P5.9 |
| D3 | Seats stay independent until a barrier. No debate rounds. A second model family takes the skeptic or contrarian seat. | R4, P5.4, P5.5, P5.6 |
| D4 | A stakes tier sets the envelope before the first query. A tier only rises. | R6, P4.3, P4.6 |
| D5 | A block is up to three levels. Each seat sees its remaining calls after each call. | R7, P2.2, P4.2 |
| D6 | A seat follows a lead only when it names the claim or decision that the lead can change. | R8, P3.11, P5.10 |
| D7 | Each seat returns a barrier report with a status from a closed set. Code (C2, planned) sets it. Until then the lead sets it by the written status rules. Verbal confidence is logged only. | R9, P2.5, P3.7 |
| D8 | Code (C2, planned; until then the lead in shadow) computes the band from coverage, stability and verification. Progress does not set the band: it detects a stall and blocks DEEPEN. Block 1 has no baseline for progress. | R10, P3.2 to P3.5 |
| D9 | The lead chooses STOP, DEEPEN, WIDEN or STOP-INCOMPLETE, cites ledger ids and cannot stop below the band of its tier. T1 needs MEDIUM with every driving claim covered. At a cap, code (C2, planned) forces STOP-INCOMPLETE. Until then the lead chooses it. | R10, R11, P3.2 |
| D10 | The lead reports to the main session at each barrier in a fixed short form. | R12 |
| D11 | The dossier ends with Decision, Rationale and Alternatives entries. Each ADR option and driver cites a finding id. | R13, P6.1, P6.4 |
| D12 | A writer marks a missing fact `NEEDS-EVIDENCE`. Each marker is a narrow one-block run, capped at five for each pass. | R14, P6.1, P6.2 |
| D13 | Code (C2, planned) owns the counts and the caps. The LLM owns the judgment of meaning. Until C2 exists, the lead does the work of code by the written rules. | P2.5, P3.5, P6.7 |
| D14 | Run in shadow first. The lead records the band, the choice and the stop reason by hand until C2 to C4 exist. | R15 |

## 6. Components

| Id | Component | Red test first | Lane or owner | Decisions |
|---|---|---|---|---|
| C1 | The skill and its references: `skills/research/SKILL.md`, `references/loop.md`, `references/ledger.md`, and this design set. **This change.** | `evolve docs ste-lint --strict` and the link check fail before the files exist. | console | D1 to D14 |
| C2 | An `evolve research` verb set in Go. It checks the ledger and the report against a schema. It computes the band, enforces the caps, keeps the depth log and records the stop reason. | A table test for the band for each row of §6 of the spec. A test that a lower tier is rejected. | lane work | D4, D7, D8, D9, D13 |
| C3 | A `NEEDS-EVIDENCE` marker gate in the docs checks. | A test that a document with an open marker fails the check. | console, because a gate is a protected surface | D12 |
| C4 | A shadow log that records the band and the stop reason of each run against later rework, to tune the thresholds. | A test that a run with a later rework is counted as a stop that was too early. | lane work | D14 |

C2 to C4 are not built in this change.

## 7. Order

1. C1. It gives the rules in text, and the lead can use them by hand at once.
2. C2. It turns the rules that a count decides into code. It needs the schema from C1.
3. C4. It needs the depth log of C2. Start it with C2, so that the first runs give data.
4. C3. It needs the marker format from C1. It is a gate, so it comes after the shadow period of the other parts.

The reason for the order: a rule must run by hand and show its flaws before code freezes it.

## 8. Verification

| Goal | Measure | Before | Target |
|---|---|---|---|
| 1 | Runs with a recorded band, choice and stop reason | 0 of 1 (the first run of the skill) | all runs |
| 2 | Runs that stop below the tier band without STOP-INCOMPLETE | not measured | 0 |
| 3 | Requests with the three lens floor | 0 of 1 | all requests |
| 4 | Barriers with a report to the main session in the fixed form | 0 of 1 (the first run had no barrier) | all barriers |
| 5 | ADR options and drivers with a finding id | not measured | 100% |
| 6 | Documents that ship with an open marker | not measured | 0 |
| Text | `evolve docs ste-lint --strict --changed refs/remotes/origin/main --added` | not run | 0 findings |

C4 measures the stops that were too early: the rework that a run needed after its stop.

## 9. Risks

| Risk | Control |
|---|---|
| The lead counts the band by hand and makes an error. | The depth log shows each input. A reviewer can recompute. C2 removes the risk. |
| The thresholds are wrong. | They are first values. The shadow log tunes them. |
| The loop costs more than a flat fan-out. | The tier caps bound the cost. T0 runs one block. |
| A seat reads the notes of another seat. | The brief forbids it. The barrier is the only point of exchange. |
| The ledger adds work that a small request does not need. | T0 has one block and a short ledger. |
| A marker loop runs without end. | Five markers for each pass and two runs for each marker, then the operator. |

## 10. Open questions

The operator owns these questions. None blocks C1.

- **Q1.** Do you want C2, C3 and C4 filed as inbox items, or built in the console? The recommendation is to file C2 and C4 as lane work and to build C3 in the console.
- **Q2.** Do you want the band thresholds of §6 in the spec to stay as first values until 10 runs are in the shadow log? The number 10 is a design choice. No source measures it.
- **Q3.** Do you want the skeptic seat to use a second model family by default when the console has one?
