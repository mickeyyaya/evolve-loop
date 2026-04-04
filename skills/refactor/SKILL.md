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
| [Validation Metrics](#validation-metrics) | Before/after metric comparison and fitness functions |
| [Automated Scan Pipeline](#automated-scan-pipeline) | Parallel tool execution for Phase 1 speed |
| [Speed Optimizations](#speed-optimizations) | Incremental analysis, caching, cycle-based execution |
| [Quick Modes](#quick-modes) | Scoped invocations for targeted refactoring |
| [Continuous Refactoring Integration](#continuous-refactoring-integration) | PR review, debt tracking, velocity |
| [Cross-Reference Map](#cross-reference-map) | Skill routing by issue type |
| [Reference](#reference-read-on-demand) | Detection catalogs, techniques, safety protocols |

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
2. Compute [Cognitive Complexity Score](reference/complexity-scoring.md) for every function in scope
3. Run [Architecture Analysis](reference/architecture-analysis.md) on the module graph
4. Apply `detect-code-smells` — check for all smells in the [Code Smell Catalog](reference/code-smells.md)
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

For composite health scoring, see [Multi-Metric Smell Scoring](reference/health-scoring.md).

Use `refactoring-decision-matrix` to:
1. Map each detected smell to its primary fix technique from the [Refactoring Technique Catalog](reference/refactoring-techniques.md)
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
1. Look up the fix technique in [Smell-to-Technique Quick Reference](reference/smell-to-technique-map.md)
2. Read the detailed technique from the relevant skill:
   - Method-level → `refactor-composing-methods`
   - Class-level → `refactor-moving-features`
   - Data-level → `refactor-organizing-data`
   - Conditional → `refactor-simplifying-conditionals`
   - Interface → `refactor-simplifying-method-calls`
   - Hierarchy → `refactor-generalization`
   - FP patterns → `refactor-functional-patterns`
3. Check [Language-Specific Notes](reference/language-notes.md) for the target language
4. Check if a design pattern would help → `design-patterns-creational-structural`, `design-patterns-behavioral`
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

Before executing, read [LLM Safety Protocols](reference/safety-protocols.md) and [Prompt Engineering](reference/prompt-engineering.md).

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
# Complexity analysis runs inline (see reference/complexity-scoring.md)
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

## Continuous Refactoring Integration

Embed refactoring into the development workflow rather than treating it as a separate activity.

### PR Review Integration

Run `/refactor scan` automatically on PR diffs to catch smells before merge:

| Trigger | Scope | Action |
|---------|-------|--------|
| PR opened/updated | Changed files only | Run scan pipeline, post smell report as PR comment |
| PR touches >5 files | Full affected subgraph | Run architecture analysis, flag boundary violations |
| PR increases complexity | Functions with delta >5 | Suggest specific refactoring technique inline |
| PR introduces duplicates | New duplicate blocks | Flag with jscpd report |

### Technical Debt Budget

Track refactoring debt as a measurable quantity:

| Metric | How to Compute | Target |
|--------|---------------|--------|
| Smell density | Total smells / total functions | <0.1 (10%) |
| Average composite score | Mean of all function health scores | <0.3 |
| Architecture violations | Count of boundary violations | 0 |
| Circular dependency count | DFS cycle count | 0 |
| Critical functions | Functions with composite score >0.7 | 0 |

### Refactoring Velocity Tracking

Track improvement over time:

```
| Sprint | Smells Found | Smells Fixed | Net Change | Debt Trend |
|--------|-------------|-------------|------------|------------|
| Week 1 | 45 | 0 | +45 | Baseline |
| Week 2 | 48 | 12 | -9 | Improving |
| Week 3 | 41 | 8 | -4 | Improving |
```

If debt trend reverses for 2+ sprints, escalate to team lead.

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
