# Eval Runner — Audit Phase Instructions

The orchestrator (or Auditor agent) uses these instructions to run eval checks as part of the audit gate in Phase 3.

## Purpose

Eval definitions are created by the Scout in Phase 1 and stored in `.evolve/evals/<task-slug>.md`. The eval runner executes these definitions and determines PASS/FAIL.

## Eval Definition Format

Each eval file in `.evolve/evals/` follows this structure (see also [examples/eval-definition.md](../../examples/eval-definition.md)):

```markdown
# Eval: <task-name>

## Code Graders (bash commands that must exit 0)
- `npm test -- --grep "feature-name"`
- `npx tsc --noEmit`
- `bash -c 'test $(wc -l < src/new-file.ts) -gt 0'`

## Regression Evals (full test suite)
- `npm test`
- `npx playwright test` (if applicable)

## Acceptance Checks (verification commands)
- `grep -r "export function newFeature" src/` → must find at least 1 match
- `npm run build` → must exit 0

## Thresholds
- All checks: pass@1 = 1.0
```

## Orchestrator Execution Steps

### 1. Locate Eval Definitions
```bash
ls .evolve/evals/
```
Read each `.md` file. If no eval files exist, log warning and auto-PASS (graceful degradation for cycle 1).

### 2. Run Code Graders
For each command in `## Code Graders`:
- Execute the command
- Record exit code, stdout, stderr
- PASS if exit code = 0, FAIL otherwise

### 3. Run Regression Evals
For each command in `## Regression Evals`:
- Execute the command
- Record exit code, stdout, stderr
- PASS if exit code = 0, FAIL otherwise

### 4. Run Acceptance Checks
For each command in `## Acceptance Checks`:
- Execute the command
- Record exit code
- PASS if exit code = 0, FAIL otherwise

### 5. Compute Verdict
- If ALL graders, regressions, and acceptance checks pass → **PASS**
- If ANY fail → **FAIL**

### 6. Write Eval Report
Write to `workspace/audit-report.md` (eval results section) or `workspace/eval-report.md` if run standalone:

```markdown
## Eval Results
| Command | Exit Code | Status |
|---------|-----------|--------|
| `<command>` | 0 | PASS |
| `<command>` | 1 | FAIL |

## Summary
- Total checks: X
- Passed: Y
- Failed: Z
```

### 7. Append Ledger Entry
```json
{"ts":"<ISO-8601>","cycle":<N>,"role":"eval","type":"eval-gate","data":{"verdict":"PASS|FAIL","total":<N>,"passed":<N>,"failed":<N>,"failedChecks":["<cmd1>","<cmd2>"]}}
```

## Retry Protocol (Hard Gate)

If verdict is **FAIL**:

1. **Iteration 1:** Re-launch Builder agent with failure details:
   - Pass failed check commands and their stderr
   - Builder fixes and re-runs its own tests
   - Re-run Auditor (Phase 3) with updated code
   - Re-run eval checks

2. **Iteration 2:** Same as iteration 1 with accumulated failure context

3. **Iteration 3 (final):** If still FAIL:
   - Log as failed approach in `state.json` under `failedApproaches`
   - Record failed eval commands and error output
   - Skip Phase 4 (SHIP)
   - Proceed to Phase 5 (LEARN) with failure context
   - Output warning: "Eval gate failed after 3 attempts. Skipping deploy."

**Max total iterations: 3** (1 initial + 2 retries)

If verdict is **PASS** → proceed to the Benchmark Delta Check (then Phase 4).

---

## Non-Code Eval Graders

For domains beyond coding (writing, research, design), bash/grep graders are insufficient. The eval-runner supports three additional grader types that use LLM judgment with anchored rubrics. These graders complement — never replace — bash graders for coding tasks.

### Rubric-Based Grader (writing, design)

Uses an LLM to score output against a structured rubric. Each criterion has anchored score points (0/25/50/75/100) with examples to prevent score drift.

```markdown
## Rubric Grader
- Criterion: "Clarity — is the text easy to understand?"
  - 0: Incomprehensible or missing
  - 25: Major ambiguities, reader must guess intent
  - 50: Understandable but requires re-reading
  - 75: Clear on first read, minor ambiguities
  - 100: Crystal clear, no ambiguity, well-structured
- Model: haiku (cost-efficient for rubric scoring)
- Threshold: average score >= 60 to pass
```

The LLM receives the rubric + the artifact + a system prompt: "Score each criterion using ONLY the anchor points. Output JSON: `{criterion: score, justification: string}`." Haiku model is sufficient for rubric application (it follows structured instructions well).

### Groundedness Check (research)

Verifies that claims in a research output are supported by cited sources. The LLM receives the output text and its cited sources, then flags unsupported claims.

```markdown
## Groundedness Check
- Input: research output + cited sources
- Model: sonnet (requires reasoning about source-claim alignment)
- Check: "For each factual claim, identify the supporting source. Flag claims with no source."
- Threshold: >= 80% of claims grounded to pass
```

