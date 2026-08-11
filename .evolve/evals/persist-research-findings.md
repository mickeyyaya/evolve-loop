# Eval: persist-research-findings

## Task
Persist research on long-horizon agentic coding, self-healing, and progress tracking to knowledge-base/research/.

## Criteria

### C1: Research file exists with required sections [code]
```bash
test -f knowledge-base/research/long-horizon-agentic-coding-2026.md \
  && grep -q "## Gap Analysis" knowledge-base/research/long-horizon-agentic-coding-2026.md \
  && grep -q "## Sources" knowledge-base/research/long-horizon-agentic-coding-2026.md \
  && grep -q "## Self-Healing" knowledge-base/research/long-horizon-agentic-coding-2026.md \
  && echo "PASS" || echo "FAIL"
```

### C2: Gap analysis maps techniques to existing subsystems [code]
```bash
grep -E "(HAVE|PARTIAL|MISSING)" knowledge-base/research/long-horizon-agentic-coding-2026.md | wc -l | \
  awk '{if ($1 >= 6) print "PASS"; else print "FAIL - expected >=6 gap entries, got " $1}'
```

### C3: Source citations present [code]
```bash
grep -cE "(http|Source|URL)" knowledge-base/research/long-horizon-agentic-coding-2026.md | \
  awk '{if ($1 >= 4) print "PASS"; else print "FAIL - expected >=4 source refs, got " $1}'
```

### C4: Negative case — file must NOT be a verbatim copy of the web research summary [code]
```bash
# File must be > 200 lines (KB dossier standard: must be more than a quick summary)
wc -l < knowledge-base/research/long-horizon-agentic-coding-2026.md | \
  awk '{if ($1 >= 100) print "PASS"; else print "FAIL - file too short, likely incomplete: " $1 " lines"}'
```
