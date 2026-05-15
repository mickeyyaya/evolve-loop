# How Evolve Loop Compares to Other Long-Running Claude Code Skills

> Honest head-to-head with the long-running autonomous-agent skills shipping in the Claude Code ecosystem as of 2026-05. Sources: WebSearch on 2026-05-15. Skip to [the comparison table](#comparison-table) if you just want the bottom line.
> Audience: people deciding whether evolve-loop is the right tool, or comparing it to what they already use.

## Table of Contents

1. [TL;DR](#tldr)
2. [Comparison Table](#comparison-table)
3. [/goal — Claude Code's Built-In Long-Running Mode](#goal--claude-codes-built-in-long-running-mode)
4. [miles990/self-evolving-agent](#miles990self-evolving-agent)
5. [alirezarezvani/self-improving-agent](#alirezarezvaniself-improving-agent)
6. [bejranonda/LLM-Autonomous-Agent-Plugin-for-Claude](#bejranondallm-autonomous-agent-plugin-for-claude)
7. [obra/superpowers](#obrasuperpowers)
8. [OpenClaw + Hermes Agent (ecosystem context)](#openclaw--hermes-agent-ecosystem-context)
9. [When to Choose Each](#when-to-choose-each)
10. [The Honest Tradeoffs](#the-honest-tradeoffs)
11. [Sources](#sources)

---

## TL;DR

Evolve Loop's distinctive value vs the other long-running skills:

| Dimension | The others (in aggregate) | Evolve Loop |
|---|---|---|
| Verdict source | LLM judge ("has goal been met?") | **Sandbox exit code from EGPS predicates** |
| Anti-gaming layers | 1 (validator LLM, MEMORY.md, or skill triggers) | **3 (structural / OS / workflow)** |
| Tamper resistance | None | **SHA-chained ledger** |
| Multi-CLI | Single Claude (sometimes Gemini-only) | **claude / gemini / codex with per-phase routing** |
| Adversarial auditor | None or same-model judge | **Different model family from Builder by default** |
| Failure → lesson | Mostly ad-hoc | **Reflexion-style YAML lessons → next-cycle Scout instinct** |
| Recovery from mid-run crash | Re-run | **Checkpoint-resume + worktree preservation** |

If your problem is "fix one bug autonomously and tell me when it's done," any of these will work. If your problem is "I want to trust the output enough to actually merge without re-reading every diff," evolve-loop's structural enforcement matters.

---

## Comparison Table

| Project | Loop pattern | Verdict source | Long-term memory | Multi-CLI | Recovery | Anti-gaming |
|---|---|---|---|---|---|---|
| **/goal** (Claude Code 2.1.139+) | Single Claude session iterates until validator says done | Small validator LLM (gameable) | Conversation only | No (Claude only) | None (single session) | None structural |
| **miles990/self-evolving-agent** | Skill triggers iterative learning | Pattern matching in MEMORY.md | MEMORY.md auto-memory | No | None | Skill-level only |
| **alirezarezvani/self-improving-agent** | Auto-memory → pattern promotion → rule enforcement | LLM analysis of MEMORY.md patterns | MEMORY.md + promoted rules | No | None | Pattern-promotion convention |
| **bejranonda/LLM-Autonomous-Agent-Plugin-for-Claude** | Autonomous skill with 40+ linters + CodeRabbit PR reviews | Linter exit codes + CodeRabbit verdict | Dashboard local DB | No | None | Linter-suite enforcement |
| **obra/superpowers** | Skills auto-trigger across hours-long sessions | Skill exit-criteria checks | Skills are reusable across sessions | No | None | Skill triggering is structural; verdicts still LLM |
| **OpenClaw / Hermes Agent** | Cross-vendor goal-mode adoption | Per-tool varies; mostly LLM judges | Per-tool varies | Sometimes | Per-tool varies | Convention-level |
| **Evolve Loop** | 8-phase pipeline w/ adversarial audit + EGPS predicates | **bash exit codes** (deterministic) | YAML lessons + state.json instincts (durable, cross-cycle) | **Yes, per-phase router** | **Checkpoint-resume + worktree preservation** | **3-tier: structural / OS / workflow** |

---

## /goal — Claude Code's Built-In Long-Running Mode

**Status:** Shipped in Claude Code 2.1.139 (May 12, 2026). Built into Anthropic's CLI; no plugin install required.

**How it works:** Operator types `/goal <high-level objective>`. A small fast validator model checks after every Claude turn whether the goal has been met. If no, Claude keeps going. If yes, the loop closes and Claude reports back. Can run for hours autonomously without operator approval per action.

**Strengths:**
- Zero install — ships with Claude Code
- Simple mental model — one prompt, one validator, one verdict
- Anthropic-curated; well-integrated with Claude Code UX (Agent View, Background Sessions)
- Industry adoption signal (Codex, Hermes, OpenClaw all adopting similar pattern)

**Weaknesses (vs evolve-loop):**

| Concern | /goal | Evolve Loop |
|---|---|---|
| **Validator is itself an LLM** — can be tricked by surface-level "looks done" | YES (gameable) | NO (bash exit codes; can't be tricked) |
| Tamper-evident execution log | None | Ledger SHA-chain with `prev_hash` |
| Multi-CLI routing | Single Claude | claude/gemini/codex per phase |
| Adversarial auditor | Same model judges itself | Different family auditor (Opus vs Builder's Sonnet) |
| Failure → lesson for next run | Ephemeral (conversation only) | Durable YAML lessons + state.json instincts |
| Mid-run quota wall | Lose the session | Checkpoint preserves worktree + state |

**Bottom line:** `/goal` is excellent for self-contained tasks where "looks done" is good enough. evolve-loop adds 3 layers of structural anti-gaming for cases where the cost of a wrong "looks done" is high (merging to main, production code, security-sensitive changes).

Sources: [explainx.ai 2026-05](https://explainx.ai/blog/claude-code-goal-command-long-running-agents-2026), [apidog.com](https://apidog.com/blog/goal-command-codex-claude-code-autonomous-agents/), [sitepoint.com](https://www.sitepoint.com/claude-code-as-an-autonomous-agent-advanced-workflows-2026/).

---

## miles990/self-evolving-agent

**Status:** GitHub: `miles990/self-evolving-agent`. Claude Code skill.

**How it works:** A Claude Code skill that enables autonomous goal achievement through iterative learning and self-improvement. Skill triggers iteration based on Claude Code's auto-memory system (v2.1.32+) that records project patterns to `MEMORY.md`. Pattern-promotion happens via the skill's analysis.

**Strengths:**
- Direct namespace competitor — same problem framing as evolve-loop
- Light footprint — single skill, no separate orchestration runtime
- Leverages Claude Code's built-in auto-memory

**Weaknesses (vs evolve-loop):**
- **No structural enforcement**: relies on the skill being invoked; bypass-able via `claude` without the skill
- **No predicate suite**: verdicts come from LLM analysis of memory patterns, not deterministic exit codes
- **No multi-CLI** — Claude only
- **No checkpoint-resume** — quota wall = work lost
- **No adversarial audit** — same-model self-judgment

**Bottom line:** A lighter-weight take on the same idea. Choose this if you want a skill (not a runtime) and trust the convention-based self-evolution. Choose evolve-loop if you need structural enforcement.

Source: [github.com/miles990/self-evolving-agent](https://github.com/miles990/self-evolving-agent).

---

## alirezarezvani/self-improving-agent

**Status:** GitHub: `alirezarezvani/claude-skills` (a 263+ skill marketplace). The `self-improving-agent` skill is a sub-skill.

**How it works:** Operates on Claude Code's auto-memory (`MEMORY.md`). Analyzes what Claude has learned across sessions, identifies recurring patterns, promotes proven patterns to enforced rules in CLAUDE.md, and extracts recurring solutions into reusable skills.

**Strengths:**
- Part of a broader 263+ skill ecosystem
- Pattern-promotion to **rules** (not just notes) is a clever lift — promoted patterns become things future Claude sessions are forced to honor
- Multi-tool support across the marketplace (Claude Code, Codex, Gemini CLI, Cursor, 8+ more)
- Lower-friction adoption than a full pipeline framework

**Weaknesses (vs evolve-loop):**
- **Pattern promotion happens periodically (not per-cycle)** — slower feedback loop
- **No phase structure** — it's a self-improvement skill layered on top of arbitrary Claude work, not a constrained pipeline
- **No tamper-evident ledger** — patterns and promoted rules live in markdown
- **No structural ship-gate** — promoted rules influence future sessions but don't block bad commits
- **No predicate suite** — verdicts are LLM analysis

**Bottom line:** Excellent if your workflow is "Claude Code over hours/weeks of unstructured work, and I want the framework to extract patterns I should follow." Less suited for "every commit must pass adversarial audit with cryptographic evidence."

Source: [github.com/alirezarezvani/claude-skills](https://github.com/alirezarezvani/claude-skills), [marketplace](https://alirezarezvani.github.io/claude-skills/skills/engineering-team/self-improving-agent/).

---

## bejranonda/LLM-Autonomous-Agent-Plugin-for-Claude

**Status:** GitHub: `bejranonda/LLM-Autonomous-Agent-Plugin-for-Claude`. Self-described as "Production-ready with 100% local processing, privacy-first."

**How it works:** Autonomous self-learning agent plugin. Integrates 40+ linters, OWASP security checks, and CodeRabbit PR reviews into the loop. Real-time dashboard for monitoring. Strong emphasis on local processing.

**Strengths:**
- **Linter-suite verdicts are real** — 40+ tools' exit codes give deterministic feedback (closest competitor to EGPS predicates)
- CodeRabbit integration for PR-level review
- Privacy-first (100% local)
- Production-ready claim with dashboard

**Weaknesses (vs evolve-loop):**
- **Linter coverage ≠ acceptance criteria** — linters check style/security/known-bug-patterns; they don't verify *functional* claims like "does this code actually solve the user's task?"
- **No phase structure** — linters run on a single Claude session; no Scout/Triage/Audit separation
- **No multi-CLI** — Claude only
- **No checkpoint-resume**
- **No Reflexion-style cross-cycle learning** — each invocation is independent

**Bottom line:** The closest competitor on the "deterministic verdict" axis (because linters return exit codes). Choose this for production-deployment security/style enforcement layered on Claude. Choose evolve-loop for end-to-end pipeline structure + cross-cycle learning.

Source: [github.com/bejranonda/LLM-Autonomous-Agent-Plugin-for-Claude](https://github.com/bejranonda/LLM-Autonomous-Agent-Plugin-for-Claude).

---

## obra/superpowers

**Status:** GitHub: `obra/superpowers`. Self-described as "An agentic skills framework & software development methodology that works."

**How it works:** Claude can work autonomously for hours at a time without deviating from the plan because the skills trigger automatically. The framework's value proposition: skills with proper exit-criteria force the model to honor scope and pause when scope is unclear.

**Strengths:**
- **Skills as exit-criteria** is a structural insight — close to evolve-loop's "skill = workflow + steps + exit criteria" model
- Mature framework with active community
- Lighter weight than evolve-loop (skills only; no separate runtime)
- Author's blog posts are influential in the broader agent-design discussion

**Weaknesses (vs evolve-loop):**
- **Skills are the only structural layer** — no shell-hook enforcement below the skill layer
- **No EGPS predicate suite** — exit-criteria are described in skill prose; not bash-verifiable
- **No multi-CLI router** — Claude only
- **No tamper-evident ledger**
- **No checkpoint-resume**
- **No Reflexion-style failure → lesson loop** — skills are reusable but don't generate new skills from failures

**Bottom line:** Evolve Loop's design owes a lot to superpowers — the Skill/Persona/Command tri-layer is directly inspired (see [tri-layer.md](../architecture/tri-layer.md) "addyosmani/agent-skills" attribution). evolve-loop is "superpowers + structural enforcement + multi-CLI + Reflexion-style learning." If you don't need the enforcement layer, superpowers may be sufficient — and is lighter.

Source: [github.com/obra/superpowers](https://github.com/obra/superpowers).

---

## OpenClaw + Hermes Agent (ecosystem context)

**Status:** OpenClaw (cited as 247k GitHub stars per ExplainX 2026-05; fastest-growing OSS in GitHub history). Hermes Agent (Nous Research). Both cross-vendor open-source agent frameworks adopting the `/goal` pattern in 2026.

**How they work:** Generic agent frameworks supporting multiple LLM providers. Goal-mode adoption (cross-pollinated from `/goal`) means operators set high-level goals and the agent loops until done. Tool ecosystems vary widely.

**Strengths:**
- **Vendor neutrality** — work across Claude / GPT / Gemini / open models
- **Massive ecosystem** — plugins, tools, integrations
- **Active development** — fastest-growing OSS gives strong signal
- **Multi-CLI by design** (closest ecosystem match to evolve-loop's CLI router)

**Weaknesses (vs evolve-loop, specifically as goal-mode-adopters):**
- **Goal-mode varies in rigor** per-tool — some implementations are validator-LLM-based, others have stricter exit-criteria
- **Cross-vendor breadth comes at the cost of vendor-specific depth** — evolve-loop's Anthropic-deep integration (`--allowedTools`, `--max-budget-usd`, sandbox-exec) requires per-vendor adapter work
- **No common predicate format** across the ecosystem — each adopter implements verdict logic differently

**Bottom line:** If you need vendor neutrality at the cost of per-vendor depth, OpenClaw or Hermes Agent are the right framework choices. evolve-loop is going the other way: deep Anthropic-native (with gemini/codex as routable peers via adapter) rather than broad multi-vendor.

Source: [explainx.ai 2026-05](https://explainx.ai/blog/goal-mode-ai-agents-complete-guide-2026), [github.com/0xNyk/awesome-hermes-agent](https://github.com/0xNyk/awesome-hermes-agent).

---

## When to Choose Each

| Your situation | Choose |
|---|---|
| "I want autonomous work on a personal project, lowest friction" | **/goal** (zero install) |
| "I run Claude Code over hours and want it to learn my codebase patterns" | **alirezarezvani/self-improving-agent** |
| "I need linter + security + style enforcement on every Claude commit" | **bejranonda/LLM-Autonomous-Agent-Plugin** |
| "I want a skills-first framework that pauses cleanly on scope ambiguity" | **obra/superpowers** |
| "I want vendor neutrality across many LLM providers" | **OpenClaw / Hermes Agent** |
| "I need cross-cycle learning, structural anti-gaming, and tamper-evident audit logs for production code" | **evolve-loop** |
| "I want adversarial cross-CLI review (Builder=Sonnet, Auditor=Opus, Scout=Gemini)" | **evolve-loop** |
| "I want to ship one well-tested commit per hour over a long unattended run with quota-wall recovery" | **evolve-loop** |
| "I want a simple `/goal "do thing"` and a 5-second mental model" | **/goal** |

---

## The Honest Tradeoffs

Evolve Loop is **not always the right choice**. The tradeoffs:

**Higher friction.** Eight phases per cycle (Calibrate → Intent → Scout → Triage → Plan-Review → Build → Audit → Ship → Memo/Retro) means more wall-clock time and more cost per cycle vs `/goal`'s "Claude iterates until done." Typical evolve-loop cycle: 10-30 minutes + $0.50-3.00. `/goal` for the same task: 3-10 minutes + $0.30-1.50.

**Higher learning curve.** Operators need to understand the trust kernel, EGPS predicates, the CLI router, recovery mechanisms. `/goal` is "type `/goal`, wait, done."

**Anthropic-deep, not vendor-neutral.** Adapters for gemini and codex exist (and the router treats them as peers), but the kernel hooks (phase-gate, role-gate, ship-gate) are bash scripts that assume Anthropic-CLI-style permission flags. Porting to a non-Anthropic ecosystem requires adapter work.

**Optimized for trust, not speed.** If your goal is "fastest possible autonomous coding," `/goal` wins. If your goal is "this commit is safe to merge without manual review," evolve-loop's 3-tier enforcement is the differentiator.

**Best fit:** organizations or solo developers running long unattended cycles on production code, where the cost of a bad merge is high. Or research labs studying agent safety and reward hacking (the project's own forensic incidents like cycle-61 are designed to be studied — see [`../incidents/cycle-61.md`](../incidents/cycle-61.md)).

---

## Sources

All comparisons here are sourced from WebSearch on 2026-05-15. Specific URLs:

- [Claude Code 2.1.139 /goal command — explainx.ai](https://explainx.ai/blog/claude-code-goal-command-long-running-agents-2026)
- [Goal mode adoption across vendors — explainx.ai](https://explainx.ai/blog/goal-mode-ai-agents-complete-guide-2026)
- [/goal command technical breakdown — apidog.com](https://apidog.com/blog/goal-command-codex-claude-code-autonomous-agents/)
- [/goal autonomous tasks explained — MindStudio](https://www.mindstudio.ai/blog/claude-code-goal-command-autonomous-tasks)
- [Claude Code as Autonomous Agent — sitepoint.com](https://www.sitepoint.com/claude-code-as-an-autonomous-agent-advanced-workflows-2026/)
- [miles990/self-evolving-agent](https://github.com/miles990/self-evolving-agent)
- [alirezarezvani/claude-skills](https://github.com/alirezarezvani/claude-skills)
- [Self-Improving Agent skill detail](https://alirezarezvani.github.io/claude-skills/skills/engineering-team/self-improving-agent/)
- [bejranonda/LLM-Autonomous-Agent-Plugin-for-Claude](https://github.com/bejranonda/LLM-Autonomous-Agent-Plugin-for-Claude)
- [obra/superpowers](https://github.com/obra/superpowers)
- [awesome-hermes-agent — Nous Research](https://github.com/0xNyk/awesome-hermes-agent)
- [Recursive Self-Improvement — David R Oliver / Medium](https://medium.com/@davidroliver/recursive-self-improvement-building-a-self-improving-agent-with-claude-code-d2d2ae941282)
- [AddyOsmani — Self-Improving Coding Agents](https://addyosmani.com/blog/self-improving-agents/)

Comparison claims are best-effort based on each project's public documentation. Where a claim is uncertain, it's labeled as such. File an issue if a competitor's claim here is wrong.
