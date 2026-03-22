# Evolve Loop — Phase 5: LEARN

Orchestrator inline + operator. This phase handles workspace archival, instinct extraction, memory consolidation, operator health checks, and context management. Meta-cycle self-improvement is in [phase6-metacycle.md](phase6-metacycle.md).

---

### Phase 5: LEARN (orchestrator inline + operator)

1. **Archive workspace:**
   ```bash
   mkdir -p .evolve/history/cycle-{N}
   cp $WORKSPACE_PATH/*.md .evolve/history/cycle-{N}/
   # builder-notes.md is included in *.md above; it is NOT cleared here so Phase 1 of the next cycle can read it
   ```

2. **Memory Consolidation Check:**
   Before extracting new instincts, check if consolidation is due:
   ```
   if (cycle % 3 === 0) OR (instinctCount > 20):
     → run Memory Consolidation (see step 3 below)
   ```
   This ensures consolidation runs at predictable intervals and prevents instinct bloat.

3. **Instinct Citation Collection:**
   Before extracting new instincts, collect citation lists from this cycle's workspace files:
   - Read `instinctsApplied` from `scout-report.md` and `build-report.md`
   - Aggregate cited inst IDs into a `citedInstincts` set for this cycle
   - For each cited instinct, increase its confidence by +0.05 (capped at 1.0) — application-driven confidence is more reliable than re-observation
   - Update `instinctSummary` in state.json with new confidence values

3b. **Instinct Extraction Trigger (forced extraction on stall):**
   Before running normal extraction, check if passive extraction has stalled:
   ```
   recentZero = evalHistory.slice(-2).every(c => c.instinctsExtracted === 0)
   if (recentZero):
     → run forced instinct extraction prompt (MemRL/MemEvolve pattern):
       "For each of the last N cycle's tasks, identify:
        (1) what approach was used,
        (2) what the audit found,
        (3) what a future agent should do differently — even under uniform success.
        Write at least one instinct per cycle. No new instincts = extraction stall."
     → this extraction block MUST produce ≥1 instinct before continuing to step 4
   ```
   This forces instinct generation when consecutive cycles with no new instincts
   signal that passive extraction has stalled under uniform success conditions.

4. **Instinct Extraction:**
   Read ALL workspace files from this cycle and think deeply about patterns:

   - **Successful patterns** — What approach worked? Why? Would it work again?
   - **Failed patterns** — What didn't work? What was the root cause? How to avoid it?
   - **Domain knowledge** — What did we learn about this specific codebase?
   - **Process insights** — Was the task sizing right? Were the evals effective?

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

   **Category assignment:**
   - Episodic: anti-pattern, successful-pattern (things that happened)
   - Semantic: convention, architecture, domain (knowledge about the codebase)
   - Procedural: process, technique (how to do things)

   **Think hard about instincts.** Each one should be specific enough to be actionable in future cycles. "Code should be clean" is useless. "This codebase uses barrel exports in index.ts files — always add new exports there" is useful.

   Update state.json `instinctCount`.

   **Update `instinctSummary` in state.json** (compact array so agents read summary instead of all YAML files):
   ```json
   "instinctSummary": [
     {"id": "inst-004", "pattern": "grep-based-evals", "confidence": 0.95, "type": "technique"},
     {"id": "inst-007", "pattern": "inline-s-tasks", "confidence": 0.9, "type": "process", "graduated": true}
   ]
   ```
   Scout and Builder read `instinctSummary` from state.json instead of reading all instinct YAML files. Full instinct files are only read during consolidation (every 3 cycles) or when `instinctCount` has changed since last cycle.

## Eval-Delta Prediction Tracking

Compare Scout's `Expected eval delta` predictions (from scout-report.md) against actual benchmark changes (from the delta check between Phase 3 and Phase 4). This feedback loop improves task selection over time. (Research basis: Eval-Driven Development — organizations using eval-guided task selection ship 5x faster; arXiv:2411.13768.)

**Steps:**
1. For each shipped task, read the Scout's `Expected eval delta` field from scout-report.md
2. Read the actual dimension delta from the benchmark delta check (if one was run this cycle)
3. Compare prediction vs actual:
   - **Accurate** (within ±2 points): Scout prediction is reliable for this task type
   - **Over-predicted** (actual < predicted by >3 points): Scout is too optimistic about this task type's impact
   - **Under-predicted** (actual > predicted by >3 points): Scout is too conservative — this task type is higher-impact than expected
