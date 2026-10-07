# Evolve-Loop Release Protocol

> This document gives the canonical vocabulary, the lifecycle and the runbook to release evolve-loop versions. It is authoritative as of v8.13.2.

## Why this document exists

The /insights audit (cycle 8200 onward) found that "publish" had more than one meaning.
Sometimes it meant "push commits," sometimes "create a versioned release," and sometimes "make installed plugins update."
Each meaning is a different operation.
When people mixed them up, stale-marketplace incidents occurred (for example, users saw v8.2.0 long after the v8.6 tag).
v8.13.2 adds a single declarative entry point (`bash legacy/scripts/release-pipeline.sh <version>`).
This document defines the meaning of each verb.

## Vocabulary

| Term | Operation | Reversible? |
|------|-----------|-------------|
| **push** | `git push origin <branch>` moves a remote ref forward (fast-forward). Pure git. | Hard (force-push only). |
| **tag** | `git tag vX.Y.Z <sha>` puts an annotated tag at a commit. | Yes (`git tag -d` + `git push origin :refs/tags/vX.Y.Z`). |
| **release** | `gh release create vX.Y.Z` creates a GitHub Release object with notes. The object is tied to a tag. | Yes (`gh release delete vX.Y.Z`). |
| **propagate** | Marketplace checkouts (`~/.claude/plugins/marketplaces/evo/`) do a `git pull` of the new tag. Then the `installed_plugins.json` registry of Claude Code refreshes. | N/A (eventually consistent; verifiable). |
| **publish** | A composite atomic operation: pre-flight → bump → changelog → audit-bound ship → propagate-verify → rollback-on-fail. | Yes (auto-rollback if propagation fails or if the post-push gh-release fails). |
| **ship** | A DEPRECATED informal alias for "push." Use **publish** for new releases. `bash legacy/scripts/lifecycle/ship.sh` stays the gate-allowlisted atomic primitive. Publish calls it internally. | — |

**Rule of thumb:** when an operator says "publish", they almost always mean *the full pipeline*, not only `git push`. If you are not sure, use `bash legacy/scripts/release-pipeline.sh <version>`.

## Architecture

```
                      bash legacy/scripts/release-pipeline.sh <version>
                                       │
        ┌──────────────────────────────┼──────────────────────────────┐
        │                              │                              │
   pre-flight gate              version bump                  changelog generation
   (preflight.sh)              (version-bump.sh)            (changelog-gen.sh)
        │                              │                              │
        └──────────────────────────────┼──────────────────────────────┘
                                       │
                            release.sh consistency-check
                                       │
                                       ▼
                       ship.sh ── atomic commit + push + gh release
                       (audit-bound; ship-gate enforced)
                                       │
                                       ▼
                          marketplace-poll.sh ── poll up to 5 min
                                       │
                                       ▼
                      release.sh refresh installed_plugins.json
                                       │
                                       ▼
                       (success exit 0)   ──or──   rollback.sh
                                                   (deletes release+tag,
                                                    creates revert commit
                                                    via EVOLVE_BYPASS_SHIP_VERIFY=1)
```

Every component supports `--dry-run`. The orchestrator sends the flag through to each component.

## Lifecycle (full publish)

| # | Step | Script | Failure → action |
|---|------|--------|------------------|
| 1 | Pre-flight | `legacy/scripts/release/preflight.sh` | exit 1; abort before any mutation |
| 2 | Auto-changelog | `legacy/scripts/release/changelog-gen.sh <prev-tag> HEAD <version>` | exit 1; abort |
| 3 | Version bump | `legacy/scripts/release/version-bump.sh <version>` | exit 1; abort |
| 4 | Consistency check | `legacy/scripts/utility/release.sh <version>` | exit 1; abort (no commits yet — the bumped files stay in the working tree, and the operator can examine them) |
| 5 | Atomic ship | `legacy/scripts/lifecycle/ship.sh "release: vX.Y.Z"` | exit 2; abort (nothing pushed) |
| 6 | Marketplace poll | `legacy/scripts/release/marketplace-poll.sh <version> --max-wait-s 300` | exit 3; auto-rollback (deletes release + tag, reverts commit) unless `--no-rollback` |
| 7 | Cache refresh | `legacy/scripts/utility/release.sh <version>` (re-run) | logged WARN; manual `bash legacy/scripts/utility/release.sh <version>` |

## Runbook

### Routine release

```bash
bash legacy/scripts/release-pipeline.sh 8.13.3
```

This command does the full lifecycle. The default deadline for marketplace propagation is 300 seconds. Auto-rollback is on. Tests run in pre-flight.

### Dry-run (recommended for first releases of the day)

```bash
bash legacy/scripts/release-pipeline.sh 8.13.3 --dry-run
```

This command simulates every step and changes nothing. It verifies that:
- the working tree is clean
- the target version is a valid bump
- the audit ledger has a recent PASS
- the gate-test suites can run
- the changelog can generate (the command prints the proposed block)
- the version markers can update
- ship.sh can run with the correct release notes
- marketplace-poll targets the correct dir

### Hot-fix flow (skip gate-test execution)

```bash
bash legacy/scripts/release-pipeline.sh 8.13.3 --skip-tests
```

CI already verified the tests, so pre-flight skips step 5. Use this flow only when necessary. The pipeline logs a WARN.

### Manual rollback (when auto-rollback was disabled)

