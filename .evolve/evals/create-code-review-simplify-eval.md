# Eval: Create Code Review Simplify Eval

## Code Graders (bash commands that must exit 0)

### Eval file exists
- `test -f /Users/danleemh/ai/claude/evolve-loop/.evolve/evals/create-code-review-simplify-skill.md`

### Has at least 5 grader commands
- `grep -c "^\- \`" /Users/danleemh/ai/claude/evolve-loop/.evolve/evals/create-code-review-simplify-skill.md | awk '{exit ($1 >= 5 ? 0 : 1)}'`

### At least 2 graders check section headings (behavioral, not just existence)
- `grep -c "grep.*##" /Users/danleemh/ai/claude/evolve-loop/.evolve/evals/create-code-review-simplify-skill.md | awk '{exit ($1 >= 2 ? 0 : 1)}'`

### No Level 0 graders (no echo, exit 0, true no-ops)
- `! grep -qE '^\- \`(echo |exit 0|true)\`' /Users/danleemh/ai/claude/evolve-loop/.evolve/evals/create-code-review-simplify-skill.md`

### Has thresholds section
- `grep -q "## Thresholds" /Users/danleemh/ai/claude/evolve-loop/.evolve/evals/create-code-review-simplify-skill.md`

## Thresholds
- All checks: pass@1 = 1.0
