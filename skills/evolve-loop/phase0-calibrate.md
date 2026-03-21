# Evolve Loop — Phase 0: CALIBRATE

Runs **once per `/evolve-loop` invocation**, not per cycle. Executes before the first Scout runs. Establishes a project-level benchmark baseline so tasks can be measured against project quality, not just process quality.

## Calibration Deduplication

Before running calibration, check if a recent calibration already exists:
```
if state.json.projectBenchmark.lastCalibrated exists
   AND (now - lastCalibrated) < 1 hour:
   Skip CALIBRATE, use existing benchmark
else:
   Run CALIBRATE normally
```
This prevents redundant benchmark scoring when multiple parallel runs start close together.

## Execution Steps

1. **Run automated checks** from [benchmark-eval.md](benchmark-eval.md):
   Execute all bash check commands for each of the 8 dimensions. Capture per-dimension automated scores (0-100).

   ```bash
   # Run each dimension's automated checks from benchmark-eval.md
   # Store results in $WORKSPACE_PATH/benchmark-automated.json
   ```

2. **Run LLM judgment pass** (model: per routing table — tier-2 for first calibration of session, tier-3 for subsequent):
   For each dimension, provide the LLM with:
   - The dimension's rubric from benchmark-eval.md
   - A sample of relevant files (max 3 files per dimension, <200 lines each)
   - The automated score for context

   The LLM outputs a score (0/25/50/75/100) with a 1-sentence justification and a confidence score (0.0-1.0). Use the anchored rubric — scores MUST match one of the anchor points exactly. Actively resist "verbosity bias" (assuming longer files are better).

3. **Compute per-dimension composite scores:**
   ```
   dimension.composite = round(0.7 * dimension.automated + 0.3 * dimension.llm)
   ```

4. **Compute overall score:**
   ```
   overall = round(mean(all 8 dimension composites), 1)
   ```

5. **Store in state.json** under `projectBenchmark`:
   ```json
   {
     "projectBenchmark": {
       "lastCalibrated": "<ISO-8601>",
       "calibrationCycle": <lastCycleNumber + 1>,
       "overall": <0-100>,
       "dimensions": {
         "documentationCompleteness": {"automated": <N>, "llm": <N>, "composite": <N>},
         "specificationConsistency": {"automated": <N>, "llm": <N>, "composite": <N>},
         "defensiveDesign": {"automated": <N>, "llm": <N>, "composite": <N>},
         "evalInfrastructure": {"automated": <N>, "llm": <N>, "composite": <N>},
         "modularity": {"automated": <N>, "llm": <N>, "composite": <N>},
         "schemaHygiene": {"automated": <N>, "llm": <N>, "composite": <N>},
         "conventionAdherence": {"automated": <N>, "llm": <N>, "composite": <N>},
         "featureCoverage": {"automated": <N>, "llm": <N>, "composite": <N>}
       },
       "history": [],
       "highWaterMarks": {}
     }
   }
   ```

6. **Compare to previous calibration** (if `projectBenchmark.history` is non-empty):
   - Append the previous calibration to `history` (keep last 5 entries)
   - Identify the 2-3 dimensions with the lowest composite scores → these are `benchmarkWeaknesses`
   - **High-water mark tracking:** For each dimension at 80+, record in `highWaterMarks`. If any dimension regresses below `(HWM - 10)`, add a mandatory remediation task to `pendingImprovements`

7. **Write `$WORKSPACE_PATH/benchmark-report.md`:**
   ```markdown
   # Project Benchmark — Calibration at Cycle {lastCycleNumber + 1}

   ## Overall Score: {overall}/100

   | Dimension | Automated | LLM | Composite | Delta |
   |-----------|-----------|-----|-----------|-------|
   | Documentation Completeness | X | X | X | +/-N |
   | ... | ... | ... | ... | ... |

   ## Weakest Dimensions
   1. <dimension> (score: X) — <1-sentence diagnosis>

   ## High-Water Mark Regressions
   - <dimension>: current X, HWM Y (REMEDIATION REQUIRED)
   ```

8. **Pass `benchmarkWeaknesses` to Scout context** — an array of `{dimension, score, taskTypeHint}` objects derived from the weakest dimensions and the dimension-to-task-type mapping in benchmark-eval.md.

## Benchmark Eval Checksum

Compute and store the checksum of `benchmark-eval.md` during Phase 0:
```bash
sha256sum skills/evolve-loop/benchmark-eval.md > $WORKSPACE_PATH/benchmark-eval-checksum.txt
```
Verify this checksum before every delta check (Phase 3→4 boundary). Builder MUST NOT modify this file.
