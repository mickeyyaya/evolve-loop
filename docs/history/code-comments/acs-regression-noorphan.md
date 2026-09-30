# Comment history: `acs/regression/noorphan`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/regression/noorphan/noorphan_test.go:64` — above `if os.IsPermission(err) && info != nil && info.IsDir() {`

```text
// A directory the phase sandbox denies (docs/private under the
// audit profile) holds no repo scripts this gate guards: skip it
// visibly instead of failing the gate on an instrument fault
// (cycles 1676/1679).
```
