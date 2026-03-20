# Project Digest — Cycle 19

## Directory Tree (2 levels, line counts)

```
.                              (root)
├── CLAUDE.md                  13
├── CHANGELOG.md               308
├── README.md                  281
├── CONTRIBUTING.md            50
├── install.sh                 146
├── publish.sh                 228
├── uninstall.sh               79
├── .claude-plugin/
│   ├── marketplace.json       35
│   └── plugin.json            30
├── agents/
│   ├── evolve-auditor.md      193
│   ├── evolve-builder.md      222
│   ├── evolve-operator.md     237
│   └── evolve-scout.md        333
├── skills/evolve-loop/
│   ├── SKILL.md               377
│   ├── benchmark-eval.md      479
│   ├── eval-runner.md         261
│   ├── memory-protocol.md     422
│   ├── phase4-ship.md         215  ← NEW since cycle 15
│   ├── phase5-learn.md        422
│   └── phases.md              471  (was 672 in cycle 15)
├── docs/                      (2,455 lines total)
│   ├── showcase.md            464
│   ├── configuration.md       259
│   ├── token-optimization.md  253
│   ├── self-learning.md       218  (grew significantly)
│   ├── architecture.md        200
│   ├── eval-grader-best-practices.md  197  ← NEW since cycle 15
│   ├── memory-hierarchy.md    183  (grew significantly)
│   └── 10 more docs           681
└── examples/
    └── eval-definition.md     22
```

Total: ~7,866 lines across 34 files.

## Tech Stack

- **Language**: Markdown + Shell (Bash)
- **Platform**: Claude Code plugin (`.claude-plugin/`)
- **Agents**: 4 evolve agents (operator, builder, scout, auditor)
- **Skills**: evolve-loop (7 skill files, 2,647 lines)

## Hotspots (largest files)

| File | Lines | Role |
|------|-------|------|
| skills/evolve-loop/benchmark-eval.md | 479 | Eval definitions |
| skills/evolve-loop/phases.md | 471 | Core phase logic |
| docs/showcase.md | 464 | Usage examples |
| skills/evolve-loop/memory-protocol.md | 422 | MUSE memory protocol |
| skills/evolve-loop/phase5-learn.md | 422 | Learning phase |

## Conventions

- Commit format: `<type>: <description> [worktree-build]`
- Docs organized by domain in `docs/`; skill logic in `skills/evolve-loop/`
- Agents follow `evolve-<role>.md` naming
- Immutable data patterns; files under 800 lines

## Recent Git History

```
b6753c3 fix: add skillEfficiency to processRewards schema in memory-protocol
341c0fa feat: add confidence-correctness alignment from research
12247b8 fix: use macOS-compatible regex for link-checker grader
4029348 fix: use repo-root-relative link for phase4-ship.md reference
f9e707d docs: trim MUSE memory categories for S-task line budget
7666727 feat: add coefficient of self-improvement metric to self-learning doc
a026a0b docs: trim experience scoring section for S-task line budget
ffba906 feat: add stepwise self-evaluation protocol from research
```
