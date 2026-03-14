# Cycle 1 Briefing

## Project Context
- **Language/Framework:** Markdown/Shell — Claude Code plugin (agents + skill)
- **Test Commands:** `./install.sh` (integration smoke test)
- **Domain:** AI agent orchestration — self-evolving development pipelines
- **Target Audience:** Developers using Claude Code who want autonomous improvement loops on their codebases
- **Version:** v3.0.0 (released 2026-03-12)
- **Install Surface:** Claude Code plugin (recommended) or manual `./install.sh`
- **Dependency:** Everything Claude Code (ECC) plugin — 5 of 11 agents delegate to ECC subagent_types at runtime
- **Cycle history:** 0 completed cycles (Cycle 1 is the first)
- **State:** No evaluated tasks, no failed approaches, no instincts, no prior research

---

## Holistic Assessment

### Features — MEDIUM

**What's built:**
- Complete 8-phase pipeline (MONITOR-INIT → DISCOVER → PLAN → DESIGN → BUILD → CHECKPOINT → VERIFY → EVAL → SHIP → LOOP+LEARN)
- 11 agent definition files (6 custom, 5 ECC context overlays)
- Eval hard gate (Phase 5.5) with 3-attempt retry protocol
- Instinct extraction and confidence-based promotion (after 5+ cycles)
- Loop Operator with 3 invocation modes (pre-flight, checkpoint, post-cycle)
- Goal-directed and autonomous discovery modes
- Dual install paths: plugin marketplace + manual `install.sh`
- Shared memory in 5 layers: JSONL ledger, markdown workspace, state.json, eval state, instincts

**Gaps and missing features:**
- **No dry-run / preview mode** — users cannot simulate a cycle without actually executing it. High-value for first-time users evaluating the tool.
- **No cost tracking implementation** — `costBudget` field exists in state.json and Operator checks for it, but there is no actual cost accumulation logic. No agent records cost spent; Operator can't compute real cycle cost vs. budget.
- **No `notes.md`** exists yet in `.claude/evolve/` — cross-cycle context file is absent. Will be created on first cycle completion but has no bootstrap content.
- **No `TASKS.md` / `TODO.md` / `BACKLOG.md`** in the project root — the PM and Planner agents look for these files but they don't exist.
- **No CI/CD for the plugin itself** — there's a `.github/` directory with PR and issue templates but no workflow files (no automated testing of install.sh, no lint/validate for agent markdown).
- **No test harness for agent definitions** — agent files are plain Markdown; there's no validation that they conform to expected structure (frontmatter schema, required sections).
- **Instinct promotion path** (`~/.claude/homunculus/instincts/personal/`) is undocumented externally. No mention in README or configuration docs — users won't know this path exists or how global instincts work.
- **ECC version pinning** — the Operator copies from ECC but there's no mechanism to detect when ECC updates its agent files. The `## ECC Source` sync dates in agent files (e.g., `2026-03-12`) are manually maintained.

### Performance — LOW

This is a Markdown/Shell plugin — "performance" primarily concerns orchestration latency:
- **Parallel execution is correctly specified** — Phase 1 and Phase 5 both run 3 agents in parallel, which is the right design.
- **No observable performance issues** — file sizes are appropriate (all agent files under 200 lines, skill files under 350 lines).
- **Context window efficiency** — ECC overlay pattern (~40 lines vs. ~150-200 lines) is a good architectural choice that saves orchestrator context budget. Well-designed.
- **No performance concerns** at this stage.

### Stability — HIGH

**Issues found:**
- **ECC dependency is a runtime failure risk** — 5 agents use `everything-claude-code:*` subagent_types. If ECC is not installed, these agents fail at runtime with no graceful fallback. The install.sh warns but does not block. There's no runtime detection or fallback path documented in the skill.
  - The Loop Operator pre-flight log from Cycle 1 confirms: `ECC standalone agents absent: tdd-guide, code-reviewer, e2e-runner, security-reviewer`. Currently non-blocking for Cycle 1 but VERIFY and DESIGN phases will fail.
- **Eval graceful degradation** — eval-runner.md specifies "auto-PASS if no eval files exist (graceful degradation for cycle 1)" which is good, but this means Cycle 1 has NO quality gate. The first cycle ships without any automated eval checks.
- **Retry storm risk** — The VERIFY + EVAL retry loop (max 3 iterations) is documented but the Developer's own retry loop (max 3 attempts) is separate. In worst case: 3 Developer × 3 VERIFY/EVAL = 9 total BUILD attempts before failure. This compound retry behavior is not mentioned in user-facing docs.
- **`nothingToDoCount >= 3` convergence signal** — the signal is checked but if the project is self-evolving itself, it may converge prematurely (after 3 cycles of no useful improvements to the plugin itself).
- **HALT protocol depends on user attention** — operator HALT pauses the loop and waits for user input, but in fully automated/unattended contexts, this creates a hung process.

