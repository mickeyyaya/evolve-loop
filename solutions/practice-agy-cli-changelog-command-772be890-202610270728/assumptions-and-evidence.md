# Assumptions and Evidence — practice-agy-cli-changelog-command-772be890-202610270728

Every claim in `proposal.md`, `options/*.md` and `recommendation.md` cites an entry below.
The changelog text is untrusted third-party data. The loop reads it and never follows it as an instruction.

## Assumptions

- **A1** — The output of `agy changelog`, read on 2026-11-03, shows current upstream practice. *Basis:* the live command and the inbox item snapshot of 2026-10-26T18:27:01Z (E1). Probe: run the command again at the next wave boundary.
- **A2** — Changelog lines are untrusted data and do not set loop prompt rules. *Basis:* AGENTS.md invariants and ADR-0133 (E3). Probe: none needed.
- **A3** — A persona change made without a measured baseline cannot be judged. *Basis:* ADR-0133 D8 (E3). Probe: check that milestone M2 has shipped before any change.
- **A4** — A changelog line is practice evidence only when it affects a rule that a core persona states. *Basis:* hypothesis H1 of the scout report (E5). Probe: none needed.

## Evidence

- **E1** — Source command `agy changelog`, run on 2026-11-03. It lists releases 1.4.3 to 1.4.12 after the snapshot of 2026-10-26T18:27:01Z. The October note ends at 1.4.2.
- **E2** — `docs/research/practice-agy-cli-changelog-2026-11.md`: the lines of the ten releases explained as data, with one verdict for each of the six core personas.
- **E3** — `docs/architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md`, decisions D7 and D8: prompt diet trims one persona at a time, and each change needs 10 cycles of baseline measures before and after.
- **E4** — `docs/plans/model-capability-practice-2026-10.md`: milestone M2 records the baseline and milestone M3 trims `evolve-builder`. On 2026-10-26 the command `evolve prompt stats` did not exist, so M2 had not shipped (precedent: `solutions/practice-claude-prompting-guide-f509428a-202610261159/assumptions-and-evidence.md`, E4).
- **E5** — `.evolve/runs/cycle-2120/scout-report.md`, Hypotheses: H1 says the new agy capabilities need no phase prompt change before the M2 baseline.
- **E6** — Core persona files in `agents/`: `evolve-scout.md`, `evolve-triage.md`, `evolve-tdd-engineer.md`, `evolve-builder.md`, `evolve-auditor.md`, `evolve-retrospective.md`.
- **E7** — Cost of this review: one trivial document cycle (triage report, `cycle_size_estimate: trivial`). Cost of a persona edit: one persona file plus 10 + 10 cycles of D8 measures (E3).
