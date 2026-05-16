# Builder Reference (Layer 3 — on-demand)

Sections here are loaded only when specific build conditions activate. In
the common build path, none of this is needed. v8.64.0 Campaign D Cycle D2
split.

The builder's compact role-card (Layer 1) lives at
`agents/evolve-builder.md` and includes a `## Reference Index` pointing here.

---

## Section: e2e-test-generation

Loaded when ANY of the following is true:

- `task.recommendedSkills` contains `everything-claude-code:e2e-testing` or `ecc:e2e`
- The eval definition at `.evolve/evals/<task-slug>.md` contains an `## E2E Graders` section
- `task.filesToModify` touches routes, pages, components, forms, or auth flows

Otherwise: skip this section entirely. Do not invoke the e2e-testing skill
speculatively.

**Workflow:**

1. Invoke the `everything-claude-code:e2e-testing` skill (or the closest
   available `e2e` alternative found in the skill inventory) via your
   native skill invocation tool. Pass a user-flow description derived
   from the task's acceptance criteria (e.g., "verify /health page renders
   with status text and correct HTTP 200").
2. The skill generates `tests/e2e/<task-slug>.spec.ts` using the Page
   Object Model pattern.
3. Run the generated test inside the worktree:
   `npx playwright test tests/e2e/<task-slug>.spec.ts --reporter=list,html`.
4. If the test fails due to an implementation gap, iterate on the
   **implementation** — not the test — until it passes. Weakening or
   skipping the generated test is eval tampering (Auditor D.5 flags
   CRITICAL).
5. Commit the generated test file(s) as part of the task's worktree commit.
6. Record the test path and pass result in `build-report.md` under a new
   `## E2E Verification` section (see Output template in Layer 1).

**Platform fallback:** If `npx playwright` is unavailable in this project,
the skill's own setup flow should run `npx playwright install --with-deps`.
If installation fails, emit a single `## E2E Verification` row with
`status: SKIPPED — reason: playwright not available` rather than halting
the build.

---

## Section: capability-gap-detection

Loaded only when the task cannot be solved with existing tools / instincts /
genes. Rare-trigger.

If the build cannot proceed with what's available:

1. Identify the gap (what tool / pattern / library is missing).
2. Search for an existing tool, library, or MCP server that fills it.
3. If still missing, write a reusable script to `.evolve/tools/<tool-name>.sh`
   with usage comment, input validation, and error handling.
4. Log a `tool-synthesis` ledger entry capturing the gap, the synthesized
   tool, and the trigger task.

---

## Section: optional-self-review

Loaded when `scripts/utility/code-review-simplify.sh` exists in the project.
Optional — non-blocking. If the script is missing or failing, skip silently.

---

## Section: worktree-isolation

Loaded for Step 0 verification. Builder runs in an isolated git worktree provisioned by `run-cycle.sh`.

```bash
MAIN_WORKTREE=$(git worktree list --porcelain | head -1 | sed 's/worktree //')
CURRENT_DIR=$(pwd)
if [ "$MAIN_WORKTREE" = "$CURRENT_DIR" ]; then
  echo "FATAL: Builder is running in the main worktree. Aborting."
fi
```

**Worktree Commit Protocol**: After self-verifying, commit all changes in worktree:
`git add -A && git commit -m "<type>: <description> [worktree-build]"`

---

## Section: tool-batching

Loaded for turn-budget optimization. Batch independent tool calls to save turns.

| ❌ SLOW (3 turns) | ✅ FAST (1 turn) |
|---|---|
| `Read(scripts/foo.sh)` → wait | `Read(scripts/foo.sh)`, `Read(scripts/bar.sh)`, `Read(agents/evolve-builder.md)` |
| `Read(scripts/bar.sh)` → wait | all results return together |

Rule: if two tool calls have no data dependency on each other, emit them in the same response.

---

## Section: egps-predicates

Loaded for EGPS Predicate Authoring (v10.1.0+).

Every AC in `build-report.md` MUST have an executable predicate at `acs/cycle-N/{NNN}-{slug}.sh`.

**Required header**:
```bash
#!/usr/bin/env bash
# AC-ID:         cycle-N-NNN
# Description:   one-line summary
# Evidence:      pointer (file:line OR commit-SHA)
# Author:        builder
# Created:       <ISO-8601>
# Acceptance-of: build-report.md AC line reference
```

Banned: `grep -q` as only check, `exit 0` no-op, `curl`, `sleep` > 2s.

After Step 5 self-verify passes, optionally run the lightweight pipeline
layer on the changes:

```bash
bash scripts/utility/code-review-simplify.sh HEAD 2>/dev/null || true
```

- If maintainability findings are reported, apply simplifications before
  reporting (Extract Method, flatten nesting, remove dead code).
- If no findings or script not found, skip silently.
- Include self-review score summary in build-report.md under
  `## Self-Review`.
- Missing or failing script does NOT block the build.

---

<!-- ANCHOR:build-research-protocol -->
## Section: build-research-protocol

Loaded for Step 2.5.

**Per-task cache check (Phase B; `EVOLVE_RESEARCH_CACHE_ENABLED=1`):** If `task.research_pointer` is non-empty, read from that path instead of doing KB scan or web search.
- `Research Source: per-task-cache` — log in `## Research Sources` of build-report.md; skip remaining sub-steps.

**Fallback (research_pointer absent or feature disabled):**
- Check `.evolve/research/` for existing Knowledge Capsules → `Research Source: knowledge-capsule`
- If needs external knowledge, follow Accurate Online Researcher Protocol (`skills/evolve-loop/online-researcher.md`) → `Research Source: web-search`
- Save capsule to `.evolve/research/<topic-slug>.md`
- If no research needed → `Research Source: no-research-needed`

**Routing:** Quick gaps → **Default WebSearch** (1-2 queries); complex architecture → **Smart Web Search**. See `online-researcher.md`.

---

<!-- ANCHOR:self-review-loop-detail -->
## Section: self-review-loop-detail

Loaded for Step 5 convergence loop.

Convergence loop (pseudocode):

```
for iter in 1..MAX_ITERS:
    all_clean = true
    for skill in split(EVOLVE_BUILDER_REVIEW_SKILLS, ','):
        invoke Skill tool with `skill` (the skill reads `git diff HEAD` itself)
        parse: composite_score (0.0-1.0), severity_counts (HIGH/CRITICAL)
        if composite_score >= THRESHOLD and HIGH+CRITICAL == 0:
            continue                         # this skill is clean
        else:
            apply fixes to worktree (Edit/Write/MultiEdit per findings)
            all_clean = false
    if all_clean: break                       # converged
record final state: converged | iter-cap-hit | error
```

Skill contract: read diff; emit composite score 0.0-1.0 + severity (HIGH/CRITICAL); parseable output. Default: `code-review-simplify`; extend via `EVOLVE_BUILDER_REVIEW_SKILLS=code-review-simplify,refactor`.

---

<!-- ANCHOR:discovery-scan-guidelines -->
## Section: discovery-scan-guidelines

Loaded for Step 8.5. Record ≥1 discovery per build:

| Category | What to Look For |
|----------|-----------------|
| `latent-bug` | Bugs in adjacent code from current change |
| `inconsistency` | Pattern/convention mismatches across related files |
| `simplification-opportunity` | Code that could be simplified or deduplicated |
| `missing-test` | Untested paths/edge cases in touched code |
| `architecture-smell` | Coupling, layering violations, abstraction leaks |
| `performance-opportunity` | Inefficient patterns spotted during implementation |

