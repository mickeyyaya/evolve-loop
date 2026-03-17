# Skill Efficiency Research — Cycle 17

Research performed 2026-03-14 to support the goal: "how to make skills more efficient and effective."

## Current Baseline

| Component | Lines | Est. Tokens |
|-----------|-------|-------------|
| Skill files (SKILL.md, phases.md, eval-runner.md, memory-protocol.md) | 1,138 | ~17,070 |
| Agent files (scout, builder, auditor, operator) | 673 | ~10,095 |
| **Total loaded per cycle** | **1,811** | **~27,165** |

Each cycle loads these files into agent contexts. With 4 agent invocations per cycle (Scout + Builder + Auditor + Operator), the instruction overhead is ~27K tokens repeated up to 4 times = ~108K tokens of instruction content per cycle.

## Findings

### 1. Codified/Pseudocode Prompting (55-87% token reduction)

**Source:** [CodeAgents: Token-Efficient Framework for Codified Multi-Agent Reasoning](https://arxiv.org/html/2507.03254v1)

Structuring agent instructions as modular pseudocode with control structures reduces input tokens 55-87% and output tokens 41-70%. The key insight: natural language instructions contain enormous redundancy. Converting to pseudocode-like format preserves semantics while eliminating filler.

**Applicability to evolve-loop:** The agent files (scout, builder, auditor, operator) use natural language workflows that could be partially codified. The "Strategy Handling" sections (~20-30 lines each, duplicated across 4 agents) are prime candidates.

### 2. Progressive Disclosure (avoid frontloading context)

**Source:** [Writing a good CLAUDE.md](https://www.humanlayer.dev/blog/writing-a-good-claude-md)

Tell agents *where to find* information rather than embedding it all upfront. This reduces context window load and improves attention allocation. Instead of including full strategy definitions in every agent prompt, reference the SKILL.md strategy table and pass only the active strategy name.

**Applicability:** The evolve-loop already partially does this (context blocks pass `strategy` as a string). But agent prompts still contain full strategy descriptions. Removing duplicated strategy text from agents saves ~80-120 lines total.

### 3. Instruction Count Limits (~150-200 max)

**Source:** [CLAUDE.md Best Practices from Optimizing Claude Code with Prompt Learning](https://arize.com/blog/claude-md-best-practices-learned-from-optimizing-claude-code-with-prompt-learning/)

Frontier LLMs can follow ~150-200 discrete instructions with reasonable consistency. Beyond that, compliance degrades. The evolve-loop's phases.md alone has ~580 lines of instructions — well beyond this limit.

**Applicability:** phases.md is the orchestrator's instruction file, not a single agent's prompt. But agent files (150-240 lines each) are at the upper boundary. Keeping agent prompts under 150 lines would improve instruction adherence.

### 4. Prompt Minification (42% token reduction, 12% quality drop)

**Source:** [OpenReview — Prompt Compression](https://openreview.net/forum?id=pzFhtpkabh)

Removing whitespace, comments, and verbosity from prompts reduces token usage ~42% with only ~12% quality degradation. Trade-off is acceptable for boilerplate sections but not for nuanced behavioral instructions.

**Applicability:** Markdown formatting (headers, tables, code blocks) adds tokens but improves readability. A selective approach — minify boilerplate sections (output templates, ledger schemas) while keeping behavioral instructions readable — could yield 20-30% savings.

### 5. AgentDropout (21.6% token reduction)

**Source:** [AgentDropout: Dynamic Agent Elimination for Token-Efficient Multi-Agent Collaboration](https://arxiv.org/abs/2503.18891)

Dynamically eliminating redundant agents from multi-agent conversations reduces prompt tokens 21.6% and completion tokens 18.4%. Not directly applicable to evolve-loop (agents run independently, not in conversation), but the concept of skipping unnecessary agents is analogous to the existing convergence short-circuit.

### 6. OPTIMA Framework (up to 90% token reduction)

**Source:** [OPTIMA: Optimizing Effectiveness and Efficiency for LLM-based Multi-Agent Systems](https://aclanthology.org/2025.findings-acl.601.pdf)

Combines communication topology optimization, LLM-based summarization, and iterative pruning. Achieves up to 90% token reduction with 2.8x performance improvement. The key technique: summarize previous agent outputs rather than passing full transcripts.

**Applicability:** The evolve-loop already does this with ledgerSummary and instinctSummary. Could extend to summarizing scout-report.md before passing to Builder (pass only the relevant task, not the full report).

### 7. Meta-Prompting Loops

**Source:** [CLAUDE.md Best Practices](https://arize.com/blog/claude-md-best-practices-learned-from-optimizing-claude-code-with-prompt-learning/)

Iteratively optimizing the system prompt via Claude itself — write prompt, evaluate performance, have Claude critique and rewrite. Yields 5%+ performance gains on coding benchmarks. The evolve-loop's meta-cycle already implements a version of this (TextGrad prompt evolution), but hasn't been applied since cycle 10.

## Recommendations

### R1: Remove duplicated strategy definitions from agent prompts
**Impact:** Save ~80-120 lines (~1,200-1,800 tokens) across 4 agent files
**How:** Each agent currently has a "Strategy Handling" section repeating the same 4 strategy descriptions. Replace with a 2-line reference: "Read `strategy` from context. See SKILL.md Strategy Presets table for definitions."
**Source:** Progressive Disclosure principle (HumanLayer)
**Effort:** S-complexity, cycle 18 candidate

### R2: Compress output template sections in agent prompts
**Impact:** Save ~40-60 lines (~600-900 tokens) per agent
**How:** The output template sections (workspace file format, ledger entry) are verbose with full markdown examples. Reduce to minimal field lists. Agents can infer markdown formatting.
**Source:** Prompt Minification research (OpenReview)
**Effort:** S-complexity, cycle 18 candidate

### R3: Extract shared patterns to a referenced conventions file
**Impact:** Eliminate duplication of cross-cutting concerns (ledger format, workspace conventions, eval schemas)
**How:** Create `docs/agent-conventions.md` referenced by all agents. Agents read it on first cycle, rely on instinct memory thereafter.
**Source:** CodeAgents modular structure principle
**Effort:** M-complexity, cycle 19+ candidate

### R4: Populate plan cache with historical task templates
**Impact:** ~30-50% cost reduction on repeated task patterns (per existing SKILL.md documentation)
**How:** Extract 3-4 generalized templates from the 42 completed tasks. Patterns: version-bump, docs-update, add-section-to-file.
**Source:** Internal mechanism already designed but never activated
**Effort:** S-complexity, cycle 18 candidate

### R5: Target 150 lines max per agent prompt
**Impact:** Improved instruction adherence per research finding #3
**How:** Audit each agent file. Scout (240 lines) and Builder (152 lines) exceed the target. Identify sections that can be moved to referenced docs or eliminated.
**Source:** Arize CLAUDE.md research
**Effort:** M-complexity, requires careful testing

### R6: Resume meta-cycle self-improvement
**Impact:** Pipeline-level optimization via split-role critique
**How:** Cycle 15 should have triggered a meta-cycle (every 5 cycles) but was skipped due to convergence. Run during cycle 18 or 19.
**Source:** Internal mechanism (docs/meta-cycle.md)
**Effort:** Built-in, just needs to be triggered

### R7: Pass only relevant task to Builder (not full scout-report)
**Impact:** Save ~500-1,000 tokens per Builder invocation
**How:** Extract the specific task section from scout-report.md and pass as inline context, not the full report with discovery summary and deferred tasks.
**Source:** OPTIMA summarization principle
**Effort:** S-complexity, orchestrator change in phases.md
