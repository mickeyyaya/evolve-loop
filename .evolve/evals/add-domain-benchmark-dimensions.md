# Eval: Add Domain-Specific Benchmark Dimension Examples

## Code Graders (bash commands that must exit 0)

- `grep -i "prose clarity\|Prose Clarity" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/benchmark-eval.md`
- `grep -i "writing\|research" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/benchmark-eval.md | grep -i "domain\|dimension" | head -1 | grep -q . && exit 0 || exit 1`
- `grep -c "^## Dimension" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/benchmark-eval.md | awk '{exit ($1 != 8)}'`

## Regression Evals (full "test suite" — structural integrity checks)

- `grep -c "Dimension 1:" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/benchmark-eval.md | awk '{exit ($1 < 1)}'`
- `grep -c "Dimension 8:" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/benchmark-eval.md | awk '{exit ($1 < 1)}'`
- `grep -c "Anti-Gaming Policy" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/benchmark-eval.md | awk '{exit ($1 < 1)}'`
- `wc -l < /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/benchmark-eval.md | awk '{exit ($1 > 550)}'`

## Acceptance Checks (verification commands)

- `grep -n "Domain-Specific\|domain-specific" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/benchmark-eval.md`
- `grep -n "Modularity\|modularity" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/benchmark-eval.md | head -3`

## Thresholds
- All checks: pass@1 = 1.0
