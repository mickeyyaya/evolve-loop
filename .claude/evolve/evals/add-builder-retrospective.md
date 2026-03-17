# Eval: Add Builder Retrospective Annotations

## Code Graders (bash commands that must exit 0)
- `grep -q "Retrospective\|retrospective" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-builder.md`
- `grep -q "builder-notes.md" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-builder.md`
- `grep -q "builder-notes.md" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-scout.md`
- `grep -q "builder-notes.md" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md`
- `grep -q "builder-notes.md" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md`

## Regression Evals (full test suite)
- `grep -q "Step 1: Read Instincts" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-builder.md`
- `grep -q "instinctsApplied" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-builder.md`
- `grep -q "Phase 1: DISCOVER" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md`
- `grep -q "scout-report.md" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md`

## Acceptance Checks (verification commands)
- `grep -c "builder-notes.md" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-scout.md | grep -q "^[1-9]"`
- `grep -c "builder-notes.md" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md | grep -q "^[1-9]"`
- `grep -c "builder-notes.md" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md | grep -q "^[1-9]"`

## Thresholds
- All checks: pass@1 = 1.0
