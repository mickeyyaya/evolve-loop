# Eval: add-memory-consolidation-pipeline-doc

> Graders for the memory consolidation pipeline documentation task.

## Graders

| # | Type | Command / Check | Expected |
|---|------|----------------|----------|
| 1 | bash | `test -f docs/memory-consolidation-pipeline.md` | exit 0 |
| 2 | bash | `grep -q "episodic" docs/memory-consolidation-pipeline.md` | exit 0 |
| 3 | bash | `grep -q "semantic" docs/memory-consolidation-pipeline.md` | exit 0 |
| 4 | bash | `grep -q "procedural" docs/memory-consolidation-pipeline.md` | exit 0 |
| 5 | bash | `grep -q "instinct\|gene" docs/memory-consolidation-pipeline.md` | exit 0 |
| 6 | bash | `grep -q "consolidat" docs/memory-consolidation-pipeline.md` | exit 0 |
| 7 | bash | `wc -l < docs/memory-consolidation-pipeline.md \| awk '{exit ($1 > 400)}'` | exit 0 |
| 8 | bash | `grep -c "^|" docs/memory-consolidation-pipeline.md \| awk '{exit ($1 < 8)}'` | exit 0 — at least 8 table rows |
| 9 | bash | `grep -q "MemRL\|A-MEM\|HiAgent" docs/memory-consolidation-pipeline.md` | exit 0 |
| 10 | bash | `grep -q "forgetting\|retention" docs/memory-consolidation-pipeline.md` | exit 0 |
