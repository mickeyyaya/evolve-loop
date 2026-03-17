# Cycle 21 Scout Report

## Discovery Summary
- Scan mode: incremental
- Files analyzed: 3 (agents/evolve-scout.md, CHANGELOG.md, docs/architecture.md)
- Research: skipped (TTL active — queries from 2026-03-14T12:00:00Z, within 12hr window)
- Instincts applied: 4 (inst-013, inst-014, inst-015, inst-016)

---

## Key Findings

### Learning Mechanics — HIGH

**Instinct application is untracked.** The instinctSummary in state.json has `id`, `pattern`, `confidence`, `type`, and optional `graduated` fields — but no `lastApplied`, `applicationCount`, or `outcomeDelta` fields. Scout and Builder are instructed to "apply relevant instincts" but there is no mechanism that records whether a given instinct was actually cited during a cycle. Confidence increases by re-confirmation (same pattern appears again) not by tracking whether applying the instinct helped. This means:
- Instincts with confidence 0.8 may have never been applied — they were just re-observed passively.
- The learn processReward cannot distinguish "extracted a useful, applied instinct" from "extracted a low-quality instinct that no agent cited."
- There is no feedback loop from application → outcome → confidence update.

### Learning Mechanics — MEDIUM

**learn processReward is structurally capped at 0.8.** The scoring rubric defines:
- `1.0` = new instincts extracted + consolidation ran if due
- `0.8` = instincts extracted, no consolidation needed
- `0.0` = no instincts extracted

Since consolidation only runs every 3 cycles, 2 out of 3 cycles can never score above 0.8 regardless of instinct quality. The score does not measure whether applied instincts actually improved cycle outcomes. This makes learn the only process reward dimension that is structurally penalized every cycle without any failure occurring. Fixing the rubric to reward instinct application (not just extraction) would also unlock the remediation loop more accurately.

### Learning Mechanics — MEDIUM

**No instinct citation protocol.** The evolve-scout.md agent prompt says "Read instinctSummary from context — apply learned patterns, avoid known anti-patterns" but gives no instruction for how to _report_ which instincts were applied. The build-report.md and scout-report.md have no `instinctsApplied` field beyond a count. Without a citation list, Phase 5 has no way to know which instincts were used this cycle — it can only infer from narrative text. This is the root cause of why instinct application tracking doesn't exist: the output format never asks for it.

---

## Introspection Pass

**Heuristic: instinctsExtracted == 0** — Not triggered. Each of cycles 17-20 extracted exactly 1 instinct.

**Heuristic: instinct with confidence >= 0.6 uncited for 3+ cycles** — Triggered for inst-013 (progressive-disclosure-over-inline, confidence 0.6, cycle 17). This instinct proposes reducing agent prompt duplication by referencing shared definitions, but it has never been applied as a task. It is a capability gap signal: the pattern was detected and learned, but the loop has not acted on it in 4 cycles.

**Capability gap signal filed:** inst-013 has been uncited for cycles 18-21 (4 cycles). Surface as a task candidate (source: capability-gap).

**heuristic: learn processReward = 0.8 sustained** — The learn score has been 0.8 for cycles 17-20 (all four cycles). While 0.8 does not trigger the remediation loop (threshold is 0.7 for 2+ consecutive cycles), it is structurally stuck due to the rubric definition. The loop cannot self-diagnose this because the remediation trigger is < 0.7, and learn never drops below 0.8 when instincts are extracted. This is a design gap.

---

## Selected Tasks

### Task 1: Add instinct citation tracking to agent output format
- **Slug:** add-instinct-citation-tracking
- **Type:** feature
- **Complexity:** M
- **Source:** codebase analysis (learning mechanics gap)
- **Rationale:** The most impactful unimplemented learning improvement. Without citation tracking, confidence scores are passive (based on re-observation) not active (based on application + outcome). Adding a structured `instinctsApplied` list to scout-report.md and build-report.md output formats, and updating Phase 5 to use citation lists when updating confidence, closes the fundamental gap in the learning loop. The learn processReward can then distinguish quality extraction (cited + applied) from quantity extraction (any YAML entry). This directly addresses why learn is capped at 0.8 and why instinct confidence updates are imprecise.
- **Acceptance Criteria:**
  - [ ] `scout-report.md` output format in `agents/evolve-scout.md` includes an `instinctsApplied` field listing inst IDs and application context
  - [ ] `build-report.md` output format in `agents/evolve-builder.md` includes an `instinctsApplied` field
  - [ ] Phase 5 LEARN in `skills/evolve-loop/phases.md` reads citation lists from workspace files when updating confidence
  - [ ] Phase 5 learn processReward rubric updated: score 1.0 if instincts extracted AND at least one instinct was cited in scout-report or build-report
- **Files to modify:**
  - `agents/evolve-scout.md` (add instinctsApplied to output schema)
  - `agents/evolve-builder.md` (add instinctsApplied to output schema)
  - `skills/evolve-loop/phases.md` (update Phase 5 confidence update + learn rubric)
