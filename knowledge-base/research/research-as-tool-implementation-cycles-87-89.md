# Research-as-Tool Refactor — Cycles 87/88/89 Implementation Dossier

> **Status:** SHIPPED (cycles 87, 88, 89 on `origin/main` as of 2026-05-19) · partial cleanup interrupted by phase-watchdog stall
> **Plan reference:** `~/.claude/plans/i-have-question-of-velvet-toast.md` § Phase 1
> **Dispatcher:** `bt3dgw9hl` · **Total cost:** ~$31 across 3 cycles + ~$0.40 in 2 manual cleanup commits

## Goal

Replace the **scheduled Phase 1 Research subagent** (which fired unconditionally before Scout) with **on-demand research tools** any phase agent can call within per-agent quotas. Make the framework LLM-agnostic by using tool-name abstractions that adapter layers translate per CLI.

Operator-stated principles:
1. Research as **independent module** invokable by any phase agent
2. **Tiered research depth** (light/medium/deep) tied to token consumption
3. **Reasonable per-agent invocation caps** to prevent runaway cost
4. Support **both online AND local knowledge-base** search

## What was implemented

### Cycle 87 — Foundation ($9.64, SHIPPED clean)

- `scripts/hooks/research-quota-gate.sh` — PreToolUse hook enforcing per-agent WebSearch/WebFetch/kb_search quotas at kernel layer
- `scripts/research/kb-search.sh` — ripgrep-based local KB search across `EVOLVE_KB_SEARCH_PATHS`
- `scripts/lifecycle/cycle-state.sh research-usage-incr` — atomic counter mutation
- `.evolve/profiles/*.json` updated with WebSearch/WebFetch/`Bash(kb-search.sh:*)` in allowedTools and per-agent `research_quota`
- `.claude/settings.json` registers the hook on PreToolUse

### Cycle 88 — Migration ($10.93, SHIPPED-WITH-WARNINGS-AND-LEARNED)

- Removed `gate_intent_to_research()` and `gate_research_to_discover()` from `phase-gate.sh`
- Added `gate_intent_to_discover()` — the new phase flow
- Updated Scout persona to call WebSearch/WebFetch inline as first action
- Renamed `phases/phase1-research.md` → `docs/architecture/research-tool.md` (docs-only rewrite)
- Updated orchestrator persona to remove Phase 1 from documented flow

### Cycle 89 — Open Access + Docs (~$10, partial ship + stall)

- Updated 6 non-Scout phase agent personas with KB-first directive + research-quota guidance
- Rewrote `agents/online-researcher.md` as a research-quality reference doc (no longer dispatched as subagent)
- Added CLAUDE.md env-var entries: `EVOLVE_ALLOW_DEEP_RESEARCH`, `EVOLVE_RESEARCH_QUOTA_SOFT`, `EVOLVE_RESEARCH_HOOK_DISABLED`, `EVOLVE_KB_SEARCH_PATHS`
- Created `docs/architecture/research-tool.md` ADR

## What worked

| Element | Evidence |
|---|---|
| Per-agent quota enforcement at kernel layer | Cycle 87 hook fired correctly; cycle 88 verified phase rename took effect |
| WARN-ship fluent posture | Cycle 88 shipped despite warnings; cycle 89 built on it without issue |
| KB-first directive in personas | 6 non-Scout agents updated; verifiable in personas on `origin/main` |
| ADR-driven documentation | `docs/architecture/research-tool.md` captures rationale for future cycles |
| Cost efficiency | $31 total vs estimated $25-35; cache_hit=99% across all cycles |

## What broke

**Phase-watchdog stall on cycle 89's learn phase.** After cycle 89's ship commit (`322dcd5`) landed, the post-ship promotion (`acs/cycle-{87,88,89}/*.sh` → `acs/regression-suite/cycle-{87,88,89}/`) and docs cleanup got interrupted by phase-watchdog stall-detection (240s inactivity threshold). The orchestrator was SIGTERM'd then SIGKILL'd. Cycles already on `origin/main`; cleanup completed manually in commits `215488b` (cleanup) and `9c6cf19` (stewardship recovery).

See companion dossier: `phase-watchdog-stall-detection-cycle-89.md`.

## Lessons learned

1. **`EVOLVE_INACTIVITY_THRESHOLD_S=240` is too tight for the learn phase.** Retrospective + memo + lesson-merge serially can legitimately take 5+ min. Per-phase threshold proposed: build/audit 240s; learn/retrospective 480s.

2. **Post-ship promotion + cleanup is non-atomic with the main commit.** Future cycle should either (a) make promotion atomic with ship, or (b) add resume-from-promotion logic when watchdog interrupts.

3. **The WARN-ship pattern is correctly tuned.** Fluent posture is doing its job — Cycle 88 shipped under WARN (green=N/N, red=0); subsequent Cycle 89 built on it cleanly.

4. **Tool-name abstractions are still Claude-coupled.** Implementation uses literal `WebSearch`/`WebFetch` in profiles. Phase 0A of the master plan generalizes these to `tool.search_web` / `tool.fetch_web` for cross-CLI portability.

5. **Builder/TDD-engineer role separation holds.** The four-layer predicate-quality defense (v10.15.0) prevented the cycle-86 retry-loop pattern; cycle-57-031 worktree-path fix (`4e120a2`) eliminated the false-positive that broke the prior batch.

## Failed approaches (negative-result documentation)

- **Hermes-agent as Anthropic proxy.** Earlier research showed hermes is NOT an HTTP proxy — it's a chat agent using OpenAI SDK internally. The `hermes proxy start` command was fabricated; corrected in `knowledge-base/research/hermes-agent-proxy-integration.md`. Informs Phase 0E (Proxy & Auth Routing Abstraction): never assume an unverified CLI command works.
- **Single broad upfront research phase.** Today's Phase 1 was always-on broad research that fed only Scout. Cycle 87+88 replaced this with per-agent on-demand research. Trade-off: Scout still does broad research itself (verified equivalent task quality); benefit: 6 other agents now have research access too.

## Cost analysis

| Cycle | Cost | Verdict |
|---|---|---|
| 87 foundation | $9.64 | SHIPPED |
| 88 migrate | $10.93 | SHIPPED-WITH-WARNINGS-AND-LEARNED |
| 89 open access + docs | ~$10 (interrupted accounting) | partial ship + stall |
| 215488b cleanup | ~$0.20 | manual |
| 9c6cf19 stewardship recovery | ~$0.20 | manual |
| **Total** | **~$31** | 3 cycles + 2 manual ships |

99% cache_hit on all cycles.

## Follow-up work (tracked in master plan)

- **Phase 0A:** rename concrete Claude tool names to abstract `tool.*` names; adapter translates at dispatch
- **Phase 4 (deferred):** per-phase `EVOLVE_INACTIVITY_THRESHOLD_S` tuning + atomic promotion + watchdog learning
- **Phase 5D:** codify the stewardship rule in AGENTS.md
- **Phase 5E:** `doc-deletion-guard.sh` PreToolUse hook to prevent silent doc deletions

## References

- Plan: `~/.claude/plans/i-have-question-of-velvet-toast.md`
- Cycle commits on `origin/main`: `322dcd5` (cycle 89 ship), `215488b` (cleanup), `9c6cf19` (stewardship recovery)
- Companion dossiers: `phase-watchdog-stall-detection-cycle-89.md`, `stewardship-rule-violation-2026-05-19.md`
- Predicate-quality four-layer defense (v10.15.0): commits `e2450b6`, `11eb0a5`, `2962749`
- Worktree-path fix prerequisite (A'): commit `4e120a2`
