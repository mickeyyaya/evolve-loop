# Eval: sdlc-phase-gap-analysis

## Task
Builder writes `knowledge-base/research/sdlc-phase-gap-analysis-2026-06.md` — a cited research note enumerating external SDLC phases and mapping each to existing evolve-loop coverage or flagging it as missing.

## Acceptance Criteria

### C1 — research note file exists [code]
```bash
test -f knowledge-base/research/sdlc-phase-gap-analysis-2026-06.md && echo PASS || echo FAIL
```
Expected: `PASS`

### C2 — gap table with ≥5 rows is present [code]
```bash
grep -c "| " knowledge-base/research/sdlc-phase-gap-analysis-2026-06.md
```
Expected: stdout integer ≥ 5

### C3 — table uses covered/missing labels [code]
```bash
grep -iE "COVERED|MISSING|covered|missing" knowledge-base/research/sdlc-phase-gap-analysis-2026-06.md | wc -l | tr -d ' '
```
Expected: integer ≥ 3

### C4 — at least two external source citations [code]
```bash
grep -cE "(https://|Source:|cite:|from )" knowledge-base/research/sdlc-phase-gap-analysis-2026-06.md
```
Expected: integer ≥ 2

### C5 — at least one concrete missing-phase proposal named [code]
```bash
grep -iE "dependency.audit|threat.model|perf.regress|benchmark.phase|spec.check|contract.valid" knowledge-base/research/sdlc-phase-gap-analysis-2026-06.md | wc -l | tr -d ' '
```
Expected: integer ≥ 1

### C6 — integration point named (phase-registry or agent file) [code]
```bash
grep -iE "phase-registry|agent.*evolve|go/internal/core|phase\.json" knowledge-base/research/sdlc-phase-gap-analysis-2026-06.md | wc -l | tr -d ' '
```
Expected: integer ≥ 1

### C7 (negative) — note does NOT propose removing any existing phase [code]
```bash
grep -iE "remove.*phase|delete.*phase|retire.*phase|drop.*intent|drop.*scout|drop.*audit" knowledge-base/research/sdlc-phase-gap-analysis-2026-06.md | wc -l | tr -d ' '
```
Expected: `0`
