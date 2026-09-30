# Comment history: `acs/cycle1410`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1410/predicates_test.go:3` — above `package cycle1410`

```text
// Package cycle1410 encodes the cycle-1410 ACS predicates for
// verify-pipeline-blocker-fix-421.
//
// The task is a VERIFICATION-AND-CONSUME step for the cd49274beab2 halt:
// PR #421 (commit 7a42d30b) made Direction-B persona/profile pairing bind only
// git-TRACKED profiles, so the untracked runtime-minted stub
// .evolve/profiles/defect-disposition-ledger.json can no longer red every lane
// ship. Predicates 001/002 re-prove that behavior live (they are pre-existing
// GREEN by design — the fix is already merged); predicates 003/004 are the RED
// contract: the durable consumed record carrying the verification evidence does
// not exist yet.
```

### `go/acs/cycle1410/predicates_test.go:27` — above `const pairingPkg = "./internal/phasecoherence"`

```text
// pairingPkg is the single named package the repo-contract pack's pairing gate
// lives in. Deliberately NOT the four-suite pack invocation: a multi-package
// sweep inside a cycle predicate is the banned flaky shape (cycles 1173/1175/
// 1178 false-REDs under fleet load).
```

### `go/acs/cycle1410/predicates_test.go:130` — above `func findCd49274Record(t *testing.T, root string) (string, consumedRecord) {`

```text
// findCd49274Record locates the NEW consumed record for the cd49274beab2
// incident. Returns the path and the parsed record.
```

### `go/acs/cycle1410/predicates_test.go:163` — above `func TestC1410_003_ConsumedRecordCarriesVerificationEvidence(t *testing.T) {`

```text
// TestC1410_003_ConsumedRecordCarriesVerificationEvidence is the RED contract:
// consuming the item requires a NEW durable record that records HOW the fix was
// verified (commit, timestamp, test evidence) — not a bare move, and not a
// clobber of the prior 2026-08-05 record for the different 96f17cfe3dfe halt.
```

### `go/acs/cycle1410/predicates_test.go:202` — above `if !strings.Contains(rec.Notes, "cd49274beab2") && !strings.Contains(rec.Verification.Evidence, "cd49274beab2") {`

```text
// The record must state the incident it closes, so a forensics sweep can
// tell this halt apart from the 96f17cfe3dfe one.
```

### `go/acs/cycle1410/predicates_test.go:209` — above `func TestC1410_004_VerificationCommitIsMergedAncestorOfHead(t *testing.T) {`

```text
// TestC1410_004_VerificationCommitIsMergedAncestorOfHead drives git itself: the
// SHA the record claims must resolve to a real commit that references PR #421
// AND be an ancestor of HEAD. A hand-typed or aspirational SHA fails here.
```

### `go/acs/cycle1410/predicates_test.go:220` — above `sha = strings.Fields(sha)[0]`

```text
// Take the first whitespace-delimited token so a "7a42d30b (#421)" style
// value still resolves.
```
