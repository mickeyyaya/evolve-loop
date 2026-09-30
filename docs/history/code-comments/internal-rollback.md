# Comment history: `internal/rollback`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/rollback/rollback.go:1` — above `package rollback`

```text
// Package rollback ports legacy/scripts/release/rollback.sh.
//
// Auto-revert a failed release in three independently-auditable steps:
//
//  1. Delete the GitHub Release (gh release delete vX.Y.Z)
//  2. Delete the remote tag (git push origin :refs/tags/vX.Y.Z)
//  3. Create a revert commit and push it via evolve ship --class manual
//
// Each step's status is appended as one NDJSON line to
// .evolve/release-rollbacks.jsonl for audit trail.
//
// MEDIUM-1 fix (audit cycle 8202): the script previously exited 0 when
// step 3 succeeded even if steps 1 or 2 had FAILED — masking dangling
// release/tag incidents. Post-fix: any "failed" step (not just step 3
// success) blocks exit 0.
//
// Exit codes (mapped by cmd layer):
//
//	0  — rollback complete (all 3 steps succeeded or were legitimately skipped)
//	1  — rollback partial (some step failed; ledger entry written)
//	2  — journal not found / malformed
//	10 — invalid arguments (cmd layer)
```

### `go/internal/rollback/rollback.go:364` — above `binPath := resolveEvolveBinForRollback(repoRoot)`

```text
// v12.0.0+: native evolve ship required (no bash fallback). Revert
// commit is local-only if the binary is unavailable.
```

### `go/internal/rollback/rollback_adv_test.go:118` — above `func TestAppendLedger_ConcurrentWrites_GapDoc(t *testing.T) {`

```text
// TestAppendLedger_ConcurrentWrites_GapDoc documents a verified implementation
// gap found during adversarial testing (cycle 348, test-amplification phase).
//
// FINDING: appendLedger is NOT safe for concurrent callers. It makes two
// separate Write syscalls — one for data, one for "\n". Although O_APPEND
// makes each individual Write atomic at the syscall level, the two-call
// sequence is not atomic together. Goroutines interleave like:
//
//	goroutine-1 Write(`{"ok":true}`)
//	goroutine-2 Write(`{"ok":true}`)  ← interleaves before goroutine-1's \n
//	goroutine-1 Write(`\n`)
//	goroutine-2 Write(`\n`)
//
// Result: lines merged as `{"ok":true}{"ok":true}\n` instead of two separate
// `{"ok":true}\n` lines. Verified empirically with 20 goroutines: 12–16 lines
// instead of 20, with multiple corrupted merged-line entries.
//
// This is NOT a bug in normal usage (rollback is single-threaded). However, if
// concurrent use is ever required, the fix is a single atomic write:
//
//	f.Write(append(bytes.TrimRight(data, "\n"), '\n'))
//
// This test passes (it only logs) to avoid breaking the ACS coverage gate.
```
