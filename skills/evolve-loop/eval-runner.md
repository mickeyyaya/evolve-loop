# Eval Runner — Audit Phase Instructions

The orchestrator (or Auditor agent) uses these instructions to run eval checks as part of the audit gate in Phase 3.

## Purpose

Eval definitions are created by the Scout in Phase 1 and stored in `.claude/evolve/evals/<task-slug>.md`. The eval runner executes these definitions and determines PASS/FAIL.

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

## Acceptance Checks (verification commands)
- `grep -r "export function newFeature" src/` → must find at least 1 match
- `npm run build` → must exit 0

## Thresholds
- All checks: pass@1 = 1.0
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

If verdict is **PASS** → proceed to Phase 4 (SHIP).
