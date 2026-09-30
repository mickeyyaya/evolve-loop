# Comment history: `acs/cycle1272`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1272/predicates_test.go:3` — above `package cycle1272`

```text
// Package cycle1272 materialises the cycle-1272 acceptance criteria for the one
// task triage committed to `## top_n`:
//
//	close-out-cycle1272-fleet-scope-verification  → CHANGELOG.md gains a dated
//	entry recording that both fleet-scope todo-ids
//	(infra-teardown-predicate-single-source, retro-fleet-worktree-dispatch) were
//	found already-implemented and verified-closed in cycle-1272, citing
//	TestInfraTeardownUnion_SpelledExactlyOnce and
//	TestRetroWorktree_FleetScratchCwdSatisfiesBridgeGuardPredicate as the proof.
//
// The two dropped todo-ids get ZERO predicates (R9.3 floor-binding: predicates
// bind only to triage-committed work).
//
// Predicate-quality note (cycle-85 ban). The deliverable of this task IS a
// documentation artifact, so 001/004 necessarily read the emitted CHANGELOG.md —
// that is an assertion on a real emitted artifact, not a source-grep standing in
// for behaviour. The degenerate failure mode the ban targets (add a magic string
// to production source and the predicate greens regardless of the fix) is closed
// here by 002 and 003, which refuse to let the entry's CLAIM be decorative:
//
//   - 002 resolves every cited test name to a real `func Test…` definition in the
//     tree — a fabricated citation FAILS (negative axis).
//   - 003 is the crux: it RUNS both cited tests and requires `--- PASS: <name>`
//     in the verbose output, so a CHANGELOG entry claiming closure while the
//     proving tests are red — or while `-run` matched nothing at all — cannot
//     pass. Documenting a false closure is exactly the defect worth catching.
//   - 004 pins the entry against the duplicated-bullet corruption already
//     visible in the 22.13.1 section of this same file (edge axis).
//
// Roots: CHANGELOG.md and the Go tree are both read under acsassert.RepoRoot(t)
// (the cycle worktree), where Builder writes. Their absence is a FAILURE, not a
// skip. The 003 subprocess narrows each invocation to ONE named package with an
// anchored `-run` (≈2s each measured at RED) per the flaky-predicate-shape rules
// — no `./...` sweep, no wall-clock bound, no literal PID, and `cmd.Dir` is set
// explicitly rather than inherited from the lane's cwd.
```