4. Log prediction accuracy in `state.json` under `evalDeltaAccuracy`:
   ```json
   {"cycle": <N>, "task": "<slug>", "predicted": {"dimension": "+N"}, "actual": {"dimension": "+N"}, "accuracy": "accurate|over|under"}
   ```
5. After 5+ entries, compute Scout's prediction calibration. If over-prediction rate >50%, extract an instinct: "Scout over-estimates impact of <task-type> tasks on <dimension>"

## Step-Level Process Reward Analysis

After instinct extraction and before graduation, analyze the Builder's step-level confidence data from `build-report.md` and the Auditor's cross-validation from `audit-report.md`. This per-step analysis enables targeted Builder prompt improvements — not shotgun corrections. (Research basis: eval-harness process rewards — scoring intermediate steps yields finer learning signal than cycle-level evaluation.)

**Aggregation steps:**
1. Read the `## Build Steps` table from `build-report.md` — extract step descriptions, confidence scores, and any "Low-confidence step" flags
2. Read CALIBRATION_MISMATCH entries from `audit-report.md` (Section D2) — these are steps where Builder confidence diverged from Auditor findings
3. Cross-reference: which step types consistently show low confidence? Which show overconfidence (high confidence + Auditor issues)?

**Pattern extraction triggers:**
- If the same step type (e.g., "write eval graders") has confidence < 0.7 across 2+ recent cycles → extract a procedural instinct targeting that weakness (e.g., "When writing eval graders for documentation tasks, always include a coverage check for required sections")
- If a step type has CALIBRATION_MISMATCH across 2+ cycles → extract an episodic instinct about overconfidence in that area
- Feed step-level patterns into the meta-cycle's Agent Effectiveness Review (Section 7c) — the Efficiency Critic and Correctness Critic should reference step-level data when evaluating Builder performance

**State tracking:**
- Append step-level summary to `state.json` under `processRewardsHistory` (keep last 5 cycles):
  ```json
  {
    "cycle": <N>,
    "steps": [
      {"description": "<step>", "builderConfidence": 0.8, "auditorIssue": false},
      {"description": "<step>", "builderConfidence": 0.6, "auditorIssue": true, "mismatchType": "overconfident"}
    ]
  }
  ```
- The meta-cycle reads `processRewardsHistory` to identify systematic Builder weaknesses across 5-cycle windows

