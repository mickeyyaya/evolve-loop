# Comment history: `internal/releaseconsistency`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d11

### `go/internal/releaseconsistency/releaseconsistency.go:1` — above `package releaseconsistency`

```text
// Package releaseconsistency ports the version-consistency-check half of
// legacy/scripts/utility/release.sh.
//
// Verifies all version markers are consistent with the target version:
//
//	.claude-plugin/plugin.json        json "version" field
//	.claude-plugin/marketplace.json   json "version" field
//	.codex-plugin/plugin.json         json "version" field (generated mirror; skipped if absent)
//	skills/loop/SKILL.md       `# Evolve Loop vX.Y` heading
//	README.md                          `Current (vX.Y)` cell
//	CHANGELOG.md                       contains `[<target>]` entry
//	README.md history                  contains `v<major.minor>` row
//
// The plugin-cache-refresh half of release.sh (~100 lines of marketplace
// pull + python registry update) is intentionally NOT ported — it's
// environment-specific and removed entirely in v12.0.0 when legacy/
// vanishes. The cache refresh is also covered by `evolve marketplace-poll`
// for the in-pipeline flow.
//
// Exit codes (cmd layer maps from ErrInconsistent):
//
//	0 = all markers consistent with target
//	1 = at least one inconsistency (ErrInconsistent)
```
