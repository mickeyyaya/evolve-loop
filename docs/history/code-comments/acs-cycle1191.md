# Comment history: `acs/cycle1191`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1191/predicates_test.go:3` — above `package cycle1191`

```text
// Package cycle1191 materialises the cycle-1191 acceptance criteria for the
// three fleet-scoped tasks pinned to this lane:
//
//   - ledger-verify-seal-anchor                        (predicates 001–005)
//   - bridgewatch-follow-macos-flake                   (predicates 006–007)
//   - loop-must-base-lanes-on-origin-main-not-stale-local (predicate 008)
//
// CONTINUATION CONTEXT. This lane inherits a salvage snapshot (8395949e,
// ADR-0076 continuation-on-fail) that already carries substantial work for all
// three tasks. The cycle-1191 bar is therefore NOT "build it" but "close the
// gap the salvage left" — every predicate below was authored against the LIVE
// artifacts in this worktree, and the RED ones name a real, reproduced defect:
//
//   - 001/004/005: the salvaged effectiveAnchorSHA (anchor.go) recognises an
//     in-band seal by `entry.Kind` having the `reset-seal-` prefix. The REAL
//     ledger carries the seal marker in `cycle_label` ("reset-seal-cycle-108")
//     with `kind:"reset"` — see .evolve/ledger.jsonl:1880. So the resolver
//     never fires on production data and `evolve ledger verify` STILL emits the
//     line-1740 wolf-cry (reproduced live on 2026-07-29). Unit-green, live-red.
//   - 002: the inbox Guard ("a seal must only anchor if itself hash-valid from
//     its own prev") is unimplemented — effectiveAnchorSHA trusts any
//     operator-role seal line, so a seal whose own prev_hash is forged silences
//     verification of everything before it.
//   - 007: the salvage fixed ONE follow test (SkipsMalformedAndEmptyLines) but
//     left its sibling TestRunBridgeWatchFollow_TailsNewLines with the identical
//     flake shape — a 10ms fixed sleep against an async offset seed inside a
//     200ms deadline (cmd_bridge_watch_test.go:311,317). The acceptance scope is
//     `-run TestRunBridgeWatchFollow`, which matches both.
//
// Predicate strategy — every predicate EXERCISES the system under test (the
// cycle-85 degenerate-predicate ban): 001–005 drive the real exported
// ledger.FileLedger.Verify over purpose-built fixture chains and over the LIVE
// ledger; 006 executes the follow test suite under -race; 008 runs the real
// looppreflight.Run against a real git repo whose local base is genuinely
// behind a real (file-remote) origin. 007 is the one shape assertion, and it is
// legitimate precisely because this task's DELIVERABLE IS the test's timing
// shape: it parses the Go AST and checks the numeric deadline literal, so it
// cannot be satisfied by adding a magic string.
```

### `go/acs/cycle1191/predicates_test.go:77` — above `const liveSealLabelPrefix = "reset-seal-"`

```text
// liveSealLabelPrefix is the marker the PRODUCTION ledger uses for an operator
// re-anchor: the `cycle_label` field, not `kind`. Verified against
// .evolve/ledger.jsonl:1880 — {"cycle_label":"reset-seal-cycle-108",
// "role":"operator","kind":"reset",...}. Predicates 001/004/005 exist because
// the salvaged resolver matches on `kind` and therefore never sees this.
```

### `go/acs/cycle1191/predicates_test.go:133` — above `func writeFixtureLedger(t *testing.T, rows []fixtureLine) string {`

```text
// writeFixtureLedger materialises rows as a real ledger.jsonl + ledger.tip in a
// fresh dir and returns that dir. Row 0 is the zero-seeded genesis; every other
// row chains from the previous line's SHA unless it is marked breakPrev. The
// tip is written to match the LAST line, so a fixture only ever fails on the
// chain property under test, never on an incidental tip mismatch.
```

### `go/acs/cycle1191/predicates_test.go:181` — above `func stateRoot(t *testing.T) string {`

```text
// stateRoot resolves the MAIN project root (the STATE root): the suite exports
// EVOLVE_PROJECT_ROOT (issue #12), else the repo root (the redteam idiom).
```

### `go/acs/cycle1191/predicates_test.go:195` — above `func TestC1191_001_seal_anchor_resolves_production_seal_shape(t *testing.T) {`

```text
// TestC1191_001_seal_anchor_resolves_production_seal_shape is the cycle-1191
// CRUX. Inbox acceptance #1: "a fixture ledger with break→seal→valid-chain
// verifies OK with an informational sealed-prefix note."
//
// The fixture writes the seal in the shape the LIVE ledger uses (cycle_label
// marker, kind "reset"). The salvaged resolver keys on kind, so today the seal
// is invisible, the walk starts at line 0, and the planted break at row 2 is
// reported as BROKEN — the exact wolf-cry the task must retire.
```

### `go/acs/cycle1191/predicates_test.go:286` — above `func TestC1191_005_live_ledger_no_1740_wolf_cry(t *testing.T) {`

```text
// TestC1191_005_live_ledger_no_1740_wolf_cry is inbox acceptance #2, asserted
// against the REAL repository ledger: "evolve ledger verify on this repo reports
// the post-seal chain status instead of the line-1740 wolf-cry."
//
// It deliberately does NOT require the live chain to be clean — the inbox itself
// expects the post-seal region may surface genuine findings (the 2026-07-22
// cycle:null promote batch). The criterion is that verification gets PAST the
// adjudicated, sealed 1740 damage. Reproduced RED on 2026-07-29:
// `evolve ledger verify` → "BROKEN: ... line 1740 prev_hash mismatch".
```

### `go/acs/cycle1191/predicates_test.go:447` — above `func behindBaseRepo(t *testing.T) string {`

```text
// behindBaseRepo builds a real work tree on branch main whose local base is one
// commit BEHIND a real (file-remote) origin/main — the exact topology that made
// every cycle-969 lane ship GIT_PUSH_REJECTED. No network is involved.
```
