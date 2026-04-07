# Refactor Workflow — Detailed Phases

> Read this file when executing the full /refactor pipeline. Covers Phases 1-5 with subagent prompts, isolation rules, and merge protocol.

## Phase Overview

```
Phase 1  SCAN ──────────── on main (read-only, parallel tool pipeline)
Phase 2  PRIORITIZE ────── on main (read-only, weighted scoring)
Phase 3  PLAN & PARTITION ─ on main (read-only, graph-based critical pair analysis)
Phase 4  EXECUTE ────────── in worktrees (parallel subagents, up to 3 passes)
Phase 5  MERGE & VERIFY ── back on main (sequential merge, final scan)
```

## Phase 1: Scan & Detect

Runs on main. Read-only — no changes.

1. Launch the [Automated Scan Pipeline](scan-pipeline.md) — run all analysis tools in parallel
2. Compute [Cognitive Complexity Score](complexity-scoring.md) for every function in scope
3. Run [Architecture Analysis](architecture-analysis.md) on the module graph
4. Apply `detect-code-smells` — check for all smells in the [Code Smell Catalog](code-smells.md)
5. Apply `anti-patterns-catalog` — check for structural design problems
6. Apply `performance-anti-patterns` — check for performance issues
7. Apply `security-patterns-code-review` — quick security scan

Report findings as a table:

```
| # | File(s) | Smell/Issue | Severity | Category | Complexity | Fan-in/out |
```

If no issues found, say so and stop. **The File(s) column is critical** — it drives the dependency analysis in Phase 3.

## Phase 2: Prioritize

Runs on main. Read-only.

### Weighted Prioritization Scoring

Compute a priority score for each detected issue using weighted factors:

| Factor | Weight | Source | Scoring |
|--------|--------|--------|---------|
| Cumulative maintenance cost | **High (3x)** | Smell severity + complexity score | Critical=4, High=3, Moderate=2, Low=1 |
| Incident severity history | **High (3x)** | Git blame + issue tracker | Count of bugs in affected files |
| Modification frequency | **Medium (2x)** | `git log --oneline <file> \| wc -l` | Normalize to 0-4 scale |
| Dependency centrality | **Medium (2x)** | Fan-in + fan-out from architecture analysis | High centrality = higher priority |
| Cyclical dependency involvement | **Critical (5x)** | Circular dependency detection | Boolean: in cycle = must-fix |

**Priority score** = sum of (factor score * weight). Sort descending.

For composite health scoring, see [health-scoring.md](health-scoring.md).

Use `refactoring-decision-matrix` to:
1. Map each detected smell to its primary fix technique from the [Refactoring Technique Catalog](refactoring-techniques.md)
2. Assess difficulty (Easy/Medium/Hard) and risk (Low/Medium/High)
3. Check "When NOT to Refactor" criteria
4. Present prioritized list to user:

```
| # | Issue | File(s) | Fix Technique | Skill | Difficulty | Risk | Priority Score |
```

**If auto mode:** Select all issues automatically and proceed to Phase 3.
**If interactive:** Ask user: **"Which issues should I fix? (all / numbers / skip)"**

## Phase 3: Plan & Partition

Runs on main. Read-only. This phase determines whether execution is sequential or parallel.

### Step 1: Plan each fix

For each selected issue:
1. Look up the fix technique in [smell-to-technique-map.md](smell-to-technique-map.md)
2. Read the detailed technique from the relevant skill:
   - Method-level → `refactor-composing-methods`
   - Class-level → `refactor-moving-features`
   - Data-level → `refactor-organizing-data`
   - Conditional → `refactor-simplifying-conditionals`
   - Interface → `refactor-simplifying-method-calls`
   - Hierarchy → `refactor-generalization`
   - FP patterns → `refactor-functional-patterns`
3. Check [language-notes.md](language-notes.md) for the target language
4. Check if a design pattern would help → `design-patterns-creational-structural`, `design-patterns-behavioral`
5. Record the **complete file set** each fix will touch (source files + test files)

