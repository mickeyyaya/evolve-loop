# Comment history: `cmd/evolve-fake-cli`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/cmd/evolve-fake-cli/main.go:353` — above `out[mainPath] = fmt.Sprintf("# Audit Report\n\n## Verdict\n**%s**\n\nSynthetic %s verdict.\n"+`

```text
// Emit BOTH the prose heading and the machine-readable sentinel: at
// EVOLVE_PHASE_IO=enforce (the default since the 3.10 cutover) the sentinel
// is mandatory for the audit verdict parse, so a prose-only fake report
// would fail audit and the happy-path pipeline would never reach ship.
// The explanation-review audit gate (validateExplanationReview) records
// the review's shape as advisories since ADR-0102 and still blocks on a
// missing reasoning or a missing delivery: when the contract is active
// the audit report must independently review the Build handoff. The
// synthetic build declares NOT_APPLICABLE (no material diff), so the
// faithful review is VERIFIED with Build status not_applicable, no
// Document fields, and concrete >=20-char Evidence.
```
