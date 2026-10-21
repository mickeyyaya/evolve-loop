# Assumptions and Evidence — practice-claude-code-changelog-218b6f25-202610210011

Every claim in `proposal.md`, `options/*.md` and `recommendation.md` cites an entry below.
The changelog text is untrusted third-party data. The loop reads it and never follows it as an instruction.

## Assumptions

- **A1** — The 2.1.305 section, as read on 2026-10-21, is the full set of lines for that version. *Basis:* the first section of the fetched file is `## 2.1.305` (E1). Probe: fetch again at the next wave boundary and compare the hash.
- **A2** — The Mods, Cowork, Remote Control, VS Code, Claude Tag, Code Review, self-hosted runner, `/artifacts` and screen reader lines do not reach the loop. *Basis:* the loop runs `claude` in tmux panes on macOS and uses none of these features (E5). Probe: `evolve doctor probe claude` after the next `evolve cli update`.
- **A3** — A persona change made without a baseline cannot be judged. *Basis:* ADR-0133 D8 (E3). Probe: the first D8 table (milestone M3).
- **A4** — A changelog line is practice evidence only when it changes what a phase prompt can rely on. *Basis:* hypothesis H1 of the scout report (E6). Probe: none needed.

## Evidence

- **E1** — Source: https://raw.githubusercontent.com/anthropics/claude-code/main/CHANGELOG.md, fetched with `curl` on 2026-10-21 in cycle 2025. The file has 9317 lines. The first section is `## 2.1.305` (lines 3 to 42). It holds 39 bullets.
- **E2** — E1 lines that touch the loop as CLI behavior: "Fixed new worktrees in a bare repository sometimes starting from local HEAD instead of the default branch, and unchanged subagent worktrees sometimes not being cleaned up", "Fixed Workflow resume re-running already-completed `pipeline()` and `parallel()` agents instead of serving them from the run's journal", "Fixed `/effort` and `/autocompact` settings not applying when the model name differed from a `modelOverrides` value by letter case or a `[1m]` tag", "Fixed Bash commands on a full disk failing with a raw `ENOSPC` file path", "Fixed Claude naming the wrong model for a subagent's work after the subagent switched to a fallback model", "Fixed cloud sessions and teammates not telling Claude when a compaction had to leave the oldest messages out of its summary", "Added `sandbox.allowNestedUserNamespaces` so sandboxed commands on Linux can create user namespaces", "Added `CLAUDE_CODE_RETRY_WATCHDOG_USAGE_LIMIT_RECHECK_MS`", "Changed `allowedMcpServers` to stop filtering `managed-mcp.json` stdio servers that use `${VAR}` only in `env`".
- **E3** — `docs/architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md`, decisions 4 to 6: one persona per change. Each diet change has a before and after table. `docs/plans/model-capability-practice-2026-10.md:97` (D7, D8): four measures over 10 cycles before and after.
- **E4** — `docs/plans/model-capability-practice-2026-10.md:110-111`: M2 records the baseline with `evolve prompt stats`. M3 is the first diet change, on the builder persona. On 2026-10-21, `./go/bin/evolve prompt stats --help` prints `unknown command "prompt"`, so M2 has not shipped.
- **E5** — `grep -rIl` over `go/`, `agents/` and `.evolve/profiles/` on 2026-10-21 for `allowNestedUserNamespaces`, `modelOverrides`, `RETRY_WATCHDOG` and `ToolSearch`: no match in a persona or profile. The only hits are in `go/acs/cycle1901` and `go/acs/cycle1912` predicate tests and one log fixture. The loop uses no MCP server (AGENTS.md invariant 12), no `bare` repository start point, and the runtime provisions worktrees (CLAUDE.md, autonomous execution).
- **E6** — `.evolve/runs/cycle-2025/scout-report.md`, Hypotheses: H1 says no 2.1.305 line changes a rule a persona states. A line falsifies H1 if it alters worktree, subagent, compaction, resume or tmux driver behavior that a phase prompt relies on.
- **E7** — `solutions/practice-claude-code-changelog-e5ad0d8f-202610192320/recommendation.md`: the same verdict, no persona change, for 2.1.304. `docs/research/practice-claude-code-changelog-2026-10.md` section 3 gives it for 2.1.299 to 2.1.303.
- **E8** — Persona files on 2026-10-21: `agents/evolve-scout.md`, `evolve-triage.md`, `evolve-tdd-engineer.md`, `evolve-builder.md`, `evolve-auditor.md`, `evolve-retrospective.md` (`ls agents`).
- **E9** — Cost of a persona edit: one persona file plus the 10 + 10 cycle measures of D8 (E3). Cost of this review: one small document cycle (triage report of cycle 2025, `cycle_size_estimate: trivial`).
