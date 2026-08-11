# Eval: Fix Schema Hygiene — Add Missing Fields to state.json Example

## Code Graders (bash commands that must exit 0)
- `grep -q "processRewardsHistory" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md`
- `grep -q "fitnessScore" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md`
- `grep -q "fitnessHistory" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md`
- `grep -q "fitnessRegression" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md`

## Regression Evals (full test suite)
- `wc -l < /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md | awk '{exit ($1 > 430)}'`

## Acceptance Checks (verification commands)
- `python3 -c "
import re, sys
content = open('/Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md').read()
# Find the state.json code block (first json block)
m = re.search(r'\x60\x60\x60json\n(\{.*?\})\n\x60\x60\x60', content, re.DOTALL)
if not m: sys.exit(1)
block = m.group(1)
for field in ['processRewardsHistory', 'fitnessScore', 'fitnessHistory', 'fitnessRegression']:
    if field not in block:
        print(f'MISSING: {field}')
        sys.exit(1)
print('All 4 fields present in JSON example block')
sys.exit(0)
"`

## Thresholds
- All checks: pass@1 = 1.0
