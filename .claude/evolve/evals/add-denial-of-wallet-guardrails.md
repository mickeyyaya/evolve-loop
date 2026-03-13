# Eval: add-denial-of-wallet-guardrails

## Code Graders (bash commands that must exit 0)
- `grep -q "maxCyclesPerSession" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md`
- `grep -q "warnAfterCycles" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md`
- `grep -q "maxCyclesPerSession" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/SKILL.md`
- `grep -q "warnAfterCycles" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/SKILL.md`

## Regression Evals (full test suite)
- `CI=true ./install.sh`

## Acceptance Checks (verification commands)
- `grep -c "maxCyclesPerSession\|warnAfterCycles" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md | grep -v "^0$"`
- `grep -q "HALT\|halt\|exceeded\|warn" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/SKILL.md`
- `grep -q "maxCyclesPerSession\|warnAfterCycles" /Users/danleemh/ai/claude/evolve-loop/.claude/evolve/state.json`

## Thresholds
- All checks: pass@1 = 1.0
