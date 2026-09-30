# Comment history: `acs/regression/cycle72`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/regression/cycle72/predicates_test.go:3` — above `package cycle72`

```text
// Package cycle72 ports the cycle-72 ACS predicates (1 bash file, 4 ACs).
```

### `go/acs/regression/cycle72/predicates_test.go:14` — above `func TestC72_001_P2InertCycle72(t *testing.T) {`

```text
// TestC72_001_P2InertCycle72 ports cycle-72/001 (P2 INERT marking verification).
// AC1: P2 row contains "INERT cycle 72"
// AC2: P2 row contains the C71 telemetry delta string
// AC3: ADR 0009 exists
// AC4: ADR 0009 contains a rollback section
```
