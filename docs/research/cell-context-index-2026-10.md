# Cell context index: what a phase agent loads first, and what it opens on demand (research, 2026-10)

> The plan that uses this research: [cell-context-index-2026-10](../plans/cell-context-index-2026-10.md). The design: [cell-context-index](../architecture/cell-context-index.md). The decision records: [ADR-0154](../architecture/adr/0154-the-pipeline-is-the-only-controller-and-each-agent-is-an-isolated-replaceable-worker.md) I-4 and [ADR-0155](../architecture/adr/0155-the-loop-heals-itself-inside-its-boundaries-an-agent-proposes-and-the-pipeline-disposes.md) I-12.

Date: 2026-10-26. Method: I read the X5 branch `feat/x5-artifact-index` at `5d6a84e00` (PR #1133) and the guidance branch `docs/guidance-index` (PR #1137). I measured the prompts and the deliverables of the last runs in the plane workspace `.evolve/runs/` with `stat` and `awk`. I ran live web searches on 2026-10-26 and read the primary pages. Each local claim names its file or its command. "Synthesis" marks our own inference.

## Table of contents

- [1. Local evidence](#1-local-evidence)
- [2. Current practice](#2-current-practice)
- [3. Adopt or reject](#3-adopt-or-reject)
- [4. Refinements to adopt](#4-refinements-to-adopt)

## 1. Local evidence

**F1.1** The X5 index `inputs/index.json` is a flat list. Each entry has `id`, `class`, `kind`, `path`, `cycle`, `phase`, `attempt`, `summary`, `bytes` and `sha256` (`go/internal/inputindex/index.go:25-36` on `5d6a84e00`). The classes are `deliverable` and `lineage` only (`index.go:20-23`).
*Implication:* the entry already has the cheap metadata of a catalog. It has no layer, no token estimate, no `required` flag and no reason.

**F1.2** The `message` class and the `context` class of [llm-isolation.md](../architecture/llm-isolation.md) §8.1 have no entries. §8.5 says so in its list of known differences.
*Implication:* the mailbox frames and the inbox item reach a cell through other paths, not through the index.

**F1.3** The prompt of a phase has a static persona and a dynamic tail after `## Cycle Context` (`go/internal/phases/runner/runner.go:242`, `:247`). `runner.StaticPrefix` cuts the prompt at that line (`runner.go:279-284`).
*Implication:* the persona is a stable prefix that a prompt cache can reuse. Only the tail changes for each cell.

**F1.4** I measured the prompts of the last 20 cycles that have each file (`stat -f %z .evolve/runs/cycle-20*/<phase>-prompt.txt`, 2026-10-26):

| Phase | Median bytes | Max bytes | Median bytes of the tail after `## Cycle Context` |
|---|---|---|---|
| scout | 20,144 | 20,671 | not measured |
| triage | 42,636 | 44,700 | 17,998 |
| tdd | 46,516 | 52,289 | 5,099 |
| build | 48,213 | 57,457 | 7,169 |
| audit | 38,220 | 41,918 | 9,020 |

*Implication:* the persona is most of each prompt. The tail is 5 to 18 KB, and it is the part that changes for each cell.

**F1.5** The prompt tails preload three bodies:

- `recalled_lessons_untrusted_json`: the recalled lessons as one JSON line, in the tail of each phase (`runner.BaseCycleContext`, `go/internal/phases/runner/runner.go:253`). In the last 10 build prompts it was 2,412 to 2,669 bytes (`grep '^- recalled_lessons_untrusted_json'`).
- `## Task Contract`, in the build tail (`go/internal/phases/build/build.go:31-59`): 2,071 to 8,077 bytes in the same 10 prompts.
- `## Prior Attempt Findings`, `## Audit Repair` and `## Standing Audit Findings` on a retry (`writeRepairContext`, `build.go:61`).

*Implication:* lessons and repair findings are lineage data, but they reach the agent as preloaded text, outside the index of X5.

**F1.6** I measured the deliverables of the last 40 cycles that have each file (2026-10-26):

| File | Count | Median bytes | Max bytes |
|---|---|---|---|
| `scout-report.md` | 40 | 2,420 | 3,878 |
| `triage-decision.json` | 40 | 402 | 682 |
| `test-report.md` | 40 | 8,001 | 21,535 |
| `build-report.md` | 40 | 3,880 | 9,564 |
| `audit-report.md` | 40 | 7,180 | 10,388 |
| `retrospective-report.md` | 28 | 6,571 | 7,614 |
| `carryover-todos.json` | 24 | 1,208 | 2,034 |
| `handoff-tdd-build.json` | 9 | 348 | 356 |

*Implication:* one deliverable is 0.4 to 22 KB. An index row with a summary of 25 words is about 300 bytes. A row costs 1 to 10 percent of its body.

**F1.7** `phaseio` declares the upstream phases of each phase: build reads scout and triage, and tdd reads scout and triage (`go/internal/phaseio/digest.go:12-22`).
*Implication:* the pipeline already knows which upstream deliverables a phase needs most. That set is the natural default of `required`.

**F1.8** `phasespec.PhaseSpec` has an `inputs` field and a `prompt_context` field (`go/internal/phasespec/phasespec.go:56-59`). A phase is configuration data, and the project rule is that phase settings come from configuration, not from code. 65 of the 73 files `.evolve/phases/*/phase.json` have an `inputs.files` list that is not empty (`jq`, 2026-10-26). Each item is a path template such as `.evolve/runs/cycle-{cycle}/scout-report.md`.

The core phases (scout, triage, tdd, build, audit) have no `phase.json`. Their specifications are in `docs/architecture/phase-registry.json`, which declares `inputs.files` too: build reads `scout-report.md` and `triage-report.md`.
*Implication:* the `required` list of a phase belongs in the phase specification or the policy, not in a Go table.

**F1.9** X5 grants a read of each indexed path inside `.evolve/runs/`. A lesson under `.evolve/instincts/lessons/` gets no grant, because the repo grant already covers it (§8.5 of the spec).
*Implication:* knowledge sources in the repo need no new grant. A knowledge layer adds no read scope.

**F1.10** X5 measured that `(with message "...")` adds its text to the kernel report of a sandbox rule, and that an allowed read logs `allow file-read-data <path>` (§8.5). The grants of X5 carry no report, so the usage reading of §8.4 is not built.
*Implication:* a report on each grant rule gives the path of each open. The pipeline can map the path to its entry, and the agent cannot write that log.

**F1.11** ADR-0153 rule 7: "Each index reads only data that the measured agent cannot write."
*Implication:* a log that the cell writes in its own workspace cannot feed a quality index.

**F1.12** The agent classes of [cli-contract.md](../architecture/cli-contract.md) §4.1: `A` runs in the cell, `R` refuses and needs a request, `D` refuses. Rule 2: an `A` verb changes no shared state. It can write telemetry in the own workspace.
*Implication:* a read verb that the agent runs in the cell is class `A`. Class `R` makes the verb refuse in the cell, so an agent cannot open an entry with it.

**F1.13** The rule registry of PR #1137 writes `guidance/index.json`, one row for each active rule, and the verb `evolve guidance index --audience --phase --cli --json` (guidance-layers.md §10 on `docs/guidance-index`). Its §10.6 says that the context index of a cell reads `guidance/index.json` as its knowledge layer. Its item `guidance-cell-index-input` merges the filtered rule rows into the cell index, and its notes give the item to the context-index builder. Each rule row gets the shared fields `id`, `layer`, `kind`, `summary`, `bytes`, `path` and `audience`.
*Implication:* the filtered rules are rows of the cell index. The field `layer` of a rule row holds a rule layer (L0 to L4), not a context layer.

**F1.14** The plane has 744 lesson files under `.evolve/instincts/lessons/`, 4,620 KB in total (`ls | wc -l`, `du -sk`, 2026-10-26).
*Implication:* a preload of the lessons is impossible. A selection or an index is necessary.

## 2. Current practice

**P2.1** Anthropic, "Effective context engineering for AI agents", 2025-09-29 ([link](https://www.anthropic.com/engineering/effective-context-engineering-for-ai-agents)). Agents "maintain lightweight identifiers (file paths, stored queries, web links, etc.)" and load data at run time. The approach "allows agents to incrementally discover relevant context through exploration." The post names a hybrid: "retrieving some data up front for speed, and pursuing further autonomous exploration at its discretion". Metadata is a signal: "file sizes suggest complexity; naming conventions hint at purpose; timestamps can be a proxy for relevance."

**P2.2** The same post: as the context grows, "the model's ability to accurately recall information from that context decreases" (context rot). LLMs have an "attention budget". A subagent "returns only a condensed, distilled summary of its work (often 1,000-2,000 tokens)."

**P2.3** The Agent Skills specification ([agentskills.io/specification](https://agentskills.io/specification), read 2026-10-26) has three levels. Metadata, about 100 tokens, loads at startup for each skill. The body, under 5,000 tokens recommended, loads when the skill activates. Resources load "only when required". The description has 1 to 1,024 characters. The body stays under 500 lines, and file references stay one level deep.

**P2.4** Claude Code skills ([code.claude.com/docs/en/skills](https://code.claude.com/docs/en/skills), read 2026-10-26): "skill descriptions are loaded into context so Claude knows what's available, but full skill content only loads when invoked." The listing truncates the combined description at 1,536 characters. On compaction, re-attached skills share 25,000 tokens and keep the first 5,000 tokens of each.

**P2.5** Two third-party reports describe the Claude Code skill listing. The listing has a budget of about 1 percent of the context window. An overflow drops the descriptions of the least-used skills first ([claudefa.st](https://claudefa.st/blog/guide/mechanics/skill-listing-budget), [dev.to](https://dev.to/rulestack/too-many-claude-code-skills-how-the-listing-budget-decides-which-descriptions-claude-sees-4a6m)). These are secondary sources, and they disagree on the numbers.

**P2.6** llms.txt ([llmstxt.org](https://llmstxt.org/), read 2026-10-26): one small file with an H1, a summary in a blockquote, and lists of links with a note for each. A section named "Optional" holds "secondary information" that an agent can skip for a shorter context.

**P2.7** Gloaguen and others, "Evaluating AGENTS.md: Are Repository-Level Context Files Helpful for Coding Agents?", arXiv:2602.11988, 2026-02-12 ([link](https://arxiv.org/html/2602.11988v1)). Generated context files lowered the resolve rate by about 0.5 to 2 points and raised the cost by about 20 to 23 percent. Written files gave about 4 points and raised the cost by up to 19 percent. The authors advise minimal files.

**P2.8** Liu and others, "Lost in the Middle", TACL 2024, as summarized in 2025 and 2026 follow-ups ([arXiv:2511.13900](https://arxiv.org/html/2511.13900v1), [arXiv:2511.05850](https://arxiv.org/pdf/2511.05850)). Recall is best at the start and the end of a long context. A 2025 Google study finds no position effect for simple fact lookup in Gemini 2.5 Flash. A second study finds that the effect depends on the model and the format.

**P2.9** Token ratios, secondary sources only, 2026 ([speedtesthq](https://speedtesthq.com/guides/ai/llm-tokens-bytes-and-bandwidth), [playcode](https://playcode.io/blog/real-price-of-frontier-models), [sqmagazine](https://sqmagazine.co.uk/glossary/ai-token.md)). English prose is about 3.5 to 4 bytes for each token, code about 3 to 3.5, and JSON about 2.5 to 3. The newer Claude tokenizer gives about 30 percent more tokens for the same text. One TypeScript file gave 1.73 times the tokens of another vendor.
*Synthesis:* `ceil(bytes / 3)` is a safe upper estimate for Markdown on the newer tokenizer. It is an estimate, not a count.

## 3. Adopt or reject

| Practice | Decision | Reason |
|---|---|---|
| A small always-on part plus identifiers that load on demand (P2.1, P2.3) | adopt | It is the design of X5 already (F1.1), and the operator asked for it. |
| Three levels: metadata, body, resources (P2.3) | adopt, as the row, the body and the knowledge sources | It maps to L1 and L2 rows with bodies, and to L3 sub-indexes. |
| A budget for the always-on part (P2.4, P2.5) | adopt, per layer, from strict policy | The tails measure 5 to 18 KB (F1.4). A budget makes growth visible. |
| Drop the least-used descriptions first on an overflow (P2.5) | adopt in a changed form | The pipeline measures use from the sandbox (F1.10), and a Go check changes the list at a boundary, never an agent (ADR-0155). |
| An "Optional" section that an agent can skip (P2.6) | adopt as `required: false` | The flag is data that a check can test. |
| A preload of large context files (P2.7) | reject | Cost rises by about 20 percent for a small gain. The persona is already 20 to 41 KB (F1.4: the median prompt minus the median tail). |
| A vector store or an embedding search for the bodies | reject | The bodies of a cell are tens of files (F1.6). A file index and a lookup by id are enough, and the house rule is CLI only. |
| An MCP resource server for the index | reject | The house rule forbids MCP ([cli-only-no-mcp.md](../conventions/cli-only-no-mcp.md)). A file plus an `A` verb gives each family the same path. |
| A summary that an LLM writes | reject | §8.3 of the spec: the pipeline takes the summary from a declared field. No LLM writes it. |
| An exact token count from a tokenizer API | reject for now | It needs a network call for each entry and a vendor tokenizer above the driver (I-17). The estimate is enough for a budget. |

## 4. Refinements to adopt

- **R1** Each entry gets `layer`, `tokens`, `required` and `why`. The builder computes each one from records. Sources: F1.1, P2.1, P2.3.
- **R2** The prompt carries L0 only: the task, the acceptance, the output contract, and one pointer line that names the index and the required ids. Sources: F1.3, F1.5, P2.1, P2.7.
- **R3** The default `required` set of a phase comes from the declared upstream edges (F1.7). The phase specification or the policy can change it (F1.8).
- **R4** Each layer has a token budget from strict policy. An overflow is a note and a signal, and it never removes a grant in silence. Sources: F1.4, P2.4, P2.5.
- **R5** The access log comes from the sandbox report of each grant. The pipeline maps the reported path to its entry through the index. A log that the cell writes is never a source. Sources: F1.10, F1.11.
- **R6** The read verbs `evolve context index` and `evolve context open` are agent class `A`. Sources: F1.12.
- **R7** The filtered rule rows of `guidance/index.json` join the cell index as L3 rows. The context index names its own layer `layer`, and a rule row keeps its rule layer in `rule_layer`. Sources: F1.13.
- **R8** At `enforce`, the recalled lessons move from the prompt tail to an L3 entry, and the repair findings move to an L2 entry. A stage key controls the move. Sources: F1.5, F1.14.
- **R9** The token estimate is `ceil(bytes / 3)`, and the index names the method. Sources: P2.9.
