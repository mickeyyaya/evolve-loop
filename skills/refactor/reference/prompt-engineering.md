> Read this file during Phase 4 execution. Prompt specificity ladder and structured subagent prompt template for refactoring tasks.

# Prompt Engineering for Refactoring

Prompt specificity dramatically affects LLM refactoring quality.

## Prompt Specificity Ladder

| Level | Prompt Template | Identification Rate |
|-------|----------------|-------------------|
| Generic | "Refactor this code" | 15.6% |
| Type-specific | "Apply Extract Method refactoring to this code" | 52.2% |
| Targeted | "Extract the loop body at lines 15-28 into a method called `processItem`" | 86.7% |
| Few-shot | Same as targeted + 2-3 examples of similar refactorings | ~95%+ |

**Rule:** Always use Level 3 (Targeted) or Level 4 (Few-shot) prompts in the refactoring pipeline.

## Prompt Template for Subagents

When dispatching refactoring work to a subagent, include:

1. **Smell identified:** The specific smell name and detection signal
2. **Technique prescribed:** The specific Fowler technique name (e.g., "Extract Method", not "refactor")
3. **Target location:** File path, line range, function/class name
4. **Expected outcome:** What the code should look like after (high-level)
5. **Constraints:** What must NOT change (public API, test behavior)
6. **Example (if available):** A similar before/after from the codebase or Fowler catalog
