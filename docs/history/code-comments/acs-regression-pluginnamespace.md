# Comment history: `acs/regression/pluginnamespace`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/regression/pluginnamespace/pluginnamespace_test.go:3` — above `package pluginnamespace`

```text
// Package pluginnamespace is the durable regression guard that locks in the
// 2026-06-24 plugin/command namespace rename evolve-loop → evo.
//
// In Claude Code the slash-command namespace IS the plugin's `name` field, so
// .claude-plugin/plugin.json and .claude-plugin/marketplace.json are the single
// source of truth for whether commands surface as /evo:loop, /evo:tdd, … (the
// /ecc:prune pattern). If anyone reverts the name in either manifest — or lets
// the two disagree — the /evo:* namespace silently breaks at install time. The
// rename itself touched 87 files, but only these two fields actually drive the
// namespace; everything else is consistency. This gate pins the field that
// matters and the agreement between the two manifests.
//
// acs-tagged like every go/acs/regression predicate; CI runs it via
//
//	go test -count=1 -tags acs ./acs/regression/...
//
// It needs no .apicover-enforce / completeness enrollment: a test-only package
// outside ./internal/... (exactly like acs/regression/noorphan, flagreaders).
```
