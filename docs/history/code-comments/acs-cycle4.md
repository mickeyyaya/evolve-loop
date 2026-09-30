# Comment history: `acs/cycle4`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle4/predicates_test.go:3` — above `package cycle4`

```text
// Package cycle4 materializes the cycle-4 acceptance criteria for:
//   - Task 1: migrate-di-seams-advisor-workspace
//   - Task 2: migrate-cli-flags-policy-platform-marketplace
```

### `go/acs/cycle4/predicates_test.go:87` — above `acsassert.FileContains(t, path, 'Name: "EVOLVE_PLATFORM", Status: StatusDeprecated')`

```text
// EVOLVE_POLICY_BYPASS row deleted in cycle-15 (bypass-policy-flag task) —
// row is fully gone, so the StatusDeprecated assertion is removed here.
```
