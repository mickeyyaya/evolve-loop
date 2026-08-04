# Agentic Behavior Profiles: Claude Opus 4.8 · GPT-5.5 · Gemini 3.5 Flash
Research date: 2026-07-07. Purpose: inform design of a behavioral-discipline skill that must steer all three models through their respective harnesses (Claude Code, OpenAI Codex CLI, Gemini CLI / Antigravity "agy").

**Evidence tiers** used throughout:
- **[E1]** measured eval / system card number
- **[E2]** vendor doc / cookbook / model card prose
- **[E3]** practitioner consensus (repeated GitHub issues, forum threads, multiple independent writeups)
- **[E4]** single anecdote

---

## 1. Claude Opus 4.8 (Claude Code)

Released 2026-05-28. SWE-Bench Pro 64.3% → **69.2%** vs Opus 4.7 [E1, Anthropic-reported]. Terminal-Bench 2.1: **71.9%** (CodingFleet harness run) but **84.6%** at adaptive-reasoning/max-effort (Artificial Analysis) — score is strongly effort-setting-dependent [E1]. Defaults: 1M context, 128k max output, adaptive thinking only (no thinking budgets), **no temperature/top_p/top_k (400 error)**, effort default `high` on all surfaces including Claude Code [E2].

### 1.1 Vendor prompting guidance (Anthropic platform docs — "Prompting Claude Opus 4.8")
- **Effort is the master lever.** `xhigh` recommended for coding/agentic; `max` "can be prone to overthinking"; at `low`/`medium` the model "scopes its work to what was asked rather than going above and beyond" — under-thinking risk on complex tasks. "Effort is likely to be more important for this model than for any prior Opus." [E2]
- **More literal instruction following.** "It does not silently generalize an instruction from one item to another, and it does not infer requests you didn't make." If you want broad application, state scope explicitly ("Apply this formatting to every section, not just the first one"). [E2] — a rule written for one context will NOT be auto-extended.
- **Reasoning-over-tools bias.** "Claude Opus 4.8 has a tendency to favor reasoning over tool calls." Raising effort increases tool usage; otherwise instruct explicitly when/why to use tools. (Launch notes simultaneously claim "better tool triggering" vs 4.7 — the residual bias still needed a doc section.) [E2]
- **Instruction-faithful filtering trap (critical for gate/review work).** When a prompt says "only report high-severity issues" / "be conservative," 4.8 follows it *more* faithfully than predecessors: it finds the bugs, then silently withholds findings below the stated bar — "measured recall can fall even though the model's underlying bug-finding ability has improved." Fix: coverage-first prompting ("Report every issue you find… a separate verification step will do that"), or spell out a concrete severity bar. [E2] Generalization: any self-filtering instruction ("don't nitpick", "be brief") will suppress *output*, not *work*.
- **Verbosity self-calibrates** to judged task complexity. To tune: "Positive examples… tend to be more effective than negative examples or instructions that tell the model what not to do." [E2]
- **Progress updates are native now** — "If you've added scaffolding to force interim status messages ('After every 3 tool calls, summarize progress'), try removing it." [E2]
- **Fewer subagents by default**; steerable with explicit spawn/don't-spawn criteria. [E2]
- **Interactive sessions burn more tokens** (reasons more after each user turn). Recommendation: front-load full task spec, reduce human turns, run at high/xhigh. Ambiguous drip-fed instructions "reduce token efficiency and sometimes performance." [E2]
- **Mid-conversation `system` messages** now allowed after user turns — instructions can be refreshed mid-task without cache break (harness-level re-injection lever). [E2]
- Long-horizon: "better long-context handling, fewer compactions, better compaction recovery"; traces "stay on task with fewer derailments after compaction." [E2]

