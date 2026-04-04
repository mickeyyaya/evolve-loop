---
name: refactor
description: Use when the user asks to refactor code, review code quality, or fix code smells — orchestrates the full refactoring pipeline from detection through fix, with parallel worktree isolation per independent refactoring group
---

# /refactor — Full Refactoring Pipeline

## Overview

Single entry point that orchestrates the complete refactoring workflow: detect smells → prioritize → partition into independent groups → execute in parallel worktrees via subagents → merge & verify.

## Auto Mode Detection

Before starting, check if the user is in **bypass/yolo mode** (auto-accept permissions enabled). Detection signals:
- User explicitly said "yolo mode", "bypass permissions", or "auto-accept"
- The session is running with `--dangerously-skip-permissions` or equivalent
- Tools are being auto-approved without user prompts

**When auto mode is detected:** Skip all confirmation prompts. Automatically select all issues, partition, launch parallel subagents, and merge passing branches without pausing. This enables fully autonomous refactoring.

**When auto mode is NOT detected:** Pause for user confirmation at each checkpoint as described below.

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

## Workflow

```
Phase 1  SCAN ──────────── on main (read-only)
Phase 2  PRIORITIZE ────── on main (read-only)
Phase 3  PLAN & PARTITION ─ on main (read-only, dependency analysis)
Phase 4  EXECUTE ────────── in worktrees (parallel subagents)
Phase 5  MERGE & VERIFY ── back on main (sequential merge, final scan)
```

### Phase 1: Scan & Detect

Runs on main. Read-only — no changes.

1. Read the target file(s) the user specified
2. Apply `detect-code-smells` — check for all 23 smells across 5 categories
3. Apply `anti-patterns-catalog` — check for structural design problems
4. Apply `performance-anti-patterns` — check for performance issues
5. Apply `security-patterns-code-review` — quick security scan

Report findings as a table:

```
| # | File(s) | Smell/Issue | Severity | Category |
```

If no issues found, say so and stop. **The File(s) column is critical** — it drives the dependency analysis in Phase 3.

### Phase 2: Prioritize

Runs on main. Read-only.

Use `refactoring-decision-matrix` to:
1. Map each detected smell to its primary fix technique
2. Assess difficulty (Easy/Medium/Hard) and risk (Low/Medium/High)
3. Check "When NOT to Refactor" criteria
4. Present prioritized list to user:

```
| # | Issue | File(s) | Fix Technique | Skill | Difficulty | Risk |
```

**If auto mode:** Select all issues automatically and proceed to Phase 3.
**If interactive:** Ask user: **"Which issues should I fix? (all / numbers / skip)"**

### Phase 3: Plan & Partition

Runs on main. Read-only. This phase determines whether execution is sequential or parallel.

#### Step 1: Plan each fix

For each selected issue:
1. Identify the specific refactoring technique from the relevant skill:
   - Method-level → `refactor-composing-methods`
   - Class-level → `refactor-moving-features`
   - Data-level → `refactor-organizing-data`
   - Conditional → `refactor-simplifying-conditionals`
   - Interface → `refactor-simplifying-method-calls`
   - Hierarchy → `refactor-generalization`
   - FP patterns → `refactor-functional-patterns`
2. Check if a design pattern would help → `design-patterns-creational-structural`, `design-patterns-behavioral`
3. Check language idioms → `language-specific-idioms`
4. Check type improvements → `type-system-patterns`
5. Record the **complete file set** each fix will touch (source files + test files)

#### Step 2: Dependency analysis & partitioning (STRICT ISOLATION)

Build a dependency graph of all planned fixes. **Each worktree MUST be fully isolated — zero file overlap between groups.**

1. For each fix, list every file it will **read, write, or import from** (source files, test files, shared config, shared types)
2. Expand transitive dependencies: if fix A writes `auth.ts` and fix B writes `middleware.ts` which imports from `auth.ts`, they are **dependent** — group them together
3. Two fixes are **independent** ONLY if:
   - Their write sets are completely disjoint
   - Neither reads a file the other writes
   - They share no import/dependency chain
   - They share no shared state files (config, constants, types, schemas)
4. Group dependent fixes together into a **refactoring group**
5. Each group gets a descriptive slug based on its primary fix

