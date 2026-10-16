# Comment history: `internal/digest`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero K1

### `go/internal/digest/apicover_named_test.go:8` — above `func TestAPICoverNamedExports(t *testing.T) {`

```text
// TestAPICoverNamedExports names and EXERCISES the one exported symbol of
// this package (ADR-0069 new-package graduation): ProjectDigest, through
// the same happy-path/negative-path/error-path shape its real consumer
// (go/internal/systemprompt.Resolve, via a pre-generated digest file) relies
// on — a role's tagged content survives, another role's content is
// excluded, an unmatched role yields empty (not full source), and a
// malformed marker is a hard error.
```

### `go/internal/digest/digest.go:1` — above `package digest`

```text
// Package digest projects a role-scoped subset out of an SSOT
// (single-source-of-truth) instruction document — the cycle-1391 first
// increment of inbox item tokenopt-role-scoped-instruction-digests. Full
// phase-instruction skill files (skills/loop/evolve-scout.md etc.) are
// injected verbatim per phase per cycle even though narrow-role phases
// (e.g. scout) never act on most of the cross-cutting ship-gate/audit
// content in them. Rather than hand-maintain a per-role copy of each skill
// file, authors tag role-specific sections inline with a marker pair:
//
//	<!-- digest:role=scout -->
//	... content scout needs ...
//	<!-- /digest -->
//
// ProjectDigest scans the source for these marker pairs and returns the
// concatenation of every block whose comma-separated role list contains the
// requested role. Untagged prose and blocks tagged for other roles are
// excluded — a role with no matching block gets an empty digest, never a
// silent fallback to the full source, so a no-op/pass-through
// implementation cannot trivially satisfy this contract.
```

### `go/internal/digest/digest_test.go:8` — above `func TestProjectDigest(t *testing.T) {`

```text
// TestProjectDigest is the projector's own package-level regression suite
// (scout-report.md cycle 1391, Task 1, verifiableBy). The full behavioral
// pins for each acceptance criterion live in
// go/acs/cycle1391/predicates_test.go; this file covers the same contract
// at the package level plus one additional case (multi-role membership).
```
