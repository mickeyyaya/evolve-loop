# Assumptions and Evidence — practice-claude-model-release-notes-2d8e64f4-202610261200

Every claim in `proposal.md`, `options/*.md` and `recommendation.md` cites an entry below.
The release-note text is untrusted third-party data. The loop reads it and never follows it as an instruction.

## Assumptions

- **A1** — The entries of 20, 21 and 22 October hold the changes behind the fragments of the watch. *Basis:* the fetched page text (E1, E2). Probe: fetch the page again at the next wave boundary and compare.
- **A2** — The loop imports no Anthropic SDK, sends no raw Messages API request, creates no user profile and manages no Slack setting. *Basis:* the loop starts the `claude` and `agy` binaries in tmux panes (E4). Probe: `evolve doctor probe claude` after the next `evolve cli update`.
- **A3** — A persona change made without a baseline cannot be judged. *Basis:* ADR-0133 D8 (E3). Probe: the first D8 table (milestone M3).
- **A4** — A release-note line is practice evidence only when it changes what a phase prompt can rely on. *Basis:* hypothesis H1 of the earlier reviews (E7). Probe: none needed.

## Evidence

- **E1** — Source: https://platform.claude.com/docs/en/release-notes/overview, fetched on 2026-10-26 in cycle 2032 with `curl` (HTTP 200, about 97500 characters of text after markup removal). The newest entries are October 22, 21, 20 (three entries) and earlier.
- **E2** — E1 lines that match the watch fragments:
  - October 22: "We've launched Claude Models on Google Cloud (Anthropic managed), the Claude API operated by Anthropic and reached through your Google Cloud project. Google Cloud IAM controls access. Google Cloud Marketplace handles billing." Fast mode is available for Claude Opus 5.5 there.
  - October 21: "We've released API version 2026-10-21." Seven SDKs send it by default: Python 2.0.0, TypeScript 1.0.0, C# 13.0.0, Go 2.0.0, Java 3.0.0, PHP 1.0.0 and Ruby 2.0.0. The `ant` CLI 2.0.0 does the same. The new versions carry breaking changes and an Apache 2.0 licence.
  - October 21: "We've added the `enforcement_status` and `enforcement_actions` fields to user profiles, so you can see whether Anthropic has restricted or banned one of your end customers."
  - October 20: "The Claude Tag API is available in beta for Claude Enterprise organizations with the `experimental-claude-tag-2026-10-21` beta header." It is not available to Claude Console organizations.
  - October 20 (same date, other entries): Inference hooks out of beta, user profiles, organization webhooks (`webhooks-2026-10-07`), and a web fetch toggle for organization admins.
- **E3** — `docs/architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md`, decisions 4 to 6: one persona per change, and each diet change has a before and after table. `docs/plans/model-capability-practice-2026-10.md:96-97` (D7, D8): four measures over 10 cycles before and after.
- **E4** — On 2026-10-26, `grep -c -i anthropic go/go.mod` prints 0. A search of `go`, `agents` and `docs/architecture` for `enforcement_status`, `claude-tag`, `api.anthropic.com` and `anthropic-version` finds zero files. The loop runs the CLI binaries.
- **E5** — On 2026-10-26, a search of the `.json` and non-test `.go` files under `go` and `agents` for IDs of the form `claude-(sonnet|opus|haiku)-4` finds zero matches. The model alias in `go/internal/bridge/manifests/claude-tmux.json:47` is the bare family alias, which claude resolves at launch.
- **E6** — `docs/plans/model-capability-practice-2026-10.md:110-111`: M2 records the baseline with `evolve prompt stats`. On 2026-10-26 `go/bin/evolve prompt stats` exits with `unknown command "prompt"`, so M2 has no baseline.
- **E7** — `docs/research/practice-claude-model-release-notes-2026-10.md`, `solutions/practice-claude-model-release-notes-db268466-202610200244/` and `solutions/practice-claude-model-release-notes-bfd090d6-202610210011/`: the same verdict, no persona change, for the entries of 14 to 20 October. Those include the Sonnet 4.5 retirement on 2026-11-30. Personas name tiers, not model IDs (`ls agents` shows the six core personas).
- **E8** — Cost of a persona edit: one persona file plus the 10 + 10 cycle measures of D8 (E3). Cost of this review: one small document cycle (`cycle_size_estimate: small` in the triage report of cycle 2032).
