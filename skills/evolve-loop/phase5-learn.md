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

3b. **Mandatory Instinct Extraction:**
   Every cycle MUST produce at least 1 instinct. No exceptions.
   For each task shipped this cycle, identify:
   1. What approach was used
   2. What the audit found
   3. What a future agent should do differently
   Write at least one instinct before continuing to step 4.

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

4c. **Failure Root Cause Attribution** (for FAILED tasks only):
   For each task that failed this cycle:
   1. **Classify** the error into one of 5 categories:
      - `planning` — wrong task scope or approach selected
      - `tool-use` — wrong tool invocation or misconfigured command
      - `reasoning` — incorrect conclusion or logic error
      - `context` — stale, missing, or wrong context provided
      - `integration` — individually correct steps combine incorrectly
   2. **Attribute** to the specific agent step from build-report.md `Build Steps` table
   3. **Record** in state.json `failedApproaches` with `errorCategory` and `failedStep` fields
   4. If 3+ failures share the same `errorCategory` across last 5 cycles → flag as systemic issue

   Update state.json `instinctCount` and `instinctSummary`:
   ```json
   "instinctSummary": [
     {"id": "inst-004", "pattern": "grep-based-evals", "confidence": 0.95, "type": "technique"},
     {"id": "inst-007", "pattern": "inline-s-tasks", "confidence": 0.9, "type": "process", "graduated": true}
   ]
   ```
   Scout and Builder read `instinctSummary` instead of all instinct YAML files. Full files read only during consolidation or when `instinctCount` changed.

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

### Strategy Playbook Update

**Playbook file:** `$WORKSPACE_PATH/strategy-playbook.md` (persists across cycles like builder-notes.md)

| Section | Contents | Updated When |
|---------|----------|-------------|
| What Worked | Successful approaches, effective patterns, high-yield task types | Task passes audit first attempt |
| What Failed | Failed approaches, root causes, proven alternatives | Task fails audit or benchmark regression |

**Update rules:** Append only. Preserve specific references (file paths, error messages, cycle numbers). Never rewrite wholesale. Cap at 50 entries per section — consolidate oldest 3 into 1 summary when exceeded.

---

### Post-Cycle Health (inline orchestrator — no Operator agent needed)

5. **Fitness computation** (inline):
   ```
   fitnessScore = round(0.25*discover + 0.30*build + 0.20*audit + 0.15*ship + 0.10*learn, 2)
   ```
   - Read process reward scores from this cycle
   - If decreased 2 consecutive cycles → `fitnessRegression: true` → HALT
   - Store in `fitnessHistory` (last 3)

6. **Next-cycle brief** (inline, deterministic):
   ```json
   {
     "weakestDimension": "<argmin of projectBenchmark.dimensions>",
     "recommendedStrategy": "<lookup: if weakest is defensiveDesign→harden, featureCoverage→innovate, else balanced>",
     "taskTypeBoosts": ["<dimension-to-taskType mapping>"],
     "avoidAreas": ["<files from failedApproaches>"],
     "cycle": "<N>"
   }
   ```
   Write to `$WORKSPACE_PATH/next-cycle-brief.json` and `.evolve/latest-brief.json`.

7. **Convergence check:** If `nothingToDoCount >= 1`, check `git log --oneline -3` for external changes. New work → reset to 0.

8. **Session summary** (`isLastCycle` only):
   Generate inline (tier-3): Key Patterns, Surprising Failures, What to Watch, Instincts Worth Reviewing. Write to `$WORKSPACE_PATH/session-summary.md`. Archive to `.evolve/history/cycle-{N}/`.

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
