# Eval: Add Agent Mailbox for Cross-Cycle Messaging

## Code Graders (bash commands that must exit 0)
- `grep -q "agent-mailbox\|mailbox" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md`
- `grep -q "mailbox" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-builder.md`
- `grep -q "mailbox" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-auditor.md`
- `grep -q "mailbox" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-scout.md`
- `grep -q "mailbox" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md`

## Regression Evals (full test suite)
- `grep -q "Layer 1: JSONL Ledger" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md`
- `grep -q "scout-report.md" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md`
- `grep -q "Step 1: Read Instincts" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-builder.md`
- `grep -q "Single-Pass Review" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-auditor.md`

## Acceptance Checks (verification commands)
- `grep -c "mailbox" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md | grep -q "^[2-9]\|^[1-9][0-9]"`
- `grep -c "mailbox" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md | grep -q "^[2-9]\|^[1-9][0-9]"`
- `grep -q "from.*to.*message\|persistent\|recipient" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md`

## Thresholds
- All checks: pass@1 = 1.0
