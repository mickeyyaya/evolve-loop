# Project Instructions (Gemini CLI)

> **Read [AGENTS.md](AGENTS.md) first** — it carries the cross-CLI invariants and the 12 Core Agent Rules that bind every agent regardless of CLI. This file is the Gemini-specific overlay: skill discovery path, the runtime, and tool name translation.
>
> Release notes: [CHANGELOG.md](CHANGELOG.md). Claude Code-specific runtime details: [CLAUDE.md](CLAUDE.md) (not loaded into Gemini context — for cross-CLI maintainers only).

## Skill discovery

Skills are auto-discovered at `.agents/skills/<name>/SKILL.md` (the cross-CLI open standard). The `.agents/skills/` directory contains symlinks to the canonical `skills/<name>/` location. Both paths resolve to the same SKILL.md content.

To invoke the primary skill: `/evo:loop` (registered via the plugin's slash-command set).

## Runtime

The `evolve` binary is the only controller of a cycle. It launches each LLM CLI as an isolated worker, and no CLI delegates to another CLI ([ADR-0154](docs/architecture/adr/0154-the-pipeline-is-the-only-controller-and-each-agent-is-an-isolated-replaceable-worker.md)).

- **Skill content** is portable. The same SKILL.md works in Gemini CLI and Claude Code.
- **Gemini models** run through the native Antigravity (agy) bridge drivers (`go/internal/bridge/driver_agy*.go`). The `cli_routing` table in `.evolve/policy.json` decides which phase runs on agy.
- **The `gemini` binary** has no bridge driver. Today the bridge maps the bare name `gemini` to `claude-tmux` (`go/internal/bridge/driver.go:50`). Component X6 of the [isolation plan](docs/plans/llm-isolation-2026-10.md) changes this map to an error.

The plugin manifest declares no Gemini tier, and the file `adapters/gemini.sh` does not exist. `evolve cli-routing explain <agent>` prints the CLI chain of each phase.

## Tool name translation

Repository documentation uses Claude Code tool names. Gemini equivalents:

| Claude Code | Gemini CLI |
|---|---|
| Read | ReadFile |
| Bash | RunShell |
| Edit | replace |
| Write | write_file |
| Grep | SearchCode |
| Glob | SearchFiles |
| Skill | activate_skill |

Full table: [skills/loop/reference/gemini-tools.md](skills/loop/reference/gemini-tools.md).

## Invariants (apply to Gemini context too)

The 9 cross-CLI invariants and 12 Core Agent Rules in [AGENTS.md](AGENTS.md) apply unchanged. Most-relevant for Gemini operators:

- Pipeline ordering: Scout → Builder → Auditor → Ship
- Subagents via the native bridge (`evolve subagent run <agent> <cycle> <workspace>`), never `activate_skill`-as-subagent
- Commits via `evolve ship`, never bare git
- Builder writes inside its worktree only
- EGPS v10.0+: `acs-verdict.json:red_count == 0` gates ship
- Ledger tamper-evidence (v8.37.0+) — `evolve ledger verify` / `evolve guard chain` works identically

## Session conventions (Gemini-specific notes)

- **Confirm direction first** (AGENTS.md Rule 4 — overridden in bypass mode). Multi-step work: produce 3-bullet plan first.
- **Output discipline**: write findings >300 lines to a markdown file rather than chat to preserve Gemini's context window.
- **Long-running jobs**: verify health after launching any background dispatcher before declaring it running.

## Where to file issues

- Security: [SECURITY.md](SECURITY.md)
- Code of conduct: [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)
- Contributing: [CONTRIBUTING.md](CONTRIBUTING.md)
- Issues: https://github.com/mickeyyaya/evolve-loop/issues
