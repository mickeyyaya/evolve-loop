# Comment history: `acs/cycle1233`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1233/predicates_test.go:3` — above `package cycle1233`

```text
// Package cycle1233 materialises the acceptance criteria for this lane's single
// triage-COMMITTED (## top_n) task:
//
//	artifact-ready-crosspoll-debounce
//
// artifactDetector.poll completes on the FIRST non-empty read of the
// deliverable (completion.go:157). Cycle-1198's gate rejected a scout-report.md
// caught mid Write→Edit — sections present, trailing verdict sentinel not yet
// appended — that parsed perfectly moments later. The deliverable-side grace
// window already shipped but covers absence/emptiness only; a "parses fine,
// wrong content" read is not retried by design. The fix closes it at the SOURCE:
// identical (size, mtime) across artifactStableTicks CONSECUTIVE poll ticks
// (~2s apart) before ready — the artifact twin of stdoutDetector's
// stdoutIdlePolls.
//
// Predicate strategy. artifactDetector, its poll method, and the wait loop are
// all UNEXPORTED, so these predicates cannot import them; each one instead
// shells `go test -run -v` at the single package internal/bridge over the RED
// contract authored this cycle in completion_debounce_test.go, and asserts the
// per-test `--- PASS:` receipt. The receipt check is load-bearing anti-gaming:
// `go test -run <deleted-test>` matches nothing and exits 0, so an exit-code-only
// predicate would go GREEN if Builder deleted the contract instead of satisfying
// it (cycle-1113 lesson).
//
//	001 — the debounce itself: positive settle + BOTH negative axes (a growing
//	      file, and the same-size/different-mtime rewrite that a size-only key is
//	      blind to). The anti-no-op half: first-sight completion fails here.
//	002 — the ctx-cancel short-circuit, with its own negative (cancellation must
//	      not manufacture completion from an absent artifact). Separable sub-fix:
//	      without it the debounce launders finished sessions into ExitArtifactTimeout.
//	003 — the single-shot relocation diagnostic survives the unstable tick that
//	      observed it, and relocation is not an exemption from the window.
//	004 — CALLER PROOF + fixture budget: the debounce is reached from the real
//	      production wait loop (Engine.LaunchArgs → runTmuxREPL → detector.poll,
//	      driver_tmux_repl.go:601), and the short-ArtifactTimeoutS fixture the
//	      prior console attempt was rolled back over stays green (MUST-ALSO (c)).
//
// Diversity: 001 carries one positive and two independent negatives, 002 and 003
// each pair a positive with a negative, 004 is end-to-end behavioural through the
// production entry point rather than the unit seam.
```

### `go/acs/cycle1233/predicates_test.go:65` — above `func runContract(t *testing.T, names ...string) (ok bool, missing []string, out string) {`

```text
// runContract runs `go test -C <worktree>/go -v -run ^(names...)$ ./internal/bridge/`
// and reports whether EVERY named test both ran and passed.
//
// Two failure shapes are distinguished deliberately:
//   - code < 0 is a genuine "could not launch" and is fatal (cycle-574 lesson);
//     a compile failure in the target package — the expected RED signal before
//     Builder implements — is a NON-ZERO EXIT, not a launch failure.
//   - a zero exit with a missing `--- PASS: <name>` receipt means the test is
//     gone, not that it passed. That is reported as a miss, not a pass.
```
