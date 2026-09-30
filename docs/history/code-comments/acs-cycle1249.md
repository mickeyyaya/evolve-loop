# Comment history: `acs/cycle1249`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1249/predicates_test.go:3` — above `package cycle1249`

```text
// Package cycle1249 materialises the acceptance criteria for this lane's single
// triage-COMMITTED (## top_n) task, artifact-ready-crosspoll-debounce.
//
// SCOPE CORRECTION, stated up front because it changes what these predicates
// gate. Scout reported the cross-poll debounce as entirely absent. That reading
// was taken against the MAIN tree, which indeed lacks it. In THIS lane's
// worktree the debounce is already present at HEAD (0d879c78 carries
// const artifactStableTicks + the (size, mtime) window at completion.go:38-43,
// :248-265, folded in from cycle-1233 by 841e676f), it is wired into the
// production wait loop (withFinalPoll, driver_tmux_repl.go:585), it is covered
// by completion_debounce_test.go, and cycle-1236 predicate 004 already
// verify-and-closed it. Re-implementing it would stack a second counter on
// d.stable and double every artifact phase's completion latency for no defect
// closed. So 001-002 below are VERIFY-AND-CLOSE of the landed half — expected
// GREEN from the first run, their job being to fail loudly if this cycle's edits
// to the shared completion.go disturb it.
//
// The RESIDUAL, which is this cycle's actual build work and is genuinely RED.
// artifactDetector.poll opens with artifactReady(d.cfg) (completion.go:223), and
// artifactReady RELOCATES a non-canonical fallback the instant it sees it
// non-empty (driver_common.go, the cycle-108/141 tolerance) — before a single
// stability observation has been made about that fallback. The window therefore
// runs strictly DOWNSTREAM of an irreversible move. On relocateFile's rename
// branch that survives (the agent's open fd follows the inode). On its
// copy+remove cross-device branch it does not: a partial file is snapshotted
// into the canonical path and the source the agent is still appending to is
// deleted, leaving a permanently stable, permanently TRUNCATED deliverable that
// the debounce then declares finished. That is the very mid-write-truncation
// class deliverable.go:180 names this mechanism as the source-side closure of,
// reached through the path scout independently flagged (hypothesis 2) as
// highest-risk because it carries the extra copy step. Predicate 003 gates it.
//
// Predicate strategy. artifactDetector, its poll method, artifactReady and the
// wait loop are all UNEXPORTED, so these predicates cannot import them. Each
// instead shells `go test -run -v` at the SINGLE named package internal/bridge
// over the contract tests, and asserts the per-test `--- PASS: <name>` receipt.
// The receipt check is load-bearing anti-gaming: `go test -run <deleted-test>`
// matches nothing and exits 0, so an exit-code-only predicate would go GREEN if
// Builder deleted the contract instead of satisfying it (cycle-1113 lesson).
// ONE named package, never a `./...` sweep — a whole-repo run is the regression
// suite's job and a false-red generator under fleet load (cycles 1173/1175/1178).
//
// Caller proof. 002 drives the REAL production entry point
// (Engine.LaunchArgs → runTmuxREPL → detector.poll at driver_tmux_repl.go:601),
// so a debounce living on a struct nothing reaches cannot satisfy it. 003's
// fault site IS artifactDetector.poll's own first statement — the seam and the
// caller are the same line — and it asserts on the filesystem SIDE EFFECT (did
// the file move?) rather than on the return value, which is already correct
// today; that is what makes it un-passable by tuning the counter.
//
// Diversity. 003 pairs a negative (an in-flight fallback must NOT be moved) with
// a positive (a settled one MUST still be moved, so "never relocate" is not a
// passing fix) and a latency regression axis (gating the move must not stack a
// second window). 001 carries both negative axes of the landed window — a
// still-growing file, and the same-SIZE rewrite a size-only key is blind to.
```

### `go/acs/cycle1249/predicates_test.go:78` — above `func runContract(t *testing.T, names ...string) (ok bool, missing []string, out string) {`

```text
// runContract runs `go test -C <worktree>/go -v -run ^(names...)$ ./internal/bridge/`
// and reports whether EVERY named test both ran and passed.
//
// Two failure shapes are distinguished deliberately:
//   - code < 0 is a genuine "could not launch" and is fatal (cycle-574 lesson);
//     a compile failure in the target package — an expected RED signal before
//     Builder implements — is a NON-ZERO EXIT, not a launch failure.
//   - a zero exit with a missing `--- PASS: <name>` receipt means the test is
//     gone, not that it passed. That is reported as a miss, not a pass.
```
