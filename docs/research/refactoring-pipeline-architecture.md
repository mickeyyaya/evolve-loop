# Automated Refactoring Pipeline Optimization Research

> Research conducted 2026-04-04. Covers architecture, speed optimization, dependency handling, and lessons for skill-based refactoring orchestrators.

## Table of Contents

- [1. LLM-Driven Multi-Agent Refactoring Pipelines](#1-llm-driven-multi-agent-refactoring-pipelines)
- [2. AST-Based Refactoring Automation](#2-ast-based-refactoring-automation)
- [3. Incremental Analysis and Change Impact](#3-incremental-analysis-and-change-impact)
- [4. Graph-Based Dependency Analysis](#4-graph-based-dependency-analysis)
- [5. Rector PHP](#5-rector-php)
- [6. OpenRewrite (Java)](#6-openrewrite-java)
- [7. Codemods / jscodeshift (Facebook)](#7-codemods--jscodeshift-facebook)
- [8. Tree-sitter](#8-tree-sitter)
- [9. Cross-Cutting Lessons for Skill-Based Orchestrators](#9-cross-cutting-lessons-for-skill-based-orchestrators)

---

## 1. LLM-Driven Multi-Agent Refactoring Pipelines

### Architecture

| Component | Role |
|---|---|
| Intent classifier agent | Determines refactoring type from user request |
| Repository analyzer agent | Builds context via RAG over codebase embeddings |
| Code generator agent | Produces refactored code |
| Validation orchestrator | Runs static analysis, compilation, tests |
| Self-reflection agent | Re-prompts on failures (up to 20 iterations) |

RepoAI (Microsoft AutoGen-based) coordinates these agents with structured handoffs. RefAgent uses iterative compile/test/fix cycles, improving functional correctness by 40-65 percentage points over single-shot LLM outputs.

### Speed Optimizations

- **pass@3/pass@5 multi-proposal generation**: Generate multiple candidates, pick the best. Balances thoroughness against compute cost.
- **Static analysis pre-filtering**: Run IDE refactoring preconditions (e.g., IntelliJ checks) before LLM generation to filter infeasible transformations.
- **RAG-based context retrieval**: Retrieve only relevant code segments rather than passing entire codebases to the LLM.
- **Few-shot prompting**: Substantially improves outcomes without proportional cost increases compared to fine-tuning.

### Dependency Handling

- Context-aware code retrieval informs transformations of cross-file dependencies.
- Modular architecture isolates local changes from global validation.
- Automated test suites validate end-to-end correctness after each transformation.

### Key Findings

- LLMs excel at localized, systematic refactorings (Magic Number elimination, Extract Method).
- LLMs underperform on context-dependent, architectural, or multi-module refactorings.
- Multi-agent feedback increases correctness by +64.7 pp over single-agent baselines.
- Hallucination rate: 6-8% without static analysis filtering.
- Human oversight remains essential for architectural transformations.

**Sources**: [RepoAI (ScienceDirect)](https://www.sciencedirect.com/science/article/abs/pii/S0167642326000432), [LLM-Driven Refactoring (EmergentMind)](https://www.emergentmind.com/topics/llm-driven-refactoring), [LLM Refactoring Opportunities (ICSE 2025)](https://conf.researchr.org/details/icse-2025/ide-2025-papers/12/LLM-Driven-Code-Refactoring-Opportunities-and-Limitations)

---

## 2. AST-Based Refactoring Automation

### Architecture

All major tools follow the same three-phase pipeline:

```
Source Code --> Parse to AST --> Transform AST --> Print back to Source
```

ASTs strip away syntactic noise (semicolons, parentheses) and represent structural/semantic content as a tree. Tools traverse the tree using the **visitor pattern**, matching node types and applying transformations.

### Speed Optimizations

- **Selective traversal**: Only visit node types relevant to the current transformation rule.
- **Single-pass processing**: Combine multiple rules into one traversal pass when rules target different node types.
- **Lazy parsing**: Only parse files that match path/extension filters before building full ASTs.

### Dependency Handling

- Rules applied in configuration order within a single traversal pass.
- Earlier transformations modify the AST in-place, so later rules see the updated tree.
- Type information must be refreshed after mutations (see Rector's `ChangedNodeScopeRefresher`).

### Lessons

- AST manipulation is deterministic and fast, making it ideal for well-defined, repeatable transformations.
- Type-aware ASTs (with semantic information) enable far more sophisticated transformations than syntax-only trees.
- The gap between AST (abstract) and CST (concrete, preserving whitespace/comments) determines output quality.

**Sources**: [AST Deep Dive (ITNEXT)](https://itnext.io/introduction-to-abstract-syntax-trees-ast-86e2fa455e2c), [AST Use Cases (Medium)](https://medium.com/@kulshreshtha.somil/in-depth-exploration-of-abstract-syntax-tree-ast-use-cases-ebe1b81f7af9)

---

## 3. Incremental Analysis and Change Impact

### Architecture

| Technique | Description |
|---|---|
| AST diffing | Compare code by structure, not text. Recognizes renames, moves, semantic equivalence. |
| Call graph analysis | Build dependency graph from function calls to calculate blast radius. |
| History mining | Use git history to identify frequently-changed files and co-change patterns. |
| Risk scoring | Combine dependency centrality + change frequency + incident history into a file-level risk score. |

### Speed Optimizations

- **Incremental parsing**: Only re-parse changed files; reuse cached ASTs for unchanged files.
- **Scope-limited analysis**: Focus impact analysis on the changed module's dependency subgraph rather than the full codebase.
- **Rollback points**: Apply transformations incrementally with checkpoints, enabling fast recovery.

### Dependency Handling

- Change impact analysis calculates the **ripple effect** of each refactoring step before execution.
- Systematic procedures assess potential consequences before implementation.
- Testing frameworks validate safety at each incremental step.

### Lessons

- Incremental analysis is critical for large-scale refactoring; re-analyzing the entire codebase after each change is prohibitively slow.
- Combining structural analysis (call graphs) with historical analysis (git blame, co-change) produces the most accurate impact predictions.
- Small, continuous refactoring changes are safer and faster than large batch migrations.

**Sources**: [Moderne AI Multi-Repo Refactoring](https://www.moderne.ai/blog/ai-automated-code-refactoring-and-analysis-at-mass-scale-with-moderne), [Change Impact Analysis (Springer)](https://link.springer.com/article/10.1007/s10664-024-10600-2), [Static Code Tools and Refactoring](https://www.in-com.com/blog/chasing-change-how-static-code-tools-handle-frequent-refactoring/)

---

## 4. Graph-Based Dependency Analysis

### Architecture

- Software components are nodes; dependencies (imports, calls, inheritance) are edges.
- Refactoring operations are modeled as **graph transformations** (formal rewriting rules on the dependency graph).
- **Critical pair analysis** detects conflicts between refactoring operations that modify overlapping graph regions.

### Speed Optimizations

- **Topological sorting**: Process refactoring operations in dependency order to avoid conflicts.
- **Subgraph extraction**: Analyze only the affected dependency neighborhood rather than the full graph.
- **Centrality-based prioritization**: Refactor high-centrality nodes first for maximum impact with minimum operations.

### Dependency Handling

- Graph transformation theory provides formal guarantees about operation ordering.
- Critical pair analysis identifies which refactoring pairs conflict (cannot run in parallel) vs. which are independent (can run in parallel).
- Cyclical dependencies are detected and flagged as high-priority refactoring targets.

### Prioritization Framework

| Factor | Weight |
|---|---|
| Cumulative maintenance cost | High |
| Incident severity history | High |
| Modification frequency | Medium |
| Dependency centrality (fan-in/fan-out) | Medium |
| Cyclical dependency involvement | Critical (must-fix) |

### Lessons

- Graph-based analysis enables **parallel execution** of independent refactoring operations.
- Formal conflict detection prevents the most common failure mode: one refactoring breaking another.
- Deep class property graphs + graph neural networks are the emerging approach for automated refactoring opportunity identification.

**Sources**: [Refactoring Dependencies via Graph Transformation (Springer)](https://link.springer.com/article/10.1007/s10270-006-0044-6), [Graph Learning for Extract Class (ACM ISSTA)](https://dl.acm.org/doi/10.1145/3650212.3685561), [Dependency Graphs for Restructuring (Understanding Legacy Code)](https://understandlegacycode.com/blog/safely-restructure-codebase-with-dependency-graphs/)

---

## 5. Rector PHP

### Architecture

```
rector.php config
    |
ProcessCommand --> ApplicationFileProcessor --> FilesFinder
    |
FileProcessor (per file):
    1. Parse PHP to AST (nikic/php-parser)
    2. Decorate nodes with type/scope info (PHPStan)
    3. Traverse AST (RectorNodeTraverser)
    4. Apply matching AbstractRector rules
    5. Refresh scope after mutations (ChangedNodeScopeRefresher)
    6. Print modified AST back to PHP (BetterStandardPrinter)
    |
Save / Generate diff --> Report
```

### Speed Optimizations

| Technique | Detail |
|---|---|
| File-level parallelization | Process multiple files simultaneously via `ApplicationFileProcessor` |
| Type resolution caching | Cache PHPStan results to avoid redundant analysis |
| Selective file processing | Only process files matching configured paths; skip rules prevent unnecessary work |
| Memory management | Controls to prevent OOM during large codebase processing |

### Dependency Handling

- Rules applied **sequentially in configuration order** within a single traversal pass.
- PHPStan type decoration ensures rules have accurate semantic context.
- `ChangedNodeScopeRefresher` updates type information after each mutation, so downstream rules see correct types.
- Modular extensions (rector-symfony, rector-doctrine) register rules without modifying core.

### Lessons for Orchestrators

- **Type-aware transformations are non-negotiable**: PHPStan integration is what makes Rector safe. Without semantic understanding, AST transformations are fragile.
- **Rule composability via modular extensions**: Framework-specific rules are isolated into packages, keeping the core lean.
- **Sequential rule ordering matters**: The configuration order directly determines transformation correctness.
- **Scope refresh after mutations**: This pattern is critical -- stale type information causes cascading failures.

**Sources**: [Rector GitHub](https://github.com/rectorphp/rector), [Rector DeepWiki](https://deepwiki.com/rectorphp/rector), [Rector Documentation](https://getrector.com/documentation)

---

## 6. OpenRewrite (Java)

### Architecture

```
Build Plugin (Maven/Gradle) --> Parse sources to LSTs
    |
Recipe Pipeline:
    1. Validate recipe configuration
    2. Execute visitor on each LST (parallelizable across files)
    3. Execute nested/chained recipes
    4. Check if another cycle is needed (max 3 cycles)
    |
Print modified LSTs back to source (preserving formatting)
```

### Core Innovation: Lossless Semantic Trees (LSTs)

Unlike traditional ASTs, LSTs preserve **everything**: whitespace, indentation, comments, line breaks, plus full type information. This means:
- Output code matches the surrounding code's formatting exactly.
- New imports are inserted in correct alphabetical position with existing indentation style.
- No manual formatting fixup needed post-transformation.

### Speed Optimizations

| Technique | Detail |
|---|---|
| Parallel visitor execution | Process LSTs concurrently across source files |
| Single-cycle completion | Recipes should complete all work in one cycle, not spread across multiple |
| Fresh visitor instances | Each `getVisitor()` returns a new instance, enabling safe parallelism |
| `doAfterVisit()` scheduling | Chain recipes within the same cycle to avoid unnecessary iteration |

### Dependency Handling

- **Execution cycles**: Up to 3 passes through the recipe chain when earlier changes trigger downstream work.
- **`causesAnotherCycle()`**: Recipes signal when their changes require re-evaluation by other recipes.
- **`doAfterVisit()`**: Schedules dependent recipes immediately after the current one, within the same cycle.
- **Declarative composition**: YAML-based recipe chains define execution order without code.
- **`getRecipeList()`**: Imperative recipes return sub-recipes to add to the pipeline.

### Moderne Platform (Multi-Repo Scale)

Moderne extends OpenRewrite to run across thousands of repositories simultaneously:
- Pre-built LSTs stored centrally, avoiding re-parsing.
- AI-powered semantic search finds relevant recipes via embeddings (not keyword matching).
- RAG pipeline samples semantically diverse code blocks to match recommendations to tested recipes.
- AI recommendations validated against existing recipes before execution, preventing hallucinations at scale.

### Lessons for Orchestrators

- **Lossless trees are worth the investment**: Preserving formatting eliminates an entire class of post-transformation issues.
- **Cycle-based execution handles emergent dependencies**: Some transformations create work for other transformations that cannot be predicted statically.
- **Declarative composition enables non-programmer recipe creation**: YAML recipe definitions lower the barrier to adding new transformations.
- **Pre-built parse trees amortize parsing cost**: Parse once, transform many times.

**Sources**: [OpenRewrite Docs](https://docs.openrewrite.org/), [OpenRewrite Recipes Concepts](https://docs.openrewrite.org/concepts-and-explanations/recipes), [OpenRewrite GitHub](https://github.com/openrewrite/rewrite), [Moderne Blog](https://www.moderne.ai/blog/ai-automated-code-refactoring-and-analysis-at-mass-scale-with-moderne)

---

## 7. Codemods / jscodeshift (Facebook)

### Architecture

```
jscodeshift CLI --> Point at directory
    |
Per file:
    1. Parse to AST (via Recast, preserving formatting)
    2. Build collection of paths (jQuery-like API)
    3. Find patterns via .find() queries
    4. Transform via .replaceWith() / .insertBefore() / etc.
    5. Print back via Recast (preserving unmodified code exactly)
```

### Speed Optimizations

| Technique | Detail |
|---|---|
| Batch directory processing | Point at a directory, transform all matching files |
| Code standardization pre-pass | Use linters to normalize style variations, reducing edge cases for transforms |
| Modular transform composition | Small, focused transforms chain together; each is independently testable |
| Recast preservation | Only modified nodes are reprinted; unchanged code passes through verbatim |

### Dependency Handling

- **Sequential chaining**: Complex refactorings decomposed into independent, testable units chained sequentially.
- **Each transform operates on the output of its predecessor**: Modified AST flows downstream.
- **No formal dependency declaration**: Ordering is manual and relies on developer knowledge.
- **Comment insertion for unhandled cases**: Transforms mark code they cannot safely modify, deferring to manual review.

### Key Design Principle

> "Break the task into smaller, independent pieces -- just like how you would normally refactor production code."
>
> -- Martin Fowler, "Refactoring with Codemods"

### Lessons for Orchestrators

- **Test-first methodology is critical**: Know exactly what inputs and outputs look like before coding the transform.
- **Source graph analysis before implementation**: Scan the codebase to understand all pattern variations before writing the transform.
- **Edge cases dominate effort**: Real codebases have import aliases, nested conditionals, variable assignments that the "happy path" transform misses.
- **Codemods as continuous quality tools**: Not one-time migrations but ongoing maintenance (deprecation cleanup, standard enforcement).
- **Facebook scale validation**: Tens of thousands of modules transformed simultaneously for API changes.

**Sources**: [jscodeshift GitHub](https://github.com/facebook/jscodeshift), [Martin Fowler: Codemods for API Refactoring](https://martinfowler.com/articles/codemods-api-refactoring.html), [Effective JavaScript Codemods (Christoph Nakazawa)](https://medium.com/@cpojer/effective-javascript-codemods-5a6686bb46fb)

---

## 8. Tree-sitter

### Architecture

```
Grammar definition (.js) --> tree-sitter generate --> Parser (.c/.wasm)
    |
Source code --> Incremental parser --> Concrete Syntax Tree (CST)
    |
Query language (.scm) --> Pattern matching on CST nodes
```

### Core Innovation: Incremental Parsing

Tree-sitter's defining feature is **incremental re-parsing**: when source code changes, only the affected portion of the syntax tree is rebuilt. This makes it orders of magnitude faster for editor integrations and continuous analysis.

### Speed Optimizations

| Technique | Detail |
|---|---|
| Incremental parsing | Only re-parse changed regions; reuse unchanged subtrees |
| 36x speedup over JavaParser | Measured in production migration from JavaParser to tree-sitter |
| Language-agnostic interface | Single API for all languages; grammars generate parsers |
| CST (not AST) | Concrete tree preserves all tokens, enabling lossless round-tripping |
| Query language | S-expression pattern matching is faster than manual tree traversal |

### Limitation: No Mutation API

Tree-sitter **lacks built-in mutation APIs**. It excels at:
- Parsing (extremely fast, incremental)
- Pattern matching (query language)
- Analysis (node types, fields, structure)

But it does NOT provide:
- Tree mutation methods
- Text diff generation from tree changes
- Code printing/serialization

These must be built on top of tree-sitter by the tool author.

### Lessons for Orchestrators

- **Use tree-sitter for the detection/analysis phase, not the transformation phase**: Its incremental parsing and query language are ideal for finding refactoring candidates fast.
- **Pair tree-sitter with a separate transformation engine**: Use tree-sitter for "what needs to change" and an AST transformer for "how to change it."
- **Language-agnostic support is a major advantage**: One infrastructure supports all languages, unlike language-specific parsers.
- **Incremental parsing is the key to speed**: Avoid re-parsing the entire codebase after each transformation.

**Sources**: [Tree-sitter GitHub](https://github.com/tree-sitter/tree-sitter), [Tree-sitter Codemod Discussion](https://github.com/tree-sitter/tree-sitter/discussions/1108), [Symflower: Parsing with Tree-sitter](https://symflower.com/en/company/blog/2023/parsing-code-with-tree-sitter/)

---

## 9. Cross-Cutting Lessons for Skill-Based Orchestrators

### Architecture Recommendations

| Principle | Rationale | Source Tool |
|---|---|---|
| **Separate detection from transformation** | Detection can be fast/incremental; transformation needs semantic context | Tree-sitter, Rector |
| **Use type-aware trees, not syntax-only** | Prevents incorrect transformations that compile but break semantics | Rector (PHPStan), OpenRewrite (LST) |
| **Preserve formatting in the tree** | Eliminates post-transformation formatting passes and reduces noise in diffs | OpenRewrite (LST), jscodeshift (Recast) |
| **Decompose into small, composable rules** | Each rule is independently testable, orderable, and parallelizable | All tools |
| **Support execution cycles** | Some transformations create work for others; fixed-point iteration catches these | OpenRewrite (max 3 cycles) |

### Speed Optimization Recommendations

| Technique | Expected Impact | Complexity |
|---|---|---|
| **Incremental parsing** (only re-parse changed files) | 10-36x speedup | Medium |
| **File-level parallelization** | Linear speedup with cores | Low |
| **Pre-built parse trees / caching** | Amortize parsing cost across runs | Medium |
| **Selective file processing** (path filters, skip rules) | Proportional to exclusion ratio | Low |
| **Single-cycle completion** (avoid multi-pass when possible) | 2-3x for recipes that would otherwise cycle | Medium |
| **Static analysis pre-filtering** (before LLM calls) | Eliminates 6-8% hallucination-driven waste | Low |
| **Scope-limited impact analysis** (subgraph, not full graph) | Proportional to codebase size | Medium |

### Dependency Handling Recommendations

| Strategy | When to Use |
|---|---|
| **Sequential ordering** (configuration-defined) | Simple, predictable chains where order is known |
| **Critical pair analysis** (graph-based conflict detection) | When rules may conflict; identifies safe parallelization opportunities |
| **Cycle-based re-evaluation** (fixed-point iteration) | When transformations create emergent work for other transformations |
| **Scope refresh after mutations** | Always; stale type/semantic info causes cascading failures |
| **Comment insertion for unhandled cases** | When a rule cannot safely transform a pattern; defers to human review |

### Anti-Patterns to Avoid

| Anti-Pattern | Why It Fails |
|---|---|
| Re-parsing the entire codebase after each transformation | O(n * m) where n=files, m=rules. Use incremental parsing. |
| Relying on text-based diff/patch for transformations | Breaks on whitespace, comments, formatting variations. Use tree-based transforms. |
| Monolithic transformation scripts | Untestable, unorderable, unparallelizable. Decompose into small rules. |
| Skipping type/semantic analysis | Syntax-correct but semantically wrong transformations. Always use type-aware trees. |
| Running LLM for every transformation | Expensive and slow. Use deterministic AST transforms for well-defined patterns; reserve LLM for ambiguous cases. |
| Ignoring edge cases in real codebases | Import aliases, nested conditionals, variable assignments break naive transforms. Scan codebase patterns first. |

### Recommended Architecture for a Skill-Based Refactoring Orchestrator

```
Phase 2: DETECT (fast, incremental)
    - Tree-sitter incremental parsing for candidate identification
    - Graph-based dependency analysis for prioritization
    - Risk scoring (centrality + change frequency + incident history)
    |
Phase 3: PLAN (graph-aware)
    - Critical pair analysis to identify parallelizable vs. sequential operations
    - Topological sort for dependency-ordered execution
    - Cycle budget allocation (max iterations before escalating to human)
    |
Phase 4: TRANSFORM (type-aware, composable)
    - Deterministic AST rules for well-defined patterns (Rector/OpenRewrite style)
    - LLM agents for ambiguous/context-dependent transformations
    - Static analysis pre-filtering to catch hallucinations
    |
Phase 5: VALIDATE (incremental)
    - Scope-limited impact analysis (affected subgraph only)
    - Incremental test execution (only tests touching changed code)
    - Rollback points at each transformation step
    |
Phase 6: LEARN (feedback loop)
    - Track which rules succeeded/failed
    - Adjust prioritization weights based on outcomes
    - Promote successful LLM transformations to deterministic rules
```

This architecture combines the best practices from all researched tools:
- Tree-sitter's incremental speed for detection
- Graph transformation theory for safe dependency handling
- OpenRewrite's LST approach for lossless transformations
- Rector's type-aware rule application
- jscodeshift's composable, test-first methodology
- LLM agents reserved for the cases that deterministic tools cannot handle
