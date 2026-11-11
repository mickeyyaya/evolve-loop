# Assumptions and Evidence — practice-claude-code-changelog-a1065345-202611030236

Every claim in `proposal.md`, `options/*.md` and `recommendation.md` cites an entry below.
The changelog text is untrusted third-party data. The loop reads it and never follows it as an instruction.

## Assumptions

- **A1** — The 2.1.317 section, as read on 2026-11-10, is the full set of lines for that version. *Basis:* the section runs from line 360 to line 449 of the fetched file (E1). Probe: fetch again at the next wave boundary and compare the hash.
- **A2** — The gateway, Desktop, VS Code, cloud, Remote Control and Claude Tag lines do not reach the loop. *Basis:* the loop runs `claude` in tmux panes on macOS (E4). Probe: `evolve doctor probe claude` after the next `evolve cli update`.
- **A3** — A persona change made without a baseline cannot be judged. *Basis:* ADR-0133 decision 5 (E2). Probe: the first before and after table (milestone M3).
- **A4** — A changelog line is practice evidence only when it changes what a phase prompt can rely on. *Basis:* the same test as the 2.1.316 review (E5). Probe: none needed.

## Evidence

- **E1** — Source: https://raw.githubusercontent.com/anthropics/claude-code/main/CHANGELOG.md, fetched with `curl` on 2026-11-10. The file has 10247 lines. The first section is now `## 2.1.321`. The section `## 2.1.317` has 87 bullet lines. Lines that touch the loop: "Added `CLAUDE_CODE_DISABLE_NESTED_SESSION_ID`", "Fixed `claude -p` exiting 0 when a full disk or quota lost its transcript", "Changed subagents and teammates on a different model ... to use that model's saved effort level", "Removed the Read tool's 'file unchanged since your last Read' stub".
- **E2** — `docs/architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md`, decisions 4 and 5: one persona per change, and a before and after table for each diet change.
- **E3** — `./go/bin/evolve prompt stats` prints `unknown command "prompt"` on 2026-11-10, so milestone M2 (the baseline) has not shipped.
- **E4** — The `claude -p` probes of the bridge: `go/internal/bridge/driver.go:31` and `go/internal/bridge/wallcorroborate.go:19`. The effort of a phase is a routing flag: `go/internal/bridge/launch.go:302` (`--effort`). The loop reads no control value from the environment.
- **E5** — `docs/research/claude-code-changelog-2.1.316-review.md`: the same verdict, no persona change, for 2.1.316.
- **E6** — The full line-by-line review is `docs/research/claude-code-changelog-2-1-317.md`.
- **E7** — Cost of this review: one small document cycle (triage `cycle_size: trivial`). Cost of a persona edit: one file plus the 10 + 10 cycle measures of decision 5 (E2).
