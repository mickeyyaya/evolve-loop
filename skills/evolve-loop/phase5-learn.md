# Evolve Loop — Phase 5: LEARN

Orchestrator inline + operator. This phase handles workspace archival, instinct extraction, memory consolidation, operator health checks, meta-cycle self-improvement, and context management.

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

   Template:

   ```markdown
   # What the Loop Learned This Session — Cycle {N} / Run {RUN_ID}

   ## Key Patterns Discovered
   <!-- Key patterns that emerged across multiple cycles this session.
        Each entry should be a concrete, reusable insight — not a one-off observation. -->
   - <key pattern 1: what was discovered and why it matters>
   - <key pattern 2: ...>

   ## Surprising Failures
   <!-- Tasks or approaches that failed unexpectedly — surprising given the task difficulty,
        prior instinct guidance, or eval setup. Include root cause if known. -->
   - <surprising failure 1: what failed, why it was unexpected, what we learned>
   - <surprising failure 2: ...>

   ## What to Watch Next Session
   <!-- Areas of concern, fragile files, or patterns worth monitoring.
        These are NOT tasks — they are signals the Scout should factor into selection. -->
   - <watch item 1>
   - <watch item 2>

   ## Instincts Worth Reviewing
   <!-- Instincts that were frequently cited, newly graduated, reversed, or contradicted this session.
        Include the instinct ID and a plain-language reason to review it. -->
   | Instinct ID | Pattern | Reason to Review |
   |-------------|---------|-----------------|
   | inst-<NNN> | <pattern name> | <cited N times / newly graduated / contradicted / reversed> |

   ## Session Snapshot
   - Cycles run: <N>
   - Tasks shipped: <X> / <Y> attempted
   - Instincts added: <N> | Graduated: <N> | Archived: <N>
   - Fitness delta: <start> → <end> (<+/- N>)
   ```

   Cross-reference: see [docs/human-learning-guide.md](../../docs/human-learning-guide.md) section 6 ("How to Learn from the Loop") for guidance on interpreting patterns and what makes a finding actionable for human operators.

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

### Meta-Cycle Self-Improvement & Context Management

