# Evolve-Loop Release Protocol

> This document gives the canonical vocabulary, the lifecycle and the runbook to release evolve-loop versions. It describes the release flow of v22.27.0. The sources are [skills/publish/SKILL.md](../../skills/publish/SKILL.md) and `go/internal/releasepipeline/`.

## Why this document exists

The /insights audit (cycle 8200 onward) found that "publish" had more than one meaning.
Sometimes it meant "push commits," sometimes "create a versioned release," and sometimes "make installed plugins update."
Each meaning is a different operation.
When people mixed them up, stale-marketplace incidents occurred (for example, users saw v8.2.0 long after the v8.6 tag).

v8.13.2 added a single declarative entry point. Today that entry point is the native Go pipeline, `evolve release <version>`, and `/evo:publish` wraps it.
This document defines the meaning of each verb.

## Vocabulary

| Term | Operation | Reversible? |
|------|-----------|-------------|
| **push** | `git push origin <branch>` moves a remote ref forward (fast-forward). Pure git. | Hard (force-push only). |
| **tag** | `git tag vX.Y.Z <sha>` puts an annotated tag at a commit. | Yes (`git tag -d` + `git push origin :refs/tags/vX.Y.Z`). |
| **release** | `gh release create vX.Y.Z` creates a GitHub Release object with notes. The object is tied to a tag. | Yes (`gh release delete vX.Y.Z`). |
| **propagate** | Marketplace checkouts (`~/.claude/plugins/marketplaces/evo/`) do a `git pull` of the new tag. Then the `installed_plugins.json` registry of Claude Code refreshes. | N/A (eventually consistent; verifiable). |
| **publish** | A composite atomic operation: pre-flight → changelog → bump → ship → propagate-verify → release-verify → rollback-on-fail. | Yes (auto-rollback if a step after the push fails). |
| **ship** | The per-commit primitive: `evolve ship --class manual`, `cycle` or `release`. Publish calls `evolve ship --class release` internally. | — |

**Rule of thumb:** when an operator says "publish", they almost always mean *the full pipeline*, not only `git push`. If you are not sure, use `/evo:publish <version>`.

## The release flow

`/evo:publish X.Y.Z` wraps `evolve release X.Y.Z`. The pipeline uses `gh` in two places:

- Pre-flight reads the newest `required.yml` run of `HEAD`. A red run, or a run that is not complete, blocks the release. If `gh` is not available or no run is visible, the check is only advisory.
- The ship step runs `gh release create`.

The pipeline does not watch CI after the push. The skill adds the CI checks before and after the run. To check the readiness only, use `/evo:release`: it runs the read-only checks and calls `/evo:publish` when they pass.

1. Make sure that the base `required CI` is green on `origin/main`:

   ```bash
   gh run list --workflow required.yml --branch main --limit 1 --json headSha,status,conclusion,url
   ```

   The run must have `headSha` equal to `git rev-parse origin/main`, `status` equal to `completed` and `conclusion` equal to `success`. If not, stop.
2. Make sure that every LLM CLI installs and that the binary answers every core subcommand:

   ```bash
   evolve release-verify-clis
   ```

   It must exit 0. If a row is not `OK`, stop and fix the target that the row names.
3. Do a dry run. It changes nothing:

   ```bash
   /evo:publish X.Y.Z --dry-run
   ```

4. Do the real run:

   ```bash
   /evo:publish X.Y.Z
   ```

5. After the run, watch the CI of the release commit and check the release assets:

   ```bash
   evolve ci watch --tag "vX.Y.Z"
   gh release view --json assets -q '.assets[].name' | grep -q '\.tar\.gz$'
   ```

   `evolve ci watch --tag` waits for the `required.yml` and `release.yml` workflows on the commit of the tag. Exit 0 means that both are green, exit 1 means that one is red, and exit 2 means that a run was not seen. The `grep` makes sure that the prebuilt `.tar.gz` binaries are on the release.

If `required.yml` is red after the release, do not roll back a propagated release. Fix forward: land the fix on `main`, then publish the next patch.

