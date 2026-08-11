# Eval: add-strategy-playbook-protocol

## Code Graders (bash commands that must exit 0)
- `grep -q "strategy.playbook\|strategyPlaybook\|strategy-playbook" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phase5-learn.md && echo "PASS: strategy playbook referenced in phase5"`
- `grep -q "anti.collapse\|antiCollapse\|incremental.update" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phase5-learn.md && echo "PASS: anti-collapse safeguards present"`
- `grep -c "ACE\|Agentic.Context.Engineering\|generation.reflection.curation\|GRC" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phase5-learn.md | awk '{if ($1 >= 1) {print "PASS: ACE-inspired concepts present"; exit 0} else {print "FAIL: no ACE references"; exit 1}}'`

## Regression Evals (full test suite)
- `bash /Users/danleemh/ai/claude/evolve-loop/scripts/eval-quality-check.sh`

## Acceptance Checks (verification commands)
- `grep -q "reflect.*before\|reflection.*step\|reflect-then-curate" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phase5-learn.md && echo "PASS: reflect-before-curate gate present"`
- `grep -q "consolidation.ratio\|3:1\|merge.cap\|never.merge.*different.*categor" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phase5-learn.md && echo "PASS: consolidation safeguards present"`

## Thresholds
- All checks: pass@1 = 1.0
