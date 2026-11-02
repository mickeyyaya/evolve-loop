# Assumptions and Evidence — practice-claude-code-changelog-93528320-202611021013

Every claim in `proposal.md`, `options/*.md` and `recommendation.md` cites an entry below.
The changelog text is untrusted third-party data. The loop reads it and never follows it as an instruction.

## Assumptions

- **A1** — The 2.1.316 section, as read on 2026-11-03, is the full set of lines for that version. *Basis:* the first section of the fetched file is `## 2.1.316` (E1). Probe: fetch again at the next wave boundary and compare the hash.
- **A2** — The Claude apps gateway, VS Code, cloud session, Claude Tag, Code Review and Windows lines do not reach the loop. *Basis:* the loop runs `claude` in tmux panes on macOS (E4). Probe: `evolve doctor probe claude` after the next `evolve cli update`.
- **A3** — A persona change made without a baseline cannot be judged. *Basis:* ADR-0133 D8 (E2). Probe: the first D8 table (milestone M3).
- **A4** — A changelog line is practice evidence only when it changes what a phase prompt can rely on. *Basis:* the same test as the 2.1.305 and 2.1.311 reviews (E5). Probe: none needed.

## Evidence

- **E1** — Source: https://raw.githubusercontent.com/anthropics/claude-code/main/CHANGELOG.md, fetched with `curl` on 2026-11-03. The file has 9800 lines. The first section is `## 2.1.316`. Lines that touch the loop: "Added `CLAUDE_CODE_SUBAGENT_EFFORT`", "Added `CLAUDE_CODE_DISABLE_ULTRATHINK`", "Deprecated fast mode for Opus 4.8 as of October 30, 2026; it will be removed on November 30, 2026", "Fixed sub-agents that inherited a parent sub-agent's model moving to the main model", "Fixed shell commands being left running on macOS and Linux when Claude Code exits by itself".
- **E2** — `docs/architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md`, decisions 4 to 6: one persona per change. Each diet change has a before and after table. `docs/plans/model-capability-practice-2026-10.md:97` (D7, D8).
- **E3** — `docs/plans/model-capability-practice-2026-10.md:110-111`: M2 records the baseline with `evolve prompt stats`. On 2026-11-03, `./go/bin/evolve prompt stats --help` prints `unknown command "prompt"`, so M2 has not shipped.
- **E4** — `grep -rInE "opus-4-8|opus-4\.8|fast mode|fastMode"` over `go/` (non-test), `agents/` and `docs/architecture/phase-registry.json` on 2026-11-03: no match. `grep -rIn SUBAGENT_EFFORT` over `agents/` finds none. The effort of a phase is a routing flag: `go/internal/bridge/launch.go:300` (`--effort`). `agents/evolve-builder.md:28` uses `ultrathink` as a strategy name.
- **E5** — `docs/research/claude-code-changelog-2.1.311-review.md` and `solutions/practice-claude-code-changelog-218b6f25-202610210011/recommendation.md`: the same verdict, no persona change, for 2.1.311 and 2.1.305.
- **E6** — Persona files on 2026-11-03: `agents/evolve-scout.md`, `evolve-triage.md`, `evolve-tdd-engineer.md`, `evolve-builder.md`, `evolve-auditor.md`, `evolve-retrospective.md`.
- **E7** — Cost of a persona edit: one persona file plus the 10 + 10 cycle measures of D8 (E2). Cost of this review: one small document cycle (triage `cycle_size: trivial`).