If `release.yml` is red, or the binaries are not on the release, the release stays a prerelease. Fix `.goreleaser.yml` or `release.yml`.

If the tag is correct, run `evolve release-promote vX.Y.Z --rerun`. It runs the failed jobs again and waits for them. Then it checks the workflow and the assets, and it makes the release a full release again. If you already ran the workflow again, run `evolve release-promote vX.Y.Z` without `--rerun`. If the tag is not correct, publish the next patch.

## Architecture

```
                           evolve release <version>
                                       │
        ┌──────────────────────────────┼──────────────────────────────┐
        │                              │                              │
   pre-flight gate              changelog generation            version bump
   (evolve release-preflight)   (evolve changelog-gen)          (evolve version-bump)
        │                              │                              │
        └──────────────────────────────┼──────────────────────────────┘
                                       │
                     rebuild go/evolve with the version stamp
                                       │
                     consistency check (evolve release-consistency)
                                       │
                                       ▼
                 evolve ship --class release ── atomic commit + push + gh release
                                       │
                                       ▼
                    evolve marketplace-poll ── poll up to 300 s
                                       │
                                       ▼
          release verify ── the committed go/evolve is the disk copy and reports the version
                                       │
                                       ▼
                       (success exit 0)   ──or──   evolve rollback
                                                   (deletes release + tag,
                                                    reverts the commit and ships
                                                    the revert with evolve ship --class manual)
```

`--dry-run` goes to each step. In a dry run, the pipeline does not rebuild, check the consistency, ship, poll or verify.

## Lifecycle (full publish)

| # | Step | Command | Failure → action |
|---|------|---------|------------------|
| 0 | Full dry-run preflight (only with `--require-preflight`) | the full dry-run harness | exit 1; abort before any change |
| 1 | Pre-flight | `evolve release-preflight` | exit 1; abort before any change |
| 2 | Auto-changelog | `evolve changelog-gen` (from the previous tag to `HEAD`) | exit 1; abort |
| 3 | Version bump | `evolve version-bump` (the 6 version markers, with the README history row) | exit 1; abort |
| 4 | Binary rebuild | `go build` of `go/evolve` with the version stamp | exit 1; abort |
| 5 | Consistency check | `evolve release-consistency <version>` | exit 1; abort (no commits yet — the bumped files stay in the working tree, and the operator can examine them) |
| 6 | Atomic ship | `evolve ship --class release` | exit 2; abort (nothing pushed) |
| 7 | Marketplace poll | `evolve marketplace-poll <version> --max-wait-s 300` | exit 3; auto-rollback unless `--no-rollback` |
| 8 | Release verify | the committed `go/evolve` matches the disk copy, reports the target version, and has a local tag | exit 3; auto-rollback unless `--no-rollback` |

Pre-flight runs these seven steps. Steps 1 to 5 are the main checks, step 6 is the CI check, and step 7 is an advisory test suite:

1. The working tree is clean.
2. The branch is attached.
3. The target is greater than the current version.
4. The most recent audit did not fail. A FAIL audit of this release tree blocks. A WARN audit passes, unless you give `--strict-pass`. The step is advisory when no audit is on disk, or when the failed audit is of another tree.
5. The gate-test suites are green, and `evolve names` finds no dead token in the tracked files.
6. The CI check: the newest `required.yml` run of `HEAD` must be green. A red run, or a run that is not complete, blocks. Without `gh`, or with no visible run, the check is only advisory.
7. The auto-respond simulation suite runs. A failure is only a WARN.

The pipeline writes a journal for each run in `.evolve/release-journal/`. A dry run writes its journal in the temp directory (`release-pipeline-dryrun-<version>.json`). At the end, the pipeline prints a note that it did not watch the GitHub CI of the release commit. Step 5 of the release flow above does that check.

