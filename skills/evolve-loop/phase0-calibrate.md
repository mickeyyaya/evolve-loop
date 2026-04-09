> Read this file when running Phase 0 (CALIBRATE). Covers deduplication check, automated + LLM scoring, composite computation, high-water mark tracking, and benchmark report writing.

# Evolve Loop — Phase 0: CALIBRATE

Runs **once per `/evolve-loop` invocation**, not per cycle. Executes before the first Scout. Establishes a project-level benchmark baseline so tasks are measured against project quality, not just process quality.

## Calibration Deduplication

```
if state.json.projectBenchmark.lastCalibrated exists
   AND (now - lastCalibrated) < 24 hours:
   Skip CALIBRATE, use existing benchmark
else:
   Run CALIBRATE normally
```

## Execution Steps

1. **Run automated checks** from [benchmark-eval.md](benchmark-eval.md):
   Execute all bash commands for each of the 8 dimensions. Capture per-dimension scores (0-100).
   ```bash
   # Store results in $WORKSPACE_PATH/benchmark-automated.json
   ```

2. **Compute composites:** `dimension.composite = automated` (automated scores only — LLM judgment removed to save ~30K tokens per calibration)

3. **Compute overall:** `overall = round(mean(all 8 composites), 1)`

4. **Store in state.json** under `projectBenchmark`:
   ```json
   {
     "projectBenchmark": {
       "lastCalibrated": "<ISO-8601>",
       "calibrationCycle": "<lastCycleNumber + 1>",
       "overall": "<0-100>",
       "dimensions": {
         "documentationCompleteness": {"composite": "<N>"},
         "specificationConsistency": {"composite": "<N>"},
         "defensiveDesign": {"composite": "<N>"},
         "evalInfrastructure": {"composite": "<N>"},
         "modularity": {"composite": "<N>"},
         "schemaHygiene": {"composite": "<N>"},
         "conventionAdherence": {"composite": "<N>"},
         "featureCoverage": {"composite": "<N>"}
       },
       "history": [],
       "highWaterMarks": {}
     }
   }
   ```

5. **Compare to previous calibration** (if `history` non-empty):
   - Append previous calibration to `history` (keep last 5)
   - Identify 2-3 lowest composite dimensions → `benchmarkWeaknesses`
   - **High-water mark tracking:** Dimensions at 80+ recorded in `highWaterMarks`. Regression below `(HWM - 10)` → log as `benchmarkWeakness` for Scout

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

8. **Pass `benchmarkWeaknesses` to Scout** — array of `{dimension, score, taskTypeHint}` from weakest dimensions and dimension-to-task-type mapping in benchmark-eval.md.

## Benchmark Eval Checksum

Compute and store during Phase 0:
```bash
sha256sum skills/evolve-loop/benchmark-eval.md > $WORKSPACE_PATH/benchmark-eval-checksum.txt
```
Verify before every delta check (Phase 4-5 boundary). Builder MUST NOT modify this file.