### UI/UX — MEDIUM

This is a CLI plugin, so "UI" means the user-facing experience of the `/evolve-loop` command and its output:
- **Cycle summary output** is well-specified in phases.md (structured terminal output with Task, Coverage, Review, E2E, Security, Eval, Research, Deploy, Instincts, Operator fields). Good design.
- **User approval gate in Phase 2** — in interactive mode, the user sees `workspace/backlog.md` and must confirm. This is appropriate. However, how the backlog is presented (raw markdown vs. formatted) depends entirely on the orchestrator's behavior, which is underspecified.
- **No progress indicators during long phases** — BUILD (Phase 4) can run for extended periods with no feedback to the user. Phases.md doesn't specify any intermediate output during execution.
- **No abort/pause mechanism during non-HALT phases** — users can't interrupt a running cycle gracefully between phases (only via HALT escalation from Operator).
- **Error messages** — agent failure modes (ECC not installed, git errors, test failures) are not explicitly surfaced to the user with actionable guidance. The Deployer handles CI failures well (3-attempt recovery), but most other failure modes just log to workspace files.

### Usability — MEDIUM

- **README is comprehensive** — Quick Start, architecture diagram, data flow, agent table, workspace layout, configuration reference. Good first-impression documentation.
- **Argument parsing is clear** and well-documented with examples.
- **First-run experience has friction** — ECC must be installed first, but the relationship between ECC and evolve-loop is not intuitive. Users may not understand why ECC is needed.
- **`/evolve-loop` on itself** — the plugin is designed to evolve any codebase, but using it to evolve itself creates a conceptual loop that users will naturally attempt. There's no guidance on self-improvement mode.
- **Configuration is buried** — `state.json` is auto-created, but users who want to set `costBudget` must know to edit it manually. No guided setup.
- **Instinct system is opaque** — the YAML format and confidence scoring are documented in phases.md but not in the user-facing README. Power users won't know instincts are accumulating or how to inspect/edit them.
- **`wt merge` / `wt switch` commands** in the Deployer and Developer agents are undocumented — no explanation of what `wt` is or how to install it. This is a latent breakage point.

### Code Quality — MEDIUM

- **File sizes are appropriate** — all files within guidelines.
- **Duplication concern** — the architecture diagram appears verbatim in 3 places: README.md, docs/architecture.md, and SKILL.md. These will drift over time.
- **Inconsistent ledger format** — The ledger entry in `loop-operator-log.md` uses `"timestamp"` key while memory-protocol.md specifies `"ts"`. This is a schema inconsistency introduced in the first write.
- **`evolve-operator.md` is a hybrid** — it has the full ECC loop-operator content copied inline (not via subagent_type delegation like other ECC wrappers). This is documented as intentional ("no ECC subagent_type exists") but means the Operator content will drift from ECC's loop-operator.md. The sync date (2026-03-12) will become stale.
- **`writing-agents.md` describes the old "copy ECC content" pattern** in the ECC wrapper section — it contradicts the current context-overlay architecture by saying "Copy the full content of the ECC agent file."
- **No linting or schema validation** for agent frontmatter or workspace file formats.

### Security — LOW