| Flag of `evolve release` | What it does |
|---|---|
| `--dry-run` | Simulates the run and changes nothing |
| `--no-rollback` | Does not roll back when a step after the push fails |
| `--skip-tests` | Skips the gate-test suites of pre-flight (hot fixes) |
| `--strict-pass` | A WARN verdict fails pre-flight |
| `--require-preflight` | Runs the full dry-run harness before any step |
| `--max-poll-wait-s N` | The deadline of the marketplace poll (default 300) |
| `--from-tag <tag>` | The start of the changelog range (default: the previous tag) |

## Runbook

### Routine release

Use the release flow above. The core command is:

```bash
evolve release 22.28.0
```

This command does the full lifecycle. The default deadline for marketplace propagation is 300 seconds. Auto-rollback is on. Tests run in pre-flight.

### Dry-run (recommended for first releases of the day)

```bash
evolve release 22.28.0 --dry-run
```

This command simulates every step and changes nothing. A dry run does these checks:

- Pre-flight checks that the target version is a valid bump.
- The changelog step checks the git range. Then it only logs "would prepend". It does not print the proposed block.
- The version bump finds the markers to change, and it writes nothing.

A dry run only logs these checks, and it does not do them:

- the clean working tree and the attached branch;
- the recent audit and the gate-test suites;
- the CI check and the simulation suite.

It also skips the rebuild, the consistency check, the ship, the poll and the release verify.

### Hot-fix flow (skip gate-test execution)

```bash
evolve release 22.28.0 --skip-tests
```

CI already verified the tests, so pre-flight skips its gate-test step. Use this flow only when necessary. The pipeline logs a WARN.

### Manual rollback (when auto-rollback was disabled)

```bash
ls .evolve/release-journal/   # find the most recent journal
evolve rollback .evolve/release-journal/<journal>.json --reason "manual"
```

The journal records what the pipeline pushed. Rollback uses the journal to know what to undo. `evolve rollback` also takes `--dry-run`.

### Just verify marketplace propagation (no publish)

```bash
evolve marketplace-poll 22.28.0 --max-wait-s 60
```

Use this command when you examine the question "is my installed plugin out of date?". It polls the marketplace checkout and compares it with an expected version.

## CHANGELOG entry format

`evolve changelog-gen` (`go/internal/changeloggen`) makes sections in the Keep-a-Changelog style from conventional commits:

| Commit prefix | Section |
|---------------|---------|
| `feat:` / `feature:` / `feat(scope):` | `### Added` |
| `fix:` / `bugfix:` / `fix(scope):` | `### Fixed` |
| `refactor:` / `perf:` / `performance:` / `stability:` / `techdebt:` | `### Changed` |
| `docs:` / `documentation:` / `doc:` | `### Documentation` |
| `chore:` / `ci:` / `test:` / `build:` / `style:` / `revert:` / `meta:` / `release:` | (skipped) |
| another typed prefix | `### Other`, with the type in parentheses |
| no prefix | `### Other` (the audit found that ~40% of commits go here) |

If CHANGELOG.md already has a `## [<version>]` block, the generator keeps it (an idempotent skip: the generator assumes that a human curated it).

## Conventional-commits guide

Use this table when you write commits during usual development:

| Goal | Subject prefix |
|------|----------------|
| New user-visible capability | `feat:` |
| Bug fix | `fix:` |
| Internal refactor with no user change | `refactor:` |
| Performance improvement | `perf:` |
| Documentation only | `docs:` |
| Tooling, CI, deps | `chore:` (not in the changelog) |
| New tests | `test:` (not in the changelog) |
| Revert of an earlier commit | `revert:` (not in the changelog; `release:` also is not) |

The scope syntax is optional: `feat(auth): add OAuth flow`. The generated changelog removes the scope.

## Marketplace topology

```
   /Users/<user>/.claude/plugins/marketplaces/evo/   ← marketplace checkout (git clone)
      .claude-plugin/plugin.json:.version  ← THIS is what evolve marketplace-poll watches
                          │
                          │ git pull (manual or via Claude Code session start)
                          ▼
   github.com/mickeyyaya/evolve-loop:main  ← origin
                          ▲
                          │ the git push of evolve ship
                          │
   <local repo>:main  ← your working copy
```

