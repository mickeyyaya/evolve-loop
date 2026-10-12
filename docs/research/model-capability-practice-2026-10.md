# A loop that trusts stronger models, and a watch on the best practice of each CLI (research, 2026-10)

> The plan that uses this research: [model-capability-practice-2026-10.md](../plans/model-capability-practice-2026-10.md). The decision record: [ADR-0133](../architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md).

Date: 2026-10-12. Method: counts over the phase prompts, personas, skills and commit history, and a read of the CLI-update and model-catalog code. Also a survey of the current guidance of Anthropic, OpenAI and Google. Each local claim names its file or command. "Synthesis" marks our own inference.

## Table of contents

1. [The request](#1-the-request)
2. [Local evidence: the loop today](#2-local-evidence-the-loop-today)
3. [Local evidence: the watch that exists](#3-local-evidence-the-watch-that-exists)
4. [Current practice for capable models](#4-current-practice-for-capable-models)
5. [Sources that a watch can read](#5-sources-that-a-watch-can-read)
6. [Adopt or reject](#6-adopt-or-reject)
7. [Refinements to adopt](#7-refinements-to-adopt)

## 1. The request

The operator wrote this on 2026-10-12:

> "I would like to learn how to optimize evo loop to better leverage the increasing capabilities on the latest model which doesn't need too much intervention and less prompt instruction, research online and create a routine to monitor the latest best practice update from each LLM clis"

The request has two parts. The first part asks for a loop with less intervention and shorter prompts. The second part asks for a routine that finds new best practice for each CLI that the loop uses.

## 2. Local evidence: the loop today

**F2.1** The phase prompts of cycle 1854 are large: build 45,128 bytes, tdd 44,573, triage 45,945, advisor plan 39,930 and scout 19,994 (`.evolve/runs/cycle-1854/*-prompt.txt`).
*Implication:* each phase reads about 10 K tokens of instructions before its task.

**F2.2** The 106 persona files in `agents/` hold 647,243 bytes. The main personas are 13,837 bytes (scout) to 30,915 bytes (tdd-engineer).

**F2.3** The personas use emphatic words in capitals (MUST, NEVER, CRITICAL, ALWAYS, IMPORTANT, MANDATORY): builder 17, tdd-engineer 15, auditor 13, scout 5, retrospective 3. The `SKILL.md` files add 69 more (`grep -oE`).
*Implication:* the current guidance says that such words cause over-triggering in newer models (P4.1).

**F2.4** Many persona rules repeat rules that the Go kernel already enforces: phase order, ship gate, audit binding, the ledger and worktree limits. AGENTS.md invariants 1 to 6 say: "STRUCTURAL: kernel hooks enforce them".
*Implication:* a prompt rule that a kernel guard enforces is a second home for the rule. It costs tokens and adds no safety.

**F2.5** Since 2026-09-28, 254 commits reached `main`. 33 of their subjects name a cycle (`git log --since=2026-09-28 --format=%s`; a heuristic on the subject text). The rest came from console lanes.
*Implication (synthesis):* most change still needs a person or the console. The loop does not yet carry most of the work.

**F2.6** On 2026-10-12, four console lanes stopped on a full disk (ENOSPC). The Go build cache held 107 GB against a 20 GB cap. `evolve gc` trims the cache only at a wave boundary, and no wave ran.
*Implication:* some interventions come from the host, not from the model. A stronger model does not remove them. A self-healing host does.

**F2.7** The build phase of a code cycle preloads 9,269 bytes of skill bodies below deep and 11,558 at deep and top ([skill-overlays.md](../architecture/skill-overlays.md)).

## 3. Local evidence: the watch that exists

**F3.1** `internal/cliupdate` updates each subscribed CLI at a loop boundary and smoke-tests a changed version. `internal/modelquery` and `internal/modelcatalog` query each CLI for its live models and map them to tiers.
*Implication:* the loop already watches versions and models. It does not watch practice: changelogs, prompting guides and best-practice pages.

**F3.2** The subscribed CLIs are Claude Code and agy (Antigravity); routing uses Claude Code and agy only (CLAUDE.md, "Since 2026-10-07 the `cli_routing` table decides every dispatch").
*Implication:* the watch must cover these two first. Codex and the Gemini CLI are optional.

**F3.3** The inbox is the queue of the loop (258 items on 2026-10-12, ranked by ADR-0121).
*Implication:* a watch can file an inbox item, and the loop then carries the follow-up work.

## 4. Current practice for capable models

**P4.1 Anthropic, prompting guide.** The guide covers the current Claude models. It says that newer models follow instructions more literally. It says that emphatic wording written to stop under-triggering now causes over-triggering: write "Use this tool when ..." in place of "CRITICAL: You MUST use this tool when ...". It also says that newer models tend to over-engineer, so state the minimal scope ([platform.claude.com](https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices); [DreamHost summary](https://www.dreamhost.com/blog/claude-prompt-engineering/)).

**P4.2 Anthropic, context engineering for the Claude 5 generation.** Secondary reports of a claude.com post say that Anthropic removed more than 80% of the Claude Code system prompt. They report no measured loss on coding evaluations. The cut rules made up for weaker models; conflicting layers stood in one request; examples moved into interfaces and skills ([techstrong.ai](https://techstrong.ai/agentic-ai/anthropic-cut-80-of-claude-codes-system-prompt-heres-why-that-matters-for-your-agents/), [dev.to](https://dev.to/thegatewayguy/anthropic-deleted-80-of-claude-codes-system-prompt-no-regression-1ieg)). A critic notes that no diff, eval set or scores were published. We did not verify the primary post.

**P4.3 Anthropic, harness design.** A harness component encodes an assumption about what the model cannot do alone. When the model improves, the component carries nothing and can go ([Addy Osmani, agent harness engineering](https://addyosmani.com/blog/agent-harness-engineering/)). A separate evaluator beats self-evaluation, because an agent rates its own work too high ([ZenML summary of the Anthropic post](https://www.zenml.io/llmops-database/long-running-agent-harness-for-multi-context-software-development)).

**P4.4 OpenAI, GPT-5 guide.** Eagerness is a dial: a lower reasoning effort reduces exploration, and many workflows are consistent at medium or low effort. A prompt states clear criteria to stop the search and the actions that are safe or risky ([OpenAI cookbook](https://developers.openai.com/cookbook/examples/gpt-5/gpt-5_prompting_guide)). For Codex, `AGENTS.md` is the place for durable guidance, and a lean, hand-written file is better than a generated one ([Codex best practices](https://developers.openai.com/codex/learn/best-practices)).

**P4.5 Google, Gemini 3.** Be direct and concise; older verbose prompts cause over-analysis. Keep the temperature at 1.0. Put constraints at the top and the task after long data ([Vertex AI guide](https://docs.cloud.google.com/vertex-ai/generative-ai/docs/start/gemini-3-prompting-guide?hl=de), [Phil Schmid](https://www.philschmid.de/gemini-3-prompt-practices)). Antigravity separates rules (standing context, 12,000 characters each) from workflows (step sequences) ([KDnuggets](https://www.kdnuggets.com/build-better-ai-agents-with-google-antigravity-skills-and-workflows)).

**P4.6 Common thread (synthesis).** All three vendors move the same way. State the goal, the done criteria and the limits. Drop emphatic and repeated rules, and keep deterministic checks outside the prompt. Measure each prompt change with an evaluation.

## 5. Sources that a watch can read

| CLI | Source | Form |
|---|---|---|
| Claude Code | `github.com/anthropics/claude-code` `CHANGELOG.md` and releases | Markdown file; GitHub releases feed by convention (not verified) |
| Claude models | the prompting guide and "What's new" pages on `platform.claude.com` | web page |
| agy (Antigravity) | `antigravity.google/docs/changelog?tab=cli`; `agy changelog` | web page; CLI command |
| Gemini models | the Gemini 3 prompting guide on Vertex AI | web page |
| Codex (optional) | `developers.openai.com/codex/changelog` (a feed); `github.com/openai/codex/releases` | feed; releases |
| Gemini CLI (optional) | `github.com/google-gemini/gemini-cli/releases` (weekly preview on Tuesdays) | releases |

Sources: [Havoptic feeds](https://www.havoptic.com/feeds), [releases.sh for Codex](https://releases.sh/openai/sources), [Antigravity changelog](https://www.antigravity.google/docs/changelog?tab=cli), [Gemini CLI installation](https://www.mintlify.com/google-gemini/gemini-cli/installation).

## 6. Adopt or reject

| Practice | Decision | Reason |
|---|---|---|
| Remove emphatic capitals from personas and skills (P4.1) | Adopt, measured | F2.3; each change goes through an A/B on simulated cycles. |
| Remove prompt rules that a kernel guard enforces (P4.2, F2.4) | Adopt, measured | One home for each rule; fewer tokens. |
| Goal, done criteria and limits in place of step lists (P4.4, P4.6) | Adopt for new and touched personas | A capable model chooses the steps. |
| Remove the persona or the guards in one step | Reject | No evaluation backs a big cut here; the 80% figure is unverified (P4.2). |
| Keep the separate auditor (P4.3) | Keep | It is the evaluator that P4.3 recommends. |
| Self-heal the host: continuous cache cap, disk guard (F2.6) | Adopt | A model cannot fix a full disk. |
| A watch on practice sources (F3.1, §5) | Adopt | Versions and models are watched; practice is not. |
| Learned prompt optimization (an optimizer that rewrites prompts) | Reject for now | It needs an evaluation set first. |

## 7. Refinements to adopt

| Id | Refinement | From |
|---|---|---|
| R1 | A practice watch reads a configured list of sources for each subscribed CLI at each wave boundary. | F3.1, F3.2, §5 |
| R2 | The watch is deterministic: fetch, normalize, hash, compare with the last snapshot. | AGENTS.md rule 5 |
| R3 | A changed source files one inbox item with the delta and its link. The loop or the console judges what to change. | F3.3 |
| R4 | A prompt diet: emphatic capitals and kernel-enforced rules leave the personas, one persona at a time, each with an A/B measure. | F2.1 to F2.4, P4.1, P4.2 |
| R5 | New and touched personas state the goal, the done criteria and the limits, not a step list. | P4.4, P4.6 |
| R6 | The host heals itself: the build cache cap applies at each lane floor and each cycle, not only at a boundary. | F2.6 |
| R7 | A per-family note (Claude, Gemini) holds the model-specific advice from the watch, so a prompt carries only the advice for its family. | P4.1, P4.5 |