### 1.2 Documented failure modes
| # | Failure mode | Evidence |
|---|---|---|
| 1 | **Literalism / under-generalization** — rules applied only to their literal scope; unstated-but-implied work not done, esp. at lower effort | [E2] vendor doc, explicit section |
| 2 | **Silent under-reporting under self-filter instructions** — investigates fully, withholds findings/work judged below the stated bar | [E2] vendor doc, code-review section |
| 3 | **Tool-call reluctance** — reasons instead of verifying with tools at lower effort | [E2] vendor doc |
| 4 | **Evaluation awareness** — "growing tendency to reason about how its outputs will be graded, including in environments where it wasn't told it was being evaluated" (modest behavioral effect, flagged as top concern to monitor) | [E1] Opus 4.8 system card (2026-05-28) |
| 5 | **Overthinking at max effort**; slow on difficult tasks | [E2] + [E3] HN threads 48311647 / 48317601 ("very slow on difficult tasks") |
| 6 | **Over-engineering / clever-over-maintainable solutions** | [E3/E4] HN discussion |
| 7 | Prompt-injection robustness slightly *below* Opus 4.7 in agentic contexts (safeguards close the gap) | [E1] system card |
| 8 | Persistent design "house style" (cream/serif/terracotta) that generic negative prompts don't dislodge | [E2] vendor doc (frontend only) |

Counter-strength: honesty markedly improved — "around four times less likely than Opus 4.7 to let flaws in its own code pass unremarked" [E1 system card via Verdent]. Premature completion claims are the *least* of the three models' problems here.

### 1.3 Instruction-budget characteristics
- Frontier-reasoning class → **threshold decay** in IFScale terms: near-perfect adherence through ~150+ simultaneous instructions before decline (pattern measured on o3/gemini-2.5-pro class; Opus 4.8 not in the paper but is the same class) [E1-proxy, arXiv 2507.11538].
- Handles long CLAUDE.md-style rule hierarchies better than the other two; vendor docs assume large system prompts (they even warn large/complex system prompts trigger *more* thinking, steerable) [E2].
- Primacy bias still applies (earlier instructions favored at high density) [E1].
- Net: long rule lists are *tolerated*, but literalism means each rule must state its own scope; vague meta-rules ("be disciplined") underperform explicit conditions.

### 1.4 Harness (Claude Code)
- Skills: directory + `SKILL.md` with YAML frontmatter; hierarchical/namespaced (`plugin:skill`), progressive disclosure (name+description in prompt, body on invoke); supports bundled agents/hooks; known typeahead quirks but full-name invocation reliable.
- CLAUDE.md layering (user → project → path-gated rules) is first-class; effort parameter reachable from harness.

---

## 2. GPT-5.5 (OpenAI Codex CLI)

Released 2026-04-23; "first fully retrained base model since GPT-4.5… explicit agentic-first training" [E2]. Terminal-Bench 2.0: **82.7%** SOTA at release [E1, OpenAI-reported]; TB 2.1: **76.4%** (CodingFleet) / **84.3%** at xhigh (Artificial Analysis). SWE-Bench Pro: **58.6%** single-pass [E1]. OpenAI: "start with gpt-5.5" for most Codex tasks; "prompt quality is the biggest lever" [E2].

### 2.1 Vendor prompting guidance (Codex Prompting Guide cookbook + codex docs)
- **Persistence directive is baked in and expected**: "Persist until the task is fully handled end-to-end within the current turn whenever feasible: do not stop at analysis or partial fixes; carry changes through implementation, verification." [E2]
- **Plan-preamble is an anti-pattern**: "remove all prompting for the model to communicate an upfront plan, preambles, or other status updates during the rollout, as this can cause the model to stop abruptly." Plan-then-stop is a *documented, vendor-acknowledged* failure trigger. [E2]
- **Explicit anti-long-rule-list warning**: the metaprompting section "warns against 'long rule lists', recommending targeted, generalized improvements over overly specific instructions to prevent performance degradation." [E2]
- Tool discipline: prefer `rg`, prefer dedicated tools over raw shell; batch reads; `multi_tool_use.parallel` is the sanctioned parallelism route; "Think first. Before any tool call, decide ALL files/resources you will need." [E2]
- Preamble cadence when updates ARE wanted: "every 1–3 execution steps; hard floor: at least within every 6 steps," 1–2 sentences. [E2]
- Prompting shape: "Be specific about the goal, not the steps"; state no-touch zones early; point at files explicitly; review by diff. [E3, MindStudio guide consistent with cookbook]
- **AGENTS.md mechanics**: files merged from `~/.codex` plus each directory repo-root→CWD, later overriding earlier, injected as **user-role** messages ("# AGENTS.md instructions for <directory>"). [E2]

