# Eval: Fix Architecture Mailbox Reference

## Code Graders (bash commands that must exit 0)
- `grep -v "mailbox\.json" docs/architecture.md`
- `grep -qi "agent-mailbox\.md\|agent_mailbox" docs/architecture.md`

## Regression Evals (full test suite)
- `test -f docs/architecture.md`
- `grep -q "Agent Mailbox" docs/architecture.md`
- `grep -q "pipeline" docs/architecture.md`

## Acceptance Checks (verification commands)
- `grep -c "mailbox\.json" docs/architecture.md | awk '{exit ($1 != 0)}'` → no occurrences of old mailbox.json reference
- `grep -qi "agent-mailbox\.md\|markdown.*table\|table.*markdown\|persistent" docs/architecture.md` → new accurate description present

## Thresholds
- All checks: pass@1 = 1.0