**Isolation verification matrix** — build this table and verify every cell is empty before approving parallel mode:

```
|            | Group A files | Group B files | Group C files |
|------------|---------------|---------------|---------------|
| Group A    | —             | (must be ∅)   | (must be ∅)   |
| Group B    | (must be ∅)   | —             | (must be ∅)   |
| Group C    | (must be ∅)   | (must be ∅)   | —             |
```

If ANY cell is non-empty (files appear in multiple groups), merge those groups into one.

**Partition result table:**

```
| Group | Slug | Issues | Write Set | Read Set | Isolated? |
|-------|------|--------|-----------|----------|-----------|
| A | extract-auth | #1, #3 | src/auth.ts, src/middleware.ts | src/types.ts | ✓ |
| B | simplify-payment | #2 | src/payment.ts | src/types.ts | ✓ |
| C | cleanup-utils | #4, #5 | src/utils.ts, src/helpers.ts | — | ✓ |
```

**Rules:**
- If ALL fixes are in 1 group → sequential mode (single worktree)
- If 2+ independent groups exist → parallel mode (multiple worktrees)
- Read-only shared files (e.g., `types.ts` read by both A and B but written by neither) do NOT create a dependency — reads are safe to share
- Shared test files that both groups run but neither modifies are safe to share
- **If ANY doubt about file overlap exists, merge the groups** — sequential is always safe, parallel with overlap is always broken
- Config files (`package.json`, `tsconfig.json`, `.env`) are implicitly shared — if a fix modifies a config file, it MUST be in its own sequential group or all groups must be merged

#### Step 3: Present the partition plan

Show the partition table and execution mode.

**If auto mode:** Proceed immediately to Phase 4.
**If interactive:** Ask user: **"Proceed with parallel execution? (yes / sequential / modify)"**

### Phase 4: Execute

#### Pre-flight

1. Ensure main is clean: `git status --porcelain` must be empty
2. Record the current HEAD: `git rev-parse HEAD` (for rollback if needed)

#### Sequential Mode (1 group)

1. Create one worktree:
   ```bash
   git worktree add ../refactor-wt-<slug> -b refactor/<slug>
   ```
2. Execute all fixes in the worktree directory, step by step
3. Follow immutability principles — never mutate, always create new
4. Run tests after each fix
5. Commit each fix as a separate commit in the branch
6. **If auto mode:** Work through all fixes without pausing. Only stop on test failure.

#### Parallel Mode (N groups)

Launch one **subagent per group**, each in its own worktree:

```
For each group G in partition:
  Agent(
    subagent_type: "general-purpose",
    isolation: "worktree",              # if supported
    prompt: <group execution prompt>,
    run_in_background: true
  )
```

If the Agent tool's `isolation: "worktree"` is not available, the orchestrator creates worktrees manually before launching subagents:

```bash
# Create all worktrees upfront
git worktree add ../refactor-wt-<slug-A> -b refactor/<slug-A>
git worktree add ../refactor-wt-<slug-B> -b refactor/<slug-B>
git worktree add ../refactor-wt-<slug-C> -b refactor/<slug-C>
```

Then launch subagents in parallel (all in a single message for true parallelism):

```
Agent(prompt: "...", description: "Refactor: <slug-A>", run_in_background: true)
Agent(prompt: "...", description: "Refactor: <slug-B>", run_in_background: true)
Agent(prompt: "...", description: "Refactor: <slug-C>", run_in_background: true)
```

#### Subagent Execution Prompt Template

Each subagent receives a prompt that enforces strict isolation:

