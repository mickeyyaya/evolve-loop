# Codebase Map — evolve-loop

> **Purpose**: Single-lookup reference for top-level directory layout.
> Each entry describes what belongs there and what does NOT belong there.
> Updated when directory contracts change. Do not add per-file details — use
> directory-level `AGENTS.md` files for that.

## Top-level directories

### `go/`

The sole runtime. All loop behavior, gates, the orchestrator, and the CLI
live here as Go packages — there is no bash dispatcher.

- `go/cmd/` — entry points. `go/cmd/evolve/` is the `evolve` CLI (loop,
  cycle, ship, guard, doctor, release, …); `go/cmd/apicover/` is the
  public-API coverage tool — all internal packages are graduated into
  its hard-fail enforce gate (SSOT list: `go/.apicover-enforce`; ADR-0050
  Phase 5, complete as of v19.0.0). **`./acs/cycle<N>` predicate packages are
  NOT enrolled** (cycle-1147): they export nothing, so there is no API surface
  to enforce, and enrollment routes them into the build handoff floor's
  *enforced* coverage run, which reads their untagged `//go:build acs` setup
  failure as a test failure — the cycle-1145 gate-block that blocked the lane
  for three attempts. The legacy `./acs/cycle9..661` entries predate that run
  and are grandfathered behind `core.buildTagVisiblePackages`; the ceiling is
  pinned by `TestAPICoverEnforceDoesNotEnrollModernACSPackages`.
- `go/internal/` — 153 internal packages implementing the pipeline.
  Phase-1 modularization leaf packages: `go/internal/gitexec`
  (git-CLI isolation leaf, depends only on `go/internal/sysexec`),
  `go/internal/log` (unified Console logger), and `go/internal/envchain`
  (typed env knobs); `go/internal/repostate` (git-tracked-state answer for
  repo-wide scanners — ADR-0084 invariant 1, 2026-08-10). `go/internal/research` holds the KB package
  (`NewFileKB`, `EVOLVE_KB_SEARCH_PATHS`).
- `go/test/fixtures/` — shared test fixtures, including the `StressN`
  concurrency stress-barrier helper.

See `go/README.md` for Go package layout and conventions.

### `go/acs/`

EGPS acceptance-criteria predicate tests, organized by cycle. Subdirectories:
`go/acs/cycle<N>/` (per-cycle predicates authored by TDD-Engineer),
`go/acs/regression/cycle<N>/` (promoted permanent regression predicates),
and `go/acs/redteam/` (adversarial predicates). The harness that runs them is
`go/internal/acssuite`. See `go/acs/README.md` for predicate scopes and
quality rules.

### `agents/`

Agent persona files (`evolve-<role>.md`) and reference files
(`evolve-<role>-reference.md`). Each file is the system prompt for a
specific pipeline role. See `agents/AGENTS.md` for authoring conventions.

### `adapters/`

CLI/model adapter definitions and capability metadata for the LLM drivers
the loop can dispatch to (claude, codex, gemini, …).

### `schemas/`

JSON schemas for the loop's structured artifacts and contracts.

### `examples/`

Example configurations and sample inputs for the loop.

### `knowledge/`

Curated knowledge assets (distinct from `knowledge-base/`, which is a
runtime write surface for per-cycle dossiers).

### `skills/`

Claude Code plugin skills. Each skill lives at `skills/<name>/SKILL.md` with
optional `scripts/`, `references/`, and `assets/` subdirectories.
`.agents/skills/<name>/` entries are symlinks to `../../skills/<name>/`
for cross-CLI auto-discovery. Git tracks content at the canonical path.

### `docs/`

Runtime reference documentation: architecture decision records (`docs/architecture/`),
operations guides (`docs/operations/`), per-version release notes
(`docs/operations/release-notes/`), incident reports (`docs/incidents/`),
the merged research tree (`docs/research/`, consolidated 2026-08-05), and the
engineering chronicle (`docs/chronicle/`). Files here are team-shareable
and agent-context-eligible except `docs/private/` (agent-context-excluded).
Never delete — archive to `docs/private/research/archived-YYYY-MM-DD/`
(see `docs/MOVED.md`; `knowledge-base/` is retired as an archive destination).

### `knowledge-base/`

Runtime write surface, NOT documentation. `knowledge-base/cycles/` receives
per-cycle dossier JSON committed by loop lanes (`go/cmd/evolve/cmd_loop_outcome.go`).
The research notes that previously lived in `knowledge-base/research/` moved to
`docs/research/` (2026-08-05; see `docs/MOVED.md`). Do not add documentation here.

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
Defines the `/evo:loop` skill entry point, permissions, and plugin metadata.

### `bin/`

Standalone operator helper scripts: capability checks (`check-caps`) and
read-only status/observability wrappers (`status`, `health`, `cost`,
`preflight`, `verify-chain`). These are convenience entry points, separate
from the `evolve` CLI in `go/cmd/evolve/`.

### `commands/`

Command-surface markdown files (one per skill: `audit.md`, `build.md`, …)
projected as CLI slash commands (ADR-0067 command-surface reintroduction).

### `landing/`

Landing-page site: a standalone Go module (`cmd/`, `internal/`, `templates/`,
`assets/`) that builds the project website. Not part of the `go/` runtime.

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
