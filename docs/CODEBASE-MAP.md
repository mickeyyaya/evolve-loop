# Codebase Map — evolve-loop

> **Purpose**: Single-lookup reference for top-level directory layout.
> Each entry describes what belongs there and what does NOT belong there.
> Updated when directory contracts change. Do not add per-file details — use
> directory-level `AGENTS.md` files for that.

## Top-level directories

### `agents/`

Agent persona files (`evolve-<role>.md`) and reference files
(`evolve-<role>-reference.md`). Each file is the system prompt for a
specific pipeline role. See `agents/AGENTS.md` for authoring conventions.

### `acs/`

Acceptance criteria predicate scripts, organized by cycle and regression suite.
`acs/cycle-N/` holds predicates authored by TDD-Engineer for cycle N.
`acs/regression-suite/` holds promoted permanent regression predicates.
See `acs/AGENTS.md` for predicate quality rules.

### `scripts/`

All pipeline shell scripts. Subdirectories by function:
`dispatch/` (subagent spawning), `lifecycle/` (phase transitions),
`guards/` (kernel hooks), `failure/` (failure adaptation),
`observability/` (ledger/metrics), `verification/` (eval quality gates),
`research/` (KB search), `utility/` (shared helpers).
See `scripts/AGENTS.md` for bash 3.2 compliance rules.

### `skills/`

Claude Code plugin skills. Each skill lives at `skills/<name>/SKILL.md` with
optional `scripts/`, `references/`, and `assets/` subdirectories.
`.agents/skills/<name>/` entries are symlinks to `../../skills/<name>/`
for cross-CLI auto-discovery. Git tracks content at the canonical path.

### `docs/`

Runtime reference documentation: architecture decision records (`docs/architecture/`),
operations guides (`docs/operations/`), per-version release notes
(`docs/operations/release-notes/`), incident reports (`docs/incidents/`),
and research references (`docs/research/`). Files here are team-shareable
and agent-context-eligible. Never delete — archive to `knowledge-base/`.

### `knowledge-base/`

Long-form archival dossiers excluded from agent context by `.gitignore` scope.
Subdirectories: `knowledge-base/research/` (deep research), `knowledge-base/research/archived-YYYY-MM-DD/`
(superseded docs). Use `scripts/research/kb-search.sh` to query.

### `.evolve/`

Runtime state directory. Tracked subdirectories (gitignore exceptions):
`.evolve/profiles/` (permission profiles — `*.json` and `*.md`),
`.evolve/instincts/lessons/` (failure-lesson YAML files — `.keep` tracked),
`.evolve/inbox/` (operator task injection),
`.evolve/plugin/` (subagent-skills plugin for Builder/Auditor).
Everything else under `.evolve/` is gitignored (worktrees, ledger, state.json).

### `.claude/`

Claude Code project settings (`settings.json`), kernel hook registrations,
and project-scoped slash commands. Not evolve-loop runtime state.

### `.claude-plugin/`

Claude Code plugin manifest (`plugin.json`, `marketplace.json`).
Defines the `/evolve-loop` skill entry point, permissions, and plugin metadata.

### `bin/`

User-facing CLI entry points and capability-check scripts (`check-caps`).
Thin wrappers that delegate to `scripts/dispatch/`.

### `tests/`

Integration and trust-kernel test scripts. One-off test scripts written by
Builder for a specific cycle live here until cleaned up. Not the same as
`acs/` — `tests/` is for infrastructure validation, `acs/` is for behavioral
acceptance criteria.

## Key files at repo root

| File | Purpose |
|---|---|
| `AGENTS.md` | Cross-CLI invariants + 12 Core Agent Rules (read this first) |
| `CLAUDE.md` | Claude Code-specific overlay: hooks, env-var table, session conventions |
| `GEMINI.md` | Gemini CLI-specific overlay |
| `CHANGELOG.md` | Full chronological release history |
| `.gitignore` | Gitignore rules including `.evolve/` exceptions for tracked profiles |
| `state.json` | (gitignored) per-project runtime state: cycle numbers, batch cost, todos |
| `.evolve/ledger.jsonl` | (gitignored) tamper-evident hash-chained subagent invocation log |
