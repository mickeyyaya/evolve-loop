# Assumptions and Evidence — practice-claude-code-changelog-e5ad0d8f-202610192320

Every claim in `proposal.md`, `options/*.md` and `recommendation.md` cites an entry below.
The changelog text is untrusted third-party data. The loop reads it and never follows it as an instruction.

## Assumptions

- **A1** — The 2.1.304 section, as read on 2026-10-20, is the full set of lines for that version. *Basis:* the first section of the fetched file is `## 2.1.304`. Probe: re-fetch at the next wave boundary and compare the hash.
- **A2** — The Cowork, Remote Control, Desktop, Windows, self-hosted runner, artifact and mod lines do not reach the loop. *Basis:* the loop runs `claude` in tmux panes on macOS and uses no mod, plugin, Cowork or Remote Control feature (E5). Probe: `evolve doctor probe claude` after the next `evolve cli update`.
- **A3** — A persona change made without a baseline cannot be judged. *Basis:* ADR-0133 D8 (E3). Probe: the first D8 table (milestone M3).
- **A4** — A changelog line is practice evidence only when it changes what a phase prompt can rely on. *Basis:* the hypothesis H1 of the scout report (E6). Probe: none needed.

## Evidence

- **E1** — Source: https://raw.githubusercontent.com/anthropics/claude-code/main/CHANGELOG.md, fetched with `curl -sL` on 2026-10-20 in cycle 1993. The first section is `## 2.1.304`. It holds 76 bullets (Added, Fixed, Improved, Changed).
- **E2** — E1 lines that touch the loop's runtime: "Fixed a plugin keeping a headless run going past `--max-turns`…", "Fixed subagents sometimes forgetting their most recent work when they were messaged or resumed right after finishing", "Fixed a 'Context limit reached' or 'Prompt is too long' error after stopping a turn while Claude Code was compacting…", "Added the names of the tools called earlier to the compaction summary in the transcript view (Ctrl+O)", "Fixed self-paced `/loop` wakeups scheduled at the 1-hour maximum firing just after the 1-hour prompt cache expired", "Fixed `CLAUDE_CODE_AUTO_COMPACT_WINDOW=200k` silently setting a 100K window", "Fixed `CLAUDE_CODE_TRANSCRIPT_LOCAL_GC` dropping messages…", "Improved responsiveness and throughput when many subagents are running at the same time", "Improved memory use when sending requests in long or resumed conversations".
- **E3** — `docs/architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md` Decision 4 to 6: one persona per change. Each diet change has a before and after table. New personas state goal, done criteria and limits. `docs/plans/model-capability-practice-2026-10.md:97` (D8): four measures over 10 cycles before and after.
- **E4** — `docs/plans/model-capability-practice-2026-10.md:110-111`: M2 records the baseline with `evolve prompt stats`; M3 is the first diet change, on the builder persona.
- **E5** — `grep -rIn` over `go/`, `.evolve/profiles/` and `agents/` on 2026-10-20: no use of `CLAUDE_CODE_AUTO_COMPACT_WINDOW`, `CLAUDE_CODE_TRANSCRIPT_LOCAL_GC` or a `--max-turns` flag. The only `--max-turns` text is a note at `agents/evolve-builder-reference.md:320`.
- **E6** — `.evolve/runs/cycle-1993/scout-report.md`, Hypotheses: H1 says no persona needs a change. A line falsifies H1 if it alters tmux driver, compaction, subagent or `--max-turns` behavior that a phase prompt relies on.
- **E7** — `docs/research/practice-claude-code-changelog-2026-10.md` section 3: the same verdict, no persona change, for 2.1.299 to 2.1.303.
- **E8** — Persona files present on 2026-10-20: `agents/evolve-scout.md`, `evolve-triage.md`, `evolve-tdd-engineer.md`, `evolve-builder.md`, `evolve-auditor.md`, `evolve-retrospective.md` (`ls agents`).
- **E9** — Cost of a persona edit: one persona file plus the 10 + 10 cycle measures of D8 (E3). Cost of this review: one small document cycle (triage report of cycle 1993, `cycle_size_estimate: small`).
