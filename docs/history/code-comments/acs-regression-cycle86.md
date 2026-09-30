# Comment history: `acs/regression/cycle86`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/regression/cycle86/predicates_test.go:3` — above `package cycle86`

```text
// Package cycle86 ports the cycle-86 ACS predicates (5 bash files).
```

### `go/acs/regression/cycle86/predicates_test.go:63` — above `t.Skipf("%s: not found in processed/ (cycle-86 not run)", procFile)`

```text
// Skip on fresh checkouts where cycle-86 hasn't run.
```

### `go/acs/regression/cycle86/predicates_test.go:69` — above `func TestC86_NoNewTestBuildAbnormal(t *testing.T) {`

```text
// TestC86_NoNewTestBuildAbnormal ports pred-no-new-test-build-abnormal.sh.
// Skips when no cycle-86 abnormal-events.jsonl exists (trivially green).
// The bash predicate uses jq --slurp to filter event_type ∈
// {ship-refused, turn-overrun} AND .details matches agent=…. The Go
// port can't do per-row filtering without parsing NDJSON, so it skips
// when both substring classes are present (likely false-positive) and
// defers to the bash predicate for authoritative judgment.
```
