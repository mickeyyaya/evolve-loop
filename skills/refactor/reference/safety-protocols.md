> Read this file during Phase 4 execution. LLM safety guardrails including RefactoringMirror pattern, iterative re-prompting, and multi-proposal generation.

# LLM Safety Protocols

Never apply LLM-generated refactored code directly. Follow the RefactoringMirror pattern.

## RefactoringMirror Pattern (arXiv:2411.04444)

Three-stage hybrid approach achieving 94.3% success with 0% unsafe edits:

| Stage | Action | Tool |
|-------|--------|------|
| 1. Detect | Use LLM to generate refactored code, then diff original vs LLM output to identify what refactorings were applied | ReExtractor or AST diff |
| 2. Extract | Extract detailed parameters for each detected refactoring (method name, target class, line range) | Custom extraction |
| 3. Reapply | Execute the identified refactorings using battle-tested, deterministic IDE refactoring engines | IntelliJ IDEA, VS Code, or manual with tests |

**Why this matters:** LLMs produce plausible but unsafe code ~7% of the time. The RefactoringMirror pattern uses the LLM as an *advisor* (what to refactor) and deterministic engines as *executors* (how to refactor).

## Iterative Re-prompting Protocol

When a refactoring fails compilation or tests:

| Round | Action | Success Rate Improvement |
|-------|--------|------------------------|
| 1 | Re-prompt with exact error message | +25-30pp |
| 2 | Re-prompt with error + stack trace + failing test | +10-15pp |
| 3 | Re-prompt with alternative approach suggestion | +5-10pp |
| 4-20 | Continue with decreasing returns | Diminishing |
| >20 | STOP — escalate to human review | — |

Total improvement from iterative re-prompting: +40-65 percentage points over single-shot.

## Multi-Proposal Generation

Generate multiple refactoring proposals and select the best:

| Strategy | Correctness Gain |
|----------|-----------------|
| pass@1 (single proposal) | Baseline |
| pass@3 | +15-20% |
| pass@5 | +28.8% |
| Best of 3 with test validation | Recommended balance of cost vs quality |

## Safety Checklist (Before Applying Any Refactoring)

- [ ] All existing tests pass on the refactored code
- [ ] No new compiler/linter warnings introduced
- [ ] Cyclomatic complexity did not increase
- [ ] Cognitive complexity did not increase
- [ ] No architecture boundary violations introduced
- [ ] No circular dependencies introduced
- [ ] Diff is minimal — only changes what was intended
