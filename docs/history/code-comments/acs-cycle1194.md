# Comment history: `acs/cycle1194`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1194/predicates_test.go:3` — above `package cycle1194`

```text
// Package cycle1194 materialises the cycle-1194 acceptance criteria for the
// two fleet-scoped tasks pinned to this lane:
//
//   - bridgewatch-follow-macos-flake                       (predicates 001–002)
//   - loop-must-base-lanes-on-origin-main-not-stale-local   (predicate 003)
//
// CONTINUATION CONTEXT. This lane inherits a salvage snapshot (df84167e, ADR-0076
// continuation-on-fail) that already carries the fix for BOTH tasks, and a prior
// lane (cycle-1191, go/acs/cycle1191/predicates_test.go) already authored and
// GREENED the identical acceptance criteria against that same code:
//
//   - the follow suite (cmd/evolve/cmd_bridge_watch_test.go) replaced its fixed
//     10ms sleep / 200ms deadline race with an event-driven wait bounded by a
//     >=10s deadline in BOTH observing follow tests
//     (TestRunBridgeWatchFollow_SkipsMalformedAndEmptyLines and
//     TestRunBridgeWatchFollow_TailsNewLines);
//   - looppreflight.Run wires a `base-divergence` check (basedivergence.go) that
//     fetches origin, HALTs when local is behind, and names `evolve sync-main`.
//
// Every predicate below was authored and run FRESH against the LIVE artifacts in
// THIS worktree (not copied verbatim from cycle1191's cache) and is GREEN on
// first run — the disposition is "predicate / pre-existing GREEN", not RED. Per
// the TDD-engineer contract's "unexpected pass" rule, that status is logged
// explicitly in test-report.md rather than force-fitting an artificial failure.
// Each predicate still EXERCISES the system under test (the cycle-85
// degenerate-predicate ban): 001 runs the real follow-test suite under -race;
// 002 parses the Go AST of the real test file and asserts on the numeric
// deadline literal (a magic string cannot satisfy it); 003 runs the real
// looppreflight.Run against a real git repo whose local base is genuinely
// behind a real (file-remote) origin.
```

### `go/acs/cycle1194/predicates_test.go:189` — above `func behindBaseRepo(t *testing.T) string {`

```text
// behindBaseRepo builds a real work tree on branch main whose local base is
// one commit BEHIND a real (file-remote) origin/main — the exact topology
// that made every cycle-969 lane ship GIT_PUSH_REJECTED. No network is
// involved.
```
