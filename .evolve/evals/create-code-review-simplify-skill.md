# Eval: Create Code Review Simplify Skill

## Code Graders (bash commands that must exit 0)

### File existence and structure
- `test -f /Users/danleemh/ai/claude/evolve-loop/skills/code-review-simplify/SKILL.md`
- `head -5 /Users/danleemh/ai/claude/evolve-loop/skills/code-review-simplify/SKILL.md | grep -q "^---"`

### YAML frontmatter has required fields
- `grep -q "^name:" /Users/danleemh/ai/claude/evolve-loop/skills/code-review-simplify/SKILL.md`
- `grep -q "^description:" /Users/danleemh/ai/claude/evolve-loop/skills/code-review-simplify/SKILL.md`

### Required sections exist (behavioral checks)
- `grep -q "## Architecture" /Users/danleemh/ai/claude/evolve-loop/skills/code-review-simplify/SKILL.md`
- `grep -q "## Single-Pass Flow" /Users/danleemh/ai/claude/evolve-loop/skills/code-review-simplify/SKILL.md`
- `grep -q "## Multi-Dimensional Scoring" /Users/danleemh/ai/claude/evolve-loop/skills/code-review-simplify/SKILL.md`
- `grep -q "## Adaptive Depth Routing" /Users/danleemh/ai/claude/evolve-loop/skills/code-review-simplify/SKILL.md`
- `grep -q "## Integration Hooks" /Users/danleemh/ai/claude/evolve-loop/skills/code-review-simplify/SKILL.md`

### Multi-dimensional scoring has at least 4 dimensions
- `grep -c "correctness\|security\|performance\|maintainability" /Users/danleemh/ai/claude/evolve-loop/skills/code-review-simplify/SKILL.md | awk '{exit ($1 >= 4 ? 0 : 1)}'`

### Hybrid architecture is defined (not pure pipeline or pure agentic)
- `grep -q "pipeline" /Users/danleemh/ai/claude/evolve-loop/skills/code-review-simplify/SKILL.md && grep -q "agentic" /Users/danleemh/ai/claude/evolve-loop/skills/code-review-simplify/SKILL.md`

### Adaptive depth has at least 2 tiers
- `grep -c "tier\|Tier\|lightweight\|multi-agent\|full review" /Users/danleemh/ai/claude/evolve-loop/skills/code-review-simplify/SKILL.md | awk '{exit ($1 >= 2 ? 0 : 1)}'`

### Integration hooks reference evolve-loop
- `grep -q "auditor\|evolve-loop\|evolve-auditor" /Users/danleemh/ai/claude/evolve-loop/skills/code-review-simplify/SKILL.md`

### File size under 400 lines
- `wc -l < /Users/danleemh/ai/claude/evolve-loop/skills/code-review-simplify/SKILL.md | awk '{exit ($1 <= 400 ? 0 : 1)}'`

### Numeric scoring present (not just PASS/FAIL)
- `grep -qE "[0-9]\.[0-9]|0-10|1-10|score.*[0-9]|[0-9].*score" /Users/danleemh/ai/claude/evolve-loop/skills/code-review-simplify/SKILL.md`

## Regression Evals
- `test -f /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/SKILL.md`
- `test -f /Users/danleemh/ai/claude/evolve-loop/skills/refactor/SKILL.md`

## Thresholds
- All checks: pass@1 = 1.0