```markdown
You are a refactoring subagent. You operate in COMPLETE ISOLATION.

## Your Worktree
Directory: <worktree-path>
Branch: refactor/<slug>

## Your Assignment
Issues: <list of issue numbers and descriptions>
Techniques: <technique per issue with skill reference>

## ALLOWED files (exclusive to this worktree — no other subagent touches these)
Write set: <explicit file list — you may modify ONLY these files>
Read set: <explicit file list — you may read these but MUST NOT modify them>

## FORBIDDEN files (other subagents own these — DO NOT TOUCH)
<explicit list of all files assigned to other groups>

## Instructions
1. cd to the worktree directory: `cd <worktree-path>`
2. Before ANY edit, verify the file is in your ALLOWED write set
3. For each assigned issue:
   a. Apply the refactoring technique step by step
   b. Follow immutability principles — never mutate, always create new
   c. Keep changes minimal — only fix what was assigned
   d. Commit each fix as a separate commit with a descriptive message
4. Run the test suite: <test command>
5. If tests fail, fix the issue — but only in your ALLOWED write set
6. Before reporting, run `git diff --name-only main` and verify EVERY changed file is in your ALLOWED write set
7. Report back: which issues were fixed, test results, files changed, any blockers

## HARD CONSTRAINTS (violations = immediate abort)
- MUST NOT modify files outside your ALLOWED write set
- MUST NOT modify files in the FORBIDDEN list
- MUST NOT modify shared config files (package.json, tsconfig.json, .env, etc.)
- MUST NOT merge into main — the orchestrator handles merging
- MUST NOT delete the worktree — the orchestrator handles cleanup
- MUST NOT create new files outside your assigned directories
- MUST verify isolation before final commit: `git diff --name-only main` must only show ALLOWED files
```

#### Post-execution isolation audit

After each subagent completes, the orchestrator MUST verify isolation before merging:

```bash
# For each completed worktree, check that only allowed files were changed
cd <worktree-path>
git diff --name-only main > /tmp/changed-files-<slug>.txt
# Compare against the group's allowed write set
# If ANY file outside the write set was modified → REJECT the branch, do NOT merge
```

If a subagent violated isolation:
1. Do NOT merge the branch
2. Report the violation (which files were touched that shouldn't have been)
3. **If auto mode:** Skip and warn
4. **If interactive:** Ask user how to proceed

#### Waiting for subagents

The orchestrator waits for all subagents to complete. As each finishes, record:

```
| Group | Slug | Status | Tests | Commits | Notes |
|-------|------|--------|-------|---------|-------|
| A | extract-auth | PASS | 42/42 | 2 | — |
| B | simplify-payment | PASS | 38/38 | 1 | — |
| C | cleanup-utils | FAIL | 35/37 | 1 | test_helpers.py failed |
```

### Phase 5: Merge & Verify

Back on main. The orchestrator handles all merging.

#### Step 1: Merge passing branches

Merge each PASS branch into main sequentially (order by group priority):

```bash
cd <main-directory>
git merge refactor/<slug-A> --no-ff -m "refactor: <description of group A fixes>"
git merge refactor/<slug-B> --no-ff -m "refactor: <description of group B fixes>"
```

**If a merge conflict occurs:**
1. Attempt auto-resolution for trivial conflicts (e.g., adjacent line changes)
2. If non-trivial conflict → pause, show the conflict to the user, ask for guidance
3. In auto mode → attempt resolution, if impossible → skip this branch and warn

#### Step 2: Handle failed branches

For branches that FAILED:
1. Do NOT merge into main
2. Report the failure: which tests failed, what the subagent attempted
3. **If auto mode:** Log the failure, continue with passing branches
4. **If interactive:** Ask user: **"Group C failed. Retry / skip / investigate?"**

#### Step 3: Run final verification on main

After all merges:
1. Run the full test suite on main
2. Re-scan merged code for new smells introduced by the combination of changes
3. Apply `review-solid-clean-code` — verify SOLID compliance
4. Apply `review-code-quality-process` — quick quality check

If final tests fail:
1. Identify which merge introduced the failure
2. `git revert -m 1 <merge-commit>` for the offending merge
3. Report the revert to the user

#### Step 4: Push and cleanup

```bash
# Push main
git push

# Remove all worktrees and branches
git worktree remove ../refactor-wt-<slug-A>
git worktree remove ../refactor-wt-<slug-B>
git worktree remove ../refactor-wt-<slug-C>
git branch -d refactor/<slug-A>
git branch -d refactor/<slug-B>
git branch -d refactor/<slug-C>
```

#### Step 5: Report

Final summary table:

```
| Group | Issues Fixed | Files Changed | Tests | Status |
|-------|-------------|---------------|-------|--------|
| A | #1, #3 | 2 | PASS | Merged |
| B | #2 | 1 | PASS | Merged |
| C | #4, #5 | 2 | FAIL | Skipped |
```

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
