# Eval: add-showcase-doc

## Code Graders (bash commands that must exit 0)
- `test -f /Users/danleemh/ai/claude/evolve-loop/docs/showcase.md`
- `wc -l < /Users/danleemh/ai/claude/evolve-loop/docs/showcase.md | awk '{if ($1 >= 80) exit 0; else exit 1}'`
- `grep -c "decisionTrace\|decision trace\|Decision Trace" /Users/danleemh/ai/claude/evolve-loop/docs/showcase.md | grep -E '^[1-9]'`
- `grep -c "agent-mailbox\|Agent Mailbox\|mailbox" /Users/danleemh/ai/claude/evolve-loop/docs/showcase.md | grep -E '^[1-9]'`
- `grep -c "showcase" /Users/danleemh/ai/claude/evolve-loop/README.md | grep -E '^[1-9]'`

## Regression Evals (full test suite)
- `grep -c "builder-notes\|Builder Notes\|retrospective" /Users/danleemh/ai/claude/evolve-loop/docs/showcase.md | grep -E '^[1-9]'`
- `grep -c "instinct" /Users/danleemh/ai/claude/evolve-loop/docs/showcase.md | grep -E '^[1-9]'`

## Acceptance Checks (verification commands)
- `head -5 /Users/danleemh/ai/claude/evolve-loop/docs/showcase.md`
- `grep "showcase" /Users/danleemh/ai/claude/evolve-loop/README.md`

## Thresholds
- All checks: pass@1 = 1.0
