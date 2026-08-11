# Eval: update-generalization-status-architecture

## Code Graders (bash commands that must exit 0)

1. Phase 2 BUILD isolation adapter mentioned with file-copy in Completed section:
   ```bash
   grep -q "Phase 2 BUILD isolation adapter" /Users/danleemh/ai/claude/evolve-loop/docs/architecture.md && \
   grep -q "file-copy" /Users/danleemh/ai/claude/evolve-loop/docs/architecture.md
   ```

2. Cycle 6 completions referenced:
   ```bash
   grep -q "cycle 6" /Users/danleemh/ai/claude/evolve-loop/docs/architecture.md
   ```

3. Domain-specific benchmark dimensions mentioned in Completed section:
   ```bash
   grep -q "Domain-specific benchmark dimensions" /Users/danleemh/ai/claude/evolve-loop/docs/architecture.md
   ```

4. All four touch points verified as completed:
   ```bash
   grep -q "build isolation" /Users/danleemh/ai/claude/evolve-loop/docs/architecture.md && \
   grep -q "ship mechanism" /Users/danleemh/ai/claude/evolve-loop/docs/architecture.md && \
   grep -q "eval graders" /Users/danleemh/ai/claude/evolve-loop/docs/architecture.md && \
   grep -q "auto-detection" /Users/danleemh/ai/claude/evolve-loop/docs/architecture.md
   ```

5. Generalization Status section structure maintained (Completed/Remaining):
   ```bash
   grep -A 20 "^### Generalization Status" /Users/danleemh/ai/claude/evolve-loop/docs/architecture.md | \
   grep -q "\*\*Completed" && \
   grep -A 30 "^### Generalization Status" /Users/danleemh/ai/claude/evolve-loop/docs/architecture.md | \
   grep -q "\*\*Remaining"
   ```

## Regression Evals (full test suite)

This is a documentation-only task. No automated test suite applies. Manual content review in Auditor phase.

## Acceptance Checks (verification commands)

1. Verify file was modified:
   ```bash
   git diff HEAD -- /Users/danleemh/ai/claude/evolve-loop/docs/architecture.md | grep -q "Phase 2 BUILD"
   ```

2. Verify no broken links introduced in section:
   ```bash
   grep -A 20 "^### Generalization Status" /Users/danleemh/ai/claude/evolve-loop/docs/architecture.md | \
   grep -oP '\[.*?\]\(\K[^)]+' | while read link; do \
     test -f "/Users/danleemh/ai/claude/evolve-loop/$link" || exit 1; \
   done
   ```

## Thresholds

- All checks: pass@1 = 1.0
- Expected execution: 5 graders, all passing
