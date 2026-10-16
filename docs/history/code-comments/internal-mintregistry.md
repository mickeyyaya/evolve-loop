# Comment history: `internal/mintregistry`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d13

### `go/internal/mintregistry/mintregistry.go:1` — above `package mintregistry`

```text
// Package mintregistry is the shared per-project record of advisor-minted
// phase names — the cross-lane channel that lets the tree-diff guard
// distinguish a concurrent lane's sanctioned mint of
// .evolve/phases/<name>/phase.json from a real deliverable leak (the
// cycle-967 false-abort: lane-970's mint was charged to lane-967's PASS
// scout because both lanes diff the SAME shared tree).
//
// The registry is written ONLY by the registrar (the trust-kernel clamp that
// persists the mint itself) and read by the guard. It is anchored to the
// PROJECT root, not the configured evolve dir, because the guard only sees
// writes under ProjectRoot — an exemption anchored anywhere else could never
// match a leaked path.
```
