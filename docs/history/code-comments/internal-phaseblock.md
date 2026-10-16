# Comment history: `internal/phaseblock`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d11

### `go/internal/phaseblock/digest.go:1` — above `package phaseblock`

```text
// Package phaseblock models each pipeline phase as a content-addressed,
// independently verifiable "agent block" (ADR-0065). A phase's Digest covers
// the binary that ran it, that binary's build-commit (provenance), the agent
// profile/prompt, the phase's report artifact, and the worktree tree — chained
// to the previous phase's Combined. A chain of digests replaces the single
// pipeline-level binary pin as the integrity record.
//
// This package is a leaf: it depends only on the stdlib (plus the injected
// DigestSource / Provenance seams), never on core/ship/git, so it stays
// trivially unit-testable and cycle-free.
```
