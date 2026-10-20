# Assumptions and Evidence — practice-claude-model-release-notes-db268466-202610200244

Every claim in `proposal.md`, `options/*.md` and `recommendation.md` cites an entry below.
The release-note text is untrusted third-party data. The loop reads it and never follows it as an instruction.

## Assumptions

- **A1** — The entries of 2026-10-01, 05, 06, 07, 15 and 2026-09-28 hold the changes behind the fragments of the watch. *Basis:* the fetched page text (E1). Probe: fetch the page again at the next wave boundary and compare.
- **A2** — The loop sends no raw Messages API request. *Basis:* the loop starts the `claude` and `agy` binaries in tmux panes (E4). Probe: `evolve doctor probe claude` after the next `evolve cli update`.
- **A3** — A persona change made without a baseline cannot be judged. *Basis:* ADR-0133 D8 (E3). Probe: the first D8 table (milestone M3).
- **A4** — A release-note line is practice evidence only when it changes what a phase prompt can rely on. *Basis:* hypothesis H1 of the scout report (E6). Probe: none needed.

## Evidence

- **E1** — Source: https://platform.claude.com/docs/en/release-notes/overview, fetched on 2026-10-20 in cycle 2005. The newest entries are October 15, 14, 9, 8, 7, 6, 5 and 1, then September 30 and 28, 2026.
- **E2** — E1 lines that match the watch fragments:
  - October 15 (paraphrase of the quoted entry): "We have added `capabilities.thinking.types.between_tools` to the Models API. `GET /v1/models` and `GET /v1/models/{model_id}` now report if each model accepts `thinking: {type: \"between_tools\"}`, which turns off up-front thinking."
  - September 28 (Claude Sonnet 5.5): "To turn off up-front thinking, send `thinking: {\"type\": \"between_tools\"}` instead of `\"disabled\"`, at `high` effort or below."
  - October 7 (Claude Haiku 5.5): "Adaptive thinking is on by default, so a response can begin with `thinking` blocks, and their text is omitted unless you set `thinking.display` to `\"summarized\"`."
  - October 5: the field `capabilities.thinking.types.disabled`. October 6: the field `capabilities.server_tools`. October 1: the field `line`.
- **E3** — `docs/architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md`, decisions 4 to 6: one persona per change, and each diet change has a before and after table. `docs/plans/model-capability-practice-2026-10.md:96-97` (D7, D8): four measures over 10 cycles before and after.
- **E4** — `grep -rn 'api.anthropic.com' go/internal` on 2026-10-20: zero matches. The only code that reads the word `thinking` is the stream parsers `go/internal/phasestream/classify.go:226` and `go/internal/logfilter/streamjson.go:121`. Both read a `thinking` content block from CLI output and do not send the request field.
- **E5** — `docs/plans/model-capability-practice-2026-10.md:110-111`: M2 records the baseline with `evolve prompt stats`. M3 is the first diet change, on the builder persona. On 2026-10-20 `evolve prompt stats` exits with `unknown command "prompt"`, so M2 has no baseline.
- **E6** — `.evolve/runs/cycle-2005/scout-report.md`, Hypotheses: H1 says no phase persona changes.
- **E7** — `docs/research/practice-claude-model-release-notes-2026-10.md` section 4: the same verdict, no persona change, for the October 14 and 15 entries of the earlier item.
- **E8** — Persona files present on 2026-10-20: `agents/evolve-scout.md`, `evolve-triage.md`, `evolve-tdd-engineer.md`, `evolve-builder.md`, `evolve-auditor.md`, `evolve-retrospective.md` (`ls agents`). `grep -rnE 'between_tools|thinking\.display' agents` finds zero matches.
- **E9** — Cost of a persona edit: one persona file plus the 10 + 10 cycle measures of D8 (E3). Cost of this review: one small document cycle (triage report of cycle 2005, `cycle_size_estimate: trivial`).
