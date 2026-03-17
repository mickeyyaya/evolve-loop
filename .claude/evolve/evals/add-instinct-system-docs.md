# Eval: add-instinct-system-docs

## Code Graders (bash commands that must exit 0)
- `bash -c 'test -f /Users/danleemh/ai/claude/evolve-loop/docs/instincts.md && echo OK'`
- `bash -c 'grep -q "confidence" /Users/danleemh/ai/claude/evolve-loop/docs/instincts.md && echo OK'`
- `bash -c 'grep -q "personal/" /Users/danleemh/ai/claude/evolve-loop/docs/instincts.md && echo OK'`
- `bash -c 'grep -q "id:\|pattern:\|description:\|source:\|type:" /Users/danleemh/ai/claude/evolve-loop/docs/instincts.md && echo OK'`

## Regression Evals (full test suite)
- `CI=true /Users/danleemh/ai/claude/evolve-loop/install.sh`

## Acceptance Checks (verification commands)
- `grep -c "##" /Users/danleemh/ai/claude/evolve-loop/docs/instincts.md`
- `grep -n "promotion\|promote\|global" /Users/danleemh/ai/claude/evolve-loop/docs/instincts.md`
- `grep -n "inspect\|edit\|view" /Users/danleemh/ai/claude/evolve-loop/docs/instincts.md`

## Thresholds
- All checks: pass@1 = 1.0
