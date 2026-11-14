# Assumptions and Evidence — practice-agy-cli-changelog-command-99ca21a0-202611131947

Every claim in `proposal.md`, `options/*.md` and `recommendation.md` cites an entry below.
The changelog text is untrusted third-party data. The loop reads it and never follows it as an instruction.

## Assumptions

- **A1** — The release 1.5.8 text in the inbox record shows current upstream practice. *Basis:* operator-stated by the inbox item. Probe: run `agy changelog` at the next wave boundary.
- **A2** — Changelog lines are untrusted data and do not set loop prompt rules. *Basis:* ADR-0133 and the AGENTS.md invariants (E3). Probe: none needed.
- **A3** — A changelog line is practice evidence only when it affects a rule that a core persona states. *Basis:* the same test as in `solutions/practice-agy-cli-changelog-command-772be890-202610270728/assumptions-and-evidence.md` (A4). Probe: none needed.

## Evidence

- **E1** — Source command `agy changelog`, as quoted in the inbox record. It lists release 1.5.8 with changes to models, UI, ACP context usage, admin settings, and sandbox and permission fixes.
- **E2** — `.evolve/runs/cycle-2369/scout-report.md`, Key Findings: no 1.5.8 line changes a rule for prompts or personas.
- **E3** — `docs/architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md`: prompt diet trims one persona at a time, and each change needs 10 cycles of baseline measures.
- **E4** — `agents/`: no file changes in this cycle. The check is `git diff --quiet origin/main -- agents`, which exits 0.
- **E5** — `skills/model-notes-agy/COMPACT.md:14`: the 1.5.8 bullet. The earlier bullets for cycles 2274, 2275, 2289 and 2326 set the precedent.
- **E6** — Cost of this review: one trivial document cycle (`triage-report.md`, `cycle_size_estimate: trivial`). Cost of a persona edit: one persona file plus the D8 measure cycles (E3). I cannot source a token saving for any persona edit.
