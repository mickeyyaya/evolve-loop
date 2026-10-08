---
name: verify-release
description: Use when the user invokes /evo:verify-release or asks to check whether a release has propagated, whether the marketplace is up to date, or whether installed plugins reflect the latest version. Wraps `evolve marketplace-poll` for standalone post-publish verification.
argument-hint: "<target-version> [--max-wait-s 60] [--marketplace-dir <path>]"
---

# /evo:verify-release

> Standalone post-publish propagation check. It polls the local marketplace checkout until the checkout shows the target version. Use it after a manual `git push` to confirm that the new version landed. Use it also when a user asks "is my installed plugin out of date?"

## What this skill does

The skill runs `evolve marketplace-poll` (`go/internal/marketplacepoll`). On each poll, the command does these steps:

1. If the marketplace directory is a git checkout, run `git fetch origin main` and `git reset --hard origin/main` in it.
2. Read the `version` field of `~/.claude/plugins/marketplaces/evo/.claude-plugin/plugin.json`.
3. Compare that version with the target version. When they match, exit 0.

The command does not refresh `installed_plugins.json`. That refresh called `release.sh`, and the Go migration removed the script. The step is now a no-op (`DefaultReleaseSh` returns nil), but the command still prints a `running release.sh` log line.

The slash command translates to:

```bash
"$CLAUDE_PROJECT_DIR/go/evolve" marketplace-poll <args>
```

## Invocation

```bash
/evo:verify-release 8.13.4                    # default: poll up to 5 min, 15s interval
/evo:verify-release 8.13.4 --max-wait-s 60    # shorter deadline (faster diagnostic)
/evo:verify-release 8.13.4 --poll-interval-s 5 # tighter loop, faster convergence detection
/evo:verify-release 8.13.4 --dry-run          # print the poll parameters; do not pull
```

## When to use this skill

- **After a manual ship** that did not use `/evo:publish` (for example, a hot fix through `evolve ship --class manual`). The marketplace does not pull by itself. This skill makes sure that it caught up.
- **Stale-plugin reports.** If a user says "I'm running v8.13.2 but the marketplace shows v8.13.1," run `/evo:verify-release 8.13.2` to force a marketplace pull.
- **After a `git push origin main`** that used the explicit `evolve guard ship --bypass` emergency path.

## When NOT to use this skill

- **During an in-flight `/evo:publish` run.** The pipeline already polls internally. Running this concurrently could race against the pipeline's poll loop.
- **For non-evo marketplaces.** This skill targets the evo marketplace specifically. Override the default path with `--marketplace-dir <path>` if you need to point elsewhere.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | The marketplace converged on the target version |
| 1 | TIMEOUT: marketplace did not converge within `--max-wait-s` |
| 2 | Runtime error (no marketplace directory, no `plugin.json`, or a target version that is not semver) |
| 10 | Bad arguments |

## Implementation note

This skill is a thin wrapper around `evolve marketplace-poll` (`go/internal/cli/opscmd/marketplace_poll.go`). The skill passes each flag to the command without change.

## Related

- [/evo:publish](../publish/SKILL.md) — full release pipeline
- [docs/guides/publishing-releases.md](../../docs/guides/publishing-releases.md) — vocabulary and topology
