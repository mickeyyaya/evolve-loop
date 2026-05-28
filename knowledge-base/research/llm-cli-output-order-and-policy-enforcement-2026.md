> Research dossier (2026-05-28): how to make an LLM CLI follow an EXACT output order and adhere to the policies we built. Synthesized from current (2025–2026) sources; mapped to evolve-loop's mechanisms and the cycle-119 open issues. Archived here per the knowledge-base stewardship policy.

# Making an LLM CLI Follow Exact Output Order + Built Policy

## TL;DR

There is no single knob. Reliability comes from a **layered enforcement stack**, strongest at the bottom. The governing principle (validated repeatedly in 2026 sources): **prompt-level instructions are probabilistic suggestions; only sub-model layers are enforcement.** "The model does not follow your instructions because it is enforced to. It follows them because that response is statistically likely." So: use prompts to *steer*, but enforce *order* at decode-time and *policy* at the runtime/orchestration layer — never trust the prompt alone.

This is exactly the cycle-119 lesson: the trust kernel lived in Claude-only prompt-time hooks (Layer 3 via one CLI) and the artifact contract used cwd-relative paths (Layer 4 bug). Both broke because the invariant wasn't enforced at a layer that holds for every CLI.

## Contents
- [The four enforcement layers](#the-four-enforcement-layers)
- [Layer 1 — In-context steering](#layer-1)
- [Layer 2 — Decode-time output order/format](#layer-2)
- [Layer 3 — Runtime policy enforcement](#layer-3)
- [Layer 4 — Orchestration + completion](#layer-4)
- [Mapping to evolve-loop + recommendations](#mapping)
- [Sources](#sources)

## The four enforcement layers

| Layer | Controls | Technique | Enforcement strength | Cross-CLI portable? |
|---|---|---|---|---|
| 1. In-context (prompt) | steering, soft policy | instruction hierarchy, `IMPORTANT`/`YOU MUST`, XML tags, numbered steps, reasoning-first | **probabilistic** (suggestion) | yes (but weak) |
| 2. Decode-time | exact output ORDER + format | constrained decoding / strict structured outputs (XGrammar, `strict:true`), JSON-schema, grammar | **strong** for shape | only on CLIs that expose it → else wrap with validate+retry |
| 3. Runtime (tool-call gate) | POLICY (what may execute) | intercept tool call before exec; code policy `permit`/`deny`/`defer`; "model gets no vote" | **deterministic** | only if the gate is CLI-agnostic (the cycle-119 gap) |
| 4. Orchestration / state | phase ORDER, completion, isolation | single authoritative state, atomic node-boundary writes, deterministic transitions, context isolation, completion = typed-output + no-tool-call | **deterministic** | yes (lives outside the model) |

**Rule of thumb:** *order* and *format* → Layer 2; *policy/authorization* → Layer 3; *sequence of phases + done-detection* → Layer 4. Layer 1 only biases; it never guarantees.

## Layer 1 — In-context steering {#layer-1}

- **Instruction hierarchy** (OpenAI/industry 2026): rank instruction sources (platform/system > developer > user > tool output) and train/prompt the model to prioritize the most trusted. Updates to the system prompt then yield "controllable and robust" behavior changes. Put built policy at the **system** level, untrusted content (tool output, scraped text) explicitly lowest.
- **Form matters:** unambiguous directives; well-formed JSON/XML tags, headings, numbered lists improve consistency; break complex tasks into smaller explicit steps; separate context/task/examples with tags.
- **Emphasis + brevity:** `IMPORTANT`/`YOU MUST` raises adherence; but if a rule keeps being missed, the prompt is **too long** and the rule is getting lost — shorten, don't add more.
- **Reasoning-first ordering:** because models generate left-to-right, **field order = prompt order**. Put reasoning/scratch fields *before* the committed answer so the model works through the problem first.
- **Ceiling:** prompting alone caps out; injection (>90% success), multi-step drift, and model updates all defeat prompt rules. Treat Layer 1 as necessary-not-sufficient.

## Layer 2 — Decode-time output order/format {#layer-2}

- **Constrained decoding / structured outputs** are the way to *guarantee* output shape and field order. Every major provider ships it (OpenAI Strict Mode since 2024; all majors by 2025–26). Self-hosted: **XGrammar** is the default backend for vLLM/SGLang/TensorRT-LLM (<40µs/token, near-zero overhead); alternatives: Guidance, Outlines.
- **Strict tool schemas:** set `strict: true` on tool definitions so calls match the schema exactly; give tool/schema definitions the same prompt-engineering care as the main prompt.
- **Validate-and-retry** is the portable fallback when a CLI lacks native constrained decoding: parse → on schema failure, feed the validation error back for one fix. **If a prompt needs 2+ retries consistently, fix the prompt/schema, not the retry count.**
- **Emerging:** ATLAS-RTC applies graduated, stage-aware runtime interventions (logit bias, temp modulation, token masking) via a closed-loop controller to enforce *stateful* output contracts — i.e., the allowed output changes per stage. Conceptually close to a per-phase output contract.

## Layer 3 — Runtime policy enforcement {#layer-3}

The strongest, most-cited 2026 guidance for *policy*: **enforce below the model, at the tool-call execution boundary**, not in the prompt.
- There is a moment between the agent *deciding* to call a tool and the tool *running*. A runtime layer intercepts there, checks the call against **code-defined** policy, and deterministically `permit` / `deny` / `defer` (human-in-loop). "The model does not get a vote."
- Policy expressed as code/DSL with **compile-time guarantees** (e.g., a `deny!` cannot be overridden by a later `permit`; child policies cannot widen a parent's deny). Pattern:
  ```
  deny!  shell/*
  defer  stripe/refund when amount > 500
  permit stripe/*       when amount <= 500
  ```
- **The agentic blind spot:** orgs enforce guardrails on the chat surface but NOT on MCP calls, tool invocations, or agent-to-agent messages. Hard boundaries must sit at the runtime/infra layer where "no prompt, jailbreak, or reasoning trick can alter them."
- **Soft vs hard:** soft guardrails (prompt) for UX nudges; hard boundaries (runtime) for anything that must hold. Layer them; never let a soft guardrail stand in for a hard one.
- **Strongest tier (research):** treat each proposed action as a conjecture — execute iff a formal/cryptographic check proves it satisfies pre-compiled policy axioms (Lean4-checked compliance; authenticated/cryptographically-signed workflows at every boundary crossing).

## Layer 4 — Orchestration + completion {#layer-4}

- **Single authoritative state, atomic at node boundaries:** LangGraph-style — one state object per run; updates applied atomically at node boundaries to prevent concurrent-write conflicts; on parallel-branch convergence, **deterministically merge** by predefined transition rules → reproducible state evolution. Deterministic multi-agent orchestration shows **zero quality variance** across trials (production-SLA-able); the value is determinism, not speed.
- **Context isolation:** make it agent-triggered, asymmetric, deterministic → eliminates cross-agent contamination without compression/retrieval.
- **Completion detection** (the cycle-119 sibling problem): the robust signal is **"final output = typed output of the expected type AND no pending tool calls."** Don't infer "done" from wall-clock or pane-idle alone. For streaming/REPL, combine a rule-based signal (expected artifact present / typed result parsed / no tool call) with a probabilistic EOT fallback; prefer an **explicit machine sentinel** the agent must emit, validated structurally.
- **Phase order** belongs in the orchestrator's state machine (legal-transition table), not in the agent's discretion — the agent proposes, the kernel disposes.

## Mapping to evolve-loop + recommendations {#mapping}

| Best practice | evolve-loop today | Gap / recommendation |
|---|---|---|
| L1 instruction hierarchy + system-level policy | `EVOLVE_INTERACTIVE_POLICY` block + `EVOLVE_<AGENT>_SYSTEM_PROMPT` prepended as `## Rules` | Keep. Ensure policy sits at **system** level for every CLI; keep blocks <200 tokens (already done); never treat as enforcement. |
| L2 strict structured output for machine-read phase outputs | advisor uses a stdout JSON contract (ADR-0027 PR1); reports are prose+anchors | For every output the orchestrator *parses* (advisor plan, verdicts, triage top_n), enforce a JSON schema; on CLIs without strict mode, add a **bridge-level validate+retry** (one fix pass) before accepting the artifact. Use reasoning-first field order. |
| L3 runtime policy at the tool-call boundary, CLI-agnostic | trust kernel = **Claude-only PreToolUse hooks** (role/phase/ship gate) | **Issue #2 (the headline gap).** Non-Claude drivers (agy/Gemini, codex) execute no Claude hooks → unguarded writes (Gemini scout wrote to `main`). Move enforcement to the **CLI-agnostic bridge/runner + OS sandbox**: confine source-writing phases to the worktree cwd via `sandbox-exec`/`bwrap` write-scope for *every* driver; post-phase, diff the main tree and reject out-of-scope source changes regardless of CLI. Until then, restrict source-writing phases to Claude drivers or an explicit trusted-CLI allowlist. |
| L3 "enabled but cannot run → fail loud" | `EVOLVE_PLAN_REVIEW=1` is a silent no-op under static routing | **Issue #3.** Emit an `inert-phase-enable` warning in `config.Load` when an enabled phase can never be reached by the active routing stage. |
| L4 deterministic state machine + atomic boundaries | Go orchestrator state machine + `cycle-state.json` atomic writes | Healthy. Keep the agent-proposes / kernel-disposes split (ADR-0024). |
| L4 cwd-independent contracts | artifact path was cwd-relative across the worktree boundary | **Issue 1 (FIXED `80f4206`):** absolutize `projectRoot`/`evolveDir` at the composition root. Generalize: any value that crosses a process/cwd boundary must be absolute. |
| L4 completion = typed output + no tool call | completion contracts: `artifact` / `stdout` / `git-evidence` (ADR-0026/0027) | Strong direction. Prefer an explicit machine sentinel / typed-artifact-present check over pane-idle; the `git-evidence` contract (HEAD-advance + verified trailer) is the most robust — extend it to more phases. |

### Priority recommendations (for the open issues)

1. **Cross-CLI enforcement (Issue 2, highest):** relocate the trust boundary from Claude hooks to the bridge/runner + OS sandbox so it is identical for Claude/Gemini/Codex. This is the single highest-leverage change and matches the 2026 consensus (enforce below the model, at the execution boundary, where no prompt/CLI choice can bypass it).
2. **Schema-enforce + validate-retry every parsed phase output** at the bridge, so a non-strict CLI can't silently emit malformed/mis-ordered output.
3. **Fail-loud on inert config (Issue 3).**
4. **Prefer machine sentinels / typed-artifact completion** over time/idle heuristics.

## Sources {#sources}

- [Your agent's guardrails are suggestions, not enforcement (DEV)](https://dev.to/brianrhall/your-agents-guardrails-are-suggestions-not-enforcement-2c8k)
- [Building Secured Agents: Soft Guardrails, Hard Boundaries (Habler, Medium)](https://idanhabler.medium.com/building-safer-agents-soft-guardrails-hard-boundaries-and-the-layers-between-14205d709b93)
- [How to Design Guardrails for Secure and Scalable AI Agents (AppSecEngineer)](https://www.appsecengineer.com/blog/how-to-design-guardrails-for-secure-and-scalable-ai-agents)
- [Type-Checked Compliance: Deterministic Guardrails via Lean 4 (arXiv 2604.01483)](https://arxiv.org/pdf/2604.01483)
- [Authenticated Workflows: Protecting Agentic AI (arXiv 2602.10465)](https://arxiv.org/pdf/2602.10465)
- [LlamaFirewall: open-source guardrails (arXiv 2505.03574)](https://arxiv.org/pdf/2505.03574)
- [LLM Structured Outputs: Schema Validation for Real Pipelines (Wilkins)](https://collinwilkins.com/articles/structured-output)
- [How Structured Outputs and Constrained Decoding Work (Let's Data Science)](https://letsdatascience.com/blog/structured-outputs-making-llms-return-reliable-json)
- [ATLAS-RTC: Token-Level Runtime Control (arXiv 2603.27905)](https://arxiv.org/pdf/2603.27905)
- [Instruction Hierarchy in LLMs 2026 Guide (gend.co)](https://www.gend.co/blog/instruction-hierarchy-llms-safety)
- [Improving instruction hierarchy in frontier LLMs (OpenAI)](https://openai.com/index/instruction-hierarchy-challenge/)
- [RNR: Teaching LLMs to Follow Roles and Rules (arXiv 2409.13733)](https://arxiv.org/pdf/2409.13733)
- [Reasoning Up the Instruction Ladder for Controllable LMs (arXiv 2511.04694)](https://arxiv.org/pdf/2511.04694)
- [Multi-Agent LLM Orchestration: Deterministic Decision Support (arXiv 2511.15755)](https://arxiv.org/abs/2511.15755)
- [Tool use with Claude — Strict tool use (Anthropic Docs)](https://docs.anthropic.com/en/docs/agents-and-tools/tool-use/overview)
- [Best practices for Claude Code (Anthropic)](https://www.anthropic.com/engineering/claude-code-best-practices)
- [Running agents — OpenAI Agents SDK (final-output = typed + no tool call)](https://openai.github.io/openai-agents-python/running_agents/)
