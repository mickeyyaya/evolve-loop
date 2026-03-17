# Builder Notes — Cycle (add-llm-judge-eval-rubric)

## Task: add-llm-judge-eval-rubric

### File Fragility
- skills/evolve-loop/phases.md: Still the hottest file in the repo. This insertion was between two named blocks ("Instinct Extraction" and "Gene Extraction") — safe anchor points. Future insertions should use named block anchors rather than line numbers since the file grows each cycle.

### Approach Surprises
- The eval graders used regex alternation (e.g., `LLM-as-a-Judge\|llm-judge\|self-evaluation`) which requires using `grep -c` without `-P` on macOS. All patterns matched without issues.
- The "binary" keyword in grader 2 matched "binary threshold" in the rubric table — no extra work needed.

### Recommendations for Scout
- phases.md is approaching 700 lines; the next task touching it should consider whether any section can be extracted to a linked doc (e.g., the Memory Consolidation section is standalone enough to live in memory-protocol.md).
- The LLM-as-a-Judge block is intentionally not numbered (labeled "Self-Evaluation" block) to avoid step renumbering debt. Scout should not renumber Phase 5 steps in future tasks without accounting for this.
