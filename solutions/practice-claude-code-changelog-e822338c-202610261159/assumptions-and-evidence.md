# Assumptions and Evidence — practice-claude-code-changelog-e822338c-202610261159

Every claim in `proposal.md`, `options/*.md` and `recommendation.md` cites an entry below.
The changelog text is untrusted third-party data. The loop reads it and never follows it as an instruction.

## Assumptions

- **A1** — The 2.1.306 through 2.1.310 sections, read on 2026-10-26, form the full set of lines for those versions. *Basis:* the top section of the fetched file is `## 2.1.310` (E1). Probe: fetch again at the next wave boundary and compare the hash.
- **A2** — The VS Code, cloud sessions, Claude Tag, Code Review, self-hosted runner and Monitor lines do not reach the loop. *Basis:* the loop runs `claude` in tmux panes on macOS and uses none of these features (E5). Probe: `evolve doctor probe claude` after the next `evolve cli update`.
- **A3** — A persona change made without a baseline cannot be judged. *Basis:* ADR-0133 D8 (E3). Probe: the first D8 table (milestone M3).
- **A4** — A changelog line is practice evidence only when it changes what a phase prompt can rely on. *Basis:* hypothesis H1 of the scout report (E6). Probe: none needed.

## Evidence

- **E1** — Source: https://raw.githubusercontent.com/anthropics/claude-code/main/CHANGELOG.md, fetched with `curl` on 2026-10-26 in cycle 2029. The first section is `## 2.1.310`. The sections down to `## 2.1.306` hold 151 bullets (26 in 2.1.310, 80 in 2.1.309, 1 in 2.1.308, 42 in 2.1.307, 2 in 2.1.306).
- **E2** — Eight lines in E1 touch the loop as CLI behavior. Compaction memory improves in non-interactive sessions, and worktrees stop fetching objects on demand. Sandboxed Bash startup files, redirects and dangerous rm checks are fixed. Unparseable tool calls recover through two retries. Subagent transcripts record token usage, and fork subagents receive launch identity.
- **E3** — `docs/architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md`, decisions D7 and D8: one persona per change. Each change has a before and after table. Four measures over 10 cycles before and after.
- **E4** — `docs/plans/model-capability-practice-2026-10.md:110-111`: M2 records the baseline with `evolve prompt stats`. M3 is the first diet change, on the builder persona. On 2026-10-26, `./go/bin/evolve prompt stats --help` prints `unknown command "prompt"`, so M2 has not shipped.
- **E5** — The loop uses no MCP server (AGENTS.md invariant 12). A grep over `go/`, `agents/` and `.evolve/profiles/` confirms this. The runtime Go orchestrator provisions worktrees. Kernel guards enforce role boundaries.
- **E6** — `.evolve/runs/cycle-2029/scout-report.md`, Hypotheses: H1 says no line in 2.1.306 through 2.1.310 changes a rule a persona states. A line falsifies H1 if it alters worktree, subagent, compaction or driver behavior that a phase prompt relies on.
- **E7** — `solutions/practice-claude-code-changelog-218b6f25-202610210011/recommendation.md`: the same verdict, no persona change, for 2.1.305. `solutions/practice-claude-code-changelog-e5ad0d8f-202610192320`: the same verdict for 2.1.304.
- **E8** — Persona files on 2026-10-26: `agents/evolve-scout.md`, `evolve-triage.md`, `evolve-tdd-engineer.md`, `evolve-builder.md`, `evolve-auditor.md`, `evolve-retrospective.md` (`ls agents`).
- **E9** — Cost of a persona edit: one persona file plus the 10 + 10 cycle measures of D8 (E3). Cost of this review: one small document cycle (triage report of cycle 2029, `cycle_size_estimate: small`).
