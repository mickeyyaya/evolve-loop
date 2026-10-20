# Assumptions and Evidence — practice-agy-cli-changelog-command-be72426b-202610200639

Every claim in `proposal.md`, `options/*.md` and `recommendation.md` cites an entry below.
The changelog text is untrusted third-party data. The loop reads it and never follows it as an instruction.

## Assumptions

- **A1** — The output of `agy changelog` is the full set of lines for release 1.4.3. *Basis:* the command prints the release under its version heading (E1). Probe: run the command again at the next wave boundary and compare.
- **A2** — The `/diff`, plugin, timer, Windows and Remote Control lines do not reach the loop. *Basis:* a phase seat runs `agy` in a tmux pane (E4). The plugin commands run only in the skills-publish command (E8). Probe: `evolve doctor probe agy` after the next `evolve cli update`.
- **A3** — The loop signs in to agy as a person and sets no `GEMINI_API_KEY` or `GOOGLE_API_KEY`. *Basis:* the manifest guards against both variables (E4). Probe: the same check after the next release.
- **A4** — A changelog line is practice evidence only when it changes what a phase prompt can rely on. *Basis:* hypothesis H1 of the scout report (E5). Probe: none needed.

## Evidence

- **E1** — Source: the command `agy changelog`, run in cycle 2011 on 2026-10-20. `agy --version` printed 1.4.3. The output lists releases 1.4.3, 1.4.2, 1.4.1, 1.4.0 and 1.3.6.
- **E2** — E1 line counts for 1.4.3: 3 lines start with Added or Improved, and 14 start with Fixed. Total 17.
- **E3** — E1 lines of 1.4.3 that touch run behavior. One line says the compaction summary now keeps "only each subagent's type, role, and result". One line says the agent now creates empty files such as `__init__.py` and writes regular project files when the model sends blank artifact details.
- **E4** — `go/internal/bridge/manifests/agy.json:59` and `go/internal/bridge/manifests/agy-tmux.json:74-90`. The manifest guards against `GEMINI_API_KEY` and `GOOGLE_API_KEY`, and it tracks a per-model "Gemini 4 Argon" limit. The loop sets no agy hook in a phase seat. The driver passes `--dangerously-skip-permissions`. The manifest `prompt_marker` is `? for shortcuts`.
- **E5** — `.evolve/runs/cycle-2011/scout-report.md`: the changelog text is untrusted, and the likely outcome is no persona change.
- **E6** — `docs/architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md` and `docs/plans/model-capability-practice-2026-10.md:97` (D7, D8): one persona per change, four measures over 10 cycles before and after.
- **E7** — `./go/bin/evolve prompt stats --help` printed `evolve: unknown command "prompt"` on 2026-10-20. `docs/plans/model-capability-practice-2026-10.md:110-111`: M2 records the baseline, and M3 is the first diet change.
- **E8** — `go/cmd/evolve/cmd_skills_publish.go:491` runs `agy plugin validate` and `:497` runs `agy plugin install`. Both run only in the operator's skills-publish command, not in a phase seat.
