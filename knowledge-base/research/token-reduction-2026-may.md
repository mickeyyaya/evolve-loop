# Token Reduction Research Dossier — 2026-05-11

> **Archive note:** This file is intentionally excluded from agent context (knowledge-base/ glob exclusion). It serves as the persistent reference for future Scouts building on Cycle 15 research. Last updated: 2026-05-11 (Cycle 15).

## Overview

This dossier archives the primary research sources for evolve-loop's token-reduction campaign (Cycles 15–19+). All sources are dated 2025–2026 and were incorporated into `docs/architecture/token-reduction-roadmap.md` as the canonical actionable roadmap.

**Baseline:** Cycle-11 forensics — $6.70 total per cycle, 142 turns across 5 phases. Cache-read 50%, cache-create 30%, output 19%. P1–P8 roadmap targets ~48% reduction (Cycles 15–18 combined).

---

## Source 1: Anthropic — Multi-Agent Research System (2025–2026)

**URL:** https://www.anthropic.com/engineering/multi-agent-research-system

**Date:** 2025–2026

**Key findings:**
- Multi-agent systems burn ~15× more tokens than single-chat workflows
- Subagents should return 1–2K condensed summaries from 10K+ internal token work
- Hierarchical result compression is the #1 lever for multi-agent cost control
- Static prompt prefixes shared across sibling subagents get near-100% cache hits

**Applicability to evolve-loop:**
- Scout 49-turn → ≤15-turn cap (P1, shipped v9.0.3) directly implements this principle
- Builder 58-turn → ≤20-turn cap (P2, Cycle 16) is the next application
- Fan-out cache prefix (EVOLVE_FANOUT_CACHE_PREFIX=1) implements shared-prefix caching

---

## Source 2: PSMAS — Phase-Scheduled Multi-Agent Systems

**URL:** https://arxiv.org/abs/2510.26585

**DOI/arXiv:** arXiv:2510.26585

**Date:** 2025

**Key findings:**
- 27.3% mean token reduction via phase scheduling vs. always-on multi-agent
- Beats learned routing by 5.6 percentage points
- Phase-skip decisions based on task classification (complexity estimate) outperform dynamic routing
- The key insight: "not every task needs every phase" — triage classification gates which phases fire

**Applicability to evolve-loop:**
- Triage `cycle_size_estimate` field was designed with PSMAS in mind
- P6 (Cycle 19+): extend `cycle_size_estimate=skip` to gate Auditor phase on trivial cycles
- Current triage already classifies `normal` vs. `large`; adding `skip` is incremental

---

## Source 3: Zylos — AI Agent Context Compression Strategies (2026-02-28)

**URL:** https://zylos.ai/research/2026-02-28-ai-agent-context-compression-strategies

**Date:** 2026-02-28

**Key findings:**
- Hierarchical summarization every 10–20 steps reduces context accumulation
- Anchor pattern achieves 4.04 accuracy on technical detail retrieval vs. 2.81 naive
- Three-tier context: hot (current task), warm (recent history), cold (archived reference)
- Lossy compression of warm tier with anchor extraction preserves critical invariants

**Applicability to evolve-loop:**
- EVOLVE_ANCHOR_EXTRACT implements anchor pattern for hot/warm split
- `role-context-builder.sh` implements three-tier layout (immediate artifacts / recent history / archived)
- P4 (Auditor anchor mode) extends anchor extraction to intent.md acceptance criteria

---

## Source 4: SupervisorAgent — Obvious Works (2026)

**URL:** https://www.obviousworks.ch/en/token-optimization-saves-up-to-80-percent-llm-costs/

**Date:** 2026

**Key findings:**
- 43% step reduction + 70% token cost reduction via tighter orchestrator-side decisions
- Orchestrator verbosity is the hidden cost driver — not the worker agents
- Pre-filtering task context at the dispatcher level before passing to workers is key
- Stop-criteria that terminate work early (not exhausting max turns) account for 40%+ of savings

