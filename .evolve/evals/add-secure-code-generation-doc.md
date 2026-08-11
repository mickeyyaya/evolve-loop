# Eval: add-secure-code-generation-doc

## Code Graders (bash commands that must exit 0)

- `test -f /Users/danleemh/ai/claude/evolve-loop/docs/secure-code-generation.md`
- `wc -l < /Users/danleemh/ai/claude/evolve-loop/docs/secure-code-generation.md | awk '{exit ($1 < 40 || $1 > 120)}'`
- `grep -q "static anal" /Users/danleemh/ai/claude/evolve-loop/docs/secure-code-generation.md`
- `grep -q "instinct" /Users/danleemh/ai/claude/evolve-loop/docs/secure-code-generation.md`
- `grep -q "eval grader" /Users/danleemh/ai/claude/evolve-loop/docs/secure-code-generation.md`
- `grep -q "2602.05868\|arXiv" /Users/danleemh/ai/claude/evolve-loop/docs/secure-code-generation.md`
- `grep -q "research-paper-index" /Users/danleemh/ai/claude/evolve-loop/docs/secure-code-generation.md`
- `grep -q "secure-code-generation\|2602.05868" /Users/danleemh/ai/claude/evolve-loop/docs/research-paper-index.md`

## Regression Evals (full test suite)

- `grep -q "research-paper-index" /Users/danleemh/ai/claude/evolve-loop/docs/secure-code-generation.md`

## Acceptance Checks (verification commands)

- `grep -c "##" /Users/danleemh/ai/claude/evolve-loop/docs/secure-code-generation.md | awk '{exit ($1 < 3)}'`

## Thresholds

- All checks: pass@1 = 1.0
