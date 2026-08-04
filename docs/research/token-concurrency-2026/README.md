# Token Optimization Under Concurrency — 2026 Research + Local Evidence

> Compiled 2026-07-07 (operator session). Companion to [token-optimization-2026/](../token-optimization-2026/) (single-agent techniques).
> This directory = the **concurrency/fleet** angle: what burns tokens when 2+ evolve-loop lanes run simultaneously, and what the 2026 literature + OSS ecosystem say about fixing it.
> Files: [papers.md](papers.md) (18 sources, 2025-2026 arXiv + provider engineering) · [oss.md](oss.md) (15 OSS projects with code-level mechanisms).

## Local burn evidence (measured on this repo, batch bqt0cmyh8, 2026-07-06)

| Signal | Value | Meaning |
|---|---|---|
| CLI session boots / 8-cycle batch | **164** | every boot re-pays system prompt + CLAUDE.md/AGENTS.md + fresh repo exploration — unmetered until token-telemetry lands |
| LLM calls / cycle | ~12 phase + advisor Plan/RePlan/Propose | all through `bridge.Engine.Launch` |
| Composed prompt text / cycle | ~100KB (~25K tok) | excludes per-session instruction stack + tool round-trips |
| Scout weight | **5×** any other phase (236KB events vs 47KB next) | top optimization target once measured |
| Advisor plan prompt | 32KB per call | second target |
| Fallback double-dispatches / batch | **20** | failed dispatch burns full prompt, fallback re-burns it (the measured waste class) |
| Quota-exhaustion (exit=85) events / batch | **40** | provider benching cascades all burn to one CLI family |
| codex benched | 19/23 dispatch moments | single-provider concentration risk realized |
| Token fields in artifacts | **all zero** under tmux drivers | why measurement (S1-S8) comes first |

## Execution state

- **Measurement first**: 8-slice token-telemetry plan (transcript-scan collector → Engine.Launch instrumentation → `llm-calls.ndjson` → rollups → `evolve tokens report` → fleet shadow join) — design at [docs/plans/token-telemetry-2026-07.md](../../../docs/plans/token-telemetry-2026-07.md), slices queued in `.evolve/inbox/` (token-telemetry-s1..s8, 0.95→0.88).
- **Phase-2 optimization** items queued at 0.72-0.80, gated on the S7 baseline; weights get re-ranked by MEASURED top consumers.

## Priority translation (fleet-specific top-5, from papers.md §7 + oss.md synthesis)

1. **Cache discipline under concurrency** (papers §2.5-2.7): byte-identical static prefixes across lanes/phases; cache-breakers (run-id, cycle, timestamps, dynamic MCP tool lists) after the stable prefix; same-prefix sessions temporally adjacent within the 5-min TTL; never resume stale >TTL sessions. On API-key traffic cache reads are ITPM-exempt → **~5× effective quota** @80% hit.
2. **Fleet admission + centralized retry + budget-checkpointing** (HiveMind, papers §4.1): pre-flight estimated-cost gate per dispatch; per-lane budgets with contracted degradation. Published: dead-agent waste −48-100%, eval cost −96%. Retry is the single most impactful primitive.
3. **Shared verified run-context / who-knows-what registry** (DeLM §1.1, MATM §1.3; Letta shared blocks oss §8): lanes consult a Go-maintained gist board before exploring. DeLM: **−50% cost with accuracy UP**.
4. **Phase-scoped evidence packs computed in Go** (Spec Kit §5.3, scoped contexts §3.3, aider repo-map oss §3): deterministic pre-phase grounding (diff, exports, failing tests, 1-2K-token repo map) replaces 12×/cycle LLM re-exploration. Up to **−82%**.
5. **Instruction-payload audit** (papers §5.4): AGENTS.md-style files measured at **+>20% cost** — the ~24×/cycle CLAUDE.md injection deserves a role-scoped digest experiment.

## Gotchas pinned by the research

- Compute context-utilization from **UNCACHED input tokens** only (opencode #10) — else compaction fires early and destroys your own cache prefix.
- Masking beats summarization (−52.7% cost, BETTER solve rate) because it is suffix-only — summarization rewrites the prefix and invalidates cache (SWE-agent #1).
- Don't build an LLM "redundancy judge" — SOTA detection is 24.88% (papers §5.1); duplicated tool calls are caught deterministically by (cmd,args,tree-SHA) memoization.
- Wide parallelism must be EARNED by real sub-goal independence (Anthropic §3.1: token spend explains 80% of variance; MAS = 15× chat tokens).
