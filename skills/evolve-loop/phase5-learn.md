> Read this file when orchestrating Phase 5 (LEARN). Covers workspace archival, instinct extraction and graduation, memory consolidation, operator check, and context management.

## Contents
- [Workspace Archival](#phase-5-learn-orchestrator-inline--operator) — copy workspace to history
- [Memory Consolidation Check](#memory-consolidation-check) — trigger conditions
- [Instinct Citation Collection](#instinct-citation-collection) — confidence updates from applied instincts
- [Instinct Extraction](#instinct-extraction) — pattern identification, YAML format, categories
- [Eval-Delta Prediction Tracking](#eval-delta-prediction-tracking) — Scout prediction calibration
- [Step-Level Process Reward Analysis](#step-level-process-reward-analysis) — Builder confidence cross-validation
- [Instinct Graduation](#instinct-graduation) — promotion to mandatory guidance
- [Counterfactual & Operator](#counterfactual--operator) — accuracy review, operator launch, notes
- [Meta-Cycle Dispatch](#meta-cycle-self-improvement-every-5-cycles) — conditional Phase 6
- [Context Checkpoint](#context-checkpoint-compaction-anchor) — handoff file, exit conditions

# Evolve Loop — Phase 5: LEARN

Orchestrator inline + operator. Meta-cycle self-improvement is in [phase6-metacycle.md](phase6-metacycle.md).

---

### Phase 5: LEARN (orchestrator inline + operator)

1. **Archive workspace:**
   ```bash
   mkdir -p .evolve/history/cycle-{N}
   cp $WORKSPACE_PATH/*.md .evolve/history/cycle-{N}/
   ```
   builder-notes.md persists across cycles (not cleared) so Phase 1 of the next cycle can read it.

2. **Memory Consolidation Check:**
   ```
   if (cycle % 3 === 0) OR (instinctCount > 20):
     run Memory Consolidation (see step below)
   ```

3. **Instinct Citation Collection:**
   - Read `instinctsApplied` from `scout-report.md` and `build-report.md`
   - Aggregate cited inst IDs into `citedInstincts` set
   - For each cited instinct, increase confidence by +0.05 (capped at 1.0)
   - Update `instinctSummary` in state.json

3b. **Instinct Extraction Trigger (forced extraction on stall):**
   ```
   recentZero = evalHistory.slice(-2).every(c => c.instinctsExtracted === 0)
   if (recentZero):
     run forced extraction prompt (MemRL/MemEvolve pattern):
       "For each of the last N cycle's tasks, identify:
        (1) what approach was used,
        (2) what the audit found,
        (3) what a future agent should do differently.
        Write at least one instinct per cycle."
     MUST produce >= 1 instinct before continuing
   ```

4. **Instinct Extraction:**
   Read ALL workspace files from this cycle. Identify patterns:

   | Pattern Type | Question |
   |-------------|----------|
   | Successful patterns | What approach worked? Why? Would it work again? |
   | Failed patterns | What didn't work? Root cause? How to avoid? |
   | Domain knowledge | What did we learn about this codebase? |
   | Process insights | Was task sizing right? Were evals effective? |

   Write instinct files to `.evolve/instincts/personal/`:
   ```yaml
   - id: inst-<NNN>
     pattern: "<short-name>"
     description: "<what was learned>"
     confidence: <0.5-1.0>  # starts at 0.5, increases with confirmation
     source: "cycle-<N>/<task-slug>"
     type: "anti-pattern|successful-pattern|convention|architecture|domain|process|technique"
     category: "episodic|semantic|procedural"
   ```

   | Category | Instinct Types |
   |----------|---------------|
   | Episodic | anti-pattern, successful-pattern (things that happened) |
   | Semantic | convention, architecture, domain (knowledge about the codebase) |
   | Procedural | process, technique (how to do things) |

   Each instinct must be specific and actionable. "Code should be clean" is useless. "This codebase uses barrel exports in index.ts — always add new exports there" is useful.

   Update state.json `instinctCount` and `instinctSummary`:
   ```json
   "instinctSummary": [
     {"id": "inst-004", "pattern": "grep-based-evals", "confidence": 0.95, "type": "technique"},
     {"id": "inst-007", "pattern": "inline-s-tasks", "confidence": 0.9, "type": "process", "graduated": true}
   ]
   ```
   Scout and Builder read `instinctSummary` instead of all instinct YAML files. Full files read only during consolidation or when `instinctCount` changed.

## Eval-Delta Prediction Tracking

Compare Scout's `Expected eval delta` predictions against actual benchmark changes. (Research: Eval-Driven Development — arXiv:2411.13768.)

1. Read Scout's `Expected eval delta` from scout-report.md for each shipped task
2. Read actual dimension delta from benchmark delta check
3. Compare:

| Result | Condition |
|--------|-----------|
| Accurate | Within +/-2 points |
| Over-predicted | Actual < predicted by >3 points |
| Under-predicted | Actual > predicted by >3 points |

4. Log in `state.json.evalDeltaAccuracy`:
   ```json
   {"cycle": "<N>", "task": "<slug>", "predicted": {"dimension": "+N"}, "actual": {"dimension": "+N"}, "accuracy": "accurate|over|under"}
   ```
5. After 5+ entries with >50% over-prediction rate, extract instinct: "Scout over-estimates impact of <task-type> on <dimension>"

## Step-Level Process Reward Analysis

Analyze Builder step-level confidence and Auditor cross-validation for targeted improvements. (Research: eval-harness process rewards.)

1. Read `## Build Steps` table from `build-report.md` — extract confidence scores and flags
2. Read CALIBRATION_MISMATCH entries from `audit-report.md` (Section D2)
3. Cross-reference for patterns

| Trigger | Action |
|---------|--------|
| Same step type confidence < 0.7 across 2+ cycles | Extract procedural instinct targeting that weakness |
| CALIBRATION_MISMATCH across 2+ cycles | Extract episodic instinct about overconfidence |

Append step-level summary to `state.json.processRewardsHistory` (keep last 5 cycles):
```json
{
  "cycle": "<N>",
  "steps": [
    {"description": "<step>", "builderConfidence": 0.8, "auditorIssue": false},
    {"description": "<step>", "builderConfidence": 0.6, "auditorIssue": true, "mismatchType": "overconfident"}
  ]
}
```
Meta-cycle reads `processRewardsHistory` to identify systematic Builder weaknesses.

## Instinct Graduation

Promotes high-confidence, repeatedly-confirmed instincts to mandatory guidance. Run after Citation Collection and before consolidation.

**Graduation threshold** — ALL three conditions:

| Condition | Requirement |
|-----------|-------------|
| Confidence | >= 0.75 |
| Confirmation | Cited in `instinctsApplied` across 3+ distinct cycles |
| No contradiction | Not contradicted by any `failedApproaches` entry |

**Operational effects:**

| Effect | Detail |
|--------|--------|
| Mark graduated | Set `"graduated": true` in `instinctSummary` |
| Builder treatment | Mandatory guidance — skip "should I apply?" evaluation |
| Scout priority | Listed first in context, attention priority over non-graduated |
| Confidence boost | +0.1 per subsequent citation (capped at 1.0) |
| Global promotion | Candidate for `~/.evolve/instincts/personal/` |

**Graduation reversal** — revert when quality evidence contradicts:
- If contradicted by 2+ consecutive build failures where instinct was applied → `graduated: false`, confidence -0.2
- If confidence drops below 0.5 → archive to `.evolve/instincts/archived/` with `archivedReason: "reversal"`
- Log in ledger as `type: "instinct-reversal"`

**Self-Evaluation (LLM-as-a-Judge):**
Model routing: tier-1 if audit retries > 1, eval failure, or calibration_error > 0.15 (richest learning signal). Otherwise inline with tier-2.

Score the cycle on 4 dimensions. For each, write chain-of-thought BEFORE scoring. Binary threshold: >= 0.7 = pass.

| Dimension | Guiding Questions | Threshold |
|-----------|------------------|-----------|
| **Correctness** | Did build produce intended behavior? Did evals pass? | >= 0.7 |
| **Completeness** | All acceptance criteria addressed? No missing edge cases? | >= 0.7 |
| **Novelty** | New patterns, techniques, or knowledge surfaced? | >= 0.7 |
| **Efficiency** | Tokens, attempts, file changes minimized? Scope right-sized? | >= 0.7 |

Scoring protocol:
1. **Stepwise Evidence Gathering (MANDATORY):** Enumerate 2-3 evidence items per dimension, assign mini-score (0.0-1.0) each.
2. Write 1-2 sentences chain-of-thought per dimension.
3. Final score = mean of evidence mini-scores.
4. If any dimension < 0.7: extract at least one instinct from that failure.

Record scores in `$WORKSPACE_PATH/build-report.md` under `## Self-Evaluation`.

**Gene Extraction:** If Builder fixed a recurring error pattern, extract as gene with selector, steps, validation. Write to `.evolve/genes/<gene-id>-<name>.yaml`. See [docs/genes.md](docs/genes.md).

**Instinct global promotion** (confidence >= 0.8, not project-specific):
1. Copy to `~/.evolve/instincts/personal/<instinct-id>.yaml`
2. Add `promotedFrom` field with project name and cycle
3. Log in ledger as `type: "instinct-promotion"`

**Memory Consolidation** (every 3 cycles or instinctCount > 20):

| Step | Action |
|------|--------|
| Cluster | Find instincts with semantic similarity > 0.85. Merge into higher-level abstraction. Confidence = max of originals. |
| Archive originals | Move merged instincts to `.evolve/instincts/archived/` with `supersededBy`. Never delete. |
| Temporal decay | Instincts not referenced in last 5 cycles: confidence -0.1 per pass. Below 0.3 → archive as stale. |
| Entropy gating | New instinct >90% similar to existing → update existing confidence instead of creating duplicate. |
| Write log | `$WORKSPACE_PATH/consolidation-log.md` with before/after counts. |

**Structured Distillation format** (arXiv:2603.13017 — 11x compression, 96% retrieval quality):
```json
{
  "exchange_core": "<key decisions and rationale>",
  "specific_context": "<concrete facts: files, errors, API shapes>",
  "thematic_assignments": "<which agents/phases this applies to>",
  "files_touched": ["<path/to/file1>"]
}
```

### Strategy Playbook Update (ACE-Inspired)

Based on Agentic Context Engineering (ACE, arXiv:2510.04618), the evolve-loop maintains a structured strategy playbook that grows incrementally across cycles. Unlike atomic instincts, the playbook organizes knowledge by domain section and uses a generation-reflection-curation (GRC) pipeline to prevent context collapse and brevity bias.

**Playbook file:** `$WORKSPACE_PATH/strategy-playbook.md` (persists across cycles like builder-notes.md)

**Playbook sections** (each section grows independently — never rewritten wholesale):

| Section | Contents | Updated When |
|---------|----------|-------------|
| Task Selection | Patterns for choosing high-impact tasks, complexity estimation heuristics | Scout discovers a selection pattern that worked/failed |
| File Handling | Fragile files, safe edit patterns, known coupling | Builder encounters file-specific issues |
| Eval Patterns | Effective vs tautological eval strategies, grader templates | Eval quality check flags issues or discovers effective patterns |
| Failure Modes | Recurring failure categories, root causes, proven alternatives | Any task fails audit or benchmark delta check |

**Incremental update protocol (anti-collapse safeguards):**

1. **Read the existing playbook section** (never start from scratch)
2. **Append new observations** as bullet points under the relevant section
3. **Preserve all specific references** (file paths, error messages, cycle numbers) — generalization loses actionable detail
4. **Never merge entries from different sections** — cross-section consolidation causes context collapse
5. **Cap consolidation ratio at 3:1** — merge at most 3 related bullets into 1 summary bullet, preserving the most specific example
6. **Never perform wholesale rewrites** — only append, refine individual bullets, or consolidate within a section

**Reflect-before-curate gate:**

Before extracting instincts (step 4), run an explicit reflection step:

1. **Reflect:** "What happened this cycle? What worked? What surprised me? What would I do differently?" — write 2-3 sentences of unstructured reflection
2. **Curate:** Based on the reflection, decide what to add to the playbook vs what to extract as an instinct vs what to discard
3. **Gate criterion:** Only add to the playbook observations that have appeared in 2+ cycles OR that caused a build failure. Single-cycle observations go to instincts (lower commitment) rather than the playbook (higher commitment)

This GRC pipeline ensures the playbook accumulates high-quality, validated knowledge while instincts handle exploratory, lower-confidence observations.

---

### Counterfactual & Operator

5. **Counterfactual Accuracy Review** (optional):
   For tasks with a `counterfactual` entry in `evaluatedTasks`, compare prediction to actual: complexity match, reward alignment, approach viability. Log instinct if clear pattern emerges.

6. **Operator Check:**
   Launch **Operator Agent** (model: tier-2 if isLastCycle, fitnessRegression, or cycle % 5 == 0; tier-3 if convergence-check or routine):
   - Context:
     ```json
     {
       "workspacePath": "<$WORKSPACE_PATH>",
       "runId": "<$RUN_ID>",
       "stateJson": "<state.json — includes ledgerSummary and instinctSummary>",
       "cycle": "<N>",
       "mode": "post-cycle|convergence-check",
       "recentLedger": "<last 5 ledger entries>",
       "recentNotes": "<last 5 cycle entries from notes.md>",
       "isLastCycle": "<true if remainingCycles == 0>"
     }
     ```
   - Operator reads `ledgerSummary` and `instinctSummary` from state.json instead of full files
   - In `"convergence-check"` mode: check `git log --oneline -3` for external changes. If new work → reset `nothingToDoCount` to 0
   - Writes `next-cycle-brief.json` to both `$WORKSPACE_PATH/` (run-local) and `.evolve/latest-brief.json` (shared)
   - If status `HALT` → pause and present issues to user
   - Cost check: if cycle >= `warnAfterCycles` → include warning in context
   - Update `lastCycleNumber` in state.json

6b. **Session Summary** (`isLastCycle` only):
   Operator writes `$WORKSPACE_PATH/session-learned.md` with: Key Patterns Discovered, Surprising Failures, What to Watch Next Session, Instincts Worth Reviewing (table), Session Snapshot. Archive to `.evolve/history/cycle-{N}/`.

6. **Update notes.md** (append under ship lock):
   ```markdown
   ## Cycle {N} ($RUN_ID) — {date}
   - **Tasks:** <list>
   - **Audit:** <verdict>
   - **Eval:** <passed/total>
   - **Shipped:** YES / NO
   - **Instincts:** <count> extracted
   - **Next cycle should consider:** <recommendations>
   ```

   **Notes Compression** (every 5 cycles, aligned with meta-cycle):
   1. Pre-compression flush: extract deferred tasks, unresolved blockers, recurring recommendations to state.json
   2. Compress entries older than 5 cycles into ~500-byte `## Summary` section
   3. Rewrite: `## Summary (cycles 1 through N-5)` + last 5 entries
   4. Full history preserved in `history/cycle-N/` archives
   5. Use tier-3 model for summarization

7. **Output cycle summary:**
   ```
   +------------------------------------------+
   | CYCLE {N} COMPLETE                       |
   +------------------------------------------+
   | Tasks:      <slug1> (PASS), <slug2> (FAIL)
   | Shipped:    <N>/<total attempted>
   | Audit:      <avg attempts per task> iterations
   | Benchmark:  <overall>/100 (delta +/-N)
   | Instincts:  <N> extracted, <N> graduated
   | Warnings:   <operator warnings or "none">
   | Next focus: <1-line from operator brief>
   +------------------------------------------+
   ```

### Meta-Cycle Self-Improvement (every 5 cycles)

If `cycle % 5 === 0`, run full meta-cycle evaluation. See [phase6-metacycle.md](phase6-metacycle.md). Skip on non-meta-cycles (saves ~4-6K tokens).

8. **Project Digest Generation** (cycle 1, or every 10 cycles during meta-cycle):
   Generate `.evolve/project-digest.md` (~2-3KB): Structure, Tech Stack, Hotspots, Conventions, Recent History. Scout reads this on cycle 2+ instead of full codebase scan.

9. **Context Checkpoint (compaction anchor):**
   Write `$WORKSPACE_PATH/handoff.md` (also `.evolve/workspace/handoff.md`): Session State, This Cycle, Carry Forward, Cumulative Stats.

   **Do NOT stop. Do NOT output a resume command. Do NOT wait for user input.** Continue immediately to next cycle.

   Under context pressure, reduce tokens by: using summaries from state.json, keeping workspace files concise, trimming agent context, activating lean mode early.

10. **Exit conditions:**
    - Cycle limit reached → STOP
    - Convergence (`stagnation.nothingToDoCount >= 3`) → STOP
    - Otherwise → next cycle
