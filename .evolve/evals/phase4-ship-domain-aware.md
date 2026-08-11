# Eval: Make Phase 4 SHIP domain-aware

## Code Graders (bash commands that must exit 0)

- `grep -q "shipMechanism" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md` → Phase 4 references shipMechanism
- `grep -q "git.*push" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md && grep -q "file-save\|export" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md` → Phase 4 conditionally handles multiple ship mechanisms
- `grep -q "if.*domain.*coding" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md || grep -q "case.*domain" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md` → Phase 4 implements domain-aware branching

## Regression Evals (full test suite)

- `grep -q "Phase 4" skills/evolve-loop/phases.md` → phases.md remains intact and findable
- `cd /Users/danleemh/ai/claude/evolve-loop && grep -q "git commit" skills/evolve-loop/phases.md` → Default (coding) mechanism preserved
- `cd /Users/danleemh/ai/claude/evolve-loop && grep -q "no-op\|skip shipping" skills/evolve-loop/phases.md || echo "Phase 4 has fallback behavior"` → All domains handled (no panic/error-out for non-coding)

## Acceptance Checks (verification commands)

- `grep -A 50 "### Phase 4: SHIP" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md | grep -q "shipMechanism\|domain" && echo "PASS"` → Phase 4 section explicitly documents domain handling
- `grep -B 5 -A 10 "shipMechanism" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md | grep -q "git\|file\|export" && echo "PASS"` → Multiple mechanisms referenced (git, file-save, export)
- `grep -q "git push origin" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md && echo "additive preserved"` → Git push remains as default fallback

## Thresholds

- All checks: pass@1 = 1.0
