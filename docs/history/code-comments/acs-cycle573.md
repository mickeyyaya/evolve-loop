# Comment history: `acs/cycle573`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle573/predicates_test.go:3` — above `package cycle573`

```text
// Package cycle573 materialises the cycle-573 acceptance criteria for the three
// triage-committed top_n tasks (see scout-report.md / triage-report.md):
//
//   - Task 1 memo-tier-envelope-fix (inbox 0.95 critical) — align the memo
//     phase's model-tier pin with its profile's model_tier_envelope, config-only.
//   - Task 2 changedpkgs-from-git (inbox 0.96 critical) — replace the extinct
//     LLM handoff-build.json changed-package source with deterministic git, so
//     the apicover CI-parity gate stops failing open.
//   - Task 3 dossier-commit-rollback (inbox 0.84 medium) — on a permanent
//     commit failure, unstage the dossier pair so it can't pollute the next
//     cycle's tree-diff guard.
//
// Predicate strategy: behavioural-via-subprocess (the cycle-549…566 precedent).
// Each predicate shells `go test -run` over the RED unit tests authored this
// cycle in the internal packages. None is a source-grep — every one exercises
// the system under test (ValidatePin over shipped config, FromGit over a real
// git repo, changedPackagesForAudit over a real git repo, commitPairGit over a
// real failing git commit) and asserts on its result. RED now: the changedpkgs
// package fails to compile (FromGit undefined) and the policy/audit/dossier
// assertions fail against current behaviour. GREEN once all three land.
```
