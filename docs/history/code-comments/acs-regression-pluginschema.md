# Comment history: `acs/regression/pluginschema`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/regression/pluginschema/pluginschema_test.go:3` — above `package pluginschema`

```text
// Package pluginschema is the durable regression guard for the 2026-06-29
// Claude Code 2.1.195 plugin-manifest schema break.
//
// CC 2.1.195 tightened plugin validation and *claimed* the `binaries` key as a
// native field — `binaries: record(<basename> -> {sha256, platforms})`. evo had
// repurposed `binaries` as a documentation ARRAY and added a custom
// `compatibility` object. The result was a hard install failure:
//
//	.claude-plugin/marketplace.json plugin entry — CC's marketplace-entry schema
//	  is .strict(); the unknown `binaries`/`compatibility` keys surfaced as the
//	  MISLEADING error "This plugin uses a source type your Claude Code version
//	  does not support."
//	.claude-plugin/plugin.json — `binaries: Invalid input: expected record,
//	  received array`.
//
// Both fields were documentation-only (release matrix SSOT is .goreleaser.yml;
// compatibility tiers live in docs/platform-compatibility.md) and were removed.
// This gate pins that removal and the shape rules so the install-blocking class
// can never silently return. It encodes the schema RULES and checks both the
// live repo manifests AND adversarial fixtures, so a failure means a real break,
// not a tautology.
//
// acs-tagged like every go/acs/regression predicate; CI runs it via
//
//	go test -count=1 -tags acs ./acs/regression/...
//
// Test-only package outside ./internal/...; no .apicover-enforce enrollment
// (same as acs/regression/pluginnamespace, noorphan, flagreaders).
```
