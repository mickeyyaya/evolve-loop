# Comment history: `acs/cycle1252`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1252/predicates_test.go:3` — above `package cycle1252`

```text
// Package cycle1252 materialises the acceptance criteria for this lane's single
// triage-COMMITTED (## top_n) task, artifact-ready-crosspoll-debounce.
//
// SCOPE, stated up front because it decides what is RED and what is a
// verify-and-close. Scout read the defect against the MAIN tree, which does
// lack the debounce entirely. In THIS lane's worktree the fix is already
// present at HEAD (5741bdba, the ADR-0076 continuation-on-fail salvage
// snapshot): const artifactStableTicks = 2 (completion.go:38-43), the
// cross-poll (path, size, mtime) window (:215-266), the window-GATED
// relocation in complete() (:275-285), the live final-poll context in the
// wait loop (driver_tmux_repl.go:583-590), and the three contract test files
// (completion_debounce_test.go, completion_relocate_stability_test.go,
// completion_cancel_parity_test.go). This cycle's work is therefore
// "verify, correct, land", not "implement" — and predicates 001-004 are
// verify-and-close of the landed behaviour, expected GREEN from the first run.
// Their job is to fail loudly if this cycle's edits to the shared
// completion.go / driver_tmux_repl.go disturb any of it before it lands.
//
// The genuine RED is 005: completion.go's finality short-circuit still
// documents itself through the WRONG function. The comment claims the check is
// "Checked AFTER artifactReady so finality can never manufacture completion
// from nothing", but the code checks finality after artifactLocate (:226) and
// reaches artifactReady only later, via complete(). The guarantee holds — a
// found artifact is already non-empty — but it is asserted through a function
// the code does not call there. That is the SAME defect class this cycle
// exists to close: deliverable.go:179-181 claimed a source-side debounce that
// did not exist. Shipping the debounce while leaving a second stale
// forward-reference in the file it points at would reproduce the bug in
// miniature.
//
// Predicate strategy. artifactDetector, its poll method, artifactReady,
// artifactLocate and the wait loop are ALL UNEXPORTED, so these predicates
// cannot import them. 001-004 each shell `go test -run -v` at the SINGLE named
// package ./internal/bridge/ over the contract tests and assert the per-test
// `--- PASS: <name>` receipt. The receipt check is load-bearing anti-gaming:
// `go test -run <deleted-test>` matches nothing and exits 0, so an
// exit-code-only predicate would go GREEN if the contract were deleted rather
// than satisfied (cycle-1113 lesson). ONE named package, always narrowed by
// -run, never a `./...` sweep — a whole-repo run is the regression suite's job
// and a false-red generator under fleet load (cycles 1173/1175/1178).
//
// Caller proof. 003 drives the REAL production entry point
// (Engine.LaunchArgs -> runTmuxREPL -> detector.poll, driver_tmux_repl.go:613
// for the main loop and :586 for the post-cancel final poll), so a window
// living on a struct nothing reaches cannot satisfy it. Both paths are
// covered: wired into one only is the same defect.
//
// Diversity. 001 carries the negative axes (a still-growing file must NOT
// complete; a same-SIZE rewrite must NOT complete — the reason mtime is in the
// key at all). 002 pairs a negative (an in-flight fallback must NOT be moved)
// with a positive (a settled one MUST still be moved, so "never relocate"
// cannot pass). 003 pairs the wiring proof with the cancel-parity edge, where
// over-strictness turns the fix into a worse false-FAIL generator
// (ExitArtifactTimeout on a delivered phase, cycle-1236). 004 is the
// boundary/OOD axis on the window constant itself.
```

### `go/acs/cycle1252/predicates_test.go:80` — above `func runContract(t *testing.T, names ...string) (ok bool, missing []string, out string) {`

```text
// runContract runs `go test -C <worktree>/go -v -run ^(names...)$ ./internal/bridge/`
// and reports whether EVERY named test both ran and passed.
//
// Two failure shapes are distinguished deliberately:
//   - code < 0 is a genuine "could not launch" and is fatal (cycle-574 lesson);
//     a compile failure in the target package — an expected RED signal before
//     the implementation exists — is a NON-ZERO EXIT, not a launch failure.
//   - a zero exit with a missing `--- PASS: <name>` receipt means the test is
//     gone, not that it passed. That is reported as a miss, not a pass.
```

### `go/acs/cycle1252/predicates_test.go:127` — above `func TestC1252_001_CrossPollStabilityWindowGatesReady(t *testing.T) {`

```text
// TestC1252_001_CrossPollStabilityWindowGatesReady verifies the core acceptance
// criterion: artifactDetector must not report ready on the FIRST sighting of a
// non-empty deliverable. It must observe the same (path, size, mtime) for
// artifactStableTicks consecutive ticks, resetting on a still-growing file
// (size axis) and on a same-SIZE rewrite (mtime axis — a fix-up Edit of equal
// length is exactly what a size-only key is blind to).
//
// Verify-and-close of the salvaged half: expected GREEN on the first run,
// RED against the main tree, and RED again if this cycle's edits to the shared
// completion.go flatten the window back to first-sight completion (cycle-1198).
```

### `go/acs/cycle1252/predicates_test.go:179` — above `func TestC1252_003_WindowReachableFromProductionWaitLoop(t *testing.T) {`

```text
// TestC1252_003_WindowReachableFromProductionWaitLoop is the caller proof, and
// it covers BOTH production poll sites, not one.
//
// Main loop (driver_tmux_repl.go:613): a deliverable rewritten on every tick
// must NOT exit ExitOK — a window on a detector the driver never consults is
// dead code. Post-cancel final poll (:586): the detector receives a LIVE,
// finalPollGrace-bounded context carrying the explicit finality marker, and a
// session that delivered before the cancel must complete rather than laundering
// into ExitArtifactTimeout. That second half is the regression-sensitive edge —
// an over-strict window turns a truncated-read fix into a worse false-FAIL
// generator, and the stdout/git contracts (which shell CapturePane and git, and
// cannot fork on a dead context) exited 81 on delivered phases before it
// (cycle-1236).
```

### `go/acs/cycle1252/predicates_test.go:237` — above `func TestC1252_005_FinalityShortCircuitDocumentsTheRightGuard(t *testing.T) {`

```text
// TestC1252_005_FinalityShortCircuitDocumentsTheRightGuard is this cycle's
// genuinely RED predicate.
//
// completion.go's finality short-circuit (:242) explains why it cannot
// manufacture completion from nothing by asserting it is "Checked AFTER
// artifactReady". It is not: poll checks finality after artifactLocate (:226),
// and artifactReady is reached only later through complete() (:276) — an
// ordering the cycle-1249 relocation gate deliberately established. The
// guarantee itself is intact (artifactLocate's found already proves a non-empty
// file), but it is attributed to the wrong function.
//
// This is the exact defect class the cycle is closing. deliverable.go:179-181
// pointed at "the bridge artifact-ready cross-poll debounce (completion.go)"
// for a mechanism that did not exist; landing the debounce makes that sentence
// true, and it would be incoherent to make one forward-reference honest while
// leaving a second stale one inside the file it points at. A reader who trusts
// this comment looks for the ordering invariant at the wrong seam and can
// reintroduce first-sight relocation without noticing they broke it.
//
// Not gameable by adding a magic string: the assertion is that the FALSE claim
// is gone AND the true one is present. Deleting the sentence outright fails the
// second half; the only passing edit is the correction.
```
