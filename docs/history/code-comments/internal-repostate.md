# Comment history: `internal/repostate`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero K1

### `go/internal/repostate/tracked.go:1` — above `package repostate`

```text
// Package repostate answers one question for repo-wide scanners: which files
// under a directory does git actually TRACK? (ADR-0084 invariant 1.)
//
// Problem. Several guard suites (profiles, phasespec, phasecoherence) and the
// ship-time persona-lint scan walk on-disk config dirs (.evolve/profiles,
// .evolve/phases) that the RUNTIME also mints untracked stubs into. A scanner
// that binds everything on disk reds on state that can never reach a CI
// checkout — the 2026-08-09 zero-ship batch (fingerprint cd49274beab2) and
// the v22.13.0 release red were both this class
// (docs/incidents/2026-08-09-zero-ship-batch.md). The per-name .gitignore
// ratchet that previously protected the scanners re-arms on every new mint.
//
// Contract. TrackedSet returns the basenames (extension stripped by the
// caller's filter) of files git tracks DIRECTLY under relDir — nested entries
// are excluded so a nested tracked file can never basename-alias a same-named
// top-level stub. Includes index-staged (uncommitted) files: the stricter
// direction, no plane-vs-CI skew. On any git failure the error carries git's
// stderr; callers MUST fall back to binding every on-disk file (strict) and
// MUST treat an unexpectedly empty set as a failure — going dark unbinds the
// gate (see phasecoherence/unpaired_test.go for the reference call site).
```
