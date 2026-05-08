# Evolve-Loop Release Protocol

> Canonical vocabulary, lifecycle, and runbook for releasing evolve-loop versions. Authoritative as of v8.13.2.

## Why this document exists

The /insights audit (cycle 8200 onward) flagged "publish" as ambiguous: sometimes interpreted as "push commits," sometimes as "create a versioned release," sometimes as "make installed plugins update." Each meaning maps to a different actual operation, and confusing them caused stale-marketplace incidents (e.g., users seeing v8.2.0 long after v8.6 was tagged). v8.13.2 introduces a single declarative entry point (`bash scripts/release-pipeline.sh <version>`) and this document defines what each verb means.

## Vocabulary

| Term | Operation | Reversible? |
|------|-----------|-------------|
| **push** | `git push origin <branch>` — fast-forwards a remote ref. Pure git. | Hard (force-push only). |
| **tag** | `git tag vX.Y.Z <sha>` — annotated tag at a commit. | Yes (`git tag -d` + `git push origin :refs/tags/vX.Y.Z`). |
| **release** | `gh release create vX.Y.Z` — creates a GitHub Release object with notes, tied to a tag. | Yes (`gh release delete vX.Y.Z`). |
| **propagate** | Marketplace checkouts (`~/.claude/plugins/marketplaces/evolve-loop/`) `git pull` the new tag, then Claude Code's `installed_plugins.json` registry refreshes. | N/A (eventually consistent; verifiable). |
| **publish** | Composite atomic operation: pre-flight → bump → changelog → audit-bound ship → propagate-verify → rollback-on-fail. | Yes (auto-rollback if propagation fails or post-push gh-release fails). |
| **ship** | DEPRECATED informal alias for "push." Use **publish** for new releases. `bash scripts/lifecycle/ship.sh` remains the gate-allowlisted atomic primitive that publish calls internally. | — |

**Rule of thumb:** when an operator says "publish", they almost always mean *the full pipeline*, not just `git push`. When in doubt, use `bash scripts/release-pipeline.sh <version>`.

## Architecture

```
                      bash scripts/release-pipeline.sh <version>
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

Every component supports `--dry-run`. The orchestrator plumbs the flag through.

## Lifecycle (full publish)

| # | Step | Script | Failure → action |
|---|------|--------|------------------|
| 1 | Pre-flight | `scripts/release/preflight.sh` | exit 1; abort before any mutation |
| 2 | Auto-changelog | `scripts/release/changelog-gen.sh <prev-tag> HEAD <version>` | exit 1; abort |
| 3 | Version bump | `scripts/release/version-bump.sh <version>` | exit 1; abort |
| 4 | Consistency check | `scripts/utility/release.sh <version>` | exit 1; abort (no commits made yet — bumped files left in working tree, operator can investigate) |
| 5 | Atomic ship | `scripts/lifecycle/ship.sh "release: vX.Y.Z"` | exit 2; abort (nothing pushed) |
| 6 | Marketplace poll | `scripts/release/marketplace-poll.sh <version> --max-wait-s 300` | exit 3; auto-rollback (deletes release + tag, reverts commit) unless `--no-rollback` |
| 7 | Cache refresh | `scripts/utility/release.sh <version>` (re-run) | logged WARN; manual `bash scripts/utility/release.sh <version>` |

## Runbook

### Routine release

```bash
bash scripts/release-pipeline.sh 8.13.3
```

Performs the full lifecycle. Default deadline for marketplace propagation is 300 seconds; auto-rollback is on; tests run in pre-flight.

### Dry-run (recommended for first releases of the day)

```bash
bash scripts/release-pipeline.sh 8.13.3 --dry-run
```

Simulates every step, mutates nothing. Verifies that:
- working tree is clean
- target version is a valid bump
- audit ledger has a recent PASS
- gate-test suites would have run
- changelog would generate (prints the proposed block)
- version markers would update
- ship.sh would run with the right release notes
- marketplace-poll would target the right dir

### Hot-fix flow (skip gate-test execution)

```bash
bash scripts/release-pipeline.sh 8.13.3 --skip-tests
```

Tests already verified in CI; pre-flight skips step 5. Use sparingly — logged WARN.

### Manual rollback (when auto-rollback was disabled)

```bash
ls .evolve/release-journal/   # find the most recent journal
bash scripts/release/rollback.sh .evolve/release-journal/8.13.3-20260427T160000Z.json --reason "manual"
```

The journal records what got pushed; rollback uses it to know what to undo.

### Just verify marketplace propagation (no publish)

```bash
bash scripts/release/marketplace-poll.sh 8.13.3 --max-wait-s 60
```

Useful when investigating "is my installed plugin out of date?" — polls the marketplace checkout against an expected version.

## CHANGELOG entry format

`changelog-gen.sh` produces Keep-a-Changelog-style sections from conventional commits:

| Commit prefix | Section |
|---------------|---------|
| `feat:` / `feature:` / `feat(scope):` | `### Added` |
| `fix:` / `bugfix:` / `fix(scope):` | `### Fixed` |
| `refactor:` / `perf:` / `performance:` / `stability:` / `techdebt:` | `### Changed` |
| `docs:` / `documentation:` | `### Documentation` |
| `chore:` / `ci:` / `test:` / `build:` / `style:` / `revert:` / `meta:` / `release:` | (skipped) |
| no prefix | `### Other` (audit found ~40% of commits go here) |

