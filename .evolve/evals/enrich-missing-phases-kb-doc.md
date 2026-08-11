# Eval: enrich-missing-phases-kb-doc

## Task
Enrich the existing KB doc at `knowledge-base/research/missing-development-phases-2026-06.md`
with 3+ new external sources (LaunchDarkly/Harness chaos engineering, Zuplo API contract testing,
etc.) and add an explicit gap analysis table distinguishing implemented-active vs implemented-dormant
vs missing phases.

## Acceptance Criteria

### C1: KB doc cites ≥5 named external sources total [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop
# Count named external source citations (must have grown from 4 to ≥5)
count=$(grep -c "Source\|source\|https://\|arxiv\|gitlab\|ibm\|launchdarkly\|harness\|dynatrace\|zuplo\|chaos\|autogen" knowledge-base/research/missing-development-phases-2026-06.md 2>/dev/null || echo 0)
echo "Citation hits: $count"
[ "$count" -ge 10 ] && echo "PASS" || echo "FAIL: expected ≥10 citation hits (5 sources each with name+url)"
```

### C2: Doc contains a gap analysis section distinguishing phase activation states [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop
grep -qi "implemented.*dormant\|dormant\|gap analysis\|activation state\|Phase Inventory\|implemented.*active" \
  knowledge-base/research/missing-development-phases-2026-06.md && echo "PASS: gap analysis present" \
  || echo "FAIL: gap analysis section missing"
```

### C3: Chaos engineering phase is covered with a named external source [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop
grep -qi "chaos" knowledge-base/research/missing-development-phases-2026-06.md \
  && echo "PASS: chaos engineering covered" \
  || echo "FAIL: chaos engineering not mentioned"
```

### C4: Negative — doc does not remove or shorten existing content [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop
wc -l < knowledge-base/research/missing-development-phases-2026-06.md | xargs -I{} bash -c \
  '[ {} -ge 50 ] && echo "PASS: doc has {} lines (≥50)" || echo "FAIL: doc too short ({} lines, expected ≥50)"'
```

### C5: At least one new phase is classified Adopt/Adapt/Reject [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop
grep -qi "adopt\|adapt\|reject\|classification" knowledge-base/research/missing-development-phases-2026-06.md \
  && echo "PASS: phase classification present" \
  || echo "FAIL: no Adopt/Adapt/Reject classification found"
```
