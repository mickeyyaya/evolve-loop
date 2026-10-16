# Comment history: `internal/binaryguard`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero K1

### `go/internal/binaryguard/binaryguard_test.go:81` — above `func TestOffender_RecordsPathAndSize(t *testing.T) {`

```text
// TestOffender_RecordsPathAndSize names the exported Offender type directly
// (ADR-0069 apicover naming gate: every exported symbol needs a test that
// references it by name) and pins its two fields as the caller-facing contract
// commitgate relies on when it logs a rejected artifact.
```