This prevents hallucinated findings in research tasks — a groundedness gate is the evolve-loop equivalent of a test suite for factual accuracy.

### Coverage Check (research, writing)

Verifies that the output addresses all required topics or questions. The LLM compares the output against a checklist of required coverage points.

```markdown
## Coverage Check
- Input: output text + required coverage points (from acceptance criteria)
- Model: haiku
- Check: "For each coverage point, does the output address it? Output JSON: {point: bool}"
- Threshold: 100% coverage to pass (all points addressed)
```

### Hybrid Mode (mixed domains)

For projects with both code and non-code artifacts, eval definitions can mix grader types:

```markdown
# Eval: add-api-docs

## Code Graders (bash — for the code changes)
- `npm test`
- `grep -q "export.*apiDocs" src/index.ts`

## Rubric Grader (LLM — for the documentation quality)
- Criterion: "Completeness — are all endpoints documented?"
  - Threshold: >= 75

## Coverage Check (LLM — for required sections)
- Required: ["Authentication", "Rate Limits", "Error Codes", "Examples"]
- Threshold: 100%
```

The eval-runner processes each section in order. ALL grader types must pass for an overall PASS verdict.

---

## Benchmark Eval Execution

Distinct from task-level evals. The benchmark eval measures project-wide quality across 8 dimensions defined in [benchmark-eval.md](benchmark-eval.md).

### Phase 0 Calibration (once per invocation)

Run the full benchmark eval before the first cycle:

1. **Execute automated checks** for all 8 dimensions from benchmark-eval.md
2. **Run LLM judgment** (haiku model) for each dimension using the anchored rubric
3. **Compute composite scores:** `0.7 * automated + 0.3 * llm` per dimension
4. **Store results** in `state.json.projectBenchmark`
5. **Write `workspace/benchmark-report.md`** with per-dimension scores and weaknesses

### Delta Check Execution (between Phase 3 and Phase 4)

Run a targeted re-evaluation after all tasks pass audit:

1. **Verify `benchmark-eval.md` checksum** — HALT if tampered
2. **Determine relevant dimensions** from the task type using the mapping in benchmark-eval.md
3. **Re-run automated checks** for relevant dimensions only (not all 8)
4. **Compare to Phase 0 baseline:**
   - Improvement (+2 or more in any dimension) → proceed to Ship
   - Stable (within +/- 1) → proceed with warning
   - Regression (-3 or more in any dimension) → block, retry once, then drop
5. **Update `state.json.projectBenchmark.dimensions`** with new scores

### Benchmark Eval vs Task Eval

| Aspect | Task Eval | Benchmark Eval |
|--------|-----------|----------------|
| Scope | Single task's acceptance criteria | Entire project quality |
| Created by | Scout (per task) | Defined in benchmark-eval.md (static) |
| Runs | After each Builder attempt | Phase 0 (full) + delta check (targeted) |
| Gate type | Hard (FAIL blocks shipping) | Soft (regression blocks, stability warns) |
| Tampering | Checksum-protected per cycle | Checksum-protected per invocation |

---

## Mutation Testing

Mutation testing validates eval grader quality by introducing deliberate defects and checking if graders catch them. Run during meta-cycles (every 5 cycles) as part of Phase 5 step 6e.

### What is a Mutation?

A mutation is a small, deliberate change to a completed task's output that should cause at least one eval grader to fail. Examples:
- **Deletion mutation:** Remove a key section heading from a markdown file
- **Value mutation:** Change a numeric threshold (e.g., 0.7 → 0.3)
- **Import mutation:** Remove a cross-reference link
- **Logic mutation:** Invert a condition (e.g., `>=` → `<`)

### Generating Mutations

For each task completed in the last 5 cycles:
1. Read the task's eval graders from `.evolve/evals/<slug>.md`
2. Generate 2-3 targeted mutations that test different graders
3. Apply each mutation to a temporary copy of the affected file
4. Run the eval graders against the mutated file
5. Record whether each grader caught the mutation

### Mutation Kill Rate

```bash
# Example grader for mutation testing itself
MUTATIONS_GENERATED=<N>
MUTATIONS_KILLED=<caught by at least one grader>
KILL_RATE=$((MUTATIONS_KILLED * 100 / MUTATIONS_GENERATED))
# Target: >= 80%. Below 60% triggers eval improvement task.
```

### Interpreting Results

| Kill Rate | Action |
|-----------|--------|
| >= 80% | Evals are robust. No action needed. |
| 60-79% | Review weak graders. Consider adding more specific checks. |
| < 60% | PRIORITY: Propose eval improvement task for next cycle. |

Mutations that survive indicate graders that are too coarse — they pass both correct and incorrect code. The fix is usually to add a more targeted grep or assertion.

For grader design patterns and anti-patterns, see `docs/eval-grader-best-practices.md`.