The propagation lag is the time from the end of `git push` to the pull of the marketplace checkout.
On the same machine, the lag is usually almost zero.
With more than one machine, or with clients that sleep, the lag is some minutes.
The 5-minute default of `evolve marketplace-poll` covers all reasonable cases. For slow networks, increase it with `--max-wait-s 600`.

## Trust boundary integration

The release pipeline runs **on top of** the kernel guards. Claude Code runs them as PreToolUse hooks:

- **The ship guard** (`evolve guard ship`) denies each `git commit`, `git push` or `gh release create` that does not go through `evolve ship`.
  The pipeline calls `evolve ship --class release`, which the guard allows.
- **The role guard** (`evolve guard role`) denies Edit/Write outside the path allowlist of the active phase while a cycle is in progress.
  The pipeline does not run during cycles. It runs at release time, when no cycle is active.
- **The phase guard** (`evolve guard phase`) denies the in-process `Agent`/`Task` dispatch while a cycle is active.
  It does not apply to the release pipeline.

The `release` ship class skips the audit binding, because the version bump changes files after the audit.
Thus pre-flight step 4 checks the most recent audit before any change. In some cases, that check is only advisory (see the pre-flight list above).

## Bypasses (emergency only)

| Bypass | Purpose |
|---------|---------|
| `evolve ship --class manual --bypass-commit-gate` | Skips the commit-gate review attestation. The ship logs the skip. |
| `evolve ship --class manual --bypass-prefix-gate` | Skips the commit-prefix gate. |
| `evolve guard phase --bypass` | The operator override of the phase guard. The guard allows the call and writes no special log line. |

The old `EVOLVE_BYPASS_SHIP_GATE`, `EVOLVE_BYPASS_SHIP_VERIFY`, `EVOLVE_BYPASS_ROLE_GATE` and `EVOLVE_BYPASS_PHASE_GATE` variables are removed. The code ignores them.
If you use a bypass as a routine, you violate CLAUDE.md. The pipeline never uses one itself.
`evolve rollback` ships its revert commit with `evolve ship --class manual` and `EVOLVE_SHIP_AUTO_CONFIRM=1`.

## Common failure modes

| Symptom | Diagnosis | Recovery |
|---------|-----------|----------|
| `preflight: target X not greater than current Y` | The target is not greater than the current version, or the version bump is already applied. | Run `cat .claude-plugin/plugin.json` to see the current version. Select a higher target. |
| `preflight: most recent audit-report.md does not declare 'Verdict: PASS'` | The last audit of this release tree was FAIL, or it was WARN and you gave `--strict-pass`. With no audit on disk, the step is only advisory. | Run a fresh audit cycle (`evolve loop`), or `evolve subagent run auditor <cycle> <workspace>`. |
| `marketplace-poll: TIMEOUT` | The marketplace checkout did not pull in the --max-wait-s time. Possible causes: network lag, a corrupted marketplace dir, or a push that did not land. | Check `git -C ~/.claude/plugins/marketplaces/evo log --oneline | head -3`. If origin/main has the new commit and the checkout does not, run `git -C <dir> pull --ff-only`. |
| `rollback: PARTIAL` | A rollback step (release-delete, tag-delete, revert) failed. | `cat .evolve/release-rollbacks.jsonl` shows the step. Finish it manually (for example, `gh release delete vX.Y.Z` if release-delete failed). |
| `SELF_SHA_TAMPERED` on the next ship | The rebuilt binary is pinned, but it is not in the release commit. | See the binary-rebuild procedure in [runtime-reference.md](../operations/runtime-reference.md). |

## Out of scope

- CDN-based marketplace propagation (the current propagation is git-based and local).
- Cross-machine cache invalidation.
- An automatic semver increment from commit types (`feat:` → minor, `fix:` → patch).
- Pre-release / RC channels (`vX.Y.Z-rc1`).
- Slack/email notifications on rollback.
