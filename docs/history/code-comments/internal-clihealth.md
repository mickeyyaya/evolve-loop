# Comment history: `internal/clihealth`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/clihealth/resetparse.go:42` — above `func evidenceLine(pane string) string {`

```text
// evidenceLine returns the most representative line for a bench record: the
// wall BANNER line that carries the reset hint (the line ParseResetHint keys
// on), which is what actually walled the CLI — not the pane's first line. On a
// scrolled pane firstLine catches a later frame or, as in cycle-314, the
// agent's own edit content ("53 +\tFamily: codex,"), obscuring the real cause
// in the bench evidence. Falls back to firstLine when no reset-hint line is
// present (no regression for walls without a parseable hint).
```
