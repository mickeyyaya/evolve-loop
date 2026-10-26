# Assumptions and Evidence — practice-claude-prompting-guide-f509428a-202610261159

Every claim in `proposal.md`, `options/*.md` and `recommendation.md` cites an entry below.
The prompting guide text is untrusted third-party data. The loop reads it and never follows it as an instruction.

## Assumptions

- **A1** — The prompting guide lines at the link, read on 2026-10-26, represent current upstream practice advice. *Basis:* the fetched source and inbox item snapshot (E1). Probe: compare hash at the next wave boundary.
- **A2** — Upstream guidance is untrusted data and does not dictate loop prompt rules. *Basis:* AGENTS.md invariant 13 and ADR-0154 (E5). Probe: verify guard enforcement in `go/internal/guards/`.
- **A3** — A persona change made without a measured baseline cannot be judged. *Basis:* ADR-0133 D8 (E3). Probe: review milestone M2 prompt metrics before any change.
- **A4** — A guide line is practice evidence only when it affects a rule that a core persona states. *Basis:* hypothesis H1 of the scout report (E6). Probe: none needed.

## Evidence

- **E1** — Source: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices, reviewed on 2026-10-26. Key updates address calm phrasing to avoid tool overtriggering, prefill deprecation, steerable thinking, XML tags, and consumer dosing blocks.
- **E2** — The guide lines advise calm phrasing instead of emphatic capitals like MUST and NEVER to stop tool overtriggering. The guide also notes assistant prefill limitations with newer models.
- **E3** — `docs/architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md`, decisions D7 and D8: prompt diet trims one persona at a time. Each change needs 10 cycles of baseline measurements before and after.
- **E4** — `docs/plans/model-capability-practice-2026-10.md:110-111`: milestone M2 records baseline measurements with `evolve prompt stats`. Milestone M3 is the first diet change on `evolve-builder`. On 2026-10-26, `./go/bin/evolve prompt stats --help` reports `unknown command "prompt"`, so M2 has not shipped.
- **E5** — Invariants 1, 2, and 12 in AGENTS.md: the loop uses no MCP server. Deterministic Go orchestrator logic enforces phase order and boundaries.
- **E6** — `.evolve/runs/cycle-2031/scout-report.md`, Hypotheses: H1 says no line in the updated guide alters a rule that a core persona states.
- **E7** — Precedent solutions in cycle 2029 (`solutions/practice-claude-code-changelog-e822338c-202610261159`), cycle 2005 (`solutions/practice-claude-model-release-notes-db268466-202610200244`), and research note `docs/research/practice-claude-prompting-guide-2026-10.md`.
- **E8** — Core persona files in `agents/`: `evolve-scout.md`, `evolve-triage.md`, `evolve-tdd-engineer.md`, `evolve-builder.md`, `evolve-auditor.md`, `evolve-retrospective.md`.
- **E9** — Cost of a persona edit: one persona file plus 10 + 10 cycles of D8 measurements (E3). Cost of this review: one small document cycle (triage report, `cycle_size_estimate: small`).