## Instinct Graduation

   Instinct graduation promotes high-confidence, repeatedly-confirmed instincts to mandatory guidance status. Run the graduation check during Phase 5 LEARN, after Instinct Citation Collection (step 3) and before consolidation — check all instincts with confidence >= 0.75 that are not yet graduated.

   **Graduation Threshold** — an instinct graduates when ALL three conditions are met:
   - Confidence >= 0.75
   - Confirmed across 3+ distinct cycles (cited in `instinctsApplied` by Scout or Builder in 3+ different cycle reports)
   - Not contradicted by any failed approach in `state.json` `failedApproaches`

   **Operational Effects of Graduation:**
   - Set `"graduated": true` on the instinct entry in `instinctSummary`
   - Builder treats graduated instincts as **mandatory guidance** — skip the "should I apply this?" evaluation and apply it directly
   - Scout lists graduated instincts first in context, giving them attention priority over non-graduated instincts
   - Each subsequent citation of a graduated instinct increases confidence by +0.1 (capped at 1.0)
   - Graduated instincts are candidates for global promotion (copied to `~/.evolve/instincts/personal/`)

   **Graduation Reversal** — revert graduation when quality evidence contradicts it:
   - If a graduated instinct is contradicted by 2+ consecutive build failures where the instinct was applied → set `graduated: false`, reduce confidence by 0.2
   - If confidence drops below 0.5 after reversal → archive the instinct (move to `.evolve/instincts/archived/` with `archivedReason: "reversal"`)
   - Log reversal in the ledger as `type: "instinct-reversal"`:
     ```json
     {"ts":"<ISO-8601>","cycle":<N>,"role":"orchestrator","type":"instinct-reversal","data":{"instinct":"<id>","reason":"2 consecutive failures","confidenceAfter":<value>}}
     ```

   **Self-Evaluation (LLM-as-a-Judge)** (after instinct extraction):
   Model routing: If this cycle had audit retries (auditAttempts > 1), any eval failure, or calibration_error > 0.15 → launch self-evaluation as a dedicated sub-task with **tier-1** model (problem cycles have the richest learning signal and need the deepest reflection to extract accurate instincts). Otherwise, run inline with the orchestrator model (tier-2).

   Score the cycle on 4 dimensions using a structured rubric. For each dimension, write a chain-of-thought justification BEFORE assigning the score. Binary threshold: ≥0.7 = pass, <0.7 = fail (triggers mandatory instinct extraction for that dimension).

   | Dimension | Guiding questions | Threshold |
   |-----------|------------------|-----------|
   | **Correctness** | Did the build produce the intended behavior? Did evals pass? | ≥0.7 |
   | **Completeness** | Were all acceptance criteria addressed? No missing edge cases? | ≥0.7 |
   | **Novelty** | Did the cycle surface new patterns, techniques, or knowledge? | ≥0.7 |
   | **Efficiency** | Were tokens, attempts, and file changes minimized? Was scope right-sized? | ≥0.7 |

   Scoring protocol:
   1. **Stepwise Evidence Gathering (MANDATORY):** For each dimension, MUST enumerate 2-3 evidence items per dimension before scoring. Assign a mini-score (0.0-1.0) to each evidence item based on observable outcomes. This per-step decomposition reduces anchoring bias and improves failure detection calibration by +15% AUC-ROC (arxiv 2511.07364, 2025). See `docs/self-learning.md` § Stepwise Confidence Scoring for the full protocol.
   2. Write 1–2 sentences of chain-of-thought reasoning per dimension.
   3. Assign a final score 0.0–1.0 as the mean of the evidence mini-scores.
   4. If any dimension scores <0.7: extract at least one instinct from that failure before moving on.

   Record scores in `$WORKSPACE_PATH/build-report.md` under a `## Self-Evaluation` heading.

   **Gene Extraction** (after instinct extraction):
   If the Builder successfully fixed a recurring error pattern this cycle:
   - Extract the fix as a gene with selector, steps, and validation commands
   - Write to `.evolve/genes/<gene-id>-<name>.yaml`
   - If multiple genes were applied in sequence, bundle as a capsule
   - See [docs/genes.md](docs/genes.md) for schema

   **Instinct global promotion** (check after every instinct extraction):
   For instincts with confidence >= 0.8 that are not project-specific:
   1. Copy to `~/.evolve/instincts/personal/<instinct-id>.yaml`
   2. Add `promotedFrom` field with project name and cycle
   3. Log promotion in the ledger as `type: "instinct-promotion"`

   **Memory Consolidation** (every 3 cycles or when instinctCount > 20):
   Review all instinct files and consolidate:

   a. **Cluster similar instincts:** Find instincts with overlapping patterns or descriptions (semantic similarity > 0.85). Merge them into a single higher-level abstraction.
      - Example: `inst-003: "use camelCase for API keys"` + `inst-007: "use camelCase for config fields"` → `inst-003: "use camelCase for all JSON keys in this codebase"` (confidence = max of originals)

   b. **Archive originals:** Move merged instincts to `.evolve/instincts/archived/` with a `supersededBy` field. Never delete — only archive.

   c. **Apply temporal decay:** Instincts not referenced in the last 5 cycles have their confidence reduced by 0.1 per consolidation pass. Instincts reaching confidence < 0.3 are archived as stale.

   d. **Entropy gating:** Before storing a new instinct, check if it adds meaningful information beyond what's already stored. If a new instinct is >90% similar to an existing one, update the existing one's confidence instead of creating a duplicate.

   e. **Write consolidation log** to `$WORKSPACE_PATH/consolidation-log.md`:
      ```markdown
      ## Memory Consolidation — Cycle {N}
      - Instincts before: <count>
      - Merged: <count> clusters
      - Decayed: <count>
      - Archived: <count>
      - Instincts after: <count>
      ```

   f. **Structured Distillation format for memory entries** (arXiv:2603.13017):
      When writing or merging instinct entries during consolidation, use the 4-field compound distillation format to maximise retrieval quality at minimum token cost. This format achieves 11x compression (371→38 tokens per entry) with 96% retrieval quality on downstream tasks.

      Each consolidated memory entry should capture:
      ```json
      {
        "exchange_core": "<key decisions and rationale from the cycles that produced this instinct>",
        "specific_context": "<concrete facts: file names, error messages, API shapes, config values>",
        "thematic_assignments": "<which agents or phases this instinct applies to>",
        "files_touched": ["<path/to/relevant/file1>", "<path/to/relevant/file2>"]
      }
      ```

      This format maps to distinct retrieval needs: `exchange_core` feeds reasoning steps; `specific_context` feeds implementation; `thematic_assignments` feeds coordination; `files_touched` feeds change-impact analysis. Apply this format when writing new instinct descriptions and when re-writing merged instincts — do not reformat existing instincts in bulk unless a consolidation pass is already running.

