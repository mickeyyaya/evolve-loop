# Eval: write-missing-phases-kb-doc

## Goal
Verify that the knowledge base research document on missing development phases exists and contains the required sections.

## Acceptance Criteria

### AC1: KB document exists [code]
```bash
test -f /Users/danleemh/ai/claude/evolve-loop/knowledge-base/research/missing-development-phases-2026-06-03.md && echo "PASS" || echo "FAIL"
```
Expected: `PASS`

### AC2: Document contains research findings section [code]
```bash
grep -q "## Research Findings\|## Missing Phases\|## Phase Taxonomy" \
  /Users/danleemh/ai/claude/evolve-loop/knowledge-base/research/missing-development-phases-2026-06-03.md && \
  echo "PASS" || echo "FAIL"
```
Expected: `PASS`

### AC3: Document covers security-scan phase [code]
```bash
grep -qi "security" \
  /Users/danleemh/ai/claude/evolve-loop/knowledge-base/research/missing-development-phases-2026-06-03.md && \
  echo "PASS" || echo "FAIL"
```
Expected: `PASS`

### AC4: Document covers dependency audit phase [code]
```bash
grep -qi "dependency\|dep-audit\|dependency-audit" \
  /Users/danleemh/ai/claude/evolve-loop/knowledge-base/research/missing-development-phases-2026-06-03.md && \
  echo "PASS" || echo "FAIL"
```
Expected: `PASS`

### AC5: Document has implementation guidance (phase.json format or spec) [code]
```bash
grep -q "phase\.json\|PhaseSpec\|optional.*true\|insert_when" \
  /Users/danleemh/ai/claude/evolve-loop/knowledge-base/research/missing-development-phases-2026-06-03.md && \
  echo "PASS" || echo "FAIL"
```
Expected: `PASS`

### AC6: Negative — document is not empty (>500 bytes) [code]
```bash
size=$(wc -c < /Users/danleemh/ai/claude/evolve-loop/knowledge-base/research/missing-development-phases-2026-06-03.md)
if [ "$size" -gt 500 ]; then echo "PASS"; else echo "FAIL: too small ($size bytes)"; fi
```
Expected: `PASS`

### AC7: Edge — document does NOT recommend mandatory phases (user phases must be optional) [code]
```bash
# The KB doc must not recommend making new phases mandatory (they break the floor)
if grep -qi "mandatory.*new\|make.*mandatory" \
  /Users/danleemh/ai/claude/evolve-loop/knowledge-base/research/missing-development-phases-2026-06-03.md; then
  echo "FAIL: document recommends mandatory new phases (violation)"
else
  echo "PASS"
fi
```
Expected: `PASS`
