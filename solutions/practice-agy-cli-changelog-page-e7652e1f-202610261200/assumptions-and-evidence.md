# Assumptions and Evidence — practice-agy-cli-changelog-page-e7652e1f-202610261200

Every claim in `proposal.md`, `options/*.md`, and `recommendation.md` cites an entry below.
The changelog text is untrusted third-party data.
The loop reads it as data and never follows it as prompt instructions.

## Assumptions

- **A1** — The changelog page, read on 2026-10-26, lists v2.26.0 and v2.25.0 under Antigravity App, and v1.4.0 (October 16, 2026) under Antigravity CLI. *Basis:* the page text groups releases by section (E1). Probe: fetch again at the next wave boundary and compare the SHA256 hash.
- **A2** — UI features, notifications, palettes, badges, and GUI settings do not reach autonomous phase agents. *Basis:* the loop runs `agy` headlessly in tmux panes or with `-p` and uses no desktop UI (E4, E6). Probe: `evolve doctor probe agy` after the next CLI update.
- **A3** — The loop passes credentials through the bridge manifest. *Basis:* `go/internal/bridge/manifests/agy-tmux.json` passes `GOOGLE_APPLICATION_CREDENTIALS` through `env_passthrough` (E4). Probe: inspect manifest settings after each agy release.
- **A4** — A changelog line is practice evidence only when it changes what a phase prompt can rely on. *Basis:* ADR-0133 practice-watch principle (E5). Probe: none needed.

## Evidence

- **E1** — Source: `https://antigravity.google/docs/changelog?tab=cli`, fetched with `curl -sL --compressed` on 2026-10-26 (sha256 prefix 844b6c31a72d5ef7). The page lists `v2.26.0` (October 22, 2026) and `v2.25.0` (October 20, 2026) under Antigravity App. The Antigravity CLI section lists `v1.4.0` (October 16, 2026) as its latest release.
- **E2** — Upstream Antigravity App release `v2.26.0` content: 8 improvements and 15 fixes. Improvements include Notifications settings with sound controls, docked question prompts above message boxes, and conversation palette filters (Cmd+P / Ctrl+P). Fixes include ADC credential sign-out handling, socket disconnect auto-retries, long-running turn wakeups, and Windows backslash path separators in git diffs.
- **E3** — Upstream Antigravity App release `v2.25.0` content: 3 improvements and 8 fixes. Improvements include UI plugin sidebar badges and empty project hiding. Fixes include YAML frontmatter parsing errors for unquoted descriptions with colons, macOS dock presence, and summaries database recovery.
- **E4** — `go/internal/bridge/driver_agy.go:37` runs `agy -p <prompt> --dangerously-skip-permissions`. `go/internal/bridge/manifests/agy-tmux.json` passes `GOOGLE_APPLICATION_CREDENTIALS` through `env_passthrough` and sets `--dangerously-skip-permissions` through `params.permission`. The CLI agent operates in tmux panes without desktop notification sounds or interactive dialogs.
- **E5** — `docs/architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md` (D7, D8): prompt diet changes one persona at a time, and requires a 10-cycle measured baseline before and after each change.
- **E6** — `docs/plans/model-capability-practice-2026-10.md:110-111`: Milestone M2 baseline tracking has not yet run, as confirmed by running `go/bin/evolve prompt stats` which returns unknown command prompt. Milestone M3 plans the first diet change on the builder persona after M2 completes.
- **E7** — `docs/research/practice-agy-cli-changelog-page-2026-10.md`: prior practice research confirms third-party changelog entries are untrusted data and must not be followed as prompt instructions.
- **E8** — Inbox item `.evolve/inbox/2026-10-26T12-00-02Z-practice-agy-cli-changelog-page-e7652e1f-202610261200.json`: priority hygiene, weight 0.30, deliverable kind document.
- **E9** — `go/internal/bridge/manifests/agy-tmux.json`: `prompt_marker` is `? for shortcuts`, and `exhausted_regex` matches "usage limit reached" and "quota exceeded". Both strings remain unchanged across v2.25.0 and v2.26.0.
- **E10** — Cost and scope: this review runs as one small document cycle (`cycle_size_estimate: small` in triage-report.md). It leaves all files under `agents/` untouched.