**Applicability to evolve-loop:**
- Orchestrator persona (21,528 bytes) is largest — Campaign D extracted 6.2%, more headroom
- P2 stop-criteria approach for Builder directly applies (structured stop vs. turn-exhaustion)
- `role-context-builder.sh` pre-filtering of role-specific context implements dispatcher-level filtering

---

## Source 5: Finout — Claude Opus 4.7 Pricing (2026)

**URL:** https://www.finout.io/blog/claude-opus-4.7-pricing-the-real-cost-story-behind-the-unchanged-price-tag

**Date:** 2026

**Key findings:**
- Opus 4.7 uses up to 35% more tokens than older tokenizers — "effective output cost rises 25–35%"
- Pricing unchanged but tokenizer verbosity tax makes real cost higher
- Sonnet 4.6 pricing: $3/$15/$3.75/$0.30 per MTok (input/output/cache-create/cache-read)
- Opus 4.7 pricing: $5/$25/$6.25/$0.50 per MTok — 67–83% more expensive than Sonnet
- Break-even: Opus 4.7 justified only when ADVERSARIAL_AUDIT quality gap is measurable

**Applicability to evolve-loop:**
- Auditor defaults to Opus (adversarial audit, different family from Builder Sonnet)
- P-NEW-2 (Cycle 17+): right-size to Sonnet on `consecutiveClean >= 3`
- At cycle-11 auditor cost ($2.10), Opus→Sonnet = ~$0.80/cycle saving on clean cycles

---

## Source 6: ACON — NeurIPS-track (OpenReview, 2024–2025)

**URL:** https://openreview.net/pdf?id=7JbSwX6bNL

**Date:** 2024–2025

**Key findings:**
- 26–54% peak-token reduction via failure-driven guideline updates
- Gradient-free, model-agnostic (works with any LLM)
- The key mechanism: failed attempts compress into one-sentence guidelines; these prepend future prompts
- Per-failure compression beats per-N-steps summarization for token efficiency

**Applicability to evolve-loop:**
- `state.json:failedApproaches[]` + retrospective YAML (`.evolve/instincts/lessons/`) implements ACON
- Each FAIL/WARN cycle compresses lesson → `instinctSummary[]` → next cycle's Scout/Builder see it
- P5 (Cycle 17): YAML template externalization reduces the inline template token cost

---

## Source 7: TOON Format — DEV.to (2026)

**URL:** https://dev.to/pockit_tools/llm-structured-output-in-2026-stop-parsing-json-with-regex-and-do-it-right-34pk

**Date:** 2026

**Key findings:**
- 30–60% structured-output token reduction vs. JSON for tabular data
- TSV + field markers outperform JSON for fixed-schema repeated structures
- LLMs produce TSV more accurately than JSON when prompted with TSV examples
- Parsers for TSV are simpler and faster than JSON parsers

**Applicability to evolve-loop:**
- `audit-report.md` Phase Outcomes table + Code Review Scores are prime TOON targets
- `triage-decision.md` top_n and deferred tables are fixed-schema repeated structures
- P7 (Cycle 18): convert both to TSV format + update `verify-eval.sh` parser

---

## Source 8: LLMLingua 2026 / TokenMix

**URL:** https://tokenmix.ai/blog/llmlingua-prompt-compression-2026

**Date:** 2026

**Key findings:**
- 20× compression in production deployments ($42K → $2.1K monthly cost)
- Best results on repetitive technical documentation (API specs, config files)
- Diminishing returns when prompts are already short (< 2K tokens) — not applicable to small phases
- External dependency (Python package) required; adds latency ~50–200ms per prompt

**Applicability to evolve-loop:**
- P8 (Cycle 20+): pre-processor for role-context-builder.sh output on long phases
- Most valuable for Auditor (35,250B context floor) and Retrospective (44,060B)
- Risk: external dependency + latency + accuracy degradation on technical invariants

---

## Source 9: Anthropic — Effective Context Engineering for AI Agents (2025–2026)

**URL:** https://www.anthropic.com/engineering/effective-context-engineering-for-ai-agents

**Date:** 2025–2026

