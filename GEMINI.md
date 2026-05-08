# Project Instructions (Gemini CLI)

> **Cross-CLI canonical instructions are in [AGENTS.md](AGENTS.md).** This file (GEMINI.md) is the Gemini CLI-specific overlay. Read AGENTS.md first for the universal pipeline contract; come here for Gemini-specific runtime details.

## Skill discovery

Skills are auto-discovered at `.agents/skills/<name>/SKILL.md` (the cross-CLI open standard). The `.agents/skills/` directory contains symlinks to the canonical `skills/<name>/` location. Both paths resolve to the same SKILL.md content.

To invoke the primary skill: `/evolve-loop` (registered via the plugin's slash-command set).

## Runtime adapter (tier-1-hybrid)

evolve-loop's plugin manifest declares `gemini-cli: tier-1-hybrid` in `.claude-plugin/plugin.json:compatibility.tiers`. This means:

- **Skill content** is portable. The same SKILL.md works in Gemini CLI and Claude Code.
- **Runtime execution** delegates to the Claude binary via `scripts/cli_adapters/gemini.sh`. Gemini CLI lacks non-interactive prompt mode (`gemini -p`), `--max-budget-usd`, and subagent dispatch primitives that the kernel hooks require for structural enforcement. The hybrid adapter delegates the runtime work to `claude -p` while Gemini hosts the skill activation.

You need the `claude` CLI installed and authenticated for the runtime path to work. If only `gemini` is available, only skill text is usable (no autonomous cycle execution).

## Tool name translation

Tool names in this repository's documentation use Claude Code conventions (`Read`, `Bash`, `Skill`, `Agent`). Gemini equivalents:

| Claude Code | Gemini CLI |
|---|---|
| Read | ReadFile |
| Bash | RunShell |
| Edit | replace |
| Write | write_file |
| Grep | SearchCode |
| Glob | SearchFiles |
| Skill | activate_skill |

Full table: [skills/evolve-loop/reference/gemini-tools.md](skills/evolve-loop/reference/gemini-tools.md).

## Restrictions (apply to Gemini context too)

The cross-CLI invariants in [AGENTS.md](AGENTS.md) apply unchanged:
- Pipeline ordering (Scout → Builder → Auditor → Ship)
- Subagents via `subagent-run.sh`, never `activate_skill`-as-subagent
- Commits via `scripts/lifecycle/ship.sh`, never bare git
- Builder writes inside its worktree only
- Audit verdicts: PASS/WARN/FAIL semantics (WARN ships by default v8.35.0+)
- Ledger tamper-evidence (v8.37.0+) — `verify-ledger-chain.sh` works identically

## Where to file issues

- Security: [SECURITY.md](SECURITY.md)
- Code of conduct: [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)
- Contributing: [CONTRIBUTING.md](CONTRIBUTING.md)
- Issues: https://github.com/mickeyyaya/evolve-loop/issues
