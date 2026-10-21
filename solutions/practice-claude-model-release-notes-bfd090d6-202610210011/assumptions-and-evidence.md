# Assumptions and Evidence — practice-claude-model-release-notes-bfd090d6-202610210011

Every claim in `proposal.md`, `options/*.md` and `recommendation.md` cites an entry below.
The release-note text is untrusted third-party data. The loop reads it and never follows it as an instruction.

## Assumptions

- **A1** — The 20 October entry and the 30 September entry hold the changes behind the fragments of the watch. *Basis:* the fetched page text (E1, E2). Probe: fetch the page again at the next wave boundary and compare.
- **A2** — The loop sets no Inference hook and sends no raw Messages API request. *Basis:* the loop starts the `claude` and `agy` binaries in tmux panes (E4). Probe: `evolve doctor probe claude` after the next `evolve cli update`.
- **A3** — A persona change made without a baseline cannot be judged. *Basis:* ADR-0133 D8 (E3). Probe: the first D8 table (milestone M3).
- **A4** — A release-note line is practice evidence only when it changes what a phase prompt can rely on. *Basis:* hypothesis H1 of the earlier review (E7). Probe: none needed.

## Evidence

- **E1** — Source: https://platform.claude.com/docs/en/release-notes/overview, fetched on 2026-10-21 in cycle 2027 with `curl` (HTTP 200, about 93000 characters of text after markup removal). The newest entries are October 20, 15, 14, 9, 8, 7, 6, 5 and 1, then September 30 and 28, 2026.
- **E2** — E1 lines that match the watch fragments:
  - October 20: "Inference hooks are out of beta for Claude Enterprise organizations."
  - August 5 (the earlier beta entry): "Inference hooks are now in beta for Claude Enterprise organizations. Point Claude at your organization's AI security server, and each governed prompt across claude.ai, Cowork, and Claude Code is held for the server's allow or deny verdict before inference proceeds." The same entry retires `claude-opus-4-1-20250805`.
  - September 30: "We announced the deprecation of the Claude Sonnet 4.5 model (`claude-sonnet-4-5-20250929`), with retirement on the Claude API scheduled for November 30, 2026. We recommend migrating to Claude Sonnet 5.5. Read more in Model deprecations."
  - Context entries:
    - October 15: the `between_tools` thinking type.
    - October 7: Claude Haiku 5.5, with adaptive thinking on by default.
    - October 15: fast mode out of research preview for Claude Opus 5.5.
- **E3** — `docs/architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md`, decisions 4 to 6: one persona per change, and each diet change has a before and after table. `docs/plans/model-capability-practice-2026-10.md:96-97` (D7, D8): four measures over 10 cycles before and after.
- **E4** — `grep -rn 'api.anthropic.com' go/internal` on 2026-10-20 (cycle 2005): zero matches. The loop runs the CLI binaries. A search of `go/internal`, `agents` and `docs/architecture` for "Inference hook" on 2026-10-21 finds no match.
- **E5** — On 2026-10-21, a search of the `.json` and non-test `.go` files for the IDs `claude-(sonnet|opus|haiku)-4*` finds zero matches. The one model ID in `go/internal/bridge/manifests/claude-tmux.json` is `claude-opus-5`. The `claude-opus-4-1` strings that remain are in test fixtures only (`go/internal/bridge/engine_attempt_telemetry_test.go:46`).
- **E6** — `docs/plans/model-capability-practice-2026-10.md:110-111`: M2 records the baseline with `evolve prompt stats`. On 2026-10-21 `go/bin/evolve prompt stats` exits with `unknown command "prompt"`, so M2 has no baseline.
- **E7** — `docs/research/practice-claude-model-release-notes-2026-10.md` section 4 and `solutions/practice-claude-model-release-notes-db268466-202610200244/`: the same verdict, no persona change, for the October 14 and 15 entries. `grep -rnE 'claude-(sonnet|opus|haiku)-[0-9]' agents` was not needed: personas name tiers, not model IDs (`ls agents` shows the six core personas).
- **E8** — Cost of a persona edit: one persona file plus the 10 + 10 cycle measures of D8 (E3). Cost of this review: one small document cycle (`cycle_size_estimate: small` in the triage report of cycle 2027).