**Key findings:**
- Static-content-first / dynamic-content-last layout maximizes prompt-cache hit rate
- Changing any byte before cached content invalidates the entire cache hit
- Agent personas should be stable across cycles (static) — only task prompts change (dynamic)
- Recommended layout: `[system prompt] → [tool definitions] → [static persona] → [dynamic task artifacts]`

**Applicability to evolve-loop:**
- `role-context-builder.sh` already implements this: persona file first, then artifacts
- Cache hits verified in cycle-11: ~50% cache-read = $0.30/MTok vs $3/MTok input
- P4 (Auditor anchor mode): pins `intent.md:acceptance_criteria` to a stable summary (static)
- P-NEW-1 flag promotion: makes the verified static-first layout the default for all users

---

## Source 10: Progressive Disclosure — MindStudio (2025)

**URL:** https://www.mindstudio.ai/blog/progressive-disclosure-ai-agents-context-management

**Date:** 2025

**Key findings:**
- Three-layer persona (card/manual/reference) prevents context rot in long-running agents
- Card (< 500 chars): always-loaded summary of role and key rules
- Manual (2–5K chars): loaded on first invocation; cached thereafter
- Reference (on-demand): loaded only when agent explicitly needs it
- Campaign D (Layer 3 extraction) implements this pattern in evolve-loop

**Applicability to evolve-loop:**
- `agents/evolve-orchestrator-reference.md` is the reference layer for orchestrator
- `agents/evolve-builder-reference.md` is the reference layer for builder
- Auditor has `evolve-auditor-reference.md`; Scout has 0% extraction (P-NEW-3 target)
- P5 (Cycle 17): Retrospective YAML template moves to reference layer

---

## Source 11: Observational Context Compression — arXiv:2604.19572 (Apr 2026)

**arXiv:** https://arxiv.org/abs/2604.19572

**Date:** April 2026

