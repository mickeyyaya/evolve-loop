# internal/releaseconsistency

## Purpose

`internal/releaseconsistency` is the Go port of the version-consistency half of `legacy/scripts/utility/release.sh`. It checks that every version marker agrees with the target version.

## Design

- **Markers.** Seven markers must agree with the target.
  - The JSON `version` field of `.claude-plugin/plugin.json` and `.claude-plugin/marketplace.json`.
  - The same field of `.codex-plugin/plugin.json`, a generated mirror.
  - The `# Evolve Loop vX.Y` heading of `skills/loop/SKILL.md`.
  - The `Current (vX.Y)` cell of `README.md`.
  - A `[<target>]` entry in `CHANGELOG.md`.
  - A `v<major.minor>` row in the README history.
- **Canonical version.** `Target` is optional. When empty, the version comes from `plugin.json`.
- **Generated mirror.** `checkJSONVersionIfPresent` returns OK with "absent (skipped)" when `.codex-plugin/plugin.json` does not exist, because the mirror is not present in every checkout. When it exists it must match the target. A release that forgot to bump the Codex surface fails here and rolls back, and does not ship drift.
- **Check order.** The first five checks are version markers. `contentMarkerStart` is the index of the first required-content check (the CHANGELOG entry, then the README history row), where the report prints a separate heading.
- **Status values.** `OK`, `MISSING`, `NO_MATCH` and `MISMATCH`. Each non-OK status counts as one inconsistency.
- **Exit codes.** The cmd layer maps `ErrInconsistent` to exit code 1, and a clean run to 0.
- **Not ported.** The plugin-cache refresh half of `release.sh` is environment-specific and goes away with `legacy/`. `evolve marketplace-poll` covers the cache refresh in the pipeline.

## Invariants

- A marker that disagrees with the target gives `ErrInconsistent`. Pinned by `releaseconsistency_test.go`.

## Findings

- None open.