```bash
ls .evolve/release-journal/   # find the most recent journal
bash legacy/scripts/release/rollback.sh .evolve/release-journal/8.13.3-20260427T160000Z.json --reason "manual"
```

The journal records what the pipeline pushed. Rollback uses the journal to know what to undo.

### Just verify marketplace propagation (no publish)

```bash
bash legacy/scripts/release/marketplace-poll.sh 8.13.3 --max-wait-s 60
```

Use this command when you examine the question "is my installed plugin out of date?". It polls the marketplace checkout and compares it with an expected version.

## CHANGELOG entry format

`changelog-gen.sh` makes sections in the Keep-a-Changelog style from conventional commits:

| Commit prefix | Section |
|---------------|---------|
| `feat:` / `feature:` / `feat(scope):` | `### Added` |
| `fix:` / `bugfix:` / `fix(scope):` | `### Fixed` |
| `refactor:` / `perf:` / `performance:` / `stability:` / `techdebt:` | `### Changed` |
| `docs:` / `documentation:` | `### Documentation` |
| `chore:` / `ci:` / `test:` / `build:` / `style:` / `revert:` / `meta:` / `release:` | (skipped) |
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
      .claude-plugin/plugin.json:.version  ← THIS is what marketplace-poll watches
                          │
                          │ git pull (manual or via Claude Code session start)
                          ▼
   github.com/mickeyyaya/evolve-loop:main  ← origin
                          ▲
                          │ ship.sh's git push
                          │
   <local repo>:main  ← your working copy
```

The propagation lag is the time from the end of `git push` to the pull of the marketplace checkout.
On the same machine, the lag is usually almost zero.
With more than one machine, or with clients that sleep, the lag is some minutes.
The 5-minute default in `marketplace-poll.sh` covers all reasonable cases. For slow networks, increase it with `--max-wait-s 600`.

## Trust boundary integration

The release pipeline runs **on top of** the v8.13.0/v8.13.1 trust-boundary gates:

- **ship-gate** denies each `git commit` / `git push` / `gh release create` that does not go through `legacy/scripts/lifecycle/ship.sh`.
  The pipeline calls ship.sh (allowed). The gate denies a direct `git push` from Claude Code, also from the pipeline's own session.
- **role-gate** denies Edit/Write outside the path allowlist of the active phase while a cycle is in progress.
  The pipeline does not run during cycles. It runs at release time, when `cycle-state.json` is absent.
  Then role-gate is a transparent passthrough.
- **phase-gate-precondition** enforces the Scout→Builder→Auditor sequence.
  It does not apply to the release pipeline (the pipeline does not call `subagent-run.sh`).

ship.sh enforces the audit-binding of the pipeline internally: a recent Auditor PASS verdict that is bound to the current HEAD + tree-state.
preflight.sh checks this again at step 1, to fail fast before any mutation.

## Bypasses (emergency only — every bypass is logged WARN)

| Env var | Purpose |
|---------|---------|
| `EVOLVE_BYPASS_SHIP_GATE=1` | Lets a command that is not `ship.sh` send ship verbs. The usual example is `git push origin main`, to merge a tagged release back to main. |
| `EVOLVE_BYPASS_SHIP_VERIFY=1` | Lets `ship.sh` push without an audit-binding match. `rollback.sh` uses it internally, because the original audit does not match a reverted HEAD. |
| `EVOLVE_BYPASS_ROLE_GATE=1` | Lets Edit/Write occur outside the per-phase path allowlist. |
| `EVOLVE_BYPASS_PHASE_GATE=1` | Lets `subagent-run.sh` start any agent, independent of the cycle-state phase. |

If you set any of these as a routine, you violate CLAUDE.md. The pipeline never sets them itself.
The only exception is `EVOLVE_BYPASS_SHIP_VERIFY=1` inside `rollback.sh`, which is a documented and tested code path.

## Common failure modes

| Symptom | Diagnosis | Recovery |
|---------|-----------|----------|
| `preflight: target X not greater than current Y` | You did not update the `--cycle` arg, or the version bump is already applied. | Run `cat .claude-plugin/plugin.json` to see the current version. Select a higher target. |
| `preflight: most recent audit-report.md does not declare 'Verdict: PASS'` | The last audit was WARN/FAIL, or you did not run an audit recently. | Start an audit: `bash legacy/scripts/dispatch/subagent-run.sh auditor <cycle> <workspace>`. |
| `marketplace-poll: TIMEOUT` | The marketplace checkout did not pull in the --max-wait-s time. Possible causes: network lag, a corrupted marketplace dir, or a push that did not land. | Check `git -C ~/.claude/plugins/marketplaces/evo log --oneline | head -3`. If origin/main has the new commit and the checkout does not, run `git -C <dir> pull --ff-only`. |
| `rollback: PARTIAL` | A rollback step (release-delete, tag-delete, revert) failed. | `cat .evolve/release-rollbacks.jsonl` shows the step. Finish it manually (for example, `gh release delete vX.Y.Z` if release-delete failed). |

## Out of scope (deferred to v8.13.3+)

- CDN-based marketplace propagation (the current propagation is git-based and local).
- Cross-machine cache invalidation.
- An automatic semver increment from commit types (`feat:` → minor, `fix:` → patch).
- Pre-release / RC channels (`vX.Y.Z-rc1`).
- Slack/email notifications on rollback.