**Key findings:**
- Identifies "expired" tool-result entries in multi-turn agent trajectories — outputs that have been summarized or acted upon and are now pure token waste
- ~40–60% input token reduction on tool-output-heavy trajectories (multi-file Builder reads)
- Operates at the trajectory layer: removes content, preserves agent state (no re-execution)
- Compatible with frozen prompt prefixes (doesn't break cache boundaries)

**Applicability to evolve-loop:**
- Direct target: Builder multi-turn read phases (cycle-43: 69 turns, $3.12)
- P-NEW-24 (cycle 46+): expired-tool-result removal for Builder; profile-level contract changes required

---

## Source 12: Token-Budget-Aware LLM Reasoning — arXiv:2412.18547 (2026)

**arXiv:** https://arxiv.org/abs/2412.18547

**Date:** 2026 (last revised)

**Key findings:**
- Preemptive budget declaration ("you have N turns") induces self-regulation from turn 1
- 10–20% turn reduction vs. stop-criterion-only baselines on tool-use tasks
- Most effective when budget is set to 75–85% of the hard max (conservative advisory value)
- Effect is additive on top of gate-based stop-criteria (P-NEW-10, P-NEW-19, P-NEW-20)

**Applicability to evolve-loop:**
- P-NEW-23 (cycle 44, DONE): `emit_budget_hint()` in `role-context-builder.sh`; `turn_budget_hint` in 6 profiles
- Conservative advisory values: scout:12/15, builder:20/25, auditor:30/40, orchestrator:45/60

---

## Source 13: Anthropic Compaction API — compact-2026-01-12 (Jan 2026)

**Date:** January 2026

**Key findings:**
- Automatic context compaction for long-running `claude -p` sessions
- 40–60% cost reduction reported on sessions > 20 turns
- Zero changes to persona prompts required — purely dispatcher-layer opt-in
- Analogous interface to existing `--cache-ttl` / `--max-budget-usd` flags

**Applicability to evolve-loop:**
- P-NEW-25 (cycle 45 investigation): probe `claude -p --help` for compaction flag
- Primary targets: Orchestrator (48 turns, $1.68) and Builder (69 turns, $3.12) in cycle-43
- Risk: unknown; investigation follows P-NEW-17 TTL research template

---

## Source 14: MCP Tool Schema Overhead Analysis — docs.bswen.com (Apr 2026)

**URL:** https://docs.bswen.com/mcp-tool-schema-analysis

**Date:** April 2026

**Key findings:**
- MCP tool schemas serialized in `claude -p` invocations account for 70–97% of system-prompt tokens in plugin-heavy configurations
- Per-phase tool filtering via `--allowedTools` reduces schema serialization
- Selective schema injection (only tools a phase uses) achieves 5–20% per-turn input reduction
- P-NEW-22 research basis: measure `claude -p` schema serialization with/without `--allowedTools`

**Applicability to evolve-loop:**
- P-NEW-22 (cycle 46+): measurement-first approach — run one cycle with/without `--allowedTools` filtering; compare `input_tokens` in usage sidecars
- Profiles already have per-role `allowed_tools` lists; question is whether they reduce *serialized* schema tokens

---

## Cycle-44 Research Summary

> Added by Builder cycle 44 (2026-05-14). Sources 11–14 synthesized from online research by Scout cycle-44.

**New roadmap items:** P-NEW-24 (observational context compression, arXiv:2604.19572) + P-NEW-25 (Anthropic native compaction, compact-2026-01-12). Both added to `docs/architecture/token-reduction-roadmap.md`.

**P-NEW-23 SHIPPED (cycle 44):** `emit_budget_hint()` in `role-context-builder.sh` + `turn_budget_hint` in 6 profiles. Expected 10–20% turn reduction on top of stop-criteria.

**Cycle-43 cost snapshot:** Total $8.40 (regression from $5.80). Root cause: Builder 69-turn bootstrap for P-NEW-20 stop-criterion shipment. P-NEW-19 auditor stop-criterion: 49→29 turns (−41%, working). Expected cycle-44 benefit from P-NEW-19/20 + P-NEW-23: ~$1.20–2.10/cycle reduction targeting $6.00–7.00/cycle.

---

## Codebase Measurements (Cycle 15 fresh, v9.1.1)

### Persona file sizes (bytes)

| File | Bytes | Lines | Layer-3 file | Layer-3 extracted % |
|------|-------|-------|--------------|---------------------|
| agents/evolve-orchestrator.md | 21,528 | 254 | 5,121 bytes | −6.2% |
| agents/evolve-scout.md | 18,405 | 334 | — | 0% (P-NEW-3 target) |
| agents/evolve-builder.md | 17,913 | 354 | 3,348 bytes | −3.1% |
| agents/evolve-auditor.md | 16,361 | 293 | 2,284 bytes | −3.1% |
| agents/evolve-retrospective.md | 12,988 | 243 | — | 0% |
| Total persona bytes | ~150,869 | — | — | — |

### Context floor (post-v9.0.0 Campaigns A–D)

| Phase | Default (no flags) | EVOLVE_CONTEXT_DIGEST=1 | With anchor extract too |
|-------|-------------------|------------------------|------------------------|
| scout | 13,615 B | 2,012 B (−85%) | ~2,012 B |
| triage | 27,733 B | 16,677 B (−40%) | ~5,000 B (−82%) |
| builder | 27,743 B | 15,810 B (−43%) | ~10,000 B (est.) |
| auditor | 35,250 B | 35,250 B (0%) | ~10,500 B (−70%, needs anchored artifacts) |
| retrospective | 44,060 B | 44,060 B (0%) | 44,060 B |

---

## Research Gap Analysis (for future Scouts)

| Gap | Status | Next action |
|-----|--------|-------------|
| PSMAS phase-skip integration with triage | Not implemented | Cycle 19+ (P6) |
| LLMLingua accuracy on evolve-loop prompts | Untested | Cycle 20+ (P8) |
| Sonnet audit quality vs Opus (adversarial) | Not benchmarked | Cycle 17 pre-requisite |
| Scout Layer-3 extraction behavioral impact | Unknown | Cycle 18 (P-NEW-3) |
| TOON format parser robustness | Not tested | Cycle 18 (P7) |
