# Comment history: `internal/commitprefixgate`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero D4

### `go/internal/commitprefixgate/commitprefixgate.go:1` — above `package commitprefixgate`

```text
// Package commitprefixgate ports legacy/scripts/guards/commit-prefix-gate.sh.
//
// Layer 1 of the Reward-Hacking Defense System (ADR-0012). Verifies that
// the commit-message prefix matches the diff scope declared in
// .evolve/commit-prefix-scope.json. Rejects mislabeled commits with rc=2.
//
// Closes the cycle-70/72/75 mislabeling pattern (feat() commits that were
// actually role-gate fixes or pure-docs).
//
// Exit codes (cmd layer maps from sentinel errors):
//
//	0 = prefix matches scope (or unknown prefix = pass-through)
//	2 = scope violation (ErrScopeViolation)
//	3 = bad arguments (ErrBadArgs)
//	4 = manifest missing or malformed (ErrBadManifest)
//
// Bash 3.2 glob semantics are reproduced — case-statement style matching
// where `*` matches arbitrary characters including `/`.
```

### `go/internal/commitprefixgate/manifest_realpaths_test.go:49` — above `func TestManifestRequiredPaths_ResolveToRealTree(t *testing.T) {`

```text
// TestManifestRequiredPaths_ResolveToRealTree guards against the dead-path
// regression the 2026-06-22 doc↔impl audit found (T1.2): after the script→Go
// migration deleted legacy/scripts/ and root acs/, every required_paths glob in
// .evolve/commit-prefix-scope.json still pointed there, so feat(guards) /
// feat(routing) / feat(audit) / feat(posthoc) / feat(token-opt) commits could
// NEVER satisfy Rule 1 ("≥1 diff path must match") — they became un-shippable
// without --bypass-prefix-gate. Every glob's anchor dir must exist in the tree.
```
