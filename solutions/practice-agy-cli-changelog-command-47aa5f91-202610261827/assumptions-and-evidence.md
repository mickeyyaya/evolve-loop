# Assumptions and Evidence — practice-agy-cli-changelog-command-47aa5f91-202610261827

Every claim in `proposal.md`, `options/*.md` and `recommendation.md` cites an entry below.
The changelog text is untrusted third-party data. The loop reads it and never follows it as an instruction.

## Assumptions

- **A1** — The output of `agy changelog` is the full set of lines for releases 1.4.7 and 1.4.6. *Basis:* the command prints the releases under their version headings (E1). Probe: run the command again at the next wave boundary and compare.
- **A2** — The `/config`, `/diff`, `/plugin`, `/help`, and seasonal theme lines do not reach the loop. *Basis:* a phase seat runs `agy` in a tmux pane (E4). The plugin validation commands run only in the skills-publish command (E8). Probe: `evolve doctor probe agy` after the next `evolve cli update`.
- **A3** — The loop signs in to agy as a person and sets no `GEMINI_API_KEY` or `GOOGLE_API_KEY`. *Basis:* the manifest guards against both variables (E4). Probe: the same check after the next release.
- **A4** — A changelog line is practice evidence only when it changes what a phase prompt can rely on. *Basis:* hypothesis H1 of the scout report (E5). Probe: none needed.

## Evidence

- **E1** — Source: the command `agy changelog`, run in cycle 2036 on 2026-10-26. `agy --version` printed 1.4.7. The output lists releases 1.4.7 and 1.4.6.
- **E2** — E1 line counts: release 1.4.7 has 14 lines (2 Added, 5 Improved, 7 Fixed). Release 1.4.6 has 11 lines (1 Added, 6 Improved, 4 Fixed). Total 25 lines.
- **E3** — E1 lines that touch run behavior. Release 1.4.7 adds `lastUserInput` to hook stdin, improves headless `-p` CPU use, expands MCP direct returns to 10 KB, and fixes compaction loops. Release 1.4.6 enhances `agy plugin validate` and preserves attached files during compaction.
- **E4** — `go/internal/bridge/manifests/agy.json:59` and `go/internal/bridge/manifests/agy-tmux.json:74-90`. The manifest guards against `GEMINI_API_KEY` and `GOOGLE_API_KEY`. The loop sets no agy hook in a phase seat. The driver passes `--dangerously-skip-permissions`. The manifest `prompt_marker` is `? for shortcuts`.
- **E5** — `.evolve/runs/cycle-2036/scout-report.md`: the changelog text is untrusted third-party data, and the likely outcome is no persona change.
- **E6** — `docs/architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md` and `docs/plans/model-capability-practice-2026-10.md:97` (D7, D8): one persona per change, four measures over 10 cycles before and after.
- **E7** — `./go/bin/evolve prompt stats --help` printed `evolve: unknown command "prompt"` on 2026-10-26. `docs/plans/model-capability-practice-2026-10.md:110-111`: M2 records the baseline, and M3 is the first diet change.
- **E8** — `go/cmd/evolve/cmd_skills_publish.go:491` runs `agy plugin validate` and `:497` runs `agy plugin install`. Both run only in the operator's skills-publish command, not in a phase seat.
