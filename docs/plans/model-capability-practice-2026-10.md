# Plan: the loop watches the practice of each CLI and trims its scaffolding by measure

- **Status:** accepted, 2026-10-12. M0 merged in PR 857, and M1 merged in PR 864 on 2026-10-12. M2 to M5 are open, and each has an inbox item (section 7).
- **Decision:** [ADR-0133](../architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md). **Research:** [model-capability-practice-2026-10.md](../research/model-capability-practice-2026-10.md).

## Table of contents

1. [Request](#1-request)
2. [Facts](#2-facts)
3. [Goals and non-goals](#3-goals-and-non-goals)
4. [Part 1: candidate approaches for less scaffolding](#4-part-1-candidate-approaches-for-less-scaffolding)
5. [Part 2: candidate approaches for the practice watch](#5-part-2-candidate-approaches-for-the-practice-watch)
6. [Decisions](#6-decisions)
7. [Components](#7-components)
8. [Order](#8-order)
9. [Verification](#9-verification)
10. [Risks](#10-risks)
11. [Open questions](#11-open-questions)

## 1. Request

The operator wrote this on 2026-10-12: "I would like to learn how to optimize evo loop to better leverage the increasing capabilities on the latest model which doesn't need too much intervention and less prompt instruction, research online and create a routine to monitor the latest best practice update from each LLM clis".

## 2. Facts

| Fact | Research |
|---|---|
| A phase prompt holds 20 to 46 KB of text before its task. | F2.1 |
| Personas use emphatic capitals: builder 17, tdd-engineer 15, auditor 13. | F2.3 |
| Many persona rules repeat rules that a kernel guard enforces. | F2.4 |
| 33 of 254 commits since 2026-09-28 name a cycle; the console carries most change. | F2.5 |
| A full disk stopped four lanes; the cache cap applies only at a boundary. | F2.6 |
| The loop watches CLI versions and models, but not practice. | F3.1 |
| Anthropic, OpenAI and Google all advise: goal, done criteria, limits; no emphatic or repeated rules; measure each change. | P4.1 to P4.6 |

## 3. Goals and non-goals

**Goals:**

- G1: the loop carries more of the change with fewer console interventions.
- G2: phase prompts get shorter, and no quality measure gets worse.
- G3: a change in the practice of Claude Code or agy reaches the inbox within one wave, with no person to look for it.
- G4: each prompt change has a before and after measure.

**Non-goals:**

- a rewrite of all personas in one step;
- a removal of a kernel guard (the guards stay; only their prompt copies go);
- a learned prompt optimizer.

## 4. Part 1: candidate approaches for less scaffolding

Each criterion has a score from 1 (bad) to 5 (good).

| Approach | What it does | Less intervention (G1) | Shorter prompts (G2) | Measured (G4) | Risk | Total |
|---|---|---|---|---|---|---|
| A. Big cut | Rewrite each persona to goal, done criteria and limits in one step, as Anthropic did for Claude Code (P4.2). | 3 | 5 | 1 | 1 | 10 |
| B. Measured diet | Trim one persona at a time: emphatic capitals first, then rules that a kernel guard enforces. Compare the quality measures before and after. | 3 | 4 | 5 | 4 | 16 |
| C. More compute | Keep the prompts. Raise the model tier and the effort so the loop needs less help. | 3 | 1 | 3 | 3 | 10 |
| D. Self-healing host | Remove the interventions that come from the host: the cache cap per cycle, a disk guard, tmux and sandbox recovery. | 5 | 1 | 4 | 4 | 14 |

**B wins** (16). It is the only approach that shortens prompts and proves that quality holds. The final design takes from the others:

| From | What the design takes |
|---|---|
| A | The end state: each persona states the goal, the done criteria and the limits. New and touched personas use this shape now. |
| C | A raise of effort for one phase only when a measure shows a gain (the house rule: start at medium). |
| D | The host self-heal as its own component (M4), because a model cannot fix a full disk (F2.6). |

## 5. Part 2: candidate approaches for the practice watch

| Approach | What it does | No person needed (G3) | Signal, not noise | Cost | Fits the loop | Total |
|---|---|---|---|---|---|---|
| E. Cloud routine | A scheduled cloud agent reads the sources each week and writes a digest. | 4 | 4 | 2 | 2 | 12 |
| F. Deterministic watch | `evolve practice watch` reads a source list at each wave boundary, compares a hash with the last snapshot, and files an inbox item on a change. | 5 | 3 | 5 | 5 | 18 |
| G. Loop phase | A practice-scout phase reads the sources with a model every N cycles. | 4 | 4 | 2 | 3 | 13 |
| H. Feed reader | The operator subscribes to release feeds and reads them. | 1 | 3 | 5 | 1 | 10 |

**F wins** (18). Detection is deterministic work, so it goes in code (AGENTS.md rule 5). The final design takes from the others:

| From | What the design takes |
|---|---|
| E and G | Judgment only on a change: the inbox item is a research task. The loop or the console reads the delta and decides what to change. No model runs when nothing changed. |
| H | Feeds and release files as sources where they exist, because they change only on a release and give less noise than a web page. |

## 6. Decisions

| Id | Decision | Because |
|---|---|---|
| D1 | A new verb, `evolve practice watch`, reads the sources in `practice_watch.sources` (policy), with compiled defaults for Claude Code and agy. | F3.1, F3.2, R1 |
| D2 | A source has an id, a family, a kind (`github-file`, `github-releases`, `page`, `command`) and a locator. | §5 of the research |
| D3 | The watch normalizes the text (strip markup and dates), hashes it, and keeps a snapshot in `.evolve/practice/<id>.json`. | R2 |
| D4 | A changed source files one inbox item, kind `research`, class `hygiene`. It holds the link and the added lines (capped). It asks for a read of the source and a proposal of prompt changes. | R3, F3.3 |
| D5 | The loop runs the watch at each wave boundary, after `evolve cli update`. A fetch failure warns and never blocks the boundary. | F3.1 |
| D6 | The first run records the snapshots and files no item. | noise control |
| D7 | The prompt diet trims one persona per change. Emphatic capitals go first, then rules that a kernel guard enforces. | R4 |
| D8 | Each diet change records four measures over 10 cycles before and 10 after. They are the ship rate, phase retries, contract-gate blocks and tokens per cycle. | G4 |
| D9 | New and touched personas state the goal, the done criteria and the limits. | R5 |
| D10 | The build cache cap also applies at the start of each cycle and each console lane floor, not only at a wave boundary. A disk guard warns at 10% free and pauses new dispatches at 5% free. | R6 |
| D11 | Model notes for each family live in one file per family. The watch items update them; an overlay loads only the notes of the family of the dispatch. | R7 |

**What changes our mind.** We stop the diet (D7) if any measure of D8 gets worse by more than its noise band over two diet changes in a row.

## 7. Components

| Id | Component | Red test first | Owner |
|---|---|---|---|
| M0 | These documents and the index entry | `evolve docs ste-lint --strict`; the link check | console |
| M1 | `evolve practice watch` and the boundary call (D1 to D6) | A changed `httptest` source files one item; an unchanged source and a first run file none. A fetch error warns, and the boundary exit code stays 0. A command source (`agy changelog`) runs through a fake CLI. | Sonnet lane. **Status: built, merged in PR 864 on 2026-10-12.** `internal/practicewatch`, `go/cmd/evolve/cmd_practice.go`, the boundary call in `cmd_loop_cliupdate.go`; see [runtime-reference](../operations/runtime-reference.md). |
| M2 | `evolve prompt stats`: the bytes and emphatic words of each persona and of each phase prompt of a run | Counts on a fixture tree | Sonnet lane. Open: inbox item `model-capability-practice-m2-prompt-stats` (2026-10-27). |
| M3 | The first diet change: the builder persona (17 emphatic words), with the D8 measures | The persona tests stay green; the measures of D8 are recorded | loop task. Open: inbox item `model-capability-practice-m3-builder-persona-diet` (2026-10-27). |
| M4 | The host self-heal (D10) | The cache trim runs at a cycle start; the guard warns and pauses at the thresholds | Sonnet lane. Open: inbox item `model-capability-practice-m4-host-self-heal` (2026-10-27). |
| M5 | Model notes for each family (D11) | An overlay loads only the notes of the family of the dispatch | Sonnet lane. Open: inbox item `model-capability-practice-m5-family-model-notes` (2026-10-27). |

## 8. Order

1. M0, because it is documents only.
2. M4 and M1, because M4 removes a cause of intervention that happened today, and M1 starts the watch.
3. M2, because M3 needs its measure.
4. M3, then one more diet change for each persona, as loop tasks.
5. M5 after the first watch items arrive.

## 9. Verification

| Goal | Measure |
|---|---|
| G1 | The share of commits on `main` that name a cycle, over 30 days (today 33 of 254). |
| G2 | `evolve prompt stats` before and after each diet change; the D8 measures do not get worse. |
| G3 | A change in a source makes an inbox item at the next wave boundary. |
| G4 | Each diet pull request has its D8 table. |

## 10. Risks

| Risk | Control |
|---|---|
| A web page changes often (dates, layout), so the watch files noise. | Normalize the text (D3); prefer feeds and release files (§5); one item per source until the console clears it. |
| A trimmed persona loses a rule that a guard does not enforce. | Remove only a rule that a guard enforces, with the guard named in the pull request (D7). |
| The measures are noisy with 10 cycles. | Two diet changes in a row must agree before the plan stops the diet. |
| The network is down at a boundary. | The watch warns and the boundary continues (D5). |

## 11. Open questions

1. Must the watch also cover Codex and the Gemini CLI? This plan adds them as optional sources with no default.
2. Which noise band must D8 use for each measure? M2 records a baseline first.
