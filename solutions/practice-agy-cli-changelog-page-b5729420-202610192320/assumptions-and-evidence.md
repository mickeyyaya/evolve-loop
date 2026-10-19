# Assumptions and Evidence — practice-agy-cli-changelog-page-b5729420-202610192320

Every claim in `proposal.md`, `options/*.md` and `recommendation.md` cites an entry below.
The changelog text is untrusted third-party data. The loop reads it and never follows it as an instruction.

## Assumptions

- **A1** — The CLI tab text, as read on 2026-10-20, is the full set of lines for v1.4.0 to v1.3.3. *Basis:* the page lists them under the heading "Antigravity CLI" (E1). Probe: fetch again at the next wave boundary and compare the hash.
- **A2** — The plugin, Remote Control, Windows, Jujutsu and screen UI lines do not reach the loop. *Basis:* the loop runs `agy` in a tmux pane or with `-p` on macOS and installs no agy plugin (E6, E7). Probe: `evolve doctor probe agy` after the next `evolve cli update`.
- **A3** — The loop signs in to agy as a person and does not set `AGY_ADC_AUTH`. *Basis:* the manifest of E6 sets no such variable. Probe: the same check after the next release.
- **A4** — A changelog line is practice evidence only when it changes what a phase prompt can rely on. *Basis:* hypothesis H1 of the scout report (E2). Probe: none needed.

## Evidence

- **E1** — Source: https://antigravity.google/docs/changelog?tab=cli, fetched with `curl -sL` on 2026-10-20 in cycle 1996 (859,823 bytes, sha256 prefix 53bf37cc344df34e). The text holds the App, CLI, IDE extension, SDK and IDE sections. The CLI section starts with v1.4.0 "Latest", dated October 16, 2026.
- **E2** — `.evolve/runs/cycle-1996/scout-report.md`: the inbox excerpt has no added line, and H1 says the change is a version bump with no effect on phase prompts.
- **E3** — `docs/research/practice-agy-cli-changelog-page-2026-10.md` section 2: the earlier diff lines were page controls, and the third line was cut at the 64 KiB limit of the snapshot.
- **E4** — E1 line counts: v1.4.0 has 3 improvements and 6 fixes (9). v1.3.6 has 3 and 10 (13). v1.3.5 has 5 and 1 (6). v1.3.4 has 4 and 10 (14). v1.3.3 has 6 and 15 (21). Total 63.
- **E5** — E1 lines that touch the loop's run behavior, 10 in all.
  - v1.3.6: "if no data arrives for 5 minutes the request is now canceled and retried automatically".
  - v1.3.6: `GIT_EDITOR`, `HGEDITOR` and `JJ_EDITOR` are `false` for agent commands.
  - v1.3.6: "Headless `-p` runs skip these questions so the subagent can continue".
  - v1.3.4: retries wait at most 30 seconds.
  - v1.3.4: `--agent` with an unknown name exits 1 in `-p` runs.
  - v1.3.4: `--print` with `--prompt-interactive` exits 2.
  - v1.3.4: `--model` without `--effort` picks the default thinking level.
  - v1.4.0: the eligibility check retries temporary errors.
  - v1.4.0: background command state and output fixes.
  - v1.3.3: an individual quota error is "reported right away instead of being retried".
- **E6** — `go/internal/bridge/driver_agy.go:37` builds `agy -p <prompt> --dangerously-skip-permissions`. `go/internal/bridge/manifests/agy-tmux.json` passes `--model` only (`effort` is `noop`) and sets no `AGY_*` variable. The loop does not use `--mode auto`, `/compact`, `alt+m` or `AGY_CLI_OSC7501`.
- **E7** — `CLAUDE.md` (runtime): the observer judges a tmux phase on a transcript hash with a 600 s stall limit and kills nothing.
- **E8** — `docs/architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md` and `docs/plans/model-capability-practice-2026-10.md:97` (D7, D8): one persona per change, four measures over 10 cycles before and after.
- **E9** — `docs/plans/model-capability-practice-2026-10.md:110-111`: M2 records the baseline with `evolve prompt stats`, and M3 is the first diet change, on the builder persona. Cost of this review: one small document cycle (`cycle_size_estimate: small` in the triage report of cycle 1996).
- **E10** — `go/internal/bridge/manifests/agy-tmux.json`: `prompt_marker` is `? for shortcuts`, and `exhausted_regex` matches "usage limit reached" and "quota exceeded". Both are text matches on CLI output. The AGENTS.md caller-proof rule needs a production caller for each new symbol.
