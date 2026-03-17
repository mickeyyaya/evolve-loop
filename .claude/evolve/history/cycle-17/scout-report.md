# Cycle 17 Scout Report

## Discovery Summary
- Scan mode: incremental (goal-directed)
- Files analyzed: 6 (SKILL.md, phases.md, eval-runner.md, memory-protocol.md, docs/writing-agents.md, .claude/evolve/state.json)
- Research: performed (prior queries expired >24hr)
- Instincts applied: 0 (none in instinctSummary)

## Key Findings

### Skill Prompt Overhead — HIGH
Current core skill file line counts:
| File | Lines | Est. Tokens |
|------|-------|-------------|
| SKILL.md | 239 | ~3,600 |
| phases.md | 580 | ~8,700 |
| eval-runner.md | 105 | ~1,575 |
| memory-protocol.md | 214 | ~3,210 |
| **Total** | **1,138** | **~17,085** |

Token estimate based on ~150 chars/line → ~15 tokens/line average. These files load into every agent context and the orchestrator on each cycle. No baseline has been formally recorded in state.json, making it impossible to track regression or improvement over time.

### Docs Gap — MEDIUM
`docs/writing-agents.md` is 68 lines and describes structural conventions (frontmatter, output format, rules) but contains no guidance on writing efficient, token-lean agent instructions. As the project matures and new agents are added, this gap leads to bloated prompts by default.

### Research Opportunity — MEDIUM
No prior research has been performed on skill/prompt efficiency. The 12hr cooldown is expired (queries from 2026-03-13, current date 2026-03-14). Research will directly serve the stated goal.

## Research

**Query 1:** "LLM agent prompt optimization techniques efficiency 2025 2026"

