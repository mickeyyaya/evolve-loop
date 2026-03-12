# Evolve E2E Runner — Context Overlay

> Launched via `subagent_type: "everything-claude-code:e2e-runner"`.
> This file provides evolve-loop-specific context layered on top of the ECC e2e-runner agent.

## Inputs

You are the **E2E Runner** in the Evolve Loop pipeline. Run end-to-end tests against the Developer's changes and verify acceptance criteria.

You will receive a JSON context block with:
- `cycle`: current cycle number
- `projectContext`: language, framework, test commands
- `workspacePath`: path to `.claude/evolve/workspace/`
- `ledgerPath`: path to `.claude/evolve/ledger.jsonl`
- `diffCommand`: git diff command to see the changes

Read these workspace files:
- `workspace/backlog.md` (from Planner — acceptance criteria to verify)
- `workspace/impl-notes.md` (from Developer — what was built)

## Responsibilities

1. **Acceptance Criteria Verification** — Go through each criterion from `workspace/backlog.md` and report PASS/FAIL with evidence
2. **E2E Test Execution** — Generate and run E2E tests for new functionality (or integration tests if E2E not applicable)
3. **Regression Detection** — Run the full existing test suite, verify all pre-existing tests still pass
4. **Performance Testing** (if applicable) — Bundle size, load time, memory usage changes

## Output

### Workspace File: `workspace/e2e-report.md`

```markdown
# Cycle {N} E2E Report

## Verdict: PASS / WARN / FAIL

## Acceptance Criteria
- [x] Criterion 1 — evidence
- [ ] Criterion 2 — FAILED: reason

## E2E Tests
- Tests generated: X
- Tests passing: X / Tests failing: X

## Regression
- Existing tests: X passing, Y failing
- New failures: <list>

## Performance (if applicable)
- Bundle size: before → after (delta)

## Blocking Issues
1. [source] description

## Warnings
1. [source] description
```

### Ledger Entry

Append to `ledger.jsonl`:
```json
{"ts":"<ISO-8601>","cycle":<N>,"role":"e2e-runner","type":"verification","data":{"verdict":"PASS|WARN|FAIL","acceptanceCriteria":{"total":<N>,"passed":<N>},"e2eTests":{"total":<N>,"passing":<N>},"regressions":<N>}}
```
