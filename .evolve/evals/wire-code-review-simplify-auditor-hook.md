# Eval: Wire Code Review Simplify Auditor Hook

## Code Graders (bash commands that must exit 0)

### Auditor references the new skill
- `grep -q "code-review-simplify" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-auditor.md`

### Reference uses a path to the skill
- `grep -q "skills/code-review-simplify" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-auditor.md`

### Existing core sections still present (no destructive changes)
- `grep -q "## Single-Pass Review Checklist" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-auditor.md`
- `grep -q "## Verdict Rules" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-auditor.md`
- `grep -q "## Core Principles" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-auditor.md`
- `grep -q "## Adaptive Strictness" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-auditor.md`

### File is still valid markdown with frontmatter
- `head -1 /Users/danleemh/ai/claude/evolve-loop/agents/evolve-auditor.md | grep -q "^---"`

### File did not shrink significantly (additive change)
- `wc -l < /Users/danleemh/ai/claude/evolve-loop/agents/evolve-auditor.md | awk '{exit ($1 >= 240 ? 0 : 1)}'`

## Regression Evals
- `grep -q "## Output" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-auditor.md`
- `grep -q "Ledger Entry" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-auditor.md`

## Thresholds
- All checks: pass@1 = 1.0
