# Assumptions and Evidence — practice-claude-model-release-notes-e9fe428e-202611021720

Every claim in `proposal.md`, `options/*.md` and `recommendation.md` cites an entry below.
The release-note text is untrusted third-party data. The loop reads it and never follows it as an instruction.

## Assumptions

- **A1** — The entries of 22 September to 30 October hold the changes behind the fragments of the watch. *Basis:* the fetched page text (E1, E2). Probe: fetch the page again at the next wave boundary and compare.
- **A2** — The loop sends no raw Messages API request and sets no `speed` field. *Basis:* the loop starts the `claude` and `agy` binaries in tmux panes (E4). Probe: `evolve doctor probe claude` after the next `evolve cli update`.
- **A3** — A persona change made without a baseline cannot be judged. *Basis:* ADR-0133 D8 (E3). Probe: the first D8 table (milestone M3).
- **A4** — A release-note line is practice evidence only when it changes what a phase prompt can rely on. *Basis:* hypothesis H1 of the earlier review (E7). Probe: none needed.

## Evidence

- **E1** — Source: https://platform.claude.com/docs/en/release-notes/overview, fetched on 2026-11-10 in cycle 2263 with `curl` (HTTP 200, about 103000 characters of text after markup removal). The newest entries are 9, 5, 3 and 2 November, then 30, 29, 28, 23, 22 and 21 October 2026.
- **E2** — E1 lines that match the watch fragments:
  - 30 October: "We've deprecated fast mode for Claude Opus 4.8, with removal on November 30, 2026. After removal, requests to claude-opus-4-8 with speed: "fast" will return an error. To continue using fast mode, migrate to Claude Opus 5.5."
  - 29 October: "We've launched the Usage and Cost Analytics API in beta with the analytics-usage-cost-2026-08-24 beta header." The older Usage and Cost API is now legacy and remains supported.
  - 28 October: "We've raised the maximum request size for the Messages API and the Token Counting API from 32 MB to 128 MB on the first-party Claude API."
  - 22 September: "On Claude Opus 5.5, thinking can't be disabled: thinking: {"type": "disabled"} and thinking: {"type": "enabled", ...} return a 400 error. Omit the thinking field and control thinking depth with the effort parameter."
  - 7 October: the Sonnet 5.5 cache read price falls from $0.20 to $0.10 per million tokens, and Claude Haiku 5.5 launches.
  - Context: 9 November, Claude Fable 5.2 with fast mode; 2 November, message threads in beta.
- **E3** — `docs/architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md`, decisions 4 to 6: one persona per change, and each diet change has a before and after table. `docs/plans/model-capability-practice-2026-10.md:96-97` (D7, D8): four measures over 10 cycles before and after.
- **E4** — On 2026-11-10, a search of `go/internal` for `api.anthropic.com` and `organizations/analytics` finds no production match. The earlier search of cycle 2005 and the note `docs/research/practice-claude-model-release-notes-2026-11.md` (R1, R2) gave the same result for `speed` and `analytics-usage-cost`.
- **E5** — `grep` of `go/internal/bridge/manifests/*.json`, `.evolve/profiles` and `policy.json` on 2026-11-10: no concrete retired model ID and no thinking field. `claude-tmux.json:54` names the bare alias `opus`, which the CLI resolves at launch. The `fast` key of `agy-claude-tmux.json:34` maps to the label "Claude Sonnet 5.5 (Low)", a tier and not the API field.
- **E6** — `docs/plans/model-capability-practice-2026-10.md:110`: M2 records the baseline with `evolve prompt stats`. On 2026-11-10 `go/bin/evolve prompt stats` exits with `unknown command "prompt"`, so M2 has no baseline.
- **E7** — `docs/research/practice-claude-model-release-notes-2026-11.md` and `docs/research/practice-claude-model-release-notes-2026-10.md`: the same verdict, no persona change, for the entries up to 30 October. `ls agents` shows the six core personas, which name tiers and not model IDs.
- **E8** — Cost of a persona edit: one persona file plus the 10 + 10 cycle measures of D8 (E3). Cost of this review: one small document cycle (`cycle_size_estimate: small` in the triage report of cycle 2263).