### Counterfactual & Operator

5. **Counterfactual Accuracy Review** (optional, shadow-run check):
   For any task completed this cycle that previously had a `counterfactual` entry in `evaluatedTasks`, compare the prediction to the actual outcome:
   - Did `predictedComplexity` match the actual complexity?
   - Did `estimatedReward` (predicted) align with the actual build outcome (PASS=1.0, FAIL=0.0, partial=0.5)?
   - Was the `alternateApproach` viable? (Did the Builder use a similar or different path?)

   Log accuracy notes as an instinct if a clear pattern emerges (e.g., "Scout consistently over-estimates complexity for config tasks"). No action required if no completed counterfactuals exist this cycle.

6. **Operator Check:**
   Launch **Operator Agent** (model: per routing table — tier-2 if isLastCycle (session synthesis needs quality narrative), tier-2 if fitnessRegression detected (HALT-worthy signal needs careful diagnosis), tier-2 if cycle % 5 == 0 (meta-cycle analysis is consequential), tier-3 if mode == "convergence-check" (simple state check), tier-3 otherwise (routine post-cycle); subagent_type: `general-purpose`):
   - Context:
     ```json
     {
       // --- Static ---
       "workspacePath": "<$WORKSPACE_PATH>",
       "runId": "<$RUN_ID>",
       // --- Semi-stable ---
       "stateJson": <state.json contents — includes ledgerSummary and instinctSummary>,
       // --- Dynamic ---
       "cycle": <N>,
       "mode": "post-cycle|convergence-check",
       "recentLedger": "<last 5 ledger entries, inline>",
       "recentNotes": "<last 5 cycle entries from notes.md, inline>",
       "isLastCycle": <true if remainingCycles == 0, false otherwise>
     }
     ```
   - Operator reads `ledgerSummary` and `instinctSummary` from state.json instead of full ledger/instinct files.
   - In `"convergence-check"` mode: Operator checks for external changes (`git log --oneline -3`), new issues, or changed project state. If new work detected, reset `nothingToDoCount` to 0.
   - Operator assesses: Did we ship? Are we stalling? Cost concerns? Recommendations?
   - Operator writes `next-cycle-brief.json` to both:
     - `$WORKSPACE_PATH/next-cycle-brief.json` (run-local, for intra-run cycles)
     - `.evolve/latest-brief.json` (shared, last-writer-wins — consumed by other parallel runs)
     Contains `weakestDimension`, `recommendedStrategy`, `taskTypeBoosts`, `avoidAreas`, and `cycle` — consumed by Scout in Phase 1 of the next cycle.
   - If status is `HALT` → pause and present issues to user

   **Cost awareness check** (inline, before launching Operator):
   - If current cycle number >= `warnAfterCycles` (from state.json, default 5): include warning in Operator context

   **Update lastCycleNumber** in state.json to the current cycle number after each cycle completes.

6b. **What the Loop Learned This Session** (operator writes at session end):

   When `isLastCycle` is true, the Operator writes a human-readable session summary to `$WORKSPACE_PATH/session-learned.md`. This file is for human consumption — written so an operator can read it and understand what the LLM learned without digging through ledger entries or instinct YAML files.

   Template sections: `Key Patterns Discovered`, `Surprising Failures`, `What to Watch Next Session`, `Instincts Worth Reviewing` (table: ID, Pattern, Reason), `Session Snapshot` (cycles, tasks shipped, instincts, fitness delta). Cross-reference: [docs/human-learning-guide.md](../../docs/human-learning-guide.md) section 6.

   **When to write:** Only on `isLastCycle: true`. Skip on mid-session cycles to avoid stale files.
   **Archive:** Copy `session-learned.md` to `.evolve/history/cycle-{N}/session-learned.md` alongside the other workspace files.

