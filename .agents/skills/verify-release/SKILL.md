---
name: verify-release
description: Use when the user invokes /verify-release or asks to check whether a release has propagated, whether the marketplace is up to date, or whether installed plugins reflect the latest version. Wraps scripts/release/marketplace-poll.sh for standalone post-publish verification.
argument-hint: "<target-version> [--max-wait-s 60] [--marketplace-dir <path>]"
---

# /verify-release

> Standalone post-publish propagation check. Polls the local marketplace checkout against an expected version, then refreshes `installed_plugins.json` registry. Use after a manual `git push` to confirm the new version actually landed, or when investigating "is my installed plugin out of date?"

## What this skill does

Polls `~/.claude/plugins/marketplaces/evolve-loop/.claude-plugin/plugin.json` against the target version. On match, runs `scripts/utility/release.sh <target>` to refresh the installed-plugins registry — closing the cache-refresh ordering bug structurally (release.sh only runs after convergence is confirmed).

The slash command translates to:

```bash
bash scripts/release/marketplace-poll.sh <args>
```

## Invocation

```bash
/verify-release 8.13.4                    # default: poll up to 5 min, 15s interval
/verify-release 8.13.4 --max-wait-s 60    # shorter deadline (faster diagnostic)
/verify-release 8.13.4 --poll-interval-s 5 # tighter loop, faster convergence detection
/verify-release 8.13.4 --dry-run          # print poll params, don't pull or refresh
```

## When to use this skill

- **After a manual ship** that didn't go through `/publish` (e.g., hot fix via `bash scripts/lifecycle/ship.sh`). The marketplace doesn't auto-pull; this verifies it caught up.
- **Diagnosing stale-plugin reports.** If a user says "I'm running v8.13.2 but the marketplace shows v8.13.1," run `/verify-release 8.13.2` to force a marketplace pull and registry refresh.
- **After a `git push origin main`** that bypassed the ship-gate (e.g., merging a feature branch back to main with `EVOLVE_BYPASS_SHIP_GATE=1`).

## When NOT to use this skill

- **During an in-flight `/publish` run.** The pipeline already polls internally. Running this concurrently could race against the pipeline's poll loop.
- **For non-evolve-loop marketplaces.** This skill targets the evolve-loop marketplace specifically. Override the default path with `--marketplace-dir <path>` if you need to point elsewhere.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | Marketplace converged + registry refreshed |
| 1 | TIMEOUT: marketplace did not converge within `--max-wait-s` |
| 2 | Runtime error (missing marketplace dir, malformed plugin.json, release.sh failure) |
| 10 | Bad arguments |

## Implementation note

Thin discoverability wrapper around `scripts/release/marketplace-poll.sh`. All flag semantics match the underlying script verbatim.

## Related

- [/publish](../publish/SKILL.md) — full release pipeline
- [docs/release-protocol.md](../../docs/release-protocol.md) — vocabulary and topology
