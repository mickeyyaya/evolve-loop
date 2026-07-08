# Token-Saving Mechanisms in OSS Agent Frameworks — Concurrent-Fleet Applicability Survey

> Researched 2026-07-07 for evolve-loop (tmux-CLI phase pipeline, ~12 fresh-boot LLM sessions/cycle × 2+ lanes).
> Constraint frame: **we cannot change the provider API; we control prompts, session lifecycle, artifacts, and scheduling.**
> Category tags: **[C]**ontext-window mgmt · **[S]**ession reuse vs fresh-boot · **[K]**shared knowledge across agents · **[B]**udget enforcement · **[P]**rompt-cache exploitation.
> Complements the paper-side survey in `knowledge-base/research/token-optimization-2026/` (this file = implemented OSS mechanisms only).

## Quick matrix

| # | Project | C | S | K | B | P | Headline number |
|---|---------|---|---|---|---|---|-----------------|
| 1 | SWE-agent | ✅ | | | ✅ | ✅ | masking −52.7% cost, better solve rate |
| 2 | mini-swe-agent | ✅ | | | ✅ | ✅ | 74% SWE-bench with append-only history |
| 3 | aider | ✅ | | ✅ | ✅ | ✅ | 1K-token repo map default; 5-min cache keepalive |
| 4 | OpenHands | ✅ | ✅ | ✅ | ✅ | ✅ | condenser: ≤½ per-turn cost, quadratic→linear scaling |
| 5 | LangGraph + LangMem | ✅ | ✅ | ✅ | ✅ | | incremental running_summary |
| 6 | AutoGen / AG2 | ✅ | | | ✅ | | 4019→215 tokens in transform demo |
| 7 | CrewAI | ✅ | | ✅ | ✅ | | respect_context_window default-on |
| 8 | Letta (MemGPT) | ✅ | ✅ | ✅ | | | sleep-time compute; shared memory blocks |
| 9 | Roo Code | ✅ | | | ✅ | | default-on condensing; 70/30 context reservation |
| 10 | opencode (+DCP plugin) | ✅ | ✅ | | ✅ | ✅ | tool-output pruning plugin ecosystem |
| 11 | claude-flow / ruflo | ✅ | ✅ | ✅ | | | claims 75–80% reduction (unverified) |
| 12 | RTK (rtk-ai/rtk) | ✅ | | | | | avg 89% CLI-output noise removed |
| 13 | OpenAI Agents SDK | ✅ | ✅ | | | ✅ | nest_handoff_history transcript collapse |
| 14 | DSPy | | | ✅ | ✅ | ✅ | compiled prompts persisted + LM-call disk cache |
| 15 | Claude Code native | ✅ | ✅ | ✅ | ✅ | ✅ | context-editing −84% on 100-turn workflows |

---

## 1. SWE-agent — deterministic observation masking + hard cost caps

