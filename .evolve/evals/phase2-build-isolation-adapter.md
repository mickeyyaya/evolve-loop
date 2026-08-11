# Eval: Phase 2 BUILD Isolation Adapter (file-copy for non-git domains)

## Code Graders (bash commands that must exit 0)

- `grep -n "file-copy\|fileCopy\|file_copy" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md | grep -qi "isolation" && exit 0 || exit 1`
- `grep -n "buildIsolation" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md | grep -qi "worktree\|file-copy" && exit 0 || exit 1`
- `grep -n "git worktree" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md | head -1 | grep -qi "default\|MANDATORY\|fallback\|worktree" && exit 0 || exit 1`

## Regression Evals (full "test suite" — structural integrity checks)

- `grep -c "Phase 2: BUILD" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md | awk '{exit ($1 < 1)}'`
- `grep -c "NEVER launch the Builder without" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md | awk '{exit ($1 < 1)}'`
- `wc -l < /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md | awk '{exit ($1 > 650)}'`

## Acceptance Checks (verification commands)

- `grep -i "projectContext.buildIsolation" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md`
- `grep -i "file-copy" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md`
- `grep -i "worktree.*default\|default.*worktree" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md`

## Thresholds
- All checks: pass@1 = 1.0
