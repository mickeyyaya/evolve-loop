# Plan: each phase cell gets a small assignment and a layered index that it opens on demand

- **Status:** proposed, 2026-10-26.
- **Design:** [cell-context-index](../architecture/cell-context-index.md). **Decision:** [ADR-0154](../architecture/adr/0154-the-pipeline-is-the-only-controller-and-each-agent-is-an-isolated-replaceable-worker.md) I-4 rule 14 and [ADR-0155](../architecture/adr/0155-the-loop-heals-itself-inside-its-boundaries-an-agent-proposes-and-the-pipeline-disposes.md) I-12. **Research:** [cell-context-index-2026-10](../research/cell-context-index-2026-10.md).
- **Builds on:** X5 (PR #1133), the artifact index of [llm-isolation.md](../architecture/llm-isolation.md) §8; the rule registry index of PR #1137 ([guidance-layers.md](../architecture/guidance-layers.md) §10).

This plan has no ADR of its own. ADR-0154 rule 14 already decides that "the agent opens only what it needs". This plan decides how the index tells the agent what to open, and how the pipeline learns from the opens.

## Table of contents

- [1. Request](#1-request)
- [2. Facts](#2-facts)
- [3. Goals and non-goals](#3-goals-and-non-goals)
- [4. Candidate designs](#4-candidate-designs)
- [5. Decisions](#5-decisions)
- [6. Components](#6-components)
- [7. Order](#7-order)
- [8. Verification](#8-verification)
- [9. Risks](#9-risks)
- [10. Open questions](#10-open-questions)

## 1. Request

The operator said on 2026-10-26: "I feel for the data that can be accessed by phase agent; it also needs to be indexed and layered, not necessary to import everything at the beginning, but loaded as needed."

The console brief of the same day named four layers: L0 assignment, L1 upstream deliverables, L2 recovery and L3 knowledge. It also named the entry fields, a budget for each layer, a CLI-neutral read path and an access log for self-heal.

## 2. Facts

| Fact | Research id |
|---|---|
| The X5 entry has a summary, the bytes and a hash, but no layer, token estimate, required flag or reason. | F1.1 |
| The `message` and `context` classes of §8.1 have no entries. | F1.2 |
| The persona is a static prefix. Only the tail after `## Cycle Context` changes. | F1.3 |
| The build prompt is 48 KB (median), and its tail is 7 KB. | F1.4 |
| The tail preloads the recalled lessons, the task contract and the repair findings. | F1.5 |
| A deliverable is 0.4 to 22 KB, and a row is about 300 bytes. | F1.6 |
| `phaseio` declares the upstream phases of each core phase. | F1.7 |
| 65 of 73 phase specifications have a list `inputs.files` that is not empty. The core phases have no `phase.json`; `docs/architecture/phase-registry.json` declares their inputs. | F1.8 |
| A repo path needs no new grant. | F1.9 |
| A sandbox message on a grant names the rule in the kernel report. | F1.10 |
| A reading must not use data that the measured agent can write. | F1.11 |
| A read verb in the cell is class `A`. | F1.12 |
| The rule registry index has rows with shared fields, and its `layer` is a rule layer. | F1.13 |
| Current practice: a small always-on part, identifiers, and bodies on demand. | P2.1, P2.3, P2.4 |
| Large preloaded context files raise the cost by about 20 percent for a small gain. | P2.7 |

## 3. Goals and non-goals

Goals: G1 to G5 of the spec §1. In short:

- a small prompt;
- an index that tells the agent what each row is and what it costs;
- a log of the opens;
- changes that come from use, through a Go check;
- the same read path for each family.

Non-goals:

- A smaller persona. ADR-0145 and X19b own the persona text.
- New read scope. The grants stay the indexed paths (spec §10).
- An exact token count. The estimate is enough for a budget (research P2.9).
- A gate on the opens. A required entry that is not opened is a signal, never a FAIL.
- A search engine over the bodies (research §3).

## 4. Candidate designs

| Design | Summary |
|---|---|
| A. Extend the flat list | Add `tokens` to each X5 entry. No layers and no required flag. Keep the preloaded tail. |
| B. One layered index (chosen) | Each entry gets a layer from its class, a token estimate, `required` and `why`. Budgets per layer. Two `A` verbs. An access log from the sandbox. A tune action. The tail moves into entries at `enforce`. |
| C. One file per layer | `inputs/l1.json`, `inputs/l2.json` and `inputs/l3.json`. The prompt names the files, and the agent opens a layer file first. |
| D. Keep the preload and compress it | A deterministic digest of each upstream body goes into the prompt, as the `phaseio` digest does today. No index change. |
| E. A search verb | `evolve context search <query>` ranks the bodies by text match, and the agent opens the hits. |

Criteria, scored 1 (poor) to 5 (good):

| Criterion | Weight | A | B | C | D | E |
|---|---|---|---|---|---|---|
| G1 prompt size | 3 | 1 | 5 | 5 | 2 | 4 |
| G2 the agent knows what to open | 3 | 2 | 5 | 4 | 3 | 2 |
| G3 the pipeline knows what was opened | 2 | 1 | 5 | 5 | 1 | 3 |
| No new read scope; no LLM decides a grant | 3 | 5 | 5 | 5 | 5 | 4 |
| Build cost (5 = cheapest) | 1 | 5 | 3 | 3 | 4 | 2 |
| One read path for each family (I-17) | 2 | 4 | 5 | 4 | 5 | 4 |
| **Weighted sum** | | 39 | **68** | 63 | 46 | 46 |

Why B wins:

- A leaves the preload in place (F1.5), so it does not meet the request.
- C gives the same layers but splits one fact into three files. The X5 grants, the hash check and the readers then have three sources to keep in step. B keeps one file and gives the layer filter as a verb flag.
- D keeps each body in each prompt. The cost grows with each new deliverable, and an agent cannot skip it (P2.7).
- E answers a question that the cell does not have. A cell has tens of bodies, and each one has a known kind (F1.6). A ranked search also hides the reason for a row.

What changes our mind: the access log can show that the cells of a phase open each L1 row on almost every run. Then the required list grows to the whole layer, and design D is cheaper for that phase. The tune action of the spec §9 shows this in data.

## 5. Decisions

| Id | Decision | Because | Rejected options and why |
|---|---|---|---|
| D1 | The class of an entry sets its layer. | F1.1, F1.2 | A layer field that each source chooses: two sources of one kind can then disagree. |
| D2 | `tokens` is `ceil(bytes / 3)`, and the header names the method. | P2.9 | A tokenizer call: it needs a network call and vendor code above the driver (I-17). |
| D3 | `why` comes from fixed templates. | §8.3 of llm-isolation.md | An LLM summary: no LLM writes the index. |
| D4 | The required list comes from the policy, then the phase specification, then the `phaseio` edges. | F1.7, F1.8 | A Go table: phase settings come from configuration. The edges only: the optional phases have their own inputs. |
| D5 | Required means "open first", never "preload". | ADR-0154 rule 14, P2.7 | Inline the required bodies: that is design D for the required set. |
| D6 | The budgets count required bodies, and an overflow clears the flag. It never removes an L1 or an L2 row. | F1.6, P2.5 | Drop the rows over the budget: that removes a grant without a decision. |
| D7 | `evolve context index` and `evolve context open` are class `A`. CX2 adds them to the class table of cli-contract.md §4.1. | F1.12 | Class `R` from the brief: the verb then refuses in the cell. |
| D8 | The access log comes from the sandbox report of each grant. The pipeline maps the reported path to its entry through the index. | F1.10, F1.11 | A log that the verb writes: the agent can write it, and a direct read skips it. File access times: a mount option can turn them off. |
| D9 | Use changes the index through one catalog action, `context-tune`, with a Go check at the boundary. | ADR-0155 I-12 | The orchestrator writes the policy: an agent then decides a grant. A fixed list forever: the index cannot learn. |
| D10 | `context-tune` can exclude only an L3 kind other than `rule`. It can demote an L1 or an L2 kind. No change grants a path outside the fixed sources. | ADR-0154 rule 11, Consequences | Let it exclude each kind: an L1 exclude removes a declared deliverable (rule 11), and an L2 exclude hides a lesson that a retry needs. |
| D11 | The builder reads `guidance/index.json` and adds the rows for the audience `phase`, the phase and the family as L3 rows. A rule row keeps its rule layer in `rule_layer`. | F1.13 | One L3 row that points at a separate filtered file: guidance-layers.md §10.6 and the item `guidance-cell-index-input` merge the rows into the cell index. A shared `layer` field for both meanings: a reader cannot tell them apart. |
| D12 | The stage key is `isolation.context.stage`, default `shadow`. `enforce` moves the tail into entries, and it lands last. | ADR-0154 rule 16, F1.5 | One stage for the fields and the prompt: a change of the prompt needs the shadow data first. |

## 6. Components

| Id | Component | Red test first | PR | Inbox item |
|---|---|---|---|---|
| CX1 | The `/2` fields, the layers, the required list and the budgets in `inputindex`; the policy keys `isolation.context.*` with a strict decode | `TestBuild_EachEntryHasTheLayerOfItsClass` | PR 2 | `context-index-layers-and-budgets` |
| CX2 | The verbs `evolve context index` and `evolve context open`, class `A`, with the envelope; the persona text in `agents/agent-templates.md`; matrix row N15 | `TestContextOpen_RefusesAnUnknownIDAForeignPathAndAChangedBody` | PR 2 | `context-index-read-verbs` |
| CX3 | The grant reports, the access log `.evolve/context-access/`, `evolve context stats` and the signals; sandbox rows N16 and N17 | `TestReportReads_WritesOneAccessRecordForEachOpenedEntry` | PR 2 | `context-index-access-log` |
| CX4 | The `message` and `context` sources of L1 | `TestBuild_ListsTheInboxItemFileOfEachItemID` | later | `context-index-message-and-context-classes` |
| CX5 | `enforce`: the L0 block and the moved bodies of spec §5 | `TestComposePrompt_AtEnforceTheTailIsTheL0BlockOnly` | later | `context-index-l0-prompt-enforce` |
| CX6 | The catalog action `context-tune` and its Go check | `TestContextTune_NoChangeGrantsAPathOutsideTheFixedSources` | later | `context-index-tune-action` |
| CX7a | The filtered rule rows of `guidance/index.json` as L3 rows | the acceptance of the item | later | `guidance-cell-index-input` (filed by PR #1137, owned here) |
| CX7b | The other L3 sources: `docs/index.md`, the recalled lessons file at `enforce`, the `evolve code` verbs | `TestBuild_ListsTheDocsIndexAndTheCodeVerbsAsKnowledgeEntries` | later | `context-index-knowledge-layer` |

## 7. Order

1. PR 1, this design set and the inbox items. Base: the X5 branch, then `main` after #1133 merges.
2. CX1, CX2 and CX3 in PR 2, after #1133 merges. CX2 and CX3 need the fields of CX1.
3. CX7a and CX7b. CX7a needs `guidance/index.json`, which the item `guidance-registry-index-and-check` of PR #1137 writes.
4. CX4. It needs only CX1.
5. CX6, after one wave of access records. It also needs the catalog of X14 (item `selfheal-action-catalog-validator-budgets`).
6. CX5 last. The operator flips `enforce` after one clean wave at `shadow`.

The reason for the order: each later component reads a fact that an earlier one writes. CX5 changes the prompt of each cell, so it waits for the shadow data.

## 8. Verification

| Goal | Before | After | How to measure |
|---|---|---|---|
| G1 | build tail 7,169 bytes (median, research F1.4) | at `enforce`, at most the L0 budget | `ISOLATION_CONTEXT_PRELOAD_BYTES` of CX5, then the tail bytes |
| G2 | 0 of the entries have `layer`, `tokens`, `required` or `why` | each entry has them | `TestBuild_EachEntryCarriesItsTokenEstimateAndItsReason` |
| G3 | no access record | one record for each opened entry on macOS | sandbox row N16; the count of records in one wave |
| G4 | no tune action | each change through the Go check | `TestContextTune_NoChangeGrantsAPathOutsideTheFixedSources` |
| G5 | the persona names workspace paths | the persona names the file and the verbs only | the guard of llm-isolation.md §28 |

## 9. Risks

| Risk | Control |
|---|---|
| An agent skips a required entry. | `ISOLATION_CONTEXT_REQUIRED_UNOPENED`, and the persona text. No FAIL, because the reports are best effort. |
| The kernel merges or drops a report, so a rate is low. | `exclude` has the strictest threshold (0.05) and a minimum of 20 cells. |
| A wrong exclude hides a needed body. | No L2 exclude; `include` reverses an exclude; the change waits for a boundary. |
| The estimate is far from the real count for one family. | The budget is a warning and a flag change, never a refusal. |
| A `/1` reader meets a `/2` file in a mixed binary. | `Read` accepts both for one release. |
| PR 1 sits on the X5 branch, and a squash merge of #1133 rewrites its commits. | Rebase PR 1 onto `main` and retarget it after #1133 merges. |

## 10. Open questions

- Q1. The brief named class `R` for the read verbs. This plan uses `A` (D7). The operator confirms or changes it.
- Q2 (closed on 2026-10-26). The field name `rule_layer` of D11 is agreed: [guidance-layers.md](../architecture/guidance-layers.md) §10.6 uses it (PR #1137).
- Q3. A Linux cell gives no access record. A Linux source, for example an `fanotify` reader, needs root. This plan has none.
- Q4. The open rate as a quality index of the ADR-0153 panel needs an amendment of ADR-0153. CX6 reads the log directly until then.