7. **Meta-Cycle Self-Improvement** (every 5 cycles):
   If `cycle % 5 === 0`, run a meta-evaluation of the evolve-loop's own effectiveness:

   a. **Collect metrics** from the last 5 cycles in `evalHistory` and `ledger.jsonl`:
      - Tasks shipped vs attempted (success rate)
      - Average audit iterations per task (Builder efficiency)
      - Stagnation pattern count
      - Instinct confidence trend (are instincts getting confirmed?)

   b. **Split-role critique** — use three specialized critic perspectives to avoid blind spots:

      | Critic | Focus | Key Question |
      |--------|-------|-------------|
      | **Efficiency Critic** | Cost, token usage, task sizing, model routing | "Are we spending tokens wisely? Could tasks be smaller?" |
      | **Correctness Critic** | Eval pass rates, audit verdicts, regression trends | "Are we shipping quality code? Are evals catching issues?" |
      | **Novelty Critic** | Instinct diversity, task variety, stagnation patterns | "Are we learning new things? Or repeating the same work?" |

      Each critic reviews the last 5 cycles independently and produces 1-3 findings. The orchestrator synthesizes findings into the meta-review, resolving conflicts by prioritizing correctness > efficiency > novelty.

   c. **Evaluate agent effectiveness** — for each agent, ask:
      - Scout: Are selected tasks the right size? Are they shipping?
      - Builder: How many attempts per task? What's the self-verify pass rate?
      - Auditor: Are WARN/FAIL verdicts being resolved or accumulating?
      - Operator: Are recommendations being followed?

   c. **Propose improvements** — write a `meta-review.md` to the workspace:
      ```markdown
      # Meta-Cycle Review — Cycles {N-4} to {N}

      ## Pipeline Metrics
      - Success rate: X/Y tasks (Z%)
      - Avg audit iterations: N
      - Stagnation patterns: N active
      - Instinct trend: growing/stable/stale

      ## Agent Effectiveness
      | Agent | Assessment | Suggested Change |
      |-------|-----------|-----------------|
      | Scout | ... | ... |
      | Builder | ... | ... |
      | Auditor | ... | ... |
      | Operator | ... | ... |

      ## Recommended Changes
      1. <specific change to agent prompt, strategy, or process>
      ```

   d. **Automated Prompt Evolution** — based on meta-review findings, the orchestrator may refine agent prompts using a critique-synthesize loop:

      1. **Critique:** Identify specific weaknesses in agent prompts based on cycle outcomes. For example, if the Builder frequently needs 3 attempts, its design step may need stronger guidance.
      2. **Synthesize:** Propose specific prompt edits (additions, rewording, new examples) that address the weakness. Each edit must be small and targeted — do not rewrite entire agent definitions.
      3. **Validate:** Before applying, check that the proposed edit doesn't contradict existing instincts or orchestrator policies.
      4. **Apply:** Make the edit to the agent file. Log the change in the meta-review with before/after and rationale.
      5. **Track:** Add a `prompt-evolution` entry to the ledger:
         ```json
         {"ts":"<ISO-8601>","cycle":<N>,"role":"orchestrator","type":"prompt-evolution","data":{"agent":"<name>","section":"<section changed>","rationale":"<why>","change":"<summary>"}}
         ```

      **TextGrad-style optimization:** For each proposed edit, generate a "textual gradient" — a natural language critique describing:
      - What the current prompt produces (observed behavior)
      - What it should produce (desired behavior)
      - The specific text change that bridges the gap (the "gradient")
      - Expected impact on process rewards for the affected phase

      This is more rigorous than free-form critique. The gradient must reference specific prompt text and specific cycle outcomes.

      **Safety constraints:**
      - Only modify non-structural sections (guidance, examples, strategy handling) — never change the agent's tools, model, or core responsibilities
      - Maximum 2 prompt edits per meta-cycle
      - All edits are committed and can be reverted with `git revert`
      - If an evolved prompt leads to worse performance in the next meta-cycle, auto-revert the change

   d2. **Skill Synthesis Check** (meta-cycle only — after prompt evolution, before mutation testing):

      After the split-role critique and prompt evolution, check whether any instinct clusters are ready to graduate into executable artifacts. This closes the loop between learning and capability expansion — instincts become structurally durable genes or skill fragments rather than passive hints. (Research basis: continuous-learning-v2 instinct→skill graduation, self-learning-agent-patterns pattern graduation pipelines.)

      **Step 1: Identify synthesis candidates**
      Query `instinctSummary` in state.json for clusters of 3+ instincts that meet ALL criteria:
      - Same `category` (all procedural, all semantic, or all episodic)
      - Related patterns (overlapping keywords, shared target file types, or similar domain)
      - All confidence >= 0.8
      - None marked `graduated: true` already (avoid re-synthesizing)

      **Step 2: Determine synthesis target** for each qualifying cluster:

      **Option A — Synthesize a Gene** (when instincts describe a repeatable fix/implementation pattern):
      Write to `.evolve/genes/<pattern-name>.md`:
      ```markdown
      # Gene: <pattern-name>
      <!-- Synthesized from instincts: inst-012, inst-017, inst-023 at cycle N -->

      ## Selector
      When to apply: <conditions under which this gene matches — task type, file patterns, error signatures>

      ## Steps
      1. <concrete implementation step>
      2. <concrete implementation step>
      3. <concrete implementation step>

      ## Validation
      - `<bash command to verify the gene was applied correctly>`
      - `<bash command to check for regressions>`

      ## Source Instincts
      - inst-012: <pattern summary>
      - inst-017: <pattern summary>
      - inst-023: <pattern summary>
      ```

      **Option B — Synthesize a Skill Fragment** (when instincts describe agent behavior improvements):
      Propose an addition to the relevant agent's prompt. Gate through the same TextGrad validation used for prompt evolution:
      1. Draft the skill fragment as a new markdown section (max 20 lines)
      2. Generate a textual gradient: current behavior → desired behavior → text change
      3. Validate against existing instincts and policies (no contradictions)
      4. Apply via the prompt evolution mechanism (counts toward the 2-edit-per-meta-cycle limit)

      **Step 3: Record synthesis** in `state.json.synthesizedTools`:
      ```json
      {
        "name": "<pattern-name>",
        "path": ".evolve/genes/<pattern-name>.md",
        "purpose": "<one-line description>",
        "sourceInstincts": ["inst-012", "inst-017", "inst-023"],
        "cycle": <N>,
        "useCount": 0,
        "type": "gene|skill-fragment"
      }
      ```

      **Step 4: Update source instincts**
      - Mark all source instincts as `graduated: true` in `instinctSummary`
      - These instincts stop decaying (confidence is preserved) but are no longer individually injected — the gene/skill fragment subsumes them

      **Safety constraints:**
      - Maximum 1 synthesis per meta-cycle (avoid bulk generation of untested artifacts)
      - Synthesized genes must pass the eval-quality-check rigor filter (no trivial validation commands)
      - Skill fragments are subject to the same auto-revert rule as prompt evolution edits
      - Log synthesis in the ledger:
        ```json
        {"ts":"<ISO-8601>","cycle":<N>,"role":"orchestrator","type":"skill-synthesis","data":{"name":"<pattern-name>","type":"gene|skill-fragment","sourceInstincts":["inst-012","inst-017","inst-023"]}}
        ```

   e. **Self-Generated Evaluation (mutation testing):**

      Test the quality of our evals by generating mutations:
      1. For each task completed in the last 5 cycles, generate 2-3 small code mutations (e.g., remove a validation, change a return value, delete an import)
      2. Run the existing eval graders against the mutated code
      3. If evals DON'T catch a mutation → the eval is weak. Propose stronger eval criteria.
      4. Track **mutation kill rate** (mutations caught / mutations generated)

      ```markdown
      ## Mutation Testing Results
      - Mutations generated: <N>
      - Mutations killed (caught by evals): <N>
      - Kill rate: <percentage>
      - Weak evals identified: <list>
      - Proposed improvements: <list>
      ```

      Target: >80% mutation kill rate. Below 60% triggers eval improvement as a priority task in the next cycle.

   f. **Workflow Topology Review:**

      Evaluate whether the current phase ordering and agent configuration is optimal:

      1. **Phase skip analysis** — were any phases redundant this meta-cycle? (e.g., Auditor always PASS → consider lighter audit)
      2. **Phase merge candidates** — could two phases be combined? (e.g., if Builder self-verify is reliable, reduce Auditor scope)
      3. **Phase addition candidates** — is there a gap? (e.g., if security issues keep recurring, add a dedicated security scan phase)
      4. **Parallel opportunities** — could independent tasks be built in parallel instead of sequentially?

      Propose topology changes in the meta-review:
      ```markdown
      ## Topology Recommendations
      - **Current:** DISCOVER → BUILD → AUDIT → SHIP → LEARN
      - **Proposed:** DISCOVER → BUILD(parallel) → AUDIT(light) → SHIP → LEARN
      - **Rationale:** <why this change would improve performance>
      ```

      **Safety:** Topology changes are proposals only — they require human approval before the orchestrator applies them. Never auto-apply topology changes.

   g. **Apply remaining changes** — update default strategy, token budgets, or other configuration based on meta-review findings. Archive the `meta-review.md` to history.

   h. **Regenerate project digest** — during meta-cycle (every 5 cycles), regenerate `.evolve/project-digest.md` (shared location) to capture any structural changes.

