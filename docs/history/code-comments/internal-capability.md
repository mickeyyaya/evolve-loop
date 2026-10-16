# Comment history: `internal/capability`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d7

### `go/internal/capability/qualitytier_test.go:11` — above `func TestQualityTier_GoldenParityWithBashManifests(t *testing.T) {`

```text
// TestQualityTier_GoldenParityWithBashManifests pins the Go port to the live
// _capability-check.sh output captured (2026-06-18) over the real, kept
// adapters/*.capabilities.json manifests. probe-true vs probe-false matches the
// two deterministic golden runs (binary present / absent). This is the
// before-deletion parity oracle for Wave A.
```