- **Repo:** [SWE-agent/SWE-agent](https://github.com/SWE-agent/SWE-agent) (~17k★, active; NeurIPS 2024, maintained through 2026)
- **[C] History processors** (`sweagent/agent/history_processors.py`, wired in `sweagent/agent/agents.py`): the classic `LastNObservations` processor elides all but the last N (default 5) environment observations, replacing each with one line: `"Old environment output: (n lines omitted)"`. Actions and reasoning are kept — only tool/environment *outputs* age out. Observations can be tagged `keep_output` (never elided) or force-elide via tags. Docs: [history processor config](https://swe-agent.com/latest/reference/history_processor_config/).
- **[B] Budget enforcement** (`sweagent/agent/models.py`, [model config](https://swe-agent.com/latest/reference/model_config/)): `per_instance_cost_limit` (default ~$3), `total_cost_limit` (whole batch), `per_instance_call_limit` (turn count when cost untrackable). Exceeding → agent stops gracefully and submits what it has, instead of dying mid-flight.
- **[P]** Masking is *suffix-only mutation in a stable transcript* — the prefix stays byte-stable so provider prompt caches keep hitting (contrast with summarization, which rewrites the prefix and invalidates cache).
- **Published numbers:** the JetBrains "Complexity Trap" study (arXiv 2508.21433) benchmarked exactly this style of masking: **−52.7% cost with BETTER solve rate than LLM summarization.**
- **Applicability:** highest-value pattern for evolve-loop. The tmux driver sees the whole transcript; a Go-side equivalent = per-phase Claude Code hook (PostToolUse) or RTK-style command wrapping that truncates stale tool output. Also: enforce `per_phase_cost_limit` + `per_phase_call_limit` in the orchestrator with graceful submit — matches finding #7 of the internal survey.

## 2. mini-swe-agent — append-only history as a cache discipline

- **Repo:** [SWE-agent/mini-swe-agent](https://github.com/SWE-agent/mini-swe-agent) (~74% SWE-bench verified from a ~100-line agent class; same Princeton/Stanford team)
- **[C][P] Linear, append-only message history:** no tools, no state machine — every step appends `(action, observation)` to one list. Deliberate design goals: (a) trivially debuggable trajectories, (b) **maximal provider prompt-cache hits** because nothing earlier in the transcript ever mutates, (c) fine-tuning-ready logs.
- **[B]** Same cost/step limits as SWE-agent, in a `LitellmModel` wrapper.
- **Applicability:** proof that *not touching the prefix* is itself an optimization. For evolve-loop: audit phase prompts for anything that rewrites/reorders earlier content mid-session (status re-injections, edited reminders); prefer appending. Cache-stable-prefix (internal item A2) is validated by a 74%-scoring production agent.

## 3. aider — token-budgeted repo map + cache keepalive pings

- **Repo:** [Aider-AI/aider](https://github.com/Aider-AI/aider) (~38k★, active)
- **[K] Repo map** (`aider/repomap.py`): tree-sitter extracts defs/refs per file (`tags.scm` per language); builds a symbol graph; **personalized PageRank** (NetworkX) ranks files by relevance to the current chat; then a **binary search packs the highest-ranked signatures into a hard token budget** — `--map-tokens`, default **1024 tokens** (docs: [repomap](https://aider.chat/docs/repomap.html)). The map replaces agent self-exploration: the model sees the right 1K tokens of repo skeleton instead of burning 20K+ on `ls`/`grep`/`cat` turns.
- **[K] Persistent exploration cache:** parsed tags live in `.aider.tags.cache.v4` (diskcache/SQLite), keyed by absolute path + mtime — repo analysis is paid **once per file change, not once per session**. `--map-refresh auto|always|files|manual` controls recompute.
- **[P] Prompt caching** ([caching docs](https://aider.chat/docs/usage/caching.html)): `--cache-prompts` orders the transcript so that **system prompt → read-only files → repo map → editable files** form the stable cached prefix. `--cache-keepalive-pings N`: aider pings the provider **every ~5 minutes up to N times** after each user message to keep Anthropic's 5-min-TTL cache warm across think-time gaps.
- **[B]** Map obeys its budget dynamically (shrinks when chat files already cover the context).
- **Applicability:** the single best fit for category K. evolve-loop lanes re-explore the repo ~12×/cycle × 2 lanes; a Go-generated repo-map artifact (tree-sitter or `gopls`-based symbol ranking, hard 1–2K token budget) injected into every phase prompt would amortize exploration exactly like aider. Cache keepalive maps to fleet scheduling: **schedule lane launches within the cache TTL window** so lane 2 lands on lane 1's warm prefix.

## 4. OpenHands — condenser pipeline, event-sourced masking, microagents

- **Repo:** [OpenHands/OpenHands](https://github.com/OpenHands/OpenHands) (~65k★, very active)
- **[C] Condenser framework** (`openhands.sdk.context.condenser`; earlier `openhands/memory/condenser/`; introduced [PR #5306](https://github.com/OpenHands/OpenHands/pull/5306)): pluggable `condense(history) → view`. `LLMSummarizingCondenser(max_size=N, keep_first=K)` keeps the first K events (system prompt) + most recent events, LLM-summarizes the middle. **`CondensationAction`** ([PR #7578](https://github.com/OpenHands/OpenHands/pull/7578)) is the key architectural move: truncation is recorded as an *event* saying which event-IDs to drop — the underlying history is never destroyed, only the **view** sent to the LLM changes (masking as event-sourcing; rewind-safe). A dedicated condenser exists just for **browser output observations** ([PR #6578](https://github.com/OpenHands/OpenHands/pull/6578)) — bulky tool classes get their own eviction policy.
- **Published numbers** ([blog](https://www.openhands.dev/blog/openhands-context-condensensation-for-more-efficient-ai-agents), [SDK guide](https://docs.openhands.dev/sdk/guides/context-condenser)): per-turn cost scaling goes **quadratic → linear**; settles at **less than half the per-turn cost**, up to 2× per-turn API cost reduction, equal-or-better SWE task performance.
- **[K][S] Microagents / Skills** (`.openhands/microagents/*.md`): markdown files with YAML frontmatter, keyword-triggered — repo conventions and gotchas load *only when relevant*, so knowledge is amortized across sessions without bloating every prompt.
- **[B]** `max_budget_per_task` config kills a task at a dollar cap. **[P]** LiteLLM layer sets Anthropic `cache_control` breakpoints (simplification tracked in [#6858](https://github.com/OpenHands/OpenHands/issues/6858)).
- **Applicability:** the event-sourced masking design translates directly: evolve-loop artifacts stay full-fidelity on disk; only the **view** handed to the next phase is pruned (internal B2/B3). Keyword-triggered knowledge files = a cheap upgrade to knowledge-base injection (scout injects only trigger-matched notes).

## 5. LangGraph + LangMem — incremental summarization, checkpoint resume, cross-thread store

- **Repos:** [langchain-ai/langgraph](https://github.com/langchain-ai/langgraph) (~20k★), [langchain-ai/langmem](https://github.com/langchain-ai/langmem)
- **[C]** `trim_messages` (token-counter-based, strategy="last") and **`SummarizationNode`** (`langmem/short_term/summarization.py`): parameters `max_tokens`, `max_tokens_before_summary`, `max_summary_tokens`. Crucially it maintains a **`running_summary`** object — on re-trigger it summarizes *only messages not previously summarized* and folds them into the existing summary, instead of re-summarizing the whole transcript (incremental compaction; avoids paying O(n) summarize cost repeatedly). Docs: [short-term memory reference](https://langchain-ai.github.io/langmem/reference/short_term/).
- **[S] Checkpointer** (`langgraph.checkpoint.*`: `InMemorySaver`, `SqliteSaver`, `PostgresSaver`): full graph state persisted per-thread per-superstep; resume/fork ("time travel") means a crashed or paused pipeline **re-enters at the failed node with its accumulated state instead of re-running earlier nodes**.
- **[K] Store API** (`BaseStore`, cross-thread memory): namespaced key-value/vector memory readable by all threads → concurrent graph branches share discovered facts rather than re-deriving them.
- **[B]** `recursion_limit` hard-caps supersteps per run.
- **Applicability:** the checkpointer is the OSS blueprint for evolve-loop `--resume` at *phase* granularity (already exists) — the gap is LangMem-style incremental summaries: memo/retro should maintain a running cycle-summary that folds in deltas, not re-digest history each cycle.

## 6. AutoGen / AG2 — composable message transforms with per-message token caps

- **Repos:** [microsoft/autogen](https://github.com/microsoft/autogen) (~50k★), [ag2ai/ag2](https://github.com/ag2ai/ag2) (community fork, active)
- **[C] `TransformMessages` capability** (`autogen/agentchat/contrib/capabilities/transforms.py`) — composable pre-send pipeline applied to *every* LLM call, including groupchat speaker selection:
  - `MessageHistoryLimiter(max_messages=N)` — keep last N messages.
  - `MessageTokenLimiter(max_tokens_per_message=A, max_tokens=B, min_tokens=C)` — **per-message truncation cap** (the mechanism most frameworks miss: one 30K-token test log gets clipped even if history is short) plus whole-context cap; `min_tokens` avoids degenerate over-trimming.
  - `TextMessageCompressor` — LLMLingua-backed compression with its own result caching.
  - Docs: [handling long contexts](https://microsoft.github.io/autogen/0.2/docs/topics/handling_long_contexts/intro_to_transform_messages/).
- **Published numbers:** docs demo shows a context going **4019 → 215 tokens** after transforms.
- **Applicability:** *per-message* caps are the right abstraction for tool-output flooding — evolve-loop equivalent: cap any single tool result at N tokens in the driver/hook layer (head+tail with elision marker), independent of total context length.

## 7. CrewAI — default-on context guard + tool-result cache

- **Repo:** [crewAIInc/crewAI](https://github.com/crewAIInc/crewAI) (~40k★, active)
- **[C]** `respect_context_window=True` (default): on `ContextWindowExceededError` the framework auto-summarizes history and retries instead of crashing — a *recovery* path, not a proactive optimizer ([agents docs](https://docs.crewai.com/en/concepts/agents)).
- **[K] Tool-result caching:** `cache=True` on agents/tools memoizes tool calls by (tool, args) across a crew run — repeated identical calls by different agents in the same crew are free.
- **[B]** `max_iter` (turn cap per agent), `max_rpm` (rate cap; scheduling lever under provider benching), `max_execution_time`; `crew.usage_metrics` aggregates prompt/completion tokens per run for attribution.
- **Applicability:** modest code-level novelty, but two transferable ideas: (a) tool-call memoization keyed on (cmd,args) within a cycle — e.g. two phases both running `go test ./...` unchanged should reuse the artifact; (b) `max_rpm` as a first-class per-agent knob is what fleet scheduling needs during rate-benching.

## 8. Letta (MemGPT) — memory blocks, shared blocks, sleep-time compute

- **Repo:** [letta-ai/letta](https://github.com/letta-ai/letta) (~18k★, active; MemGPT lineage)
- **[C] Memory blocks:** context = labeled, **size-capped** segments (`persona`, `human`, custom) that the agent itself edits via `memory_replace`/`memory_insert` tools; overflow pages out to recall/archival storage (vector DB) retrieved on demand — the OS-paging metaphor implemented ([memory blocks blog](https://www.letta.com/blog/memory-blocks/)).
- **[K] Shared blocks across agents:** a block is a first-class DB object attachable to **multiple agents by block_id** — write once, every attached agent sees the update. This is the cleanest OSS implementation of "concurrent agents share a live knowledge segment."
- **[S] Server-side persistence:** agents are durable objects (Postgres/SQLite); a "session" never re-boots — new work is a new message to an existing agent whose context is already assembled.
- **[C] Sleep-time compute** ([blog](https://www.letta.com/blog/sleep-time-compute/), arXiv 2504.13171): a background agent runs during idle time to reorganize/compress the primary agent's memory blocks. Published: **~5× test-time token reduction** for comparable accuracy (paper), +13–18% accuracy in some settings.
- **Applicability:** shared blocks ≈ a size-capped, single-writer `fleet-brief.md` artifact injected into every lane's prompt and updated by the orchestrator between phases. Sleep-time compute ≈ running memo/retro consolidation **between cycles on idle lanes** (scheduling-only change) so scout boots into a pre-digested brief.

## 9. Roo Code — production condensing thresholds + context reservation math

- **Repo:** [RooCodeInc/Roo-Code](https://github.com/RooCodeInc/Roo-Code) (~20k★, very active; VS Code agent, Cline fork)
- **[C] Intelligent Context Condensing** (default-on since v3.19, [docs](https://docs.roocode.com/features/intelligent-context-condensing)): at a configurable **threshold % of the context window** (per-API-config profiles), older messages are LLM-summarized — optionally using a **cheaper model than the main agent** for the summarization call itself. Manual trigger + per-profile thresholds.
- **[C] Reservation math:** 30% of the window is always reserved (20% output + 10% safety), 70% usable for history — condensing triggers against the 70%, not the raw window (prevents overflow-crash → expensive retry loops).
- **[C] Error-triggered fallback:** on provider context-window errors (detected per provider), auto-cut context by 25% and retry.
- **[C] Non-destructive:** condensed/truncated messages are preserved internally; checkpoint rewind restores full history.
- **Applicability:** the *threshold-and-reserve* arithmetic is directly portable to tmux drivers that can estimate transcript tokens: trigger phase-level compaction (or graceful phase wrap-up) at 70% of window; never let a phase die on overflow, because the retry re-pays the entire context.

## 10. opencode + dynamic-context-pruning — pluggable pruning in a CLI agent

- **Repos:** [sst/opencode](https://github.com/sst/opencode) (~30k★+, very active) · plugin [Opencode-DCP/opencode-dynamic-context-pruning](https://github.com/Opencode-DCP/opencode-dynamic-context-pruning)
- **[C]** Core: overflow detection via `MessageV2.Assistant.tokens` + `isOverflow`; auto-compaction **using a smaller model** to summarize; tool-output pruning strategies (see DeepWiki: [context management & compaction](https://deepwiki.com/sst/opencode/2.4-context-management-and-compaction)).
- **[C] DCP plugin:** stale tool-output pruning as a *plugin* on the session event stream — proof that observation-masking can be retrofitted onto a CLI agent without forking it.
- **[S]** Sessions persist with explicit IDs; `opencode-sessions` plugin ([malhashemi/opencode-sessions](https://github.com/malhashemi/opencode-sessions)) spawns **child sessions for multi-agent collaboration** sharing parent context.
- **[P] Cache-aware gotcha** ([#4416](https://github.com/sst/opencode/issues/4416)): cached tokens inflate naive "context used" counters and fire compaction far too early — token accounting must separate `cache_read` from fresh input. Fork-aware `prompt_cache_key` reuse discussed in [#4317](https://github.com/sst/opencode/issues/4317).
- **Applicability:** two lessons: (a) pruning-as-plugin = evolve-loop can implement masking in Claude Code **hooks** without touching the CLI; (b) when adding compaction triggers, compute utilization from *uncached* input tokens or the fleet will compact needlessly and destroy its own cache prefix.

## 11. claude-flow / ruflo — SQLite swarm memory for concurrent Claude Code agents

- **Repo:** [ruvnet/ruflo](https://github.com/ruvnet/ruflo) (formerly `ruvnet/claude-flow`, ~10k★; ~100K MAU claimed; the largest Claude Code orchestration layer)
- **[K] Shared swarm memory:** SQLite DB at `.swarm/memory.db` — all concurrent agents read/write coordination state, discovered facts, and task artifacts through MCP memory tools instead of passing transcripts around. Hive-mind sessions resume from the DB (**[S]**).
- **[C]** Background workers use local vector retrieval (RuVector) and local execution "so they do not consume extra tokens"; artifact-centric coordination (agents exchange file refs, not content) per the [playbook gist](https://gist.github.com/ruvnet/9b066e77dd2980bfdcc5adf3bc082281).
- **Claimed numbers:** 75–80% token reduction, ~250% effective subscription capacity. **Treat as marketing-grade — no reproducible benchmark published.** The architectural patterns (shared DB, artifact-passing, local-compute-for-deterministic-work) are still sound and mirror evolve-loop's own Go-native direction.
- **Applicability:** validates evolve-loop's existing artifact-file handoff; the delta worth stealing is a **single shared, queryable cross-lane memory** (evolve-loop's knowledge-base is per-repo but phases don't query it mid-session; an MCP/file-based lookup would let lane 2 reuse lane 1's discoveries in real time).

## 12. RTK ("Rust Token Killer") — command-aware CLI output compression at the source

- **Repo:** [rtk-ai/rtk](https://github.com/rtk-ai/rtk) (Apache-2.0, single Rust binary, active 2026 newcomer)
- **[C] Mechanism:** a CLI proxy — `rtk git status`, `rtk cargo test`, `rtk kubectl ...` (100+ commands) — that applies **command-aware filtering/compression** to stdout before it enters the LLM transcript (not naive truncation: it knows a passing test suite compresses to one line, a `git status` to a short delta). Integration for Claude Code/Codex/Gemini CLI via **hooks that rewrite Bash commands** (`git status` → `rtk git status`) pre-execution; <10ms overhead.
- **Published numbers:** measured across 2,900+ real commands: avg **89% of CLI output noise removed**; `cargo test` −91.8%, `git status` −80.8%, `find` −78.3%, `grep` −49.5%; ~118K tokens burned by routine shell output in a 30-min session ([writeup](https://dev.to/arshtechpro/how-rtk-reduces-llm-token-usage-for-ai-coding-agents-2kfd)).
- **Applicability:** *drop-in for evolve-loop today* — a PreToolUse hook in phase-agent settings rewrites Bash invocations; zero provider/API change, works identically under tmux. Compresses at the **source** so tokens are never paid, which beats masking (which only stops re-paying). Biggest single lever for build/tdd/audit phases dominated by `go test`/`git` output.

## 13. OpenAI Agents SDK — sessions + handoff-history collapse

- **Repo:** [openai/openai-agents-python](https://github.com/openai/openai-agents-python) (~15k★, active)
- **[S] Sessions** (`SQLiteSession` et al.): automatic conversation persistence across `Runner.run()` calls — multi-step work resumes an existing history object instead of rebuilding it.
- **[C][P] `RunConfig.nest_handoff_history`:** on agent→agent handoff, the runner **collapses the prior transcript into a single assistant summary message** wrapped in a `<CONVERSATION HISTORY>` block; subsequent handoffs append into the same block. The receiving agent boots with a compact structured brief instead of the full donor transcript ([handoffs docs](https://openai.github.io/openai-agents-python/handoffs/)). `handoff_input_filter` allows custom pruning (e.g., strip all tool calls from handed-off history).
- **Applicability:** this is exactly the phase-boundary problem: evolve-loop's phase N+1 should receive a schema-bound `<CYCLE HISTORY>` digest (orchestrator-assembled from artifacts), never raw prior-phase transcripts. `handoff_input_filter` ≈ per-edge artifact pruning (internal B2).

## 14. DSPy — compile once, cache every repeated call

- **Repo:** [stanfordnlp/dspy](https://github.com/stanfordnlp/dspy) (~30k★, active)
- **[K][B] Compiled prompts:** MIPROv2/GEPA optimize instructions + few-shot demos offline against a metric; the compiled program is **persisted** (`program.save()` / load) so optimization cost is paid once, not per run. 2026 ecosystem note: "Agent Capsules" (arXiv 2605.00410) reports compiled pipelines at **19–68% fewer tokens at quality parity** vs uncompiled.
- **[P] LM-call caching** (`dspy/clients/cache.py`): two-tier in-memory + disk cache keyed on (model, messages, params) — deterministic repeated calls (routing, classification) cost zero after first execution.
- **Applicability:** not for the interactive phases, but for evolve-loop's **advisor/router/triage LLM calls**: (a) cache verdicts keyed on (inputs-hash) — identical triage/routing questions across lanes/cycles should not re-hit the API; (b) treat phase prompt text as a compiled artifact tuned offline against cycle-outcome metrics rather than hand-grown.

## 15. Claude Code native + Agent SDK — the substrate evolve-loop already drives

- **Sources:** [cost docs](https://code.claude.com/docs/en/costs), [prompt caching](https://platform.claude.com/docs/en/build-with-claude/prompt-caching), Anthropic context-editing announcement
- **[C] Context editing / tool-result clearing (API-level, but harness-exposed):** stale tool results auto-cleared; Anthropic measured **−84% tokens on 100-turn workflows**. Microcompact clears old tool outputs silently; `/compact <instructions>` supports custom compaction focus.
- **[S] Session lifecycle:** `claude -p --resume <session-id>` resumes (and forks) headless sessions — a phase agent can be *continued* rather than re-booted when the same phase retries, re-paying only new tokens (system prompt + CLAUDE.md + exploration already in-transcript and largely cache-read).
- **[K] Subagent isolation:** verbose work (test runs, doc fetches) delegated to subagents whose transcripts never enter the parent; only the final message returns. Per-subagent `model: haiku` for cheap tiers.
- **[P]** CLI transcripts are cache-aligned by construction (system prompt + CLAUDE.md stable prefix); **anything the harness injects per-cycle near the top (cycle numbers, timestamps, run IDs) breaks it** — same discipline as #2.
- **[B]** OTEL/cost telemetry per session; `/cost`; per-agent model pinning.
- **Applicability:** evolve-loop controls all of this from Go: retry-same-phase via `--resume` instead of fresh boot; keep injected preamble byte-stable; route verbose verification into subagents with 1–2K-token return contracts.

---

## Cross-cutting synthesis → evolve-loop's four control surfaces

**Prompts (we assemble them):**
- Repo-map artifact with a hard token budget (aider #3) replaces per-agent exploration — the biggest fresh-boot tax is 12×/cycle re-exploration; a 1–2K-token Go-generated map amortizes it to ~zero marginal cost.
- Handoff digests, not transcripts (OpenAI SDK #13): schema-bound `<CYCLE HISTORY>` block per phase edge.
- Byte-stable preamble; per-cycle variables at the tail (mini-swe-agent #2, Claude Code #15).

**Session lifecycle (we own tmux boot/kill):**
- Phase retry → `--resume` same session, never fresh boot (#15).
- Idle-lane sleep-time consolidation between cycles (Letta #8) so scout boots into a pre-digested brief.
- Event-sourced pruning: full artifacts on disk, pruned *views* injected (OpenHands #4).

**Artifacts (we define contracts):**
- Tool-call memoization keyed (cmd, args, tree-SHA) across phases/lanes (CrewAI #7).
- Single-writer shared fleet brief / queryable cross-lane memory (Letta blocks #8, claude-flow #11).
- Per-message tool-output caps with head+tail elision (AG2 #6, SWE-agent #1) — enforceable in hooks.
- RTK-style command-aware output compression via PreToolUse hook rewriting (#12) — cheapest big win; tokens never paid at all.

**Scheduling (we launch lanes):**
- Launch lane 2 within the provider cache TTL of lane 1's shared prefix; keepalive pings during gaps (aider #3).
- Threshold-and-reserve math (Roo #9): act at 70% of window; overflow-crash retries re-pay everything.
- Compute utilization from *uncached* input tokens only, or compaction fires early and destroys the cache (opencode #10).
- Hard per-phase cost/call caps with graceful submit, not kill (SWE-agent #1).