8. **Project Digest Generation** (cycle 1, or every 10 cycles during meta-cycle):

   Generate `.evolve/project-digest.md` (shared location, ~2-3KB):
   ```markdown
   # Project Digest — Generated Cycle {N}

   ## Structure
   <project directory tree with file sizes, max 2 levels deep>

   ## Tech Stack
   - Language: <detected>
   - Framework: <detected>
   - Test command: <detected>
   - Build command: <detected>

   ## Hotspots
   <files with highest fan-in: most imported/referenced by other files>
   <largest files by line count>
   <files with most recent churn: git log --format='%H' --follow -- <file> | wc -l>
   These are high-impact targets for Scout task selection — changes here have large blast radius.

   ## Conventions
   <key patterns detected: naming, file org, exports, etc.>

   ## Recent History
   <git log --oneline -10>
   ```

   On cycle 1 (`mode: "full"`): Scout generates this after full codebase scan.
   On cycle 2+: Scout reads this instead of re-scanning. Only changed files (from `changedFiles`) are read directly.

9. **Context Checkpoint (compaction anchor):**

   After each cycle completes, write a **cycle handoff file** to `$WORKSPACE_PATH/handoff.md` (also copy to `.evolve/workspace/handoff.md` for backward compat) as a safety checkpoint and compaction anchor:
   ```markdown
   # Cycle Handoff — Cycle {N}

   ## Session State
   - Cycles completed: <N> | Remaining: <M>
   - Strategy: <strategy> | Goal: <goal or null>
   - Benchmark: <overall>/100 (delta: +/-N from session start)

   ## This Cycle
   - Tasks: <slug1> (PASS), <slug2> (PASS), <slug3> (FAIL)
   - Audit iterations: <avg attempts per task>
   - Instincts extracted: <N>

   ## Carry Forward
   - Stagnation: <patterns or "none">
   - Operator warnings: <list or "none">
   - Research cooldowns: <expiry times>
   - Next cycle priorities: <from operator brief>

   ## Cumulative Session Stats
   - Total shipped: <N> | Failed: <N>
   - Benchmark trajectory: <start> → <current>
   - Active instincts: <count> (last extracted: cycle <N>)
   ```

   This handoff serves as the **compaction anchor** — when the host LLM auto-compacts conversation history, this structured summary ensures cross-cycle continuity survives summarization. All essential state is captured so the orchestrator can continue without re-reading prior cycle history.

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
