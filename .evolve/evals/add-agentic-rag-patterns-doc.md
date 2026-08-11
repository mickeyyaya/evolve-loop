# Eval: add-agentic-rag-patterns-doc

> Graders for the agentic RAG patterns documentation task.

## Graders

| # | Type | Command / Check | Expected |
|---|------|----------------|----------|
| 1 | bash | `test -f docs/agentic-rag-patterns.md` | exit 0 |
| 2 | bash | `grep -q "hierarchical\|Hierarchical" docs/agentic-rag-patterns.md` | exit 0 |
| 3 | bash | `grep -q "keyword.*search\|semantic.*search\|chunk.*read" docs/agentic-rag-patterns.md` | exit 0 |
| 4 | bash | `grep -q "GraphRAG\|RAPTOR\|LightRAG\|HippoRAG" docs/agentic-rag-patterns.md` | exit 0 |
| 5 | bash | `grep -q "retrieval\|Retrieval" docs/agentic-rag-patterns.md` | exit 0 |
| 6 | bash | `wc -l < docs/agentic-rag-patterns.md \| awk '{exit ($1 > 400)}'` | exit 0 — under 400 lines |
| 7 | bash | `grep -c "^|" docs/agentic-rag-patterns.md \| awk '{exit ($1 < 5)}'` | exit 0 — at least 5 table rows |
| 8 | bash | `grep -q "anti-pattern\|Anti-pattern\|ANTI-PATTERN" docs/agentic-rag-patterns.md` | exit 0 |
| 9 | bash | `grep -q "Scout\|Builder\|Auditor" docs/agentic-rag-patterns.md` | exit 0 — maps to evolve-loop agents |
