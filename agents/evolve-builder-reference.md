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

Loaded when `legacy/scripts/utility/code-review-simplify.sh` exists in the project.
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
| `Read(legacy/scripts/foo.sh)` → wait | `Read(legacy/scripts/foo.sh)`, `Read(legacy/scripts/bar.sh)`, `Read(agents/evolve-builder.md)` |
| `Read(legacy/scripts/bar.sh)` → wait | all results return together |

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
bash legacy/scripts/utility/code-review-simplify.sh HEAD 2>/dev/null || true
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

## Section: tool-hygiene-rules

Loaded for Step 2 (after Skills, before Design). Consolidates three turn-budget / context-budget protocols.

### Tool-Result Hygiene (P-NEW-6)

Avoid context saturation from accumulated tool results:
- After each `Read`, summarize the content in 2-3 lines; reference the summary in subsequent turns, not the raw file.
- After each `Bash` or `WebFetch` with large output, extract the relevant lines; discard the full output from your working context.
- No speculative pre-loading: use Glob+Grep to locate before Reading.
- Line-range Reads for large files (>200 lines): `Read(file, offset=N, limit=50)`.

### Tool-Result Trajectory Compression (P-NEW-21)

During multi-turn file reading phases, "expired" tool results (file already read, content already extracted) accumulate in your trajectory. Actively prune:
- Do not output or repeat the contents of old tool results in your thought process.
- When `context_clear_trigger_tokens` threshold is hit, emit a summary turn condensing pending state, dropping file contents, before the next tool call.

### Parallel Tool-Call Batching (P-NEW-29)

When reading 2+ independent files or searching 2+ independent patterns, emit all tool calls in **one turn**:

```
# SLOW (2 turns): Read(file_a), then Read(file_b)
# FAST  (1 turn): Read(file_a), Read(file_b)  ← emit together
```

Only serialize when result B depends on result A. Each sequential call wastes a full turn plus tool-schema overhead.

### Skills Invoked Record Format

After skill invocations (Step 2.7), record in `build-report.md`:

```markdown
## Skills Invoked
| Skill | Priority | Outcome | Useful? |
|-------|----------|---------|---------|
| `everything-claude-code:security-review` | primary | Guided input validation approach | yes |
| `python-review-patterns` | supplementary | Skipped — instinct covered pattern | skipped |
```

Ledger entry: `"skillsInvoked": [{"name": "<skill>", "useful": true|false|"skipped"}]` in `data`.

---

## Section: posthoc-enforcement

Loaded when authoring `build-report.md` metrics or AC-existence claims.

**You are FORBIDDEN from self-quoting these 8 truthable metrics** (canonical list at [docs/architecture/posthoc-schema.md](../docs/architecture/posthoc-schema.md)):

| Metric | Ground-truth artifact |
|---|---|
| `total_cost_usd` | `<role>-usage.json` |
| `num_turns` | `<role>-usage.json` |
| `duration_ms` | `<role>-timing.json` |
| `input_tokens` | `<role>-usage.json` |
| `output_tokens` | `<role>-usage.json` |
| `cache_read_input_tokens` | `<role>-usage.json` |
| `files_changed` | `git show <sha> --numstat` |
| `lines_added` / `lines_removed` | `git show <sha> --numstat` |

Plus all **AC-existence claims** ("file X exists" or "command Y exits 0").

**Required format** in build-report.md:

```markdown
| num_turns | pending <!-- POSTHOC: jq '.num_turns' .evolve/runs/cycle-N/builder-usage.json --> |
| docs/architecture/foo.md exists | pending <!-- POSTHOC: test -f docs/architecture/foo.md && echo OK || echo MISSING --> |
```

The Auditor will execute every POSTHOC command and substitute the ground-truth value. **Authored-prose `exit 0` text after a `test -f` command is forbidden** — that pattern is what cycle 75 fabricated and was caught FAIL@0.99 confidence.

**INERT marker discipline (v10.10.0 Layer 3):** If you mark a piece of work `INERT`, you MUST include `re_attempt_by_cycle: N` where N ≤ current_cycle + 5. INERT without a deadline is treated as permanent abandonment and violates the constitutional audit checklist (Layer 4 P5).

```markdown
> **INERT cycle 76** — re_attempt_by_cycle: 81 — Advisory turn-budget cannot constrain
> the implementer that writes the telemetry. Case A (programmatic kill) blocked by no
> --max-turns flag in claude -p. Re-attempt when claude CLI exposes a turn limit.
```

---

## Section: builder-notes-template

Loaded for Step 9 Retrospective.

Write `workspace/builder-notes.md` (≤20 lines) using this template:

```markdown
# Builder Notes — Cycle {N}
## Task: <slug>
### File Fragility
- <file>: <observation about brittleness, coupling, blast radius>
### Approach Surprises
- <unexpected findings>
### Recommendations for Scout
- <sizing/scoping suggestions, areas to avoid>
```

---
