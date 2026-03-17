# Eval: Add Auditor Adaptive Strictness

## Code Graders (bash commands that must exit 0)
- `grep -q "Adaptive Strictness\|auditorProfile\|adaptiveStrictness" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-auditor.md`
- `grep -q "passFirstAttempt\|pass_first_attempt\|reliabilityScore\|reliability" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-auditor.md`
- `grep -q "auditorProfile" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md`
- `grep -q "auditorProfile" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/SKILL.md`

## Regression Evals (full test suite)
- `grep -q "Single-Pass Review Checklist" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-auditor.md`
- `grep -q "Verdict Rules" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-auditor.md`
- `grep -q "Phase 3: AUDIT" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md`
- `grep -q "Eval Tamper Detection" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-auditor.md`

## Acceptance Checks (verification commands)
- `grep -c "auditorProfile" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md | grep -q "^[2-9]\|^[1-9][0-9]"`
- `grep -c "auditorProfile" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/SKILL.md | grep -q "^[1-9]"`

## Thresholds
- All checks: pass@1 = 1.0