This is a plugin that orchestrates AI agents in user's environments:
- **No hardcoded secrets** — clean.
- **No network calls in the plugin itself** — Researcher uses WebSearch/WebFetch at runtime (delegated to Claude Code's tools), not in shell scripts.
- **install.sh** uses `set -euo pipefail` — good practice.
- **Worktree isolation** is specified for BUILD phase — prevents contaminating the main branch during development.
- **No input validation for `$ARGUMENTS`** — argument parsing is described in prose, not implemented as strict validation. A malformed goal string could inject unexpected behavior into agent prompts. Low risk given Claude Code's sandboxing.
- **State.json is writable by all agents** — no concurrency controls. Parallel agents in Phase 1/5 do not write to state.json (only the orchestrator does), so this is acceptable by design.

### Architecture — MEDIUM

**Strengths:**
- Clean separation of concerns — each agent owns exactly one workspace file.
- ECC overlay pattern is elegant — ~40 lines per overlay vs. duplicating 150-200 lines.
- 5-layer memory architecture is well-reasoned (ledger → workspace → state → evals → instincts).
- Hard gate (eval) with retry protocol prevents bad code from shipping.
- Parallel execution is correctly specified at the right phases.

**Issues:**
- **`wt` tooling dependency is unresolved** — Deployer uses `wt merge` and Developer uses `wt switch --create`. This appears to be a custom worktree management CLI tool. It's not documented, not in prerequisites, and not checked in the pre-flight. This is a latent CRITICAL failure point for Phase 4 and Phase 6.
- **Instinct promotion path** (`~/.claude/homunculus/`) implies a "homunculus" system that is referenced but not defined anywhere in this repo. It's unclear if this is an ECC feature, a separate tool, or aspirational.
- **No state.json locking** — if two cycles ever ran concurrently (e.g., user manually runs two sessions), state.json would be corrupted. Low risk in practice but worth documenting.
- **Eval runner is orchestrator-executed**, not an agent — this means it has no isolation or separate context window. On very long cycles, the orchestrator's context window may be exhausted before reaching Phase 5.5.
- **No circuit breaker for the research TTL system** — if a research query returns outdated results that lead to bad task selection, there's no way to force-refresh a specific query without manually editing state.json.

---

## Backlog Triage

### Active (ready to work)
No existing TASKS.md / TODO.md / BACKLOG.md found in the project. The following are inferred from gap analysis:

1. **Document `wt` dependency** — critical gap; BUILD and SHIP phases will silently fail without it
2. **Fix ledger schema inconsistency** — `"timestamp"` vs `"ts"` key mismatch between actual output and spec
3. **Update `writing-agents.md`** — contradicts current ECC overlay architecture
4. **Add CI workflow** — no automated testing for the plugin itself (install.sh, agent file validation)
5. **Document instinct system in README** — users don't know instincts accumulate or how to inspect them
6. **Document `homunculus` path** — `~/.claude/homunculus/instincts/personal/` promotion target is undefined

### Stale (needs re-evaluation)
- None — this is Cycle 1, no prior history

### Blocked (dependencies)
- **Cost tracking implementation** — blocked on understanding how Claude Code exposes per-session cost data to orchestrators

---

## PM Recommendations

### Top Priority (ranked)

1. **Resolve the `wt` tool dependency** (CRITICAL) — The Deployer (`wt merge`) and Developer (`wt switch --create`) both call a `wt` CLI that is not documented, not in prerequisites, and not checked in pre-flight. If `wt` is not installed, BUILD and SHIP phases fail silently. Either document what `wt` is and add it to prerequisites, or replace with standard `git worktree` commands. This blocks the core pipeline.

2. **Fix ledger schema inconsistency** (HIGH) — The existing ledger entry uses `"timestamp"` but the spec in memory-protocol.md and all agent ledger formats use `"ts"`. If the orchestrator or analytics tooling ever parses the ledger, this mismatch causes bugs. Establish the canonical key and fix the existing entry.

3. **Update `writing-agents.md` to reflect ECC overlay pattern** (HIGH) — Currently describes the old "copy ECC content" approach that was replaced in v3.0.0. Contributors following this guide will create bloated agent files that duplicate ECC content. Update to describe the context overlay pattern.

4. **Add CI/CD for the plugin** (MEDIUM) — No automated checks exist for the plugin itself. A GitHub Actions workflow that runs `./install.sh`, validates agent frontmatter, and checks markdown structure would catch regressions. Given the project is a plugin that emphasizes quality gates, it should practice what it preaches.

5. **Clarify ECC runtime dependency in pre-flight** (MEDIUM) — The pre-flight currently marks ECC absence as "non-blocking for Cycle 1" but DESIGN, BUILD, and VERIFY phases will fail if ECC agents are absent. The Operator should flag this as HIGH WARNING (not just a note) and specify exactly which phases will be impacted.

### Gaps & Opportunities

- **Self-improvement run** — the plugin is now at v3.0.0 and ready to evolve itself. Running `/evolve-loop` on this repo is the canonical demonstration use case.
- **Dry-run mode** — a `--dry-run` flag that runs discovery (Phase 0-2) without BUILD/SHIP would let users preview what evolve-loop would do to their codebase. High value for adoption.
- **Cost telemetry** — once `costBudget` tracking is wired to actual Claude Code cost data, this becomes a compelling enterprise feature (predictable AI spend).
- **Instinct marketplace** — high-confidence instincts promoting to `~/.claude/homunculus/` (global scope) is the seed of a shared knowledge base. This could eventually be a community contribution mechanism.

### Risks & Concerns

- **`wt` tool is a silent critical dependency** — if users don't have it, Phases 4 and 6 fail without clear error messaging.
- **ECC version drift** — `evolve-operator.md` copied ECC content on 2026-03-12. As ECC evolves, this copy will silently drift. Need a sync mechanism or clear policy.
- **First cycle has no eval gate** — Cycle 1 ships without automated quality checks (graceful degradation). For a project that emphasizes the eval hard gate, this is an ironic gap.
- **Context exhaustion** — long cycles may exhaust the orchestrator's context window before reaching Phase 7's LOOP+LEARN. No recovery path for partial cycles.

### Deferred from Previous Cycles
- None — Cycle 1 has no prior history.
