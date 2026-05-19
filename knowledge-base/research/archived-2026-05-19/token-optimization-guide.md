# Token Optimization Guide

> Measured token footprints, research-backed optimization techniques, and actionable recommendations for reducing token usage across all evolve-loop skills without losing flow quality.

## Table of Contents

1. [Token Footprint Baseline](#token-footprint-baseline)
2. [Optimization Techniques](#optimization-techniques)
3. [Per-File Recommendations](#per-file-recommendations)
4. [Implementation Priority](#implementation-priority)
5. [Research References](#research-references)

---

## Token Footprint Baseline

Measured using `scripts/observability/token-profiler.sh` (1 line ≈ 15 tokens):

### By Category

| Category | Lines | ~Tokens | % of Total |
|----------|-------|---------|-----------|
| skill:evolve-loop | 4,405 | 66,075 | 44% |
| agent (all 5) | 1,274 | 19,110 | 13% |
| skill:refactor | 1,161 | 17,415 | 12% |
| skill:inspirer | 746 | 11,190 | 7% |
| script (all) | 1,881 | 28,215 | 19% |
| skill:code-review-simplify | 271 | 4,065 | 3% |
| **Total** | **~10,000** | **~150,000** | **100%** |

### Top 10 Heaviest Files

| Rank | File | Lines | ~Tokens |
|------|------|-------|---------|
| 1 | skills/evolve-loop/phases.md | 694 | 10,410 |
| 2 | skills/evolve-loop/smart-web-search.md | 654 | 9,810 |
| 3 | skills/refactor/SKILL.md | 653 | 9,795 |
| 4 | skills/evolve-loop/benchmark-eval.md | 507 | 7,605 |
| 5 | scripts/observability/cycle-health-check.sh | 443 | 6,645 |
| 6 | skills/evolve-loop/phase6-learn.md | 435 | 6,525 |
| 7 | scripts/lifecycle/phase-gate.sh | 422 | 6,330 |
| 8 | skills/evolve-loop/memory-protocol.md | 392 | 5,880 |
| 9 | agents/evolve-scout.md | 372 | 5,580 |
| 10 | skills/inspirer/SKILL.md | 336 | 5,040 |

### Key Insight

Only **SKILL.md** loads at skill invocation time (~2.1K tokens for evolve-loop). Phase files, agents, and reference files load **on demand** when the orchestrator reads them for specific phases. The real optimization targets are files read repeatedly across cycles.

---

## Optimization Techniques

Research-backed techniques ranked by effort-to-benefit ratio:

### 1. Three-Tier Progressive Disclosure (95-98% startup savings)

**Already partially implemented.** The evolve-loop SKILL.md is 141 lines (2.1K tokens) — it acts as a compact entry point that references detailed phase files on demand.

| Tier | What Loads | When | Cost |
|------|-----------|------|------|
| L1: Discovery | Skill name + description | Session start | ~15 tokens/skill |
| L2: Activation | Full SKILL.md | On invocation | 2-10K tokens |
| L3: Execution | Phase files, agents, reference | During specific phases | Variable |

**Source:** Lazy Skills (Boliv 2025) — 42 skills from 21K → 630 tokens at startup.

**Recommendation:** No action needed for evolve-loop (already uses this pattern). For refactor/SKILL.md (653 lines), consider splitting into SKILL.md (entry point) + reference files.

### 2. Context Block Ordering for Cache Hits (20-40% savings)

Place **static content first, dynamic content last** in agent context blocks. This maximizes Anthropic prompt cache hits.

| Position | Content Type | Example |
|----------|-------------|---------|
| First (static) | Shared values, skill instructions | Agent role, behavioral rules |
| Middle (semi-stable) | State summaries, instincts | state.json excerpt |
| Last (dynamic) | Cycle-specific data | Changed files, recent ledger |

**Source:** Anthropic Prompt Caching docs — 0.1x cost for cache reads vs 1x for uncached.

**Already implemented** in phases.md (Per-Phase Context Selection Matrix) and agent-templates.md.

### 3. AgentDiet Trajectory Compression (40-60% savings between phases)

Remove useless, redundant, and expired information from agent context between phase transitions.

| After Phase | Keep | Remove |
|-------------|------|--------|
| DISCOVER | Task list + eval definitions | Full Scout analysis |
| BUILD | Build-report summary + SHA | Full Builder output |
| AUDIT | Verdict + issue list | Full Auditor analysis |
| SHIP | Commit SHA + benchmark delta | Commit details |

**Never prune:** challenge token, eval checksums, failed approach details.

**Source:** AgentDiet (arXiv:2509.23586, FSE 2026) — 39.9-59.7% input token reduction, zero performance degradation.

**Already defined** in phases.md (lines 320-332). No measurement script exists to verify actual savings.

### 4. Event-Driven System Reminders (prevents instruction fade-out)

Re-inject critical instructions periodically rather than relying on them surviving in long context. Counteracts "lost in the middle" where instructions at context midpoint lose 20%+ effectiveness.

**Source:** OPENDEV (arXiv:2603.05344), Stanford "Lost in the Middle" (2023).

**Recommendation:** For cycles 4+, re-inject the core behavioral rules (shared values, challenge token) at the start of each agent context instead of relying on them from cycle 1.

### 5. Per-Phase Context Selection Matrix (eliminates unnecessary fields)

Each agent receives ONLY the fields it needs — not the full state:

| Field | Scout | Builder | Auditor |
|-------|:-----:|:-------:|:-------:|
| Full stateJson | Y | - | - |
| projectDigest | Y | - | - |
| task (from scout) | - | Y | - |
| buildReport | - | - | Y |
| instinctSummary | Y | Y | - |

Saves ~3-5K tokens per agent invocation.

**Already implemented** in phases.md. Verify agents aren't reading extra state beyond their matrix allocation.

---

## Per-File Recommendations

Actionable optimizations for the heaviest files:

| File | Current | Recommendation | Expected Savings |
|------|---------|---------------|-----------------|
| **phases.md** (694 lines) | Monolithic phase instructions | Already well-structured with on-demand reading. The Per-Phase Context Matrix and AgentDiet sections minimize what's actually loaded per cycle. | Low further savings — already optimized |
| **smart-web-search.md** (654 lines) | Full 6-stage pipeline | Only loaded for Phase 1 research. Could split into core (stages 1-3) + advanced (stages 4-6) with lazy loading markers | ~3K tokens when only basic search needed |
| **refactor/SKILL.md** (653 lines) | Monolithic skill file | Split into SKILL.md (entry point, ~150 lines) + reference files for technique catalog, smell mapping | ~7K tokens saved at invocation |
| **benchmark-eval.md** (507 lines) | 8-dimension scoring rubric | Only loaded once per invocation (Phase 0). Already appropriate — calibration is expensive but infrequent | No change needed |
| **phase6-learn.md** (435 lines) | Learn phase instructions | Only loaded once per cycle for Phase 5. Appropriate size for its scope | No change needed |
| **policies.md** (176 lines) | **Compressed this cycle** from 318 lines | 44% reduction achieved. Removed duplicate handoff template, compressed verbose pseudocode into tables | **2.1K tokens saved** |

---

## Implementation Priority

| Priority | Action | Effort | Token Savings | Status |
|----------|--------|--------|---------------|--------|
| **DONE** | Compress policies.md (318→176 lines, 44% reduction) | S | 2.1K/read | Cycle 175 |
| **DONE** | Create token-profiler.sh with --json mode | M | Enables measurement | Cycle 175 |
| **DONE** | Add --save-baseline and --compare to profiler | S | Enables delta tracking | Cycle 176 |
| P1 | Split refactor/SKILL.md into entry + reference | M | ~7K/invocation | Future cycle |
| P2 | Split smart-web-search.md into core + advanced | S | ~3K when basic only | Future cycle |
| P3 | Add AgentDiet measurement to token-profiler | S | Validates 40-60% claim | Future cycle |
| P4 | Verify per-phase context matrix compliance | S | ~3-5K/agent if violations found | Future cycle |

---

## Optimization Tracking

Use `scripts/observability/token-profiler.sh` to measure and track token footprint over time.

### How to Track

```bash
# Save a baseline (do this before optimization)
bash scripts/observability/token-profiler.sh --save-baseline

# After optimization, compare against baseline
bash scripts/observability/token-profiler.sh --compare

# Full report with JSON for programmatic use
bash scripts/observability/token-profiler.sh --json
```

### Optimization History

| Cycle | File | Before | After | Reduction | Technique |
|-------|------|--------|-------|-----------|-----------|
| 175 | policies.md | 318 lines (4,770 tokens) | 176 lines (2,640 tokens) | 44% | Removed duplicate handoff template, compressed verbose pseudocode into tables |

### Baseline

First baseline saved cycle 176: **149,175 tokens** across 9,945 lines (47 files scanned).

---

## Research References

| Paper | Key Finding | Relevance |
|-------|-------------|-----------|
| AgentDiet (arXiv:2509.23586, FSE 2026) | 40-60% token reduction via trajectory compression | AgentDiet pattern already in phases.md |
| OPENDEV (arXiv:2603.05344) | Lazy tool discovery, adaptive compaction, event-driven reminders | Progressive disclosure and system reminders |
| CEMM (arXiv:2603.09619) | Context Engineering Maturity Model — 5 quality criteria | Relevance, Sufficiency, Isolation, Economy, Provenance |
| Prompt Compression Survey (arXiv:2410.12388, NAACL 2025) | Hard vs soft compression taxonomy | LLMLingua-2 for potential future compression |
| Agentic Plan Caching (arXiv:2506.14852, NeurIPS 2025) | 46-50% cost reduction via plan template reuse | Plan Reuse section in policies.md |
| Lazy Skills (Boliv 2025) | 97% startup token reduction via 3-tier loading | Already partially used |
| Martin Fowler Context Engineering | 5 pillars: progressive disclosure, compression, routing, retrieval, tool management | Framework for ongoing optimization |
