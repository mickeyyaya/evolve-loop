# Eval Runner — Phase 5.5 Instructions

The orchestrator uses these instructions to run the eval hard gate after VERIFY (Phase 5) and before SHIP (Phase 6).

## Purpose

Eval definitions are created by the Planner in Phase 2 and stored in `.claude/evolve/evals/<task-name>.md`. The eval runner executes these definitions and determines PASS/FAIL.

## Eval Definition Format

Each eval file in `.claude/evolve/evals/` follows this structure:

```markdown
# Eval: <task-name>

## Code Graders (bash commands that must exit 0)
- `npm test -- --grep "feature-name"`
- `npx tsc --noEmit`
- `bash -c 'test $(wc -l < src/new-file.ts) -gt 0'`

## Regression Evals (full test suite)
- `npm test`
- `npx playwright test` (if applicable)

## Acceptance Checks (manual verification commands)
- `grep -r "export function newFeature" src/` → must find at least 1 match
- `npm run build` → must exit 0

## Thresholds
- Code graders: pass@1 = 1.0 (all must pass)
- Regression: pass@1 = 1.0 (all must pass)
- Acceptance: pass@1 = 1.0 (all must pass)
```

## Orchestrator Execution Steps

### 1. Locate Eval Definitions
```bash
ls .claude/evolve/evals/
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
Write to `workspace/eval-report.md`:

```markdown
# Cycle {N} Eval Report

## Verdict: PASS / FAIL

## Code Graders
| Command | Exit Code | Status |
|---------|-----------|--------|
| `npm test -- --grep "..."` | 0 | PASS |
| `npx tsc --noEmit` | 1 | FAIL |

## Regression Evals
| Command | Exit Code | Status |
|---------|-----------|--------|
| `npm test` | 0 | PASS |

## Acceptance Checks
| Command | Exit Code | Status |
|---------|-----------|--------|
| `grep -r "..." src/` | 0 | PASS |

## Failures (if any)
### `npx tsc --noEmit`
```
<stderr output>
```

## Summary
- Total checks: X
- Passed: Y
- Failed: Z
- Pass rate: Y/X
```

### 7. Append Ledger Entry
```json
{"ts":"<ISO-8601>","cycle":<N>,"role":"eval","type":"eval-gate","data":{"verdict":"PASS|FAIL","total":<N>,"passed":<N>,"failed":<N>,"failedChecks":["<cmd1>","<cmd2>"]}}
```

## Retry Protocol (Hard Gate)

If verdict is **FAIL**:

1. **Iteration 1:** Re-launch Developer agent with failure details:
   - Pass failed check commands and their stderr
   - Developer fixes and re-runs its own tests
   - Re-run VERIFY (Phase 5) — all 3 parallel agents
   - Re-run EVAL (Phase 5.5)

2. **Iteration 2:** Same as iteration 1 with accumulated failure context

3. **Iteration 3 (final):** If still FAIL:
   - Log as failed approach in `state.json` under `failedApproaches`
   - Record failed eval commands and error output
   - Skip Phase 6 (SHIP)
   - Proceed to Phase 7 (LOOP+LEARN) with failure context
   - Output warning: "Eval gate failed after 3 attempts. Skipping deploy."

**Max total iterations: 3** (1 initial + 2 retries)

If verdict is **PASS** → proceed to Phase 6 (SHIP).
