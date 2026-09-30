# Comment history: `acs/cycle1034`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1034/predicates_test.go:3` — above `package cycle1034`

```text
// Package cycle1034 materializes the cycle-1034 acceptance criteria for this
// fleet lane's sole assigned item, failure-disposition-router (scout selected
// slices S1 failure-digest-assembler + S2 disposition-contract-gate; S3/S4
// deferred). Per R9.3 predicates bind ONLY to these two committed slices.
//
// Predicate strategy (cycle-85 non-degenerate rule): every AC here is a
// BEHAVIORAL contract on new go/internal/core code, so each predicate SHELLS the
// system-under-test — it runs the corresponding default-suite unit test as a
// subprocess and requires an explicit `--- PASS: <name>` line. A bare exit-0 is
// insufficient: `go test -run` on a pattern matching no test exits 0 with "no
// tests to run", so asserting on the PASS line (not the exit code) is what makes
// a still-unlanded / renamed / build-tag-hidden test RED instead of false-GREEN.
// -count=1 defeats the test cache so current source is always exercised. No
// source-grep predicate appears in this file.
//
// AC map (1:1 with the two eval files' [code] ACs):
//
//	S1 failure-digest-assembler:
//	  001 → AC1 pre-class buckets from real artifacts
//	  002 → AC2 fingerprint stable + phase-composed
//	  003 → AC3 recurrence read through the ledger
//	  004 → AC4 (negative) missing artifacts degrade to unknown, no abort
//	  005 → AC5 (edge) digest written atomically as valid JSON
//	S2 disposition-contract-gate:
//	  006 → AC1 retro fails loud without a valid disposition
//	  007 → AC2 fingerprint cross-checked against the digest
//	  008 → AC3 (negative) out-of-vocabulary enums rejected
//	  009 → AC4 (edge) salvage pointer floor
//	  010 → AC5 (wiring) gate invoked on the composed retro-completion path
//
// RED today: the internal/core surface (AssembleFailureDigest / VerifyDisposition
// / finalizeRetroCompletion) does not exist, so its test package fails to
// COMPILE — every subprocess below produces no `--- PASS:` line and each
// predicate fails.
```
