---
name: refactor
description: Use when the user asks to refactor code, review code quality, or fix code smells — orchestrates the full refactoring pipeline from detection through fix, with parallel worktree isolation per independent refactoring group
---

# /refactor — Full Refactoring Pipeline

> Orchestrate the complete refactoring workflow: detect smells, prioritize by weighted scoring, partition into independent groups, execute in parallel worktrees via subagents, merge and verify. Enhanced with cognitive complexity scoring, architecture analysis, and speed-optimized parallel scan pipelines.

## Table of Contents

| Section | Description |
|---------|-------------|
| [Auto Mode Detection](#auto-mode-detection) | Detect bypass/yolo mode for autonomous execution |
| [Git Isolation](#git-isolation-mandatory) | Worktree-based isolation for all refactoring work |
| [Workflow](#workflow) | 5-phase pipeline: Scan, Prioritize, Plan, Execute, Merge |
| [Automated Scan Pipeline](#automated-scan-pipeline) | Parallel tool execution for Phase 1 speed |
| [Cognitive Complexity Scoring](#cognitive-complexity-scoring) | SonarQube-derived complexity algorithm |
| [Architecture Analysis](#architecture-analysis) | Module graph analysis, circular deps, fan-in/fan-out |
| [Code Smell Detection Catalog](#code-smell-detection-catalog) | Complete 22-smell catalog with thresholds |
| [Multi-Metric Smell Scoring](#multi-metric-smell-scoring) | Composite health scores and git-history enrichment |
| [Refactoring Technique Catalog](#refactoring-technique-catalog) | Complete 66-technique catalog by category |
| [Speed Optimizations](#speed-optimizations) | Incremental analysis, caching, cycle-based execution |
| [LLM Safety Protocols](#llm-safety-protocols) | RefactoringMirror pattern, re-prompting, multi-proposal |
| [Prompt Engineering for Refactoring](#prompt-engineering-for-refactoring) | Prompt specificity ladder and subagent templates |
| [Validation Metrics](#validation-metrics) | Before/after metric comparison and fitness functions |
| [Quick Modes](#quick-modes) | Scoped invocations for targeted refactoring |
| [Smell-to-Technique Quick Reference](#smell-to-technique-quick-reference) | Fast lookup: smell → fix technique |
| [Language-Specific Refactoring Notes](#language-specific-refactoring-notes) | TS/JS, Python, Go, Java guidance |
| [Cross-Reference Map](#cross-reference-map) | Skill routing by issue type |

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
Phase 1  SCAN ──────────── on main (read-only, parallel tool pipeline)
Phase 2  PRIORITIZE ────── on main (read-only, weighted scoring)
Phase 3  PLAN & PARTITION ─ on main (read-only, graph-based critical pair analysis)
Phase 4  EXECUTE ────────── in worktrees (parallel subagents, up to 3 passes)
Phase 5  MERGE & VERIFY ── back on main (sequential merge, final scan)
```

### Phase 1: Scan & Detect

Runs on main. Read-only — no changes.

1. Launch the [Automated Scan Pipeline](#automated-scan-pipeline) — run all analysis tools in parallel
2. Compute [Cognitive Complexity Score](#cognitive-complexity-scoring) for every function in scope
3. Run [Architecture Analysis](#architecture-analysis) on the module graph
4. Apply `detect-code-smells` — check for all smells in the [Code Smell Detection Catalog](#code-smell-detection-catalog)
5. Apply `anti-patterns-catalog` — check for structural design problems
6. Apply `performance-anti-patterns` — check for performance issues
7. Apply `security-patterns-code-review` — quick security scan

Report findings as a table:

```
| # | File(s) | Smell/Issue | Severity | Category | Complexity | Fan-in/out |
```

If no issues found, say so and stop. **The File(s) column is critical** — it drives the dependency analysis in Phase 3.

### Phase 2: Prioritize

Runs on main. Read-only.

#### Weighted Prioritization Scoring

Compute a priority score for each detected issue using weighted factors:

| Factor | Weight | Source | Scoring |
|--------|--------|--------|---------|
| Cumulative maintenance cost | **High (3x)** | Smell severity + complexity score | Critical=4, High=3, Moderate=2, Low=1 |
| Incident severity history | **High (3x)** | Git blame + issue tracker | Count of bugs in affected files |
| Modification frequency | **Medium (2x)** | `git log --oneline <file> \| wc -l` | Normalize to 0-4 scale |
| Dependency centrality | **Medium (2x)** | Fan-in + fan-out from architecture analysis | High centrality = higher priority |
| Cyclical dependency involvement | **Critical (5x)** | Circular dependency detection | Boolean: in cycle = must-fix |

**Priority score** = sum of (factor score * weight). Sort descending.

Use `refactoring-decision-matrix` to:
1. Map each detected smell to its primary fix technique from the [Refactoring Technique Catalog](#refactoring-technique-catalog)
2. Assess difficulty (Easy/Medium/Hard) and risk (Low/Medium/High)
3. Check "When NOT to Refactor" criteria
4. Present prioritized list to user:

```
| # | Issue | File(s) | Fix Technique | Skill | Difficulty | Risk | Priority Score |
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

#### Step 2: Graph-based critical pair analysis & partitioning (STRICT ISOLATION)

Build a conflict graph of all planned fixes using graph transformation theory. **Each worktree MUST be fully isolated — zero file overlap between groups.**

##### Conflict Graph Construction

Model each planned fix as a node. Add an edge between two fixes (a "critical pair") when they conflict:

1. For each fix, compute the **write set** (files it modifies) and **read set** (files it reads/imports)
2. Add an edge between fix A and fix B if ANY of these hold:
   - `write(A) ∩ write(B) ≠ ∅` — both write the same file
   - `write(A) ∩ read(B) ≠ ∅` — A writes what B reads
   - `read(A) ∩ write(B) ≠ ∅` — B writes what A reads
   - Transitive: if fix A writes `auth.ts` and fix B writes `middleware.ts` which imports from `auth.ts`, they are **dependent**
3. Expand shared state: config files, constants, types, schemas create implicit edges

##### Partition via Connected Components

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

#### Cycle-Based Execution (Up to 3 Passes)

Refactoring can create emergent work — extracting a method may reveal a new smell in the extracted code. Support up to 3 execution passes per group:

| Pass | Purpose | Trigger |
|------|---------|---------|
| Pass 1 | Execute all planned fixes | Always |
| Pass 2 | Fix emergent smells from Pass 1 | Re-scan detects new issues in changed files |
| Pass 3 | Final cleanup pass | Re-scan still detects issues (rare) |

After each pass, re-scan ONLY the files modified in that pass. If no new issues are detected, stop early. Never exceed 3 passes — remaining issues carry to next `/refactor` invocation.

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
| Group | Slug | Status | Tests | Commits | Passes | Notes |
|-------|------|--------|-------|---------|--------|-------|
| A | extract-auth | PASS | 42/42 | 2 | 1 | — |
| B | simplify-payment | PASS | 38/38 | 1 | 2 | Pass 2 fixed emergent smell |
| C | cleanup-utils | FAIL | 35/37 | 1 | 1 | test_helpers.py failed |
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
| Group | Issues Fixed | Files Changed | Tests | Passes | Status |
|-------|-------------|---------------|-------|--------|--------|
| A | #1, #3 | 2 | PASS | 1 | Merged |
| B | #2 | 1 | PASS | 2 | Merged |
| C | #4, #5 | 2 | FAIL | 1 | Skipped |
```

---

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

---

## Automated Scan Pipeline

Run all static analysis tools in parallel during Phase 1 to minimize wall-clock time. Launch these simultaneously:

| Tool | Purpose | Output |
|------|---------|--------|
| jscpd | Duplicate code detection (>25 token threshold) | Duplicate block locations |
| knip | Dead code and unused export detection | Unused files, exports, dependencies |
| dependency-cruiser | Module graph and architecture boundary validation | Circular deps, violations |
| Complexity analyzer | Cognitive complexity per function | Per-function scores |
| ESLint/Biome/Ruff | Language-specific lint rules | Lint violations |

### Pipeline Execution

```bash
# Launch all tools in parallel
jscpd --min-tokens 25 --reporters json src/ &
npx knip --reporter json &
npx depcruise --output-type json src/ &
# Complexity analysis runs inline (see Cognitive Complexity Scoring)
wait
```

### Incremental Analysis

When re-running on a previously scanned codebase, only analyze changed files:

1. Compute changed files: `git diff --name-only <last-scan-commit>..HEAD`
2. Run tools only on changed files
3. Merge with cached results for unchanged files
4. Invalidate cache entries for files whose dependencies changed

### Static Analysis Pre-filtering

Run IDE-level static checks before any LLM-based analysis to eliminate hallucination waste:

1. Type checker (tsc, mypy, pyright) catches type errors the LLM might miss
2. Linter catches formatting and convention issues
3. Only pass genuine code smells and architectural issues to LLM analysis
4. This eliminates approximately 6-8% of false positives from LLM-only analysis

---

## Cognitive Complexity Scoring

Compute cognitive complexity for every function in scope using the SonarQube-derived algorithm. This measures how hard a function is to *understand*, not just how long it is.

### Algorithm

| Increment | Condition | Nesting penalty |
|-----------|-----------|-----------------|
| +1 | `if`, `else if`, `else` | +1 per nesting level |
| +1 | `switch` | +1 per nesting level |
| +1 | `for`, `while`, `do-while`, `for...of`, `for...in` | +1 per nesting level |
| +1 | `catch` | +1 per nesting level |
| +1 | Ternary operator `? :` | +1 per nesting level |
| +1 | Mixed logical operator sequence (`a && b \|\| c`) | No nesting penalty |
| +1 | Recursion (function calls itself) | No nesting penalty |

### What Does NOT Count

| Construct | Reason |
|-----------|--------|
| Null-coalescing (`??`, `?.`) | Simplifies code, not complexity |
| Early returns / guard clauses | Reduce nesting, improve readability |
| Lambda/arrow function definitions | Definition is not control flow |
| Simple `try` blocks (without logic) | Structural, not cognitive |
| `break`, `continue` | Flow interruption already counted at loop level |

### Thresholds

| Score | Rating | Action |
|-------|--------|--------|
| 0-10 | Good | No action required |
| 11-15 | Moderate | Consider refactoring if in hot path |
| 16-25 | High | Refactor — extract methods, simplify conditionals |
| 26+ | Critical | Must refactor — function is unmaintainable |

### Scoring Example

```javascript
function processOrder(order, user) {        // function declaration: 0
  if (order.items.length === 0) {            // +1 (if)
    return null;                             // early return: 0
  }
  for (const item of order.items) {          // +1 (for)
    if (item.quantity > 0) {                 // +2 (if + 1 nesting)
      if (item.price > 100                   // +3 (if + 2 nesting)
          && user.isPremium                  // +0 (same operator)
          || item.isOnSale) {               // +1 (mixed operators)
        applyDiscount(item);
      }
    }
  }
}                                            // Total: 8 (Good)
```

---

## Architecture Analysis

Run during Phase 1 to detect structural problems in the module dependency graph.

### Circular Dependency Detection

Use depth-first search on the module import graph:

1. Build directed graph: each file is a node, each import is an edge
2. Run DFS, track visited and in-stack nodes
3. When a back-edge is found (visiting a node already in the stack), record the cycle
4. Report all cycles with the full path

| Cycle length | Severity | Action |
|--------------|----------|--------|
| 2 (A↔B) | Critical | Break immediately — extract shared interface |
| 3-4 | High | Refactor — introduce mediator or event bus |
| 5+ | High | Architectural redesign needed |

### Architecture Boundary Validation

Define allowed dependency directions using configurable path rules:

```
| Source pattern | May depend on | Must NOT depend on |
|----------------|--------------|-------------------|
| src/ui/** | src/services/**, src/types/** | src/db/**, src/infra/** |
| src/services/** | src/db/**, src/types/** | src/ui/** |
| src/db/** | src/types/** | src/ui/**, src/services/** |
```

Flag any import that violates the boundary rules as an architecture smell.

### Fan-in / Fan-out Centrality Analysis

Compute centrality metrics for each module to identify high-risk refactoring targets:

| Metric | Definition | Significance |
|--------|-----------|--------------|
| Fan-in | Number of modules that import this module | High fan-in = many dependents, high-risk changes |
| Fan-out | Number of modules this module imports | High fan-out = high coupling, smell indicator |
| Instability | Fan-out / (Fan-in + Fan-out) | 0 = stable (many dependents), 1 = unstable (many dependencies) |

**Prioritization rule:** Modules with high fan-in (>10) should be refactored with extreme care. Modules with high fan-out (>10) are prime candidates for decomposition.

### Orphan Module Detection

Identify modules with zero fan-in (no other module imports them) that are not entry points:

1. Build the full import graph
2. Find all nodes with fan-in = 0
3. Exclude known entry points (main, index, test files, config)
4. Remaining nodes are orphan candidates — likely dead code

---

## Code Smell Detection Catalog

Complete catalog of 22 code smells organized by category. Use during Phase 1 scan.

### Bloaters

| # | Smell | Detection Signal | Threshold |
|---|-------|-----------------|-----------|
| 1 | Long Method | Line count or cognitive complexity | >20 lines OR complexity >15 |
| 2 | Large Class | Line count, field count, method count | >300 lines, >10 fields, OR >20 methods |
| 3 | Primitive Obsession | Repeated primitive params representing a concept | 3+ primitives that belong together |
| 4 | Long Parameter List | Parameter count per function | >3 parameters |
| 5 | Data Clumps | Same group of fields appearing together | Same 3+ fields in 2+ locations |

### Object-Orientation Abusers

| # | Smell | Detection Signal | Threshold |
|---|-------|-----------------|-----------|
| 6 | Switch Statements | Switch/if-else chains on type codes | >3 cases dispatching on same value |
| 7 | Temporary Field | Fields only set/used in certain paths | Field null/undefined in >50% of methods |
| 8 | Refused Bequest | Subclass ignores parent methods/fields | Overrides >50% of inherited interface |
| 9 | Alt Classes, Different Interfaces | Two classes doing the same thing differently | Similar method bodies, different signatures |

### Change Preventers

| # | Smell | Detection Signal | Threshold |
|---|-------|-----------------|-----------|
| 10 | Divergent Change | One class changed for many different reasons | >3 unrelated change reasons in git history |
| 11 | Shotgun Surgery | One change requires edits across many files | Change spans >5 files |
| 12 | Parallel Inheritance | Creating subclass in one hierarchy forces subclass in another | 1:1 subclass correspondence across hierarchies |

### Dispensables

| # | Smell | Detection Signal | Threshold |
|---|-------|-----------------|-----------|
| 13 | Duplicate Code | Token-level similarity between code blocks | >25 tokens duplicated |
| 14 | Dead Code | Unreachable or unused code | Zero references (fan-in = 0, not entry point) |
| 15 | Lazy Class | Class does too little to justify its existence | <3 methods AND <50 lines |
| 16 | Speculative Generality | Abstractions created for future use that never came | Abstract class/interface with single implementation |
| 17 | Data Class | Class with only fields and getters/setters, no behavior | 0 business-logic methods |
| 18 | Excessive Comments | Comments compensating for unclear code | Comment-to-code ratio >0.5 in a function |

### Couplers

| # | Smell | Detection Signal | Threshold |
|---|-------|-----------------|-----------|
| 19 | Feature Envy | Method uses another class's data more than its own | >50% of references are to external class |
| 20 | Inappropriate Intimacy | Two classes access each other's internals | Bidirectional private/protected access |
| 21 | Message Chains | Long chains of method calls (a.b().c().d()) | >3 chained calls |
| 22 | Middle Man | Class delegates most work to another class | >50% of methods are pure delegation |

---

## Multi-Metric Smell Scoring

Use composite scores instead of single thresholds for more accurate smell detection.

### Composite Health Score per Function

| Metric | Weight | Threshold | Score Formula |
|--------|--------|-----------|--------------|
| Cognitive complexity | 30% | >15 = smell | min(score/25, 1.0) |
| Cyclomatic complexity | 20% | >10 = smell | min(score/20, 1.0) |
| Lines of code | 15% | >50 = smell | min(lines/100, 1.0) |
| Parameter count | 10% | >3 = smell | min(params/6, 1.0) |
| Nesting depth | 15% | >4 = smell | min(depth/6, 1.0) |
| Fan-out (dependencies) | 10% | >10 = smell | min(fanout/15, 1.0) |

**Composite smell score** = weighted sum (0.0 = clean, 1.0 = severely smelly)

| Score | Rating | Action |
|-------|--------|--------|
| 0.0-0.3 | Clean | No action |
| 0.3-0.5 | Mild | Monitor, refactor if in hot path |
| 0.5-0.7 | Moderate | Refactor in next sprint |
| 0.7-1.0 | Severe | Refactor immediately |

### Git-History Enrichment

Enrich smell scores with git history signals:

| Signal | How to Compute | Impact on Priority |
|--------|---------------|-------------------|
| Change frequency | `git log --oneline <file> \| wc -l` | High churn + high smell = urgent |
| Bug correlation | Count fix commits touching this file | Bug-prone + smelly = critical |
| Co-change coupling | Files that always change together | Indicates hidden dependencies |
| Author count | Distinct authors in last 6 months | Many authors = communication cost |

---

## Refactoring Technique Catalog

Complete catalog of 66 refactoring techniques organized by category. Use during Phase 3 to select the right technique for each detected smell.

### Composing Methods (9 Techniques)

| # | Technique | When to Apply | Detection Signal |
|---|-----------|---------------|-----------------|
| 1 | Extract Method | Long method, code comment explaining a block | Function >20 lines or complexity >15 |
| 2 | Inline Method | Method body is as clear as its name | Single-line method called once |
| 3 | Extract Variable | Complex expression hard to understand | Expression with 3+ operators |
| 4 | Inline Temp | Temp variable used once, assigned simple expression | Single-use temp with trivial RHS |
| 5 | Replace Temp with Query | Temp holds a computed value reusable elsewhere | Temp assigned then used in multiple places |
| 6 | Split Temporary Variable | One temp assigned multiple times for different purposes | Variable reassigned with different semantics |
| 7 | Remove Assignments to Parameters | Function parameter is reassigned | Parameter on LHS of assignment |
| 8 | Replace Method with Method Object | Long method with many local variables preventing extraction | >5 local variables in a long method |
| 9 | Substitute Algorithm | Algorithm can be replaced with a clearer one | Complex loop replaceable by built-in or library call |

### Moving Features Between Objects (8 Techniques)

| # | Technique | When to Apply | Detection Signal |
|---|-----------|---------------|-----------------|
| 10 | Move Method | Method uses more features of another class | >50% external references (Feature Envy) |
| 11 | Move Field | Field used more by another class | >50% access from external class |
| 12 | Extract Class | One class doing the work of two | Class has 2+ distinct responsibility clusters |
| 13 | Inline Class | Class does too little | <3 methods, <50 lines (Lazy Class) |
| 14 | Hide Delegate | Client calls through an object to get to another | Message Chain >3 links |
| 15 | Remove Middle Man | Class has too many delegating methods | >50% delegation (Middle Man) |
| 16 | Introduce Foreign Method | Utility method needed on a class you cannot modify | Repeated helper code for external class |
| 17 | Introduce Local Extension | Multiple foreign methods needed for same class | 3+ foreign methods for one class |

### Organizing Data (13 Techniques)

| # | Technique | When to Apply | Detection Signal |
|---|-----------|---------------|-----------------|
| 18 | Self Encapsulate Field | Direct field access causes coupling issues | Subclass needs to override field access |
| 19 | Replace Data Value with Object | Primitive represents a concept with behavior | Primitive + related validation/formatting logic |
| 20 | Change Value to Reference | Many identical objects should be one shared instance | Equality checks on data that should be identity |
| 21 | Change Reference to Value | Reference object is simple and immutable | Small object with no side effects |
| 22 | Replace Array with Object | Array elements mean different things by position | Array with positional semantics (arr[0] = name) |
| 23 | Duplicate Observed Data | Domain data trapped in UI class | UI class holds business state |
| 24 | Change Unidirectional to Bidirectional | Two classes need to reference each other | Class A uses B, and B needs A |
| 25 | Change Bidirectional to Unidirectional | Bidirectional reference no longer needed | One direction is never traversed |
| 26 | Replace Magic Number with Constant | Hardcoded number with special meaning | Numeric literal in condition or calculation |
| 27 | Encapsulate Field | Public field with no access control | Public field on a class |
| 28 | Encapsulate Collection | Getter returns raw collection | Mutable collection returned by reference |
| 29 | Replace Type Code with Class | Type code (int/string) represents a category | String/int constants used in conditionals |
| 30 | Replace Type Code with Subclasses | Type code affects behavior | Switch/if on type code in multiple methods |

### Simplifying Conditional Expressions (8 Techniques)

| # | Technique | When to Apply | Detection Signal |
|---|-----------|---------------|-----------------|
| 31 | Decompose Conditional | Complex conditional expression | If-condition with 3+ clauses |
| 32 | Consolidate Conditional Expression | Multiple conditionals with same result | Adjacent if-blocks returning same value |
| 33 | Consolidate Duplicate Conditional Fragments | Same code in all branches | Identical statements in if and else |
| 34 | Remove Control Flag | Boolean flag controlling loop exit | Variable set to break out of loop |
| 35 | Replace Nested Conditional with Guard Clauses | Deep nesting from sequential checks | Nesting depth >3 from if-chains |
| 36 | Replace Conditional with Polymorphism | Switch on type determines behavior | Switch statement in >1 method on same type |
| 37 | Introduce Null Object | Repeated null checks for same object | >3 null checks for same variable |
| 38 | Introduce Assertion | Code assumes a condition but does not verify it | Implicit precondition without validation |

### Simplifying Method Calls (14 Techniques)

| # | Technique | When to Apply | Detection Signal |
|---|-----------|---------------|-----------------|
| 39 | Rename Method | Name does not reveal intent | Name is generic (process, handle, doStuff) |
| 40 | Add Parameter | Method needs additional data from caller | Caller computes value method should receive |
| 41 | Remove Parameter | Parameter is no longer used | Parameter unused in method body |
| 42 | Separate Query from Modifier | Method both returns value and changes state | Method with return value AND side effects |
| 43 | Parameterize Method | Multiple methods do similar things with different values | 2+ methods differing only in a constant |
| 44 | Replace Parameter with Explicit Methods | Method behavior depends entirely on parameter value | Boolean/enum param selecting code path |
| 45 | Preserve Whole Object | Extracting values from object to pass individually | 3+ fields extracted then passed |
| 46 | Replace Parameter with Method Call | Param value can be obtained by callee | Caller passes value callee can compute itself |
| 47 | Introduce Parameter Object | Group of parameters always passed together | Same 3+ params in multiple signatures |
| 48 | Remove Setting Method | Field should be set only at creation time | Setter called only in constructor |
| 49 | Hide Method | Method only used inside its own class | Zero external callers |
| 50 | Replace Constructor with Factory Method | Complex construction logic or multiple creation paths | Constructor with conditional logic |
| 51 | Replace Error Code with Exception | Method returns error codes | Return value checked for error sentinel |
| 52 | Replace Exception with Test | Exception used for control flow | Try/catch wrapping expected conditions |

### Dealing with Generalization (14 Techniques)

| # | Technique | When to Apply | Detection Signal |
|---|-----------|---------------|-----------------|
| 53 | Pull Up Field | Duplicate field in sibling subclasses | Same field in 2+ subclasses |
| 54 | Pull Up Method | Duplicate method in sibling subclasses | Same method body in 2+ subclasses |
| 55 | Pull Up Constructor Body | Duplicate constructor logic in subclasses | Identical constructor lines in subclasses |
| 56 | Push Down Method | Method only relevant to one subclass | Method used by only 1 of N subclasses |
| 57 | Push Down Field | Field only relevant to one subclass | Field accessed by only 1 of N subclasses |
| 58 | Extract Subclass | Class has features used only in some instances | Fields/methods only used for subset of objects |
| 59 | Extract Superclass | Two classes with similar features | 2 classes sharing 3+ similar methods/fields |
| 60 | Extract Interface | Multiple classes share a subset of methods | Classes used interchangeably for some operations |
| 61 | Collapse Hierarchy | Subclass adds no real behavior | Subclass with 0 additional methods/fields |
| 62 | Form Template Method | Subclasses have methods with same structure but different steps | Similar method outlines in sibling classes |
| 63 | Replace Inheritance with Delegation | Subclass uses only part of parent interface | Refused Bequest — overrides >50% of parent |
| 64 | Replace Delegation with Inheritance | Class delegates everything to another class | Middle Man — >50% pure delegation |
| 65 | Tease Apart Inheritance | One hierarchy serving two responsibilities | Inheritance tree splits along 2 dimensions |
| 66 | Convert Procedural Design to Objects | Procedural code in an OO context | Long functions with data + behavior separated |

---

## Speed Optimizations

Apply these patterns to reduce refactoring wall-clock time.

### Parallel Scan Pipeline

Run all Phase 1 analysis tools simultaneously rather than sequentially. See [Automated Scan Pipeline](#automated-scan-pipeline) for the full list. Expected speedup: 3-5x on typical codebases.

### Incremental Re-analysis

| Scenario | Strategy |
|----------|----------|
| First scan | Full analysis of all files in scope |
| Re-scan after fix | Only re-analyze files in the fix's write set |
| Re-scan after merge | Only re-analyze files changed since last scan commit |
| Cached results | Store per-file analysis keyed by file content hash |

### Scope-Limited Impact Analysis

When planning a fix, do NOT analyze the full codebase for impact. Instead:

1. Build the affected subgraph: start from the file being modified
2. Walk outward through direct importers (fan-in, depth 1)
3. Walk outward through importers of importers (depth 2) only if the public API changes
4. Stop at depth 2 — changes rarely propagate further
5. Only run tests that cover files in the affected subgraph

### Analysis Ordering Strategy

Run cheap deterministic checks before expensive LLM analysis:

| Order | Check | Cost | Catches |
|-------|-------|------|---------|
| 1 | Type checker | Low | Type errors, null safety |
| 2 | Linter | Low | Style, conventions, simple bugs |
| 3 | Complexity analyzer | Low | Function complexity scores |
| 4 | Duplicate detector | Medium | Copy-paste code blocks |
| 5 | Architecture validator | Medium | Boundary violations, cycles |
| 6 | LLM-based smell detection | High | Semantic smells, design issues |

This pipeline eliminates 6-8% of false positive work from LLM-only analysis.

### Cycle-Based Execution

Support up to 3 refactoring passes per group when refactoring creates emergent work. See [Phase 4: Execute](#phase-4-execute) for details. Limit to 3 passes to prevent infinite loops from oscillating fixes.

---

## LLM Safety Protocols

Never apply LLM-generated refactored code directly. Follow the RefactoringMirror pattern:

### RefactoringMirror Pattern (arXiv:2411.04444)

Three-stage hybrid approach achieving 94.3% success with 0% unsafe edits:

| Stage | Action | Tool |
|-------|--------|------|
| 1. Detect | Use LLM to generate refactored code, then diff original vs LLM output to identify what refactorings were applied | ReExtractor or AST diff |
| 2. Extract | Extract detailed parameters for each detected refactoring (method name, target class, line range) | Custom extraction |
| 3. Reapply | Execute the identified refactorings using battle-tested, deterministic IDE refactoring engines | IntelliJ IDEA, VS Code, or manual with tests |

**Why this matters:** LLMs produce plausible but unsafe code ~7% of the time. The RefactoringMirror pattern uses the LLM as an *advisor* (what to refactor) and deterministic engines as *executors* (how to refactor).

### Iterative Re-prompting Protocol

When a refactoring fails compilation or tests:

| Round | Action | Success Rate Improvement |
|-------|--------|------------------------|
| 1 | Re-prompt with exact error message | +25-30pp |
| 2 | Re-prompt with error + stack trace + failing test | +10-15pp |
| 3 | Re-prompt with alternative approach suggestion | +5-10pp |
| 4-20 | Continue with decreasing returns | Diminishing |
| >20 | STOP — escalate to human review | — |

Total improvement from iterative re-prompting: +40-65 percentage points over single-shot.

### Multi-Proposal Generation

Generate multiple refactoring proposals and select the best:

| Strategy | Correctness Gain |
|----------|-----------------|
| pass@1 (single proposal) | Baseline |
| pass@3 | +15-20% |
| pass@5 | +28.8% |
| Best of 3 with test validation | Recommended balance of cost vs quality |

### Safety Checklist (Before Applying Any Refactoring)

- [ ] All existing tests pass on the refactored code
- [ ] No new compiler/linter warnings introduced
- [ ] Cyclomatic complexity did not increase
- [ ] Cognitive complexity did not increase
- [ ] No architecture boundary violations introduced
- [ ] No circular dependencies introduced
- [ ] Diff is minimal — only changes what was intended

---

## Prompt Engineering for Refactoring

Prompt specificity dramatically affects LLM refactoring quality.

### Prompt Specificity Ladder

| Level | Prompt Template | Identification Rate |
|-------|----------------|-------------------|
| Generic | "Refactor this code" | 15.6% |
| Type-specific | "Apply Extract Method refactoring to this code" | 52.2% |
| Targeted | "Extract the loop body at lines 15-28 into a method called `processItem`" | 86.7% |
| Few-shot | Same as targeted + 2-3 examples of similar refactorings | ~95%+ |

**Rule:** Always use Level 3 (Targeted) or Level 4 (Few-shot) prompts in the refactoring pipeline.

### Prompt Template for Subagents

When dispatching refactoring work to a subagent, include:

1. **Smell identified:** The specific smell name and detection signal
2. **Technique prescribed:** The specific Fowler technique name (e.g., "Extract Method", not "refactor")
3. **Target location:** File path, line range, function/class name
4. **Expected outcome:** What the code should look like after (high-level)
5. **Constraints:** What must NOT change (public API, test behavior)
6. **Example (if available):** A similar before/after from the codebase or Fowler catalog

---

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

## Smell-to-Technique Quick Reference

Fast lookup: given a detected smell, which technique(s) to apply.

| Smell | Primary Technique | Secondary Technique | Skill |
|-------|------------------|--------------------|----|
| Long Method | Extract Method | Replace Temp with Query | `refactor-composing-methods` |
| Large Class | Extract Class | Extract Subclass | `refactor-moving-features` |
| Primitive Obsession | Replace Data Value with Object | Introduce Parameter Object | `refactor-organizing-data` |
| Long Parameter List | Introduce Parameter Object | Preserve Whole Object | `refactor-simplifying-method-calls` |
| Data Clumps | Extract Class | Introduce Parameter Object | `refactor-moving-features` |
| Switch Statements | Replace Conditional with Polymorphism | Replace Type Code with Subclasses | `refactor-simplifying-conditionals` |
| Temporary Field | Extract Class | Introduce Null Object | `refactor-moving-features` |
| Refused Bequest | Replace Inheritance with Delegation | Push Down Method | `refactor-generalization` |
| Divergent Change | Extract Class | Move Method | `refactor-moving-features` |
| Shotgun Surgery | Move Method | Inline Class | `refactor-moving-features` |
| Parallel Inheritance | Replace Inheritance with Delegation | — | `refactor-generalization` |
| Duplicate Code | Extract Method | Pull Up Method | `refactor-composing-methods` |
| Dead Code | Remove (delete it) | — | — |
| Lazy Class | Inline Class | Collapse Hierarchy | `refactor-moving-features` |
| Speculative Generality | Collapse Hierarchy | Inline Class | `refactor-generalization` |
| Data Class | Move Method | Encapsulate Field | `refactor-organizing-data` |
| Feature Envy | Move Method | Extract Method | `refactor-moving-features` |
| Inappropriate Intimacy | Move Method | Hide Delegate | `refactor-moving-features` |
| Message Chains | Hide Delegate | Extract Method | `refactor-moving-features` |
| Middle Man | Remove Middle Man | Inline Method | `refactor-moving-features` |
| Excessive Comments | Extract Method | Rename Method | `refactor-composing-methods` |
| High Complexity (>15) | Decompose Conditional | Replace Nested Conditional with Guard Clauses | `refactor-simplifying-conditionals` |

## Language-Specific Refactoring Notes

### TypeScript/JavaScript
| Concern | Guidance |
|---------|----------|
| Type narrowing | Prefer discriminated unions over type assertions when replacing conditionals |
| Barrel exports | When extracting classes/modules, update `index.ts` barrel exports |
| React components | Extract Method → Extract Component; watch for hook rules (no conditional hooks) |
| Async patterns | When refactoring promise chains, prefer async/await; avoid mixing styles |

### Python
| Concern | Guidance |
|---------|----------|
| Dataclasses | Replace Data Class smell with `@dataclass` + methods |
| Type hints | Add type hints when applying Extract Method or Introduce Parameter Object |
| Dunder methods | When encapsulating fields, use `@property` not Java-style getters |
| Module structure | Python favors flat module hierarchies; avoid deep nesting when extracting |

### Go
| Concern | Guidance |
|---------|----------|
| Interfaces | Extract Interface → define small interfaces at the consumer side |
| Error handling | When simplifying conditionals, preserve explicit error handling (no swallowing) |
| Packages | When extracting classes, prefer package-level organization over deep nesting |
| Exported names | Extracted public functions must have doc comments |

### Java
| Concern | Guidance |
|---------|----------|
| Records | Replace Data Class smell with Java records (Java 16+) |
| Sealed classes | Use sealed classes when replacing type codes with subclasses |
| Streams | When refactoring loops, consider Stream API but avoid nested streams |
| Dependency injection | When moving methods, update DI container configuration |

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