Key findings:
- Codified/pseudocode prompting (CodeAgents) reduces input token usage 55-87% and output 41-70% by structuring all agent interaction components — Task, Plan, Feedback, roles — as modular pseudocode with control structures. (source: https://arxiv.org/html/2507.03254v1)
- Multi-technique stacks outperform single-technique: strong prompt design + RAG + eval-driven iteration + lightweight fine-tuning (source: https://gaurav-sharma11.medium.com/llm-optimization-techniques-to-maximize-efficiency-in-2026-b3e51cc06804)
- Evolutionary optimization with semantic genetic algorithms outperforms manual tuning for LLM agents (source: https://www.turintech.ai/blog/how-evolutionary-optimization-outperforms-manual-tuning-for-llm-based-agents)

**Query 2:** "Claude Code agent instruction compression system prompt best practices"

Key findings:
- CLAUDE.md (and by extension skill files) should contain as few instructions as possible — only universally applicable ones. Code style guidelines add irrelevant context window content, degrading performance. (source: https://arize.com/blog/claude-md-best-practices-learned-from-optimizing-claude-code-with-prompt-learning/)
- Frontier LLMs can follow ~150-200 instructions with reasonable consistency; smaller models fewer. Instruction count is a practical limit. (source: https://www.humanlayer.dev/blog/writing-a-good-claude-md)
- Use Progressive Disclosure: tell the agent how to find information, not all the information upfront. Avoids bloating context.
- Optimizing only the system prompt of Claude Code yielded 5%+ gains in general coding performance.
- Minimize boilerplate through pattern recognition — focus instructions on edge cases and domain-specific conventions, not standard behaviors the model already handles.

**Query 3:** "multi-agent system prompt efficiency token reduction instruction design patterns"

Key findings:
- AgentDropout: dynamic elimination of redundant agents reduces prompt token consumption 21.6% and completion tokens 18.4% (source: https://arxiv.org/abs/2503.18891)
- SupervisorAgent: lightweight runtime supervision reduces token consumption 29.45% without compromising success rate (source: https://arxiv.org/html/2510.26585v1)
- Prompt minification (removing whitespace/verbosity) reduces average input token usage 42% with only 12% drop in resolution rate
- OPTIMA training framework achieves up to 90% token reduction with 2.8x performance improvement (source: https://aclanthology.org/2025.findings-acl.601.pdf)

**Deep fetch:** Arize CLAUDE.md optimization article — confirmed actionable techniques: meta-prompting loops for iterative improvement, domain-specific instruction encoding, and minimizing generic boilerplate in favor of repo-specific conventions.

## Selected Tasks

### Task 1: Measure current skill prompt overhead baseline
- **Slug:** measure-skill-prompt-metrics
- **Type:** techdebt
- **Complexity:** S
- **Rationale:** No baseline exists for skill file token overhead. Adding `skillMetrics` to state.json creates a measurable foundation — the cycle 17 goal is "efficiency," which requires knowing the current state before optimizing. ~20K tokens to implement.
- **Acceptance Criteria:**
  - [ ] `state.json` contains a `skillMetrics` key
  - [ ] `skillMetrics` documents `lineCount` and `estimatedTokens` for each of the 4 core skill files
  - [ ] `skillMetrics` includes `totalLines` and `totalEstimatedTokens` aggregates
  - [ ] `skillMetrics` includes a `measuredAt` ISO-8601 timestamp
- **Files to modify:** `.claude/evolve/state.json`
- **Eval:** written to `evals/measure-skill-prompt-metrics.md`

### Task 2: Document skill efficiency research findings
- **Slug:** research-skill-efficiency-patterns
- **Type:** techdebt
- **Complexity:** M
- **Rationale:** Research has been performed this cycle with concrete findings across 3 queries. Documenting findings in a persistent workspace file creates a reusable reference for future optimization tasks. Findings cover codified prompting, progressive disclosure, instruction compression, and meta-prompting loops — all directly applicable to the evolve-loop skill files. ~40K tokens to implement.
- **Acceptance Criteria:**
  - [ ] `workspace/skill-efficiency-research.md` created with at least 50 lines
  - [ ] Contains `## Findings` section with research-backed techniques
  - [ ] Contains `## Recommendations` section with actionable items specific to evolve-loop skill files
  - [ ] Each recommendation cites a source
  - [ ] Includes token overhead analysis of current skill files
- **Files to modify:** `.claude/evolve/workspace/skill-efficiency-research.md` (new file)
- **Eval:** written to `evals/research-skill-efficiency-patterns.md`

### Task 3: Add skill authoring efficiency guidelines to docs/writing-agents.md
- **Slug:** add-skill-efficiency-guidelines
- **Type:** techdebt
- **Complexity:** S
- **Rationale:** `docs/writing-agents.md` is the canonical contributor guide for creating agents. It currently has no guidance on prompt efficiency. Adding an "Efficiency Guidelines" section closes this gap and ensures future agents are written with token overhead in mind. Directly informed by this cycle's research. ~20K tokens to implement.
- **Acceptance Criteria:**
  - [ ] `docs/writing-agents.md` gains a new `## Efficiency Guidelines` section (or equivalent heading)
  - [ ] Section covers: progressive disclosure, instruction count limits (~150-200 max), avoiding generic boilerplate, using references over inline content, and prompt compression techniques
  - [ ] Section is grounded in research findings (references or paraphrases from Task 2)
  - [ ] File line count increases meaningfully (>68 lines current)
- **Files to modify:** `docs/writing-agents.md`
- **Eval:** written to `evals/add-skill-efficiency-guidelines.md`

## Deferred
- Actual prompt compression/refactoring of phases.md or SKILL.md: deferred to cycle 18+ — requires the metrics baseline (Task 1) and research (Task 2) to be in place first. Attempting compression without a baseline risks degrading quality unmeasurably.
- Island model evolution implementation: L complexity, previously deferred — no change.
- Codified/pseudocode prompting migration for agent files: significant rewrite, should be proposed as a multi-cycle effort after Task 2 findings are reviewed.
