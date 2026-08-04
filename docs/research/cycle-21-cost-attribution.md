# Cycle-21 Cost Attribution Research

> Source: cycle-21 scout-report.md F-4. Compiled 2026-05-12. Covers cost attribution for cycle-19 and cycle-20 with top-3 cost drivers ranked by delta.

## Per-Phase Cost Breakdown: Cycles 19 vs 20

| Phase | Model | Cycle-19 cost | Cycle-19 turns | Cycle-20 cost | Cycle-20 turns | Delta cost |
|-------|-------|--------------|----------------|--------------|----------------|------------|
| Intent | Opus | $0.73 | 5 | $0.75 | 6 | +$0.02 |
| Scout | Sonnet | $1.23 | 41 | $1.29 | 44 | +$0.06 |
| Triage | Sonnet | $0.32 | 7 | $0.28 | 4 | -$0.04 |
| **Builder** | Sonnet | **$0.80** | **40** | **$1.97** | **64** | **+$1.17** |
| **Auditor** | Opus (c20) / Sonnet (c19) | **$0.58** | **19** | **$1.84** | **19** | **+$1.26** |
| Orchestrator | Sonnet | $2.31 | 63 | $1.14 | 38 | -$1.17 |
| Retrospective | Sonnet | $0.43 | 10 | $0.41 | 9 | -$0.02 |
| **Total** | | **$6.40** | **185** | **$7.68** | **184** | **+$1.28** |

## Top-3 Cost Drivers (Cycle 19 → Cycle 20)

| Rank | Driver | Delta | Root cause |
|------|--------|-------|-----------|
| 1 | **Auditor model: Sonnet → Opus** | +$1.26 | Auditor profile `model_tier_default: "opus"`. Cycle-19 auditor used Sonnet; cycle-20 used Opus (same 19 turns, 3.2× price difference). Likely a profile change between cycles — cycle-19 may have predated the adversarial-audit default-Opus policy. |
| 2 | **Builder turn inflation: 40 → 64 turns** | +$1.17 | Cycle-20 had 5 distinct tasks (SKILL.md edit, eval-grader-best-practices.md, review-skill-catalog.md, roadmap update, inline Self-Review section authoring ~50 lines). More files to read + larger build-report output tokens. |
| 3 | **Orchestrator efficiency: 63 → 38 turns** | -$1.17 | Cycle-20 orchestrator completed with fewer turns than cycle-19 (cleaner WARN path). Offsets builder increase entirely. |

## Baseline Discrepancy Note

The cycle-21 goal mentions "$7.69 vs $3.20 baseline." The $7.69 matches cycle-20 ($7.68 actual). The $3.20 baseline likely refers to an earlier, simpler cycle (pre-retrospective phases, cycle 17 or 18 with smaller goals). Cycle-19 total was $6.40 — already elevated due to Sonnet orchestrator at 63 turns.

## Key Insight: Auditor Model Selection

The most actionable cost reduction available is auditor model right-sizing. Sonnet auditors at 19 turns cost $0.58 vs Opus at $1.84 — a recurring **$1.26/cycle delta** if auditor is always Opus.

The P-NEW-2 plan (Auditor Sonnet right-sizing on clean cycles) targets exactly this. The prerequisite is `consecutiveClean >= 1`, which requires the Builder self-review skill loop to actually work so it can pre-validate clean builds.

## Builder Turn Inflation Analysis

The 64-turn cycle-20 builder was inflated by:
- Manual Self-Review section authoring (~15 turns) — shouldn't have happened; a genuine skill-loop invocation takes 3-5 turns
- 5 distinct multi-file tasks requiring ~6-8 turns each
- Larger build-report output (150+ lines) adding ~5 output tokens overhead

Genuine skill-loop invocation (3-5 turns) is cheaper than 15 turns of manual simulation. Unblocking the Skill tool in cycle-21 is the load-bearing prerequisite for both accurate self-review AND builder turn reduction.

## Connection to P-NEW-2

P-NEW-2 (Auditor Sonnet right-sizing) is blocked until:
1. Builder self-review loop verified working (cycle-21 target)
2. `consecutiveClean >= 1` — first clean cycle with real Skill invocation

If cycle-21 ships clean with a verified Skill invocation, P-NEW-2 is unblocked for cycle-22 assessment. Potential savings: ~$1.26/cycle recurring.