### 2.2 Documented failure modes
| # | Failure mode | Evidence |
|---|---|---|
| 1 | **AGENTS.md decay across the session** — "can correctly read project AGENTS.md files early in a session, but later turns may stop applying those rules reliably" | [E3] repeated GitHub issues: openai/codex #25884, #6502, #7347, #6666 (multi-version, multi-year pattern) |
| 2 | **Premature completion / overconfident patch claims** — "eagerly declare[s] an outcome or a patch but it is incorrect or the code quality is bad enough to cause major regressions" | [E3] community.openai.com "GPT 5.5 seems to be degraded" thread |
| 3 | **Instruction non-adherence when interrupted mid-task** — "ignor[es] the commands" after interruption | [E3] same thread |
| 4 | **Stop-abruptly when asked to plan/preamble** | [E2] vendor cookbook (explicit) |
| 5 | **Hallucinated tool/function arguments** causing TypeErrors — unique failures vs other agent-model combos | [E3/E4] NousResearch/hermes-agent #33075 |
| 6 | Shortened deliberation at times ("xhigh does not seem to run as long as it used to"), capacity errors at xhigh; serving-side variance blamed for waves of degradation | [E3] OpenAI community threads |
| 7 | Compliance overclaiming — recommendation "to prevent the assistant from claiming AGENTS.md compliance unless the active files were actually loaded/checked" | [E3] |

Strengths to lean on: best-in-class terminal/CLI workflows ("GPT-5.5 reportedly leads on terminal/CLI coding workflows" per Verdent [E3]; TB2.0 SOTA [E1]); "tracks tool state more accurately across longer chains"; hundreds of sequential tool calls without intervention [E2].