6. **Update notes.md** (rolling window — keeps file size bounded):

   Append the new cycle entry (under ship lock in Phase 4, so no concurrent writes):
   ```markdown
   ## Cycle {N} ($RUN_ID) — {date}
   - **Tasks:** <list of what was built>
   - **Audit:** <verdict>
   - **Eval:** <passed/total>
   - **Shipped:** YES / NO
   - **Instincts:** <count> extracted
   - **Next cycle should consider:** <recommendations>
   ```

   **Notes Compression** (every 5 cycles, aligned with meta-cycle):
   If `cycle % 5 === 0`:
   1. **Pre-compression memory flush** (inspired by OpenClaw's pre-compaction flush):
      Before compressing, extract durable items from old entries into state.json:
      - Deferred tasks → add to `evaluatedTasks` with `decision: "deferred"` and `revisitAfter`
      - Unresolved decisions/blockers → add to `operatorWarnings`
      - Recurring recommendations → validate they're captured in instincts
      This prevents information loss that a ~500-byte summary can't capture.
   2. Compress entries older than 5 cycles into a fixed-size `## Summary` section at the top (~500 bytes: total tasks shipped, key milestones, count of active deferred items)
   3. Rewrite notes.md with: `## Summary (cycles 1 through N-5)` + last 5 cycle entries only
   4. Full history is preserved in `history/cycle-N/` archives
   5. Use tier-3 model for the compression summarization (it's a straightforward summarization task)

   This caps notes.md at ~5KB regardless of cycle count.

7. **Output cycle summary:**
   ```
   ┌─────────────────────────────────────────┐
   │ CYCLE {N} COMPLETE                      │
   ├─────────────────────────────────────────┤
   │ Tasks:      <slug1> (PASS), <slug2> (FAIL)
   │ Shipped:    <N>/<total attempted>
   │ Audit:      <avg attempts per task> iterations
   │ Benchmark:  <overall>/100 (Δ +/-N)
   │ Instincts:  <N> extracted, <N> graduated
   │ Warnings:   <operator warnings or "none">
   │ Next focus: <1-line from operator brief>
   └─────────────────────────────────────────┘
   ```

   This summary is the **primary user-facing output per cycle**. It must be concise enough to scan but informative enough to track session progress without reading workspace files.

### Meta-Cycle Self-Improvement (every 5 cycles)

7. **Meta-Cycle Dispatch** (conditional):
   If `cycle % 5 === 0`, run the full meta-cycle evaluation. For detailed instructions, see [phase6-metacycle.md](phase6-metacycle.md).

   The meta-cycle covers: split-role critique (Efficiency/Correctness/Novelty), agent effectiveness review, automated prompt evolution, skill synthesis, mutation testing, and topology review.

   **Skip on non-meta-cycles** — this saves ~4-6K tokens per cycle by not loading meta-cycle logic.

8. **Project Digest Generation** (cycle 1, or every 10 cycles during meta-cycle):

   Generate `.evolve/project-digest.md` (shared location, ~2-3KB) with sections: `Structure` (directory tree, 2 levels deep), `Tech Stack` (language, framework, test/build commands), `Hotspots` (high fan-in files, largest files, most churn — high-impact targets for Scout), `Conventions` (naming, file org, exports), `Recent History` (git log --oneline -10).

   On cycle 1 (`mode: "full"`): Scout generates this after full codebase scan.
   On cycle 2+: Scout reads this instead of re-scanning. Only changed files (from `changedFiles`) are read directly.

9. **Context Checkpoint (compaction anchor):**

   After each cycle completes, write a **cycle handoff file** to `$WORKSPACE_PATH/handoff.md` (also copy to `.evolve/workspace/handoff.md` for backward compat) as a safety checkpoint and compaction anchor. Sections: `Session State` (cycles completed/remaining, strategy, benchmark), `This Cycle` (tasks, audit iterations, instincts), `Carry Forward` (stagnation, warnings, cooldowns, priorities), `Cumulative Session Stats` (shipped/failed, benchmark trajectory, active instincts).

   This handoff serves as the **compaction anchor** — when the host LLM auto-compacts conversation history, this structured summary ensures cross-cycle continuity survives summarization.

   **IMPORTANT: Do NOT stop. Do NOT output a resume command. Do NOT wait for user input.** The handoff file is a checkpoint only — if the session is externally interrupted, a new session can read it to resume. The orchestrator MUST continue immediately to the next cycle.

   If context window pressure is high, reduce token usage by:
   - Using `instinctSummary` and `ledgerSummary` from state.json instead of reading full files
   - Keeping workspace files concise
   - Trimming agent context to essential fields only
   - Activating lean mode behaviors (see phases.md) even before cycle 4 if needed

10. **Exit conditions** (in order):
   - Cycle limit reached → STOP
   - Convergence (`stagnation.nothingToDoCount >= 3`) → STOP
   - Otherwise → next cycle
