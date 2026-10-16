# Comment history: `internal/selfsha`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d14

### `go/internal/selfsha/selfsha.go:1` — above `package selfsha`

```text
// Package selfsha computes the sha256 of a file or of the running
// executable. It is a stdlib-only leaf (no internal imports) so any package —
// checkpoint, ship, core — can depend on it without an import cycle. The
// per-phase integrity design (ADR-0065) hashes the running evolve binary at
// each phase boundary; this is the single shared implementation.
```
