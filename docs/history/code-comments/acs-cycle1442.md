# Comment history: `acs/cycle1442`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1442/predicates_test.go:3` — above `package cycle1442`

```text
// Package cycle1442 materialises the cycle-1442 acceptance criteria for the one
// fleet-scoped task pinned to this lane:
//
//	schema-aligned-salvage-layer → fix-salvage-content-persistence
//
// The defect (cycle-1441 audit H1, HIGH). salvageVerdictWith re-verifies the
// REPAIRED bytes and, when that re-verify passes, returns a Result built by
// struct-copying the ORIGINAL res and flipping only OK/Violations — the
// `repaired` string is validated and then dropped on the floor. Every
// downstream consumer that reads Result.Content, or re-reads the artifact from
// ArtifactPath, still sees the malformed bytes while the gate reports OK=true.
// The gate approves a byte stream that is not the byte stream it approved.
//
// Predicate strategy — each predicate drives the REAL production seam and
// asserts on an observable effect (returned bytes, on-disk bytes), never a
// source-grep of the fix (the cycle-85 degenerate-predicate ban):
//
//   - 001 is the in-memory crux: it takes the Result the production Verify
//     entry point actually produces for a malformed-but-recoverable audit
//     report, salvages it, and then re-verifies the RETURNED Content through
//     the same production verifier. Clean bytes verify OK; the original
//     malformed bytes do not — so this predicate cannot pass unless Content
//     really carries the repaired bytes. A no-op fix that only flips OK (today's
//     code) fails it.
//   - 002 is the wiring/reachability proof through the production CALLER
//     (Reviewer.Review, go/internal/deliverable/reviewer.go:138 — which discards
//     the salvaged Result entirely, making the on-disk write the ONLY channel by
//     which repaired bytes can reach a downstream phase). It runs the real gate
//     over a real workspace and asserts the artifact ON DISK re-verifies clean
//     afterwards. A fix that sets Content but never persists fails it.
//   - 003 is the negative/refusal invariant: a REFUSED salvage must leave both
//     the returned Content and the on-disk artifact byte-identical. This is the
//     anti-overreach guard on 001/002 — it is expected PRE-EXISTING GREEN and
//     must stay green, so the persistence fix cannot be implemented as an
//     unconditional write.
//   - 004 is the package regression floor: the one named package the fix touches
//     must stay green (single named package, no `./...` sweep — flaky-shape rule).
```

### `go/acs/cycle1442/predicates_test.go:57` — above `const recoverableFencedVerdict = "## Verdict\n" +`

```text
// recoverableFencedVerdict is the canonical single-candidate shape salvage
// exists for: the verdict payload is present and unambiguous, but it is fenced
// JSON rather than a canonical evolve-verdict sentinel, so the strict parse
// raises bad_verdict as the SOLE violation. The "## Verdict" heading is
// required — without it the report also fails missing_section, which is the
// multi-violation shape salvage refuses outright (cycle-1392).
```

### `go/acs/cycle1442/predicates_test.go:101` — above `func TestC1442_001_SalvagedResultCarriesTheRepairedBytes(t *testing.T) {`

```text
// TestC1442_001_SalvagedResultCarriesTheRepairedBytes is the crux.
//
// It asserts the returned Result.Content is bytes that VERIFY CLEAN through the
// production verifier. The original malformed bytes provably do not (asserted
// as an in-test negative control), so the only way to pass is to actually
// thread `repaired` into the returned Result — the exact line cycle-1441 audit
// H1 says is missing.
```
