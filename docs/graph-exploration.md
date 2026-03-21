# Graph-Based Code Exploration for Scout

<!-- challenge token: e191a9b7aee996f9 -->

How the Scout phase can use graph-based traversal to navigate large codebases efficiently without loading every file.

---

## Motivation

Full codebase scans work well for small projects but become expensive as a repo grows. Reading every file in a 500-file codebase to find three relevant ones wastes tokens and degrades Scout focus. Graph-based exploration offers a structured alternative: build a lightweight map of the codebase and traverse only the nodes relevant to the current task.

---

## Research Basis

**RepoMaster** (arXiv:2505.21577) demonstrates graph-based repo exploration at scale:
- Constructs a hierarchical graph of files, modules, and call relationships
- Agents traverse the graph selectively — following import edges, call chains, and dependency links — rather than reading files linearly
- Reported results: **95% token reduction** on large repo navigation tasks and a **110% improvement** on SWE-bench submission scores vs. flat file reading baselines
- Key insight: most tasks are local — they touch 3-7 files even in a 500-file repo. A graph makes this locality explicit.

**GraphReader** (arXiv:2406.14550) applies the same principle to long-document QA:
- A 4K-context agent operating on a graph outperforms a 128K-context agent reading raw text
- Demonstrates that structured traversal beats brute-force context expansion
- Relevance to evolve-loop: Scout operates under a turn budget. Graph traversal lets it answer "which files are relevant?" in 2-3 turns instead of 8-10.

---

## How Graph Exploration Works

### 1. Build the Graph (once per full scan)

On cycle 1 or after a major restructure, Scout constructs a lightweight index:

```
nodes: files, directories, exported symbols
edges: imports, function calls, shared type references
metadata per node: path, size, last_modified, role tag (agent|skill|config|test|doc)
```

This index is stored as a compact JSON structure in `state.json` or a sidecar file. It is NOT a deep AST — it is derived from surface-level signals (import statements, directory structure, file naming conventions).

### 2. Traverse for the Current Task

Given a task (e.g., "add a token budget check to Builder"), Scout:
1. Identifies seed nodes matching the task keywords (builder, token, budget)
2. Expands one hop along import and call edges
3. Reads only the nodes in the resulting subgraph (typically 3-7 files)
4. Ranks by edge centrality to prioritize the most connected relevant files

### 3. Return Ranked File List

The output is an ordered list of `(file, relevance_score, reason)` tuples — the same format Scout already uses in scout-report.md. Graph exploration is the *mechanism* for generating that list, not a different output format.

---

## When to Use Graph Exploration vs. Full File Reading

Graph exploration is a **complement** to full file reading, not a replacement.

| Situation | Approach |
|-----------|----------|
| Large codebase (50+ files), finding relevant files | Graph traversal first, then read ranked results |
| Agent definition files (SKILL.md, CLAUDE.md, agent prompts) | Always read in full — these are load-bearing |
| Skill files and instinct YAMLs | Always read in full — partial reads miss critical rules |
| Configuration files (state.json, package.json) | Always read in full |
| Docs and markdown files already known by name | Read directly — no graph traversal needed |
| Cycle 1 or after a major restructure | Full scan to build/refresh the graph index |
| Incremental cycles (cycle 2+) with a known task | Graph traversal on changed files only |

The rule: **graph exploration answers "which files?" — full reading answers "what exactly does this file say?"**. Never substitute graph metadata for full file content when the file is load-bearing for the task.

---

## Integration with Evolve-Loop Scout

The Scout already performs incremental scans on cycle 2+ (see `docs/token-optimization.md` — Incremental Scan section). Graph-based exploration extends this:

- **Incremental scan** skips unchanged files
- **Graph traversal** finds which changed or new files are actually relevant to the task

Both mechanisms compound: incremental scan narrows the candidate set, graph traversal ranks what remains.

**Turn budget alignment:** With a 5-turn Scout budget, graph traversal fits naturally — 1 turn to check the index, 1-2 turns to traverse and rank, 1-2 turns to read the top-ranked files in full.

---

## Implementation Notes

- The graph index should be regenerated whenever `git diff --name-only` shows structural changes (new directories, deleted files, renamed modules)
- Edge extraction can use simple regex on import/require statements — no full AST parser needed
- For repos without import statements (shell scripts, markdown-only projects), use directory co-location and naming conventions as proxy edges
- Store the index under `.evolve/workspace/codebase-graph.json`; treat it as ephemeral (regenerate on full scan, update incrementally on cycle 2+)
