# Automated Scan Pipeline & Speed Optimizations

> Read this file during Phase 1 (Scan) to launch parallel analysis tools and optimize wall-clock time on re-scans.

## Automated Scan Pipeline

Run all static analysis tools in parallel during Phase 1 to minimize wall-clock time. Launch these simultaneously:

| Tool | Purpose | Output |
|------|---------|--------|
| jscpd | Duplicate code detection (>25 token threshold) | Duplicate block locations |
| knip | Dead code and unused export detection | Unused files, exports, dependencies |
| dependency-cruiser | Module graph and architecture boundary validation | Circular deps, violations |
| Complexity analyzer | Cognitive complexity per function | Per-function scores |
| ESLint/Biome/Ruff | Language-specific lint rules | Lint violations |

### Pipeline Execution

```bash
# Launch all tools in parallel
jscpd --min-tokens 25 --reporters json src/ &
npx knip --reporter json &
npx depcruise --output-type json src/ &
# Complexity analysis runs inline (see complexity-scoring.md)
wait
```

### Incremental Analysis

When re-running on a previously scanned codebase, only analyze changed files:

1. Compute changed files: `git diff --name-only <last-scan-commit>..HEAD`
2. Run tools only on changed files
3. Merge with cached results for unchanged files
4. Invalidate cache entries for files whose dependencies changed

### Static Analysis Pre-filtering

Run IDE-level static checks before any LLM-based analysis to eliminate hallucination waste:

1. Type checker (tsc, mypy, pyright) catches type errors the LLM might miss
2. Linter catches formatting and convention issues
3. Only pass genuine code smells and architectural issues to LLM analysis
4. This eliminates approximately 6-8% of false positives from LLM-only analysis

## Speed Optimizations

Apply these patterns to reduce refactoring wall-clock time.

### Parallel Scan Pipeline

Run all Phase 1 analysis tools simultaneously rather than sequentially. See the table above for the full list. Expected speedup: 3-5x on typical codebases.

### Incremental Re-analysis

| Scenario | Strategy |
|----------|----------|
| First scan | Full analysis of all files in scope |
| Re-scan after fix | Only re-analyze files in the fix's write set |
| Re-scan after merge | Only re-analyze files changed since last scan commit |
| Cached results | Store per-file analysis keyed by file content hash |

### Scope-Limited Impact Analysis

When planning a fix, do NOT analyze the full codebase for impact. Instead:

1. Build the affected subgraph: start from the file being modified
2. Walk outward through direct importers (fan-in, depth 1)
3. Walk outward through importers of importers (depth 2) only if the public API changes
4. Stop at depth 2 — changes rarely propagate further
5. Only run tests that cover files in the affected subgraph

### Analysis Ordering Strategy

Run cheap deterministic checks before expensive LLM analysis:

| Order | Check | Cost | Catches |
|-------|-------|------|---------|
| 1 | Type checker | Low | Type errors, null safety |
| 2 | Linter | Low | Style, conventions, simple bugs |
| 3 | Complexity analyzer | Low | Function complexity scores |
| 4 | Duplicate detector | Medium | Copy-paste code blocks |
| 5 | Architecture validator | Medium | Boundary violations, cycles |
| 6 | LLM-based smell detection | High | Semantic smells, design issues |

This pipeline eliminates 6-8% of false positive work from LLM-only analysis.

### Cycle-Based Execution

Support up to 3 refactoring passes per group when refactoring creates emergent work. See [workflow.md](workflow.md) § Phase 4 for details. Limit to 3 passes to prevent infinite loops from oscillating fixes.
