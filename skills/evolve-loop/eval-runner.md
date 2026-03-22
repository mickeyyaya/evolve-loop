> Read this file when running eval checks during Phase 3 (AUDIT). Covers grader types, eval definition format, execution steps, retry protocol, non-code graders, benchmark eval execution, and mutation testing.

## Contents
- [Grader Type Taxonomy](#grader-type-taxonomy) — code, model, human grader types
- [Eval Definition Format](#eval-definition-format) — file structure in `.evolve/evals/`
- [Orchestrator Execution Steps](#orchestrator-execution-steps) — locate, run, compute, report
- [Retry Protocol](#retry-protocol-hard-gate) — 3-attempt max, failure handling
- [Non-Code Eval Graders](#non-code-eval-graders) — rubric, groundedness, coverage checks
- [Benchmark Eval Execution](#benchmark-eval-execution) — Phase 0 calibration, delta checks
- [Mutation Testing](#mutation-testing) — eval quality validation via deliberate defects

# Eval Runner — Audit Phase Instructions

Eval definitions are created by the Scout in Phase 1, stored in `.evolve/evals/<task-slug>.md`. The eval runner executes these definitions and determines PASS/FAIL.

## Grader Type Taxonomy

Scout MUST tag each command when creating eval definitions. Type determines execution method, cost, and reliability.

| Type | Tag | Properties | When to Use |
|------|-----|-----------|-------------|
| **Code-Based** | `[code]` | Fast, reproducible, zero token cost, no hallucination risk | Default. Use whenever criterion can be expressed as bash command. |
| **Model-Based** | `[model]` | ~2-5K tokens, requires anchored rubric, non-deterministic | Only when no code-based grader can capture the criterion. Max 2 per eval definition. |
| **Human-Gated** | `[human]` | Blocks pipeline until human responds | Only for security-sensitive, irreversible operations, or architectural decisions. |

**Code-Based examples:** `npm test`, `grep -r "export" src/`, `test -f output.json`, `python -c "import module"`

**Model-Based:** Scout writes rubric with anchored score points (0/25/50/75/100). Auditor invokes tier-3 model judge. PASS threshold: average >= 60.

**Human-Gated:** Auditor presents evidence and halts with clear question. Human responds PASS/FAIL with reasoning.

### pass@k Tracking

After each task ships, record attempts to pass all evals. Track per task type in `taskArms`:
- `pass@1` = first attempt (ideal)
- `pass@2` = second attempt after retry
- `pass@3` = third attempt (max retries)

---

## Eval Definition Format

Each file in `.evolve/evals/` (see also [examples/eval-definition.md](examples/eval-definition.md)):

```markdown
# Eval: <task-name>

## Code Graders (bash commands that must exit 0)
- `[code]` `npm test -- --grep "feature-name"`
- `[code]` `npx tsc --noEmit`
- `[code]` `bash -c 'test $(wc -l < src/new-file.ts) -gt 0'`

## Regression Evals (full test suite)
- `[code]` `npm test`
- `[code]` `npx playwright test` (if applicable)

## Acceptance Checks (verification commands)
- `[code]` `grep -r "export function newFeature" src/`
- `[code]` `npm run build`

## Model-Based Checks (optional)
- `[model]` Rubric: "Rate documentation clarity 0-100..." — threshold: >= 60

## Human-Gated Checks (optional)
- `[human]` "Review auth middleware changes for privilege escalation risk"

## Thresholds
- All checks: pass@1 = 1.0
- Grader type tags: `[code]` default, `[model]` requires rubric, `[human]` requires evidence
```

## Orchestrator Execution Steps

### 1. Locate Eval Definitions
```bash
ls .evolve/evals/
```
If no eval files exist, log warning and auto-PASS (graceful degradation for cycle 1).

### 2. Run Code Graders
Execute each command in `## Code Graders`. Record exit code, stdout, stderr. PASS if exit 0.

### 3. Run Regression Evals
Execute each command in `## Regression Evals`. Record exit code, stdout, stderr. PASS if exit 0.

### 4. Run Acceptance Checks
Execute each command in `## Acceptance Checks`. Record exit code. PASS if exit 0.

### 5. Compute Verdict
ALL graders, regressions, and acceptance checks pass → **PASS**. ANY fail → **FAIL**.

### 6. Write Eval Report
Write to `workspace/audit-report.md` or `workspace/eval-report.md`:

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
{"ts":"<ISO-8601>","cycle":"<N>","role":"eval","type":"eval-gate","data":{"verdict":"PASS|FAIL","total":"<N>","passed":"<N>","failed":"<N>","failedChecks":["<cmd1>"]}}
```

## Retry Protocol (Hard Gate)

| Iteration | Action |
|-----------|--------|
| 1 (initial fail) | Re-launch Builder with failed check commands and stderr. Builder fixes, re-runs tests. Re-audit, re-run evals. |
| 2 | Same as iteration 1 with accumulated failure context. |
| 3 (final) | Log as failed approach in `state.json`. Record failed commands. Skip Phase 4. Proceed to Phase 5 with failure context. Output: "Eval gate failed after 3 attempts. Skipping deploy." |

**Max total iterations: 3** (1 initial + 2 retries). If PASS → proceed to Benchmark Delta Check (then Phase 4).

---

## Non-Code Eval Graders

For domains beyond coding (writing, research, design). These complement — never replace — bash graders for coding tasks.

### Rubric-Based Grader (writing, design)

```markdown
## Rubric Grader
- Criterion: "Clarity — is the text easy to understand?"
  - 0: Incomprehensible or missing
  - 25: Major ambiguities, reader must guess intent
  - 50: Understandable but requires re-reading
  - 75: Clear on first read, minor ambiguities
  - 100: Crystal clear, no ambiguity, well-structured
- Model: tier-3
- Threshold: average score >= 60
```

LLM receives rubric + artifact + system prompt: "Score each criterion using ONLY the anchor points. Output JSON: `{criterion: score, justification: string}`."

### Groundedness Check (research)

```markdown
## Groundedness Check
- Input: research output + cited sources
- Model: tier-2 (requires reasoning about source-claim alignment)
- Check: "For each factual claim, identify supporting source. Flag claims with no source."
- Threshold: >= 80% grounded
```

### Coverage Check (research, writing)

```markdown
## Coverage Check
- Input: output text + required coverage points
- Model: tier-3
- Check: "For each coverage point, does the output address it? Output JSON: {point: bool}"
- Threshold: 100% coverage
```

### Hybrid Mode (mixed domains)

Eval definitions can mix grader types. ALL types must pass for overall PASS.

```markdown
# Eval: add-api-docs

## Code Graders
- `npm test`
- `grep -q "export.*apiDocs" src/index.ts`

## Rubric Grader
- Criterion: "Completeness — all endpoints documented?" — Threshold: >= 75

## Coverage Check
- Required: ["Authentication", "Rate Limits", "Error Codes", "Examples"]
- Threshold: 100%
```

---

## Benchmark Eval Execution

Distinct from task-level evals. Measures project-wide quality across 8 dimensions from [benchmark-eval.md](skills/evolve-loop/benchmark-eval.md).

### Phase 0 Calibration (once per invocation)

1. Execute automated checks for all 8 dimensions
2. Run LLM judgment (tier-3, tier-2 for first calibration) with anchored rubric
3. Compute composite: `0.7 * automated + 0.3 * llm` per dimension
4. Store in `state.json.projectBenchmark`
5. Write `workspace/benchmark-report.md`

### Delta Check Execution (between Phase 3 and Phase 4)

1. Verify `benchmark-eval.md` checksum — HALT if tampered
2. Determine relevant dimensions from task type mapping
3. Re-run automated checks for relevant dimensions only
4. Compare to baseline: improvement → ship; stable → warn; regression → block/retry/drop
5. Update `state.json.projectBenchmark.dimensions`

| Aspect | Task Eval | Benchmark Eval |
|--------|-----------|----------------|
| Scope | Single task acceptance criteria | Entire project quality |
| Created by | Scout (per task) | Defined in benchmark-eval.md (static) |
| Runs | After each Builder attempt | Phase 0 (full) + delta check (targeted) |
| Gate type | Hard (FAIL blocks shipping) | Soft (regression blocks, stability warns) |
| Tampering | Checksum-protected per cycle | Checksum-protected per invocation |

---

## Mutation Testing

Validates eval grader quality by introducing deliberate defects. Run during meta-cycles (every 5 cycles) as part of Phase 5.

### Mutation Types

| Mutation | Example |
|----------|---------|
| Deletion | Remove key section heading from markdown |
| Value | Change numeric threshold (0.7 -> 0.3) |
| Import | Remove cross-reference link |
| Logic | Invert condition (>= -> <) |

### Generating Mutations

For each task completed in the last 5 cycles:
1. Read eval graders from `.evolve/evals/<slug>.md`
2. Generate 2-3 targeted mutations testing different graders
3. Apply each to a temporary copy
4. Run eval graders against mutated file
5. Record whether each grader caught the mutation

### Interpreting Results

| Kill Rate | Action |
|-----------|--------|
| >= 80% | Evals robust. No action needed. |
| 60-79% | Review weak graders. Add more specific checks. |
| < 60% | PRIORITY: Propose eval improvement task for next cycle. |

Surviving mutations indicate graders too coarse. Fix by adding more targeted grep or assertion. See `docs/eval-grader-best-practices.md`.