- **Eval:** written to `evals/add-instinct-citation-tracking.md`
- **Eval Graders** (inline — Builder reads these directly):
  - `grep -n "instinctsApplied" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-scout.md` → expects match
  - `grep -n "instinctsApplied" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-builder.md` → expects match
  - `grep -n "instinctsApplied\|citation" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md` → expects match
  - `grep -n "cited" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md` → expects match (learn rubric update)

---

### Task 2: Fix learn processReward rubric to reward citation quality
- **Slug:** fix-learn-reward-rubric
- **Type:** stability
- **Complexity:** S
- **Source:** codebase analysis (learn score structurally capped)
- **Rationale:** The current learn rubric structurally caps at 0.8 for 2 out of 3 cycles because it conditions score 1.0 on consolidation running. Consolidation is a maintenance operation (every 3 cycles), not a learning quality signal. The rubric should reward learning quality — did the loop extract actionable instincts that get applied? Fixing the rubric is a prerequisite for the remediation loop to ever detect real learn degradation, and it enables a more accurate processRewardsHistory trend. This is an S task: one table row edit in phases.md plus updating the rubric notes.
- **Acceptance Criteria:**
  - [ ] learn rubric in phases.md updated: score 1.0 if instincts extracted AND at least one was cited in this cycle's reports (after Task 1 lands) OR consolidation ran
  - [ ] learn rubric score 0.5 defined for: instincts extracted, none cited this cycle
  - [ ] A comment in the rubric explains the change rationale (one line)
  - [ ] Rubric table in phases.md still has 3 columns (1.0 / 0.5 / 0.0)
- **Files to modify:**
  - `skills/evolve-loop/phases.md` (learn row in processRewards scoring rubric table)
- **Eval:** written to `evals/fix-learn-reward-rubric.md`
- **Eval Graders** (inline):
  - `grep -A3 "learn.*Score" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md | grep -i "cited\|citation\|applied"` → expects match
  - `grep -c "0.5" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md` → expects >= 1 (0.5 row present in table)

---

### Task 3: Apply inst-013 — extract shared strategy table from agent prompts
- **Slug:** apply-inst-013-strategy-deduplication
- **Type:** techdebt
- **Complexity:** M
- **Source:** capability-gap (inst-013 uncited for 4 cycles, confidence 0.6)
- **Rationale:** inst-013 (progressive-disclosure-over-inline) has been in the instinct system since cycle 17 but never activated. It proposes that agent prompts should reference shared strategy/output schema definitions instead of duplicating them inline. Currently, the Strategy Handling section in evolve-scout.md says "See SKILL.md Strategy Presets table for definitions" — this is already the right pattern. But evolve-builder.md and evolve-auditor.md each contain inline strategy handling text. Applying inst-013 to these agents will reduce prompt size (directly improving skillEfficiency), confirm the instinct, raise its confidence from 0.6 to 0.7, and close the capability gap. The instinct activation itself is the learning signal: by applying it and measuring the token delta, the loop validates whether the instinct's predicted value (80-120 line savings) is accurate.
- **Acceptance Criteria:**
  - [ ] `agents/evolve-builder.md` strategy section replaces inline strategy text with a reference to SKILL.md strategy presets table
  - [ ] `agents/evolve-auditor.md` strategy section does the same
  - [ ] Line counts for builder and auditor decrease (net reduction >= 10 lines total)
  - [ ] SKILL.md still contains the strategy presets table (not removed)
  - [ ] inst-013 confidence updated to 0.7 in a new cycle-21-instincts.yaml file
- **Files to modify:**
  - `agents/evolve-builder.md`
  - `agents/evolve-auditor.md`
  - `.claude/evolve/instincts/personal/cycle-21-instincts.yaml` (new file — inst-013 confidence update)
- **Eval:** written to `evals/apply-inst-013-strategy-deduplication.md`
- **Eval Graders** (inline):
  - `grep -c "SKILL.md\|strategy presets\|Strategy Presets" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-builder.md` → expects >= 1
  - `grep -c "SKILL.md\|strategy presets\|Strategy Presets" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-auditor.md` → expects >= 1
  - `wc -l /Users/danleemh/ai/claude/evolve-loop/agents/evolve-builder.md` → expects < 147 (current line count)
  - `test -f /Users/danleemh/ai/claude/evolve-loop/.claude/evolve/instincts/personal/cycle-21-instincts.yaml` → expects exit 0
  - `grep "confidence: 0.7" /Users/danleemh/ai/claude/evolve-loop/.claude/evolve/instincts/personal/cycle-21-instincts.yaml` → expects match

---

## Deferred

- **Add instinct verification harness (active testing):** The idea of actively testing instincts by running code with/without applying them is compelling but requires an experimental framework that doesn't exist yet. Deferred — too complex for this cycle.
- **Instinct quality score (semantic richness):** Could score instinct descriptions by specificity (does it name a file? a function? a pattern with conditions?). Interesting but would require a new measurement pass and risks over-engineering the learning phase. Deferred.
- **Instinct expiry on contradiction:** Mechanism to lower confidence when the loop tries an instinct-suggested approach and it fails. Currently only works passively via temporal decay. Deferred — needs failedApproaches cross-reference logic.