If a `## [<version>]` block already exists in CHANGELOG.md, the generator preserves it (idempotent skip — assume a human curated it).

## Conventional-commits guide

When writing commits during normal development:

| Goal | Subject prefix |
|------|----------------|
| New user-visible capability | `feat:` |
| Bug fix | `fix:` |
| Internal refactor with no user change | `refactor:` |
| Performance improvement | `perf:` |
| Documentation only | `docs:` |
| Tooling, CI, deps | `chore:` (won't appear in changelog) |
| Test additions | `test:` (won't appear in changelog) |
| Reverting a prior commit | `revert:` (won't appear in changelog; `release:` either) |

Scope syntax (optional): `feat(auth): add OAuth flow`. The scope is stripped in the generated changelog.

## Marketplace topology

```
   /Users/<user>/.claude/plugins/marketplaces/evolve-loop/   ← marketplace checkout (git clone)
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

Propagation lag = time between `git push` finishing and the marketplace checkout having pulled. Typically near-instant for the same machine; minutes if multiple machines or sleeping clients. The 5-minute default in `marketplace-poll.sh` covers all reasonable cases; bump with `--max-wait-s 600` for slow networks.

## Trust boundary integration

The release pipeline runs **on top of** the v8.13.0/v8.13.1 trust-boundary gates:

- **ship-gate** denies any `git commit` / `git push` / `gh release create` not via `scripts/lifecycle/ship.sh`. The pipeline calls ship.sh (allowed). Direct `git push` from Claude Code is denied — even from the pipeline's own session.
- **role-gate** denies Edit/Write outside the active phase's path allowlist when a cycle is in progress. The pipeline doesn't run during cycles; it runs at release time when `cycle-state.json` is absent → role-gate is transparent passthrough.
- **phase-gate-precondition** enforces Scout→Builder→Auditor sequence. Doesn't apply to the release pipeline (no `subagent-run.sh` invocations).

The pipeline's audit-binding is enforced by ship.sh internally: a recent Auditor PASS verdict bound to current HEAD + tree-state. preflight.sh re-checks this at step 1 to fail fast before any mutation.

## Bypasses (emergency only — every bypass is logged WARN)

| Env var | Purpose |
|---------|---------|
| `EVOLVE_BYPASS_SHIP_GATE=1` | Lets a non-`ship.sh` command emit ship verbs (typically `git push origin main` for merging a tagged release back to main). |
| `EVOLVE_BYPASS_SHIP_VERIFY=1` | Lets `ship.sh` push without an audit-binding match. Used internally by `rollback.sh` because the original audit no longer matches a reverted HEAD. |
| `EVOLVE_BYPASS_ROLE_GATE=1` | Lets Edit/Write happen outside the per-phase path allowlist. |
| `EVOLVE_BYPASS_PHASE_GATE=1` | Lets `subagent-run.sh` invoke any agent regardless of cycle-state phase. |

Routinely setting any of these is a CLAUDE.md violation. The pipeline never sets them itself except for `EVOLVE_BYPASS_SHIP_VERIFY=1` inside `rollback.sh` — a documented and tested code path.

## Common failure modes

| Symptom | Diagnosis | Recovery |
|---------|-----------|----------|
| `preflight: target X not greater than current Y` | You forgot to update `--cycle` arg, or the version bump was already applied. | Run `cat .claude-plugin/plugin.json` to check current; pick a higher target. |
| `preflight: most recent audit-report.md does not declare 'Verdict: PASS'` | Last audit was WARN/FAIL, or you haven't run an audit recently. | Spawn an audit: `bash scripts/dispatch/subagent-run.sh auditor <cycle> <workspace>`. |
| `marketplace-poll: TIMEOUT` | Marketplace checkout didn't pull within --max-wait-s. Possible causes: network lag, marketplace dir corrupted, push didn't actually land. | Check `git -C ~/.claude/plugins/marketplaces/evolve-loop log --oneline | head -3`. If origin/main has the new commit but checkout doesn't, `git -C <dir> pull --ff-only`. |
| `rollback: PARTIAL` | Some rollback step (release-delete, tag-delete, revert) failed. | `cat .evolve/release-rollbacks.jsonl` shows which step. Manually finish (e.g., `gh release delete vX.Y.Z` if release-delete failed). |

## Out of scope (deferred to v8.13.3+)

- CDN-based marketplace propagation (current is git-based local).
- Cross-machine cache invalidation.
- Auto-incrementing semver from commit types (`feat:` → minor, `fix:` → patch).
- Pre-release / RC channels (`vX.Y.Z-rc1`).
- Slack/email notifications on rollback.
