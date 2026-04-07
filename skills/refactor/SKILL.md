---
name: refactor
description: Use when the user asks to refactor code, review code quality, or fix code smells — orchestrates the full refactoring pipeline from detection through fix, with parallel worktree isolation per independent refactoring group
---

# /refactor — Full Refactoring Pipeline

> Orchestrate the complete refactoring workflow: detect smells, prioritize by weighted scoring, partition into independent groups, execute in parallel worktrees via subagents, merge and verify. Enhanced with cognitive complexity scoring, architecture analysis, and speed-optimized parallel scan pipelines.

## Contents

| Section | Purpose |
|---------|---------|
| [Auto Mode Detection](#auto-mode-detection) | Detect bypass/yolo mode for autonomous execution |
| [Git Isolation](#git-isolation-mandatory) | Worktree-based isolation for all refactoring work |
| [Workflow Summary](#workflow-summary) | 5-phase pipeline overview |
| [Quick Modes](#quick-modes) | Scoped invocations for targeted refactoring |
| [Cross-Reference Map](#cross-reference-map) | Skill routing by issue type |
| [Reference](#reference-read-on-demand) | Detailed phase docs, catalogs, techniques |

## Auto Mode Detection

Before starting, check if the user is in **bypass/yolo mode** (auto-accept permissions enabled). Detection signals:
- User explicitly said "yolo mode", "bypass permissions", or "auto-accept"
- The session is running with `--dangerously-skip-permissions` or equivalent
- Tools are being auto-approved without user prompts

**When auto mode is detected:** Skip all confirmation prompts. Automatically select all issues, partition, launch parallel subagents, and merge passing branches without pausing. This enables fully autonomous refactoring.

**When auto mode is NOT detected:** Pause for user confirmation at each checkpoint described in [reference/workflow.md](reference/workflow.md).

## Git Isolation (MANDATORY)

All refactoring work MUST be done in isolated git worktrees branched from main. Never commit directly to main.

### Execution Modes

The orchestrator selects the execution mode based on the partition result from Phase 3:

| Condition | Mode | Worktrees |
|-----------|------|-----------|
| 1 group or all issues touch the same files | **Sequential** | 1 worktree, 1 branch |
| N independent groups with no file overlap | **Parallel** | N worktrees, N branches, N subagents |

### Worktree Naming Convention

```
../refactor-wt-<group-slug>     →  branch: refactor/<group-slug>
```

Examples:
```
../refactor-wt-extract-auth     →  refactor/extract-auth
../refactor-wt-simplify-payment →  refactor/simplify-payment
../refactor-wt-cleanup-utils    →  refactor/cleanup-utils
```

## Workflow Summary

```
Phase 1  SCAN ──────────── on main (read-only, parallel tool pipeline)
Phase 2  PRIORITIZE ────── on main (read-only, weighted scoring)
Phase 3  PLAN & PARTITION ─ on main (read-only, graph-based critical pair analysis)
Phase 4  EXECUTE ────────── in worktrees (parallel subagents, up to 3 passes)
Phase 5  MERGE & VERIFY ── back on main (sequential merge, final scan)
```

For the full phase-by-phase protocol including subagent prompts, isolation rules, and merge handling, read [reference/workflow.md](reference/workflow.md).

For scan tool launch and speed optimizations, read [reference/scan-pipeline.md](reference/scan-pipeline.md).

## Quick Modes

The user can scope the refactoring with arguments:

| Command | Behavior |
|---------|----------|
| `/refactor` | Full pipeline on current file or user-specified files |
| `/refactor scan` | Phase 1 only — detect and report, no changes |
| `/refactor fix <smell>` | Skip to Phase 3-4 for a specific known smell |
| `/refactor security` | Security-focused scan using `security-patterns-code-review` |
| `/refactor performance` | Performance-focused scan using `performance-anti-patterns` |
| `/refactor review` | Full code review using `review-cheat-sheet` as guide |
| `/refactor auto` | Force auto mode — plan, partition, execute in parallel, merge without confirmation |
| `/refactor arch` | Architecture-only analysis — circular deps, boundaries, centrality |
| `/refactor complexity` | Cognitive complexity report only — no fixes |
| `/refactor health` | Composite health score per function — multi-metric scoring |
| `/refactor diff` | Scan only files changed since last commit |
| `/refactor hotspots` | Git-history analysis — find high-churn, high-smell files |

## Cross-Reference Map

| When you find... | Use skill... |
|------------------|-------------|
| Code smells | `detect-code-smells` |
| Anti-patterns | `anti-patterns-catalog` |
| Which technique to use | `refactoring-decision-matrix` |
| Long/complex methods | `refactor-composing-methods` |
| Misplaced responsibilities | `refactor-moving-features` |
| Data handling issues | `refactor-organizing-data` |
| Complex conditionals | `refactor-simplifying-conditionals` |
| Bad method interfaces | `refactor-simplifying-method-calls` |
| Inheritance problems | `refactor-generalization` |
| FP improvements | `refactor-functional-patterns` |
| Type safety gaps | `type-system-patterns` |
| Design pattern opportunity | `design-patterns-creational-structural`, `design-patterns-behavioral` |
| Architecture violations | `architectural-patterns` |
| Security vulnerabilities | `security-patterns-code-review` |
| Performance issues | `performance-anti-patterns` |
| Database problems | `database-review-patterns` |
| Error handling gaps | `error-handling-patterns` |
| Concurrency bugs | `concurrency-patterns` |
| Test quality issues | `testing-patterns` |
| Observability gaps | `observability-patterns` |
| DI/coupling problems | `dependency-injection-module-patterns` |
| Language anti-idioms | `language-specific-idioms` |
| API contract issues | `review-api-contract` |
| SOLID violations | `review-solid-clean-code` |
| Full code review | `review-cheat-sheet` |

## Reference (read on demand)

### Pipeline Execution
| File | When to read |
|------|-------------|
| [reference/workflow.md](reference/workflow.md) | Detailed Phases 1-5 with subagent prompts and merge protocol |
| [reference/scan-pipeline.md](reference/scan-pipeline.md) | Phase 1: parallel tool execution + speed optimizations |
| [reference/continuous-integration.md](reference/continuous-integration.md) | PR review integration, debt tracking, velocity dashboards |

### Detection & Scoring
| File | When to read |
|------|-------------|
| [reference/code-smells.md](reference/code-smells.md) | Phase 1: detecting code smells (22 smells, thresholds) |
| [reference/complexity-scoring.md](reference/complexity-scoring.md) | Phase 1: computing cognitive complexity scores |
| [reference/architecture-analysis.md](reference/architecture-analysis.md) | Phase 1: circular deps, boundaries, fan-in/fan-out |
| [reference/health-scoring.md](reference/health-scoring.md) | Phase 2: computing composite priority scores |

### Techniques & Mapping
| File | When to read |
|------|-------------|
| [reference/refactoring-techniques.md](reference/refactoring-techniques.md) | Phase 3: selecting technique for each detected smell (66 techniques) |
| [reference/smell-to-technique-map.md](reference/smell-to-technique-map.md) | Quick lookup: smell → fix technique + skill |

### Safety & Execution
| File | When to read |
|------|-------------|
| [reference/safety-protocols.md](reference/safety-protocols.md) | Phase 4: LLM safety, RefactoringMirror, re-prompting |
| [reference/prompt-engineering.md](reference/prompt-engineering.md) | Phase 4: crafting subagent prompts (specificity ladder) |

### Language & Examples
| File | When to read |
|------|-------------|
| [reference/language-notes.md](reference/language-notes.md) | Phase 3-4: TS/JS, Python, Go, Java specifics |
| [reference/worked-example.md](reference/worked-example.md) | First time: full pipeline walkthrough (UserService) |
