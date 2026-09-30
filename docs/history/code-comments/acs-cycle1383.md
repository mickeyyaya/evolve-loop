# Comment history: `acs/cycle1383`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1383/predicates_test.go:3` — above `package cycle1383`

```text
// Package cycle1383 materializes the acceptance criteria of this lane's sole
// fleet-scoped todo-id `triage-protected-surface-admission-wire-verdict`
// (triage top_n — "retire-stale-protected-surface-admission-inbox-item").
//
// FINDING (read-first, rule 8 — this cycle changes NO production code):
// scout verified the substantive fix the inbox item asks for already shipped
// in cycle-1312 (commit 0d07b200): `go/internal/phases/triage/triage.go`'s
// `Classify` runs `protectedTopNViolation` (triage.go:311), a second,
// commit-time admission check that FAILs any `## top_n` card whose
// `files=` / `files={}` segment names a `guards.IsProtectedSurface` path —
// independent of the prompt-side `inboxbatch.PartitionConsole` screen.
// `go/internal/phases/triage/protected_surface_admission_test.go` (5 tests)
// pins that contract and is GREEN; §F4 of
// `docs/operations/batch-integrity-review-2026-08-04.md` documents it.
//
// The only remaining work is deterministic bookkeeping: the inbox item that
// requested the fix was never retired, so it keeps resurfacing as live
// backlog. This cycle removes it. The predicates below therefore split into
// two groups:
//
//   - 001/002/003 — RED at authoring time: the retirement itself (file gone
//     from disk, git sees a tracked-file deletion, and no renamed/suffixed
//     survivor variant is left behind under .evolve/inbox/).
//   - 004/005 — pre-existing GREEN, and deliberately so: they are the
//     anti-over-deletion and no-regression guards. AC3 is "the shipped fix
//     is verified UNTOUCHED, not re-implemented", so its predicate must be
//     green both before and after; it goes RED only if the builder damages
//     the admission contract or mass-deletes sibling backlog while retiring
//     one item.
```

### `go/acs/cycle1383/predicates_test.go:152` — above `func TestC1383_005_ProtectedSurfaceAdmissionStillEnforced(t *testing.T) {`

```text
// TestC1383_005_ProtectedSurfaceAdmissionStillEnforced is AC3: the shipped
// cycle-1312 admission check must be verified UNTOUCHED, not re-implemented.
// Behavioral, not a source grep — it drives the real
// `hooks.Classify` -> `protectedTopNViolation` path through the contract
// suite that pins it, so it goes RED if the builder edits triage.go or
// weakens the test file while doing the bookkeeping.
//
// Shape rules: ONE named package, narrowed with -run (never a `/...` sweep),
// cmd.Dir set explicitly (never a cwd-relative `go test`), and a PASS-count
// floor so a -run pattern that matches nothing cannot report a hollow `ok`.
//
// Pre-existing GREEN by construction (see package doc).
```

### `go/acs/cycle1383/predicates_test.go:167` — above `pattern := "TestTriageClassify_(" + strings.Join([]string{`

```text
// Name all five cycle-1312 cases explicitly rather than a loose prefix:
// a renamed or deleted case must show up as a missing PASS, not silently
// drop out of a fuzzy pattern.
```