### Step 2: Graph-based critical pair analysis & partitioning (STRICT ISOLATION)

Build a conflict graph of all planned fixes using graph transformation theory. **Each worktree MUST be fully isolated — zero file overlap between groups.**

#### Conflict Graph Construction

Model each planned fix as a node. Add an edge between two fixes (a "critical pair") when they conflict:

1. For each fix, compute the **write set** (files it modifies) and **read set** (files it reads/imports)
2. Add an edge between fix A and fix B if ANY of these hold:
   - `write(A) ∩ write(B) ≠ ∅` — both write the same file
   - `write(A) ∩ read(B) ≠ ∅` — A writes what B reads
   - `read(A) ∩ write(B) ≠ ∅` — B writes what A reads
   - Transitive: if fix A writes `auth.ts` and fix B writes `middleware.ts` which imports from `auth.ts`, they are **dependent**
3. Expand shared state: config files, constants, types, schemas create implicit edges

#### Partition via Connected Components

1. Compute connected components of the conflict graph
2. Each connected component = one **refactoring group** (must execute sequentially within the group)
3. Disconnected components = **independent groups** (safe to execute in parallel)

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
| Group | Slug | Issues | Write Set | Read Set | Isolated? | Critical Pairs |
|-------|------|--------|-----------|----------|-----------|----------------|
| A | extract-auth | #1, #3 | src/auth.ts, src/middleware.ts | src/types.ts | ✓ | #1↔#3 |
| B | simplify-payment | #2 | src/payment.ts | src/types.ts | ✓ | — |
| C | cleanup-utils | #4, #5 | src/utils.ts, src/helpers.ts | — | ✓ | #4↔#5 |
```

**Rules:**
- If ALL fixes are in 1 group → sequential mode (single worktree)
- If 2+ independent groups exist → parallel mode (multiple worktrees)
- Read-only shared files (e.g., `types.ts` read by both A and B but written by neither) do NOT create a dependency — reads are safe to share
- Shared test files that both groups run but neither modifies are safe to share
- **If ANY doubt about file overlap exists, merge the groups** — sequential is always safe, parallel with overlap is always broken
- Config files (`package.json`, `tsconfig.json`, `.env`) are implicitly shared — if a fix modifies a config file, it MUST be in its own sequential group or all groups must be merged

### Step 3: Present the partition plan

Show the partition table and execution mode.

**If auto mode:** Proceed immediately to Phase 4.
**If interactive:** Ask user: **"Proceed with parallel execution? (yes / sequential / modify)"**

## Phase 4: Execute

Before executing, read [safety-protocols.md](safety-protocols.md) and [prompt-engineering.md](prompt-engineering.md).

### Pre-flight

1. Ensure main is clean: `git status --porcelain` must be empty
2. Record the current HEAD: `git rev-parse HEAD` (for rollback if needed)

### Sequential Mode (1 group)

1. Create one worktree:
   ```bash
   git worktree add ../refactor-wt-<slug> -b refactor/<slug>
   ```
2. Execute all fixes in the worktree directory, step by step
3. Follow immutability principles — never mutate, always create new
4. Run tests after each fix
5. Commit each fix as a separate commit in the branch
6. **If auto mode:** Work through all fixes without pausing. Only stop on test failure.

### Parallel Mode (N groups)

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

### Cycle-Based Execution (Up to 3 Passes)

Refactoring can create emergent work — extracting a method may reveal a new smell in the extracted code. Support up to 3 execution passes per group:

| Pass | Purpose | Trigger |
|------|---------|---------|
| Pass 1 | Execute all planned fixes | Always |
| Pass 2 | Fix emergent smells from Pass 1 | Re-scan detects new issues in changed files |
| Pass 3 | Final cleanup pass | Re-scan still detects issues (rare) |

After each pass, re-scan ONLY the files modified in that pass. If no new issues are detected, stop early. Never exceed 3 passes — remaining issues carry to next `/refactor` invocation.

### Subagent Execution Prompt Template

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
6. Re-scan changed files for emergent smells (up to 3 passes total)
7. Before reporting, run `git diff --name-only main` and verify EVERY changed file is in your ALLOWED write set
8. Report back: which issues were fixed, test results, files changed, any blockers

## HARD CONSTRAINTS (violations = immediate abort)
- MUST NOT modify files outside your ALLOWED write set
- MUST NOT modify files in the FORBIDDEN list
- MUST NOT modify shared config files (package.json, tsconfig.json, .env, etc.)
- MUST NOT merge into main — the orchestrator handles merging
- MUST NOT delete the worktree — the orchestrator handles cleanup
- MUST NOT create new files outside your assigned directories
- MUST verify isolation before final commit: `git diff --name-only main` must only show ALLOWED files
```

