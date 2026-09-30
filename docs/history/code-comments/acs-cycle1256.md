# Comment history: `acs/cycle1256`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1256/predicates_test.go:3` — above `package cycle1256`

```text
// Package cycle1256 materialises the acceptance criteria for this lane's single
// triage-COMMITTED (## top_n) task, artifact-ready-crosspoll-debounce.
//
// SCOPE CORRECTION, stated up front because it changes what these predicates
// gate and what Builder is expected to do.
//
// Scout reported the cross-poll debounce as entirely ABSENT (scout-report.md
// Finding 1: "No such debounce exists in completion.go"). That reading was
// taken against the MAIN tree, which indeed lacks it — `git show
// main:go/internal/bridge/completion.go` has no artifactStableTicks, no
// artifactLocate, and an artifactDetector.poll that completes on first sight.
// THIS lane's worktree is not main. Its base commit (b0d89a71, the ADR-0076
// salvage snapshot) already carries the whole mechanism, landed by cycles
// 1233 (the window) and 1249 (gating the relocation with it):
//
//   - const artifactStableTicks = 2                  completion.go:38-43
//   - the (size, mtime, path) cross-poll window       completion.go:215-270
//   - artifactLocate, the read-only half              driver_common.go:198-…
//   - complete(), the ONLY caller of artifactReady    completion.go:272-…
//   - contract tests                                  completion_debounce_test.go,
//     completion_relocate_stability_test.go
//
// Re-implementing it would stack a second counter on d.stable and double every
// artifact phase's completion latency while closing no defect. So 001-003 are
// VERIFY-AND-CLOSE of the landed work: expected GREEN from the first run, their
// job being to fail loudly if this cycle's edits disturb the sibling contract.
// This is declared, not hidden — see test-report.md's AC-Materialization table.
//
// The RESIDUAL, which is this cycle's actual build work and is genuinely RED.
// Scout's task text has two halves; only the first landed. The second —
// "Update deliverable.go:178-181's comment to describe the debounce accurately"
// — is untouched: that comment is byte-identical to main's and still only NAMES
// the mechanism ("closed at the SOURCE by the bridge artifact-ready cross-poll
// debounce (completion.go)") without describing it. That is the exact doc/code
// drift scout opened the report with, merely inverted: the claim was false when
// written and is now true but unverifiable from the text, so the next reader
// cannot tell which. 004 gates it.
//
// Predicate strategy. artifactDetector, its poll method, artifactReady and the
// wait loop are all UNEXPORTED, so these predicates cannot import them. 001-003
// instead shell `go test -run -v` at the SINGLE named package internal/bridge
// over the contract tests and assert the per-test `--- PASS: <name>` receipt.
// The receipt check is load-bearing anti-gaming: `go test -run <deleted-test>`
// matches nothing and exits 0, so an exit-code-only predicate would go GREEN if
// Builder deleted the contract instead of satisfying it (cycle-1113 lesson).
// ONE named package, -C-anchored, never a `./...` sweep — a whole-repo run is
// the regression suite's job and a false-red generator under fleet load
// (cycles 1173/1175/1178).
//
// Caller proof. 002 drives the REAL production entry point
// (Engine.LaunchArgs → runTmuxREPL → detector.poll, driver_tmux_repl.go), so a
// debounce living on a struct nothing reaches cannot satisfy it. 003 asserts on
// the filesystem SIDE EFFECT (did the fallback move?), not on a return value.
//
// Diversity. 001 carries both negative axes of the window — a still-growing
// file (size) and the same-SIZE rewrite a size-only key is blind to (mtime) —
// plus the never-settles case scout named in AC-4. 003 pairs a negative (an
// in-flight fallback must NOT be moved) with a positive (a settled one MUST
// still be moved, so "never relocate" cannot pass) and a latency axis. 004
// pairs its doc assertion with a code-side existence check, so the comment
// cannot be made to describe a mechanism that is not there.
```

### `go/acs/cycle1256/predicates_test.go:84` — above `func runContract(t *testing.T, names ...string) (ok bool, missing []string, out string) {`

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

### `go/acs/cycle1256/predicates_test.go:131` — above `func TestC1256_001_CrossPollStabilityWindowHolds(t *testing.T) {`

```text
// TestC1256_001_CrossPollStabilityWindowHolds is the verify-and-close of AC-1
// and AC-4. artifactDetector must not complete on the first sighting of a
// deliverable, must RESET its counter on a still-growing file (size axis — this
// is also scout's AC-4 "never stabilizes" negative: the file changes on every
// one of six consecutive polls and must never report ready), must reset on a
// same-SIZE rewrite (mtime axis, the reason mtime is in the key at all), must
// still short-circuit for the wait loop's final post-cancel look, and its window
// constant must remain a real window rather than being flattened to 1.
//
// Expected GREEN on the first run: landed by cycles 1233/1249, carried in this
// lane's base commit. It fails only if this cycle regresses the shared
// completion.go.
```

### `go/acs/cycle1256/predicates_test.go:206` — above `func TestC1256_004_DeliverableDocDescribesTheRealMechanism(t *testing.T) {`

```text
// TestC1256_004_DeliverableDocDescribesTheRealMechanism is this cycle's BUILD
// gate and the genuinely RED predicate.
//
// acs-predicate: config-check — WAIVER RATIONALE. This AC is a documentation
// accuracy criterion (scout task text, second half: "Update deliverable.go's
// comment to describe the debounce accurately"). A prose claim has no runtime
// behaviour to invoke, so the assertion is necessarily over the text. It is not
// the cycle-85 degenerate shape for two reasons: (a) it is not the only
// load-bearing predicate in this package — 001-003 are behavioural and carry
// the mechanism itself; (b) the second half below asserts against
// completion.go's CODE, so the comment cannot be greened by describing a
// mechanism that does not exist. Builder cannot satisfy it with a magic string.
//
// Why it is RED. deliverable.go's LAYERING comment is byte-identical to main's
// and reads only: "mid-write truncation is closed at the SOURCE by the bridge
// artifact-ready cross-poll debounce (completion.go)". It names a mechanism
// without describing it, which is precisely the drift scout opened the report
// with (Finding 1) — when that sentence was written the mechanism did not
// exist, and nothing in the text let a reader tell. The fix is to state what
// the debounce actually does: consecutive poll ticks keyed on (size, mtime),
// and that the window gates the relocation.
```
