# Comment history: `acs/cycle1457`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1457/predicates_test.go:3` — above `package cycle1457`

```text
// Package cycle1457 materialises the cycle-1457 acceptance criteria for the one
// fleet-scoped task triage committed to this lane:
//
//   - tokenopt-attribution-marker-anchored → tokenusage.attributes() must match
//     the assembler's literal "Artifact path: <path>" marker, not a bare
//     ArtifactPath substring anywhere in the first user message.
//
// The defect. go/internal/tokenusage/scanner.go:209 attributes a transcript to a
// launch with `strings.Contains(firstUserText(lines), w.ArtifactPath)`. Every
// production launch DOES carry the path — but so does any transcript that merely
// MENTIONS it in prose. `.evolve/profiles/retrospective.json:118` instructs a
// phase to "Read .evolve/runs/cycle-{cycle}/build-report.md and
// audit-report.md": under the bare-substring rule that transcript attributes to
// the BUILDER's Window and its tokens are billed to the builder. Anchoring the
// match to the literal marker both assemblers stamp closes the vector.
//
// Predicate strategy — every predicate drives the REAL production entry point
// `tokenusage.ScanConfigRoot` over a real on-disk transcript fixture and asserts
// on the returned Usage/Source (the cycle-85 degenerate-predicate ban: no
// predicate here is load-bearing on a source grep). Source greps appear only as
// auxiliary coupling checks in 003 and 004, never as the sole assertion.
//
//   - 001 both real assembler forms still attribute (anti-regression: the fix
//     must not narrow attribution for genuine launches). Expected pre-existing
//     GREEN — it guards the fix from over-correcting.
//   - 002 is the crux: a prose-only mention of the ArtifactPath, with a
//     non-matching cwd so the fallback cannot rescue it, must NOT attribute.
//     RED today — the bare substring matches.
//   - 003 label drift: a near-miss marker label ("Artifact-path:",
//     "artifact path:") must NOT attribute, and both assemblers must still emit
//     the canonical "Artifact path: " label. RED today for the same reason as
//     002.
//   - 004 runs the tokenusage unit suite's two contract tests as a subprocess
//     (ONE named package, -run-narrowed per the flaky-shape rules) and requires
//     both to PASS: the updated S1 concurrent-sessions test and the new
//     TestAttributes_MarkerAnchored. RED today — the latter does not exist.
```