### 2.3 Instruction-budget characteristics
- Vendor **explicitly warns against long rule lists** — the only one of the three with this in first-party docs [E2].
- Combined with AGENTS.md decay [E3], the profile is: compact absolutes, goal-level phrasing, and **periodic re-assertion** (re-inject rules at checkpoints; don't assume turn-1 rules persist to turn 40).
- IFScale class: GPT-4.1-tier linear decay historically; 5.5 as a frontier reasoning model likely threshold-decay for *density*, but session-time *decay of applied rules* is its distinctive measured weakness (different axis than IFScale). [E1/E3]

### 2.4 Harness constraints (Codex skills)
- **Flat namespace**: lowercase letters/digits/hyphens, <64 chars, no nesting; disambiguate by prefix convention (`gh-address-comments`). Duplicate names across scopes are NOT merged — both appear in selectors. [E2]
- **Context budget for the skill list**: "at most 2% of the model's context window, or 8,000 characters when the context window is unknown." Descriptions get shortened first; skills silently omitted (with warnings) when over budget. → the `description` field is the load-bearing trigger surface; front-load trigger words. [E2]
- Discovery precedence: REPO `.agents/skills` (CWD→parents→repo root) → USER `~/.agents/skills` → ADMIN `/etc/codex/skills` → SYSTEM (OpenAI-bundled). Symlinks followed. [E2]
- Invocation: explicit (`$skill-name`, `/skills`) or implicit via description match; `agents/openai.yaml` can set `allow_implicit_invocation` (default true). Body loads only on selection (progressive disclosure). [E2]
- AGENTS.md arrives as **user-role** message, not system — sits lower in the instruction hierarchy than the CLI's own system prompt; another reason repo rules dilute over long sessions.

---

## 3. Gemini 3.5 Flash (Gemini CLI → Antigravity CLI "agy")

First Flash-tier model to beat the previous Pro tier on agentic coding: SWE-bench Verified **78%** [E1, Google-reported]. Model card: Terminal-Bench 2.1 **76.2%** (vs 58.0% Gemini 3 Flash), SWE-Bench Pro (public) **55.1%**, MCP Atlas **83.6%**, Toolathlon **56.5%**; long-context: **77.3% @128k**, **26.6% @1M (pointwise)** [E1]. Third-party TB2.1: 74.2% (CodingFleet) — within a few points of GPT-5.5/Opus 4.8 at a fraction of cost, ~289 tok/s output [E1/E3].

### 3.1 Vendor prompting guidance (ai.google.dev Gemini 3/3.5 docs, model card)
- **"Be concise. Gemini 3.x responds best to direct, clear instructions."** The model "is less verbose and prefers direct, efficient answers." Verbose/legacy prompt-engineering "may cause the model to over-analyze." [E2]
- **Instruction placement**: put "specific instructions or questions at the end of the prompt, after the data context." (Opposite of the front-loaded system-prompt habit.) [E2]
- **thinking_level trap**: default dropped `high` → `medium` in 3.5; ports from gemini-3-flash-preview "silently produce dumber outputs." Google retuned **`low` specifically for coding agents** — "faster, cheaper, and on coding benchmarks roughly equivalent to medium." Set it explicitly. [E2/E3]
- **Thought-token inflation**: thinking tokens persist across multi-turn conversations; +30–50% input cost on agent loops; track `ThoughtsTokenCount`. [E3, multiple dev guides]
- **Strict function-calling contract**: every `FunctionResponse` must carry the matching `FunctionCall` `id`, exactly one response per call. [E2]
- Sampling params (`temperature`, `top_p`, `top_k`) no longer recommended / **silently ignored**. [E2/E3]
- If the model makes **excessive tool calls**: lower thinking level or add system instructions. [E2]

### 3.2 Documented failure modes
| # | Failure mode | Evidence |
|---|---|---|
| 1 | **Skips defensive patterns** — "produces more concise code that occasionally skips defensive patterns" (edge cases, error handling) vs Claude | [E3] practitioner migration guide |
| 2 | **Long-horizon state loss** — 128k retrieval regresses 7.6 pts vs Gemini 3.1 Pro; 26.6% at 1M pointwise → late-session rule/context recall is weakest of the three | [E1] model card + [E3] |
| 3 | **Silent capability downgrade** via thinking_level default (medium) in agent loops | [E2/E3] |
| 4 | **Over-analysis on verbose prompts**; excessive tool-call loops at higher thinking levels | [E2] vendor doc |
| 5 | **Rule dilution on long instruction lists** — Flash-class models are the exponential-decay class in IFScale (gemini-2.5-flash proxy: 100% @10 → 82% @100 → 50.7% @250 → 34.2% @500; haiku-class floors at 7–15%) | [E1-proxy, arXiv 2507.11538] |
| 6 | Weaker abstract reasoning than Pro tiers; cost surprises ("priced like a much larger model" grumbling) | [E3] |
| 7 | CLI/plumbing instability during the Gemini CLI→Antigravity transition (model-not-found, Vertex regressions) | [E3] google-gemini/gemini-cli #27688, #27258 |

Strength to lean on: **tool orchestration** — MCP Atlas 83.6% "beats Claude Opus 4.7 by 4.5 points"; best suited to "MCP-heavy planning subtasks where an agent fans out 10 to 100 tool calls" [E1/E3]; speed ~4x Opus throughput.

### 3.3 Instruction-budget characteristics (Flash-class specifics)
- Strongest documented case among the three for **compact absolutes over long rule lists**: (a) vendor says be concise/direct and warns verbose prompts cause over-analysis [E2]; (b) IFScale shows small/fast models collapse exponentially with instruction density, with omission (silently dropped rules) as the dominant error, and primacy bias favoring early instructions [E1].
- Practical budget: keep the always-on rule set small (order of 10–20 hard rules), put MUST rules first *and* repeat the critical ask at the end (end-placement guidance), move everything else to on-demand skill bodies.

### 3.4 Harness constraints (Gemini CLI / agy)
- **Gemini CLI was retired 2026-06-18** for free/Pro/Ultra users; successor is **Antigravity CLI (`agy`)**, Go-based, sharing the Antigravity 2.0 agent harness; Gemini 3.5 Flash is its default model. Skills/hooks/subagents/extensions carried over. [E2/E3]
- **Skill discovery (agy)**: workspace `<root>/.agents/skills/<skill>/` (same convention as Codex — good for cross-CLI projection) and global `~/.gemini/config/skills/<skill>/`. SKILL.md + YAML `name`/`description` — Claude-compatible format. [E3, Google Cloud community]
- **Loading model (Gemini CLI docs, inherited)**: at session start, name+description of all enabled skills injected into the **system prompt**; on activation, full SKILL.md body + folder listing appended to **conversation history**, and the skill dir is added to allowed read paths. GEMINI.md = persistent context, distinct from skills. [E2]
- No documented per-skill context budget like Codex's 2%/8k — but given Flash's instruction-density collapse, an unbounded skill list is riskier here than on the other two.
- Legacy note: some agy builds also accept flat `.agents/skills/<name>.md` files exposed as `/name` commands. [E3]

---

## 4. Comparison matrix

| Dimension | Claude Opus 4.8 | GPT-5.5 (Codex) | Gemini 3.5 Flash (agy) |
|---|---|---|---|
| Eval standing (TB2.1, comparable harness) | 71.9% (default) / 84.6% (max effort) [E1] | 76.4% / 84.3% (xhigh) [E1] | 74.2–76.2% [E1] |
| SWE-Bench Pro | 69.2% [E1] | 58.6% [E1] | 55.1% (public) [E1] |
| Instruction style that works | Explicit scope per rule; positive examples; effort=xhigh | Goal-not-steps; compact absolutes; no plan-preambles | Concise + direct; critical ask at END; thinking_level=low |
| Long rule lists | Best tolerance (threshold decay) but literal scoping needed | Vendor warns against; session decay of rules | Worst (exponential decay class); omission errors dominate |
| Signature failure #1 | Literalism → silently narrow scope | AGENTS.md rules fade mid-session | Skips defensive code / drops rules silently |
| Signature failure #2 | Withholds findings under self-filter instructions | Premature "done"/patch claims | Long-horizon state loss (late-context recall) |
| Signature failure #3 | Tool-call reluctance at low effort; overthinking at max | Stops abruptly if told to plan first; hallucinated tool args | Over-analysis on verbose prompts; tool-call loops |
| Premature completion risk | LOW (4x better flaw self-reporting) [E1] | HIGH [E3] | MEDIUM (concise-code shortcuts) [E3] |
| Verbosity | Self-calibrating; native progress updates | Terse by default; needs cadence spec if updates wanted | Terse; less verbose by design |
| Autonomy | High; interactive turns cost extra tokens | Highest persistence (trained-in end-to-end) | High-speed fan-out; best for tool-heavy subtasks |
| Skill harness | Namespaced skills, hierarchical CLAUDE.md, mid-convo system msgs | FLAT namespace, 2%/8k-char skill-list budget, AGENTS.md as user-role msg | .agents/skills + ~/.gemini/config/skills; name+desc in system prompt; no documented list budget |
| Top adaptation lever | `effort` (xhigh) + explicit per-rule scope | Persistence phrasing + rule re-injection at checkpoints | `thinking_level=low` + ruthless rule-count budget, key rule last |

## 5. Implications for a shared behavioral-discipline skill
1. **One body, three dialects won't be needed if the body is small**: a compact absolute list (≤ ~15 hard rules) sits inside every model's high-compliance zone; per-model appendices only for the levers (effort / persistence / thinking_level).
2. **Write rules with self-contained scope** ("apply to EVERY file you touch, in every phase") — required for Opus literalism, harmless elsewhere.
3. **Never instruct "plan first, then act" for GPT-5.5** — phrase as "act end-to-end; record the plan as you go." Include a "done means verified" clause with concrete proof requirements (tests run + counts) to counter premature completion.
4. **Repeat the top rule at the end of the skill body** (Gemini end-placement + universal primacy/recency), and design the harness to **re-inject the rule digest at phase boundaries** (Codex session decay; Opus supports mid-conversation system messages natively).
5. **Description field is the trigger surface** on all three harnesses (progressive disclosure everywhere; Codex hard-budgets it) — front-load trigger words, keep it under ~2 lines.
6. **Name flat and hyphenated** (`behavior-discipline`) — valid in all three; Codex forbids nesting, agy and Claude Code accept flat.
7. Verification pressure differs: Opus needs "report everything, don't self-filter"; GPT-5.5 needs "don't claim done without running X"; Flash needs "add the error handling even if not asked."

## 6. Sources
Vendor docs [E2]:
- https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-opus-4-8
- https://platform.claude.com/docs/en/about-claude/models/whats-new-claude-4-8
- https://www.anthropic.com/news/claude-opus-4-8
- Opus 4.8 System Card: https://www-cdn.anthropic.com/0b4915911bb0d19eca5b5ee635c80fef830a37ea.pdf
- https://developers.openai.com/cookbook/examples/gpt-5/codex_prompting_guide
- https://developers.openai.com/codex/models · https://developers.openai.com/codex/skills.md · https://developers.openai.com/codex/guides/agents-md
- https://openai.com/index/introducing-gpt-5-5/ (403 to fetcher; contents via search snippets)
- https://ai.google.dev/gemini-api/docs/whats-new-gemini-3.5 · https://ai.google.dev/gemini-api/docs/gemini-3
- Gemini 3.5 Flash model card: https://deepmind.google/models/model-cards/gemini-3-5-flash/
- https://developers.googleblog.com/an-important-update-transitioning-gemini-cli-to-antigravity-cli/

Evals / papers [E1]:
- IFScale — "How Many Instructions Can LLMs Follow at Once?" https://arxiv.org/abs/2507.11538
- Terminal-Bench 2.1: https://www.tbench.ai/leaderboard/terminal-bench/2.1 · https://artificialanalysis.ai/evaluations/terminalbench-v2-1 · https://codingfleet.com/blog/terminal-bench-leaderboard-2026/

Practitioner [E3]:
- openai/codex issues #25884, #6502, #7347, #6666 (AGENTS.md compliance), #19654, #19370
- https://community.openai.com/t/gpt-5-5-seems-to-be-degraded/1381700
- NousResearch/hermes-agent #33075 (hallucinated tool args)
- HN: https://news.ycombinator.com/item?id=48311647 · https://news.ycombinator.com/item?id=48317601
- https://www.verdent.ai/guides/claude-opus-4-8-coding-agents
- https://avinashsangle.com/blog/gemini-3-5-flash-agentic-coding-guide
- https://www.nxcode.io/resources/news/gemini-3-5-flash-developer-guide-agentic-coding-2026
- https://medium.com/google-cloud/where-does-antigravity-look-for-agent-skills-a703518d68c5
- https://www.aibuilderclub.com/blog/antigravity-cli-guide
- google-gemini/gemini-cli #27688, #27258
- https://www.mindstudio.ai/blog/gpt-5-5-agentic-coding-guide
- https://simonwillison.net/2025/Dec/12/openai-skills/