### Post-execution isolation audit

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

### Waiting for subagents

The orchestrator waits for all subagents to complete. As each finishes, record:

```
| Group | Slug | Status | Tests | Commits | Passes | Notes |
|-------|------|--------|-------|---------|--------|-------|
| A | extract-auth | PASS | 42/42 | 2 | 1 | — |
| B | simplify-payment | PASS | 38/38 | 1 | 2 | Pass 2 fixed emergent smell |
| C | cleanup-utils | FAIL | 35/37 | 1 | 1 | test_helpers.py failed |
```

## Phase 5: Merge & Verify

Back on main. The orchestrator handles all merging.

### Step 1: Merge passing branches

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

### Step 2: Handle failed branches

For branches that FAILED:
1. Do NOT merge into main
2. Report the failure: which tests failed, what the subagent attempted
3. **If auto mode:** Log the failure, continue with passing branches
4. **If interactive:** Ask user: **"Group C failed. Retry / skip / investigate?"**

### Step 3: Run final verification on main

After all merges:
1. Run the full test suite on main
2. Re-scan merged code for new smells introduced by the combination of changes
3. Apply `review-solid-clean-code` — verify SOLID compliance
4. Apply `review-code-quality-process` — quick quality check

If final tests fail:
1. Identify which merge introduced the failure
2. `git revert -m 1 <merge-commit>` for the offending merge
3. Report the revert to the user

### Step 4: Push and cleanup

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

### Step 5: Report

Final summary table:

```
| Group | Issues Fixed | Files Changed | Tests | Passes | Status |
|-------|-------------|---------------|-------|--------|--------|
| A | #1, #3 | 2 | PASS | 1 | Merged |
| B | #2 | 1 | PASS | 2 | Merged |
| C | #4, #5 | 2 | FAIL | 1 | Skipped |
```

## Validation Metrics

After every refactoring, compute before/after metrics to verify improvement:

### Required Before/After Comparison

| Metric | Must Not Increase | Must Not Decrease |
|--------|------------------|-------------------|
| Cyclomatic complexity | Yes | — |
| Cognitive complexity | Yes | — |
| Coupling between objects | Yes | — |
| Duplicate code percentage | Yes | — |
| Test count | — | Yes |
| Test coverage | — | Yes |
| Architecture boundary violations | Yes | — |
| Circular dependency count | Yes | — |

If ANY "must not increase" metric increased, the refactoring is REJECTED.
If ANY "must not decrease" metric decreased, the refactoring requires justification.

### Architecture Fitness Functions

Run as part of validation to catch architectural regressions:

| Rule | Validation Command |
|------|-------------------|
| No circular dependencies | `npx depcruise --output-type err-long --validate src/` |
| Layer boundaries respected | `npx depcruise --validate .dependency-cruiser.js src/` |
| No orphan modules | `npx depcruise --output-type err --include-only "^src" src/` |
| Complexity under threshold | Custom: compute cognitive complexity, fail if any function >25 |
