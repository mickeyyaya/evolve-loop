# Comment history: `acs/cycle1435`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1435/predicates_test.go:3` — above `package cycle1435`

```text
// Package cycle1435 encodes the cycle-1435 ACS predicates for the two tasks
// triage committed from inbox item `ledger-fleet-concurrency-chain`:
//
//   - console-ledger-rebaseline-live — the root-cause code (flock-serialized
//     append, anchor-ambiguity rejection, `evolve ledger rebaseline`) already
//     shipped, but the LIVE console-plane ledger was never repaired:
//     `evolve ledger verify --deep` against the project-root `.evolve/` exits 2
//     with `BROKEN: line 114368 prev_hash mismatch`. The deliverable is an
//     operator action (run rebaseline against the real file) plus its evidence
//     artifact, not a source patch.
//   - ledger-tip-witness-doc — `knowledge/architecture/state-and-ledger.md`
//     documents `Append`'s tip rewrite but never states that `Verify`'s `want`
//     tip is `walkChain`'s re-derived `lastSha` from `effectiveAnchorSHA`
//     forward, NOT a raw `ledger.tip` sidecar read.
//
// Predicate shape notes:
//   - 001 drives the REAL compiled `evolve` CLI (go/cmd/evolve) against the real
//     project-root state directory — the wiring proof: a `Rebaseline` that was
//     never invoked against the live file leaves it RED.
//   - 002 is the anti-no-op guard for 001. 001 could be "greened" by neutering
//     verify itself, so 002 plants a known break in a synthetic ledger and
//     requires verify --deep to still exit non-zero and still say BROKEN.
//   - 004 pins the byte-for-byte sha256 of the ledger's first 114400 lines,
//     measured at RED time (35510006 bytes). Rebaseline is append-only by
//     construction; a rewrite/truncate "repair" must fail this.
//   - 005 is the only content-shaped predicate (a doc criterion) and carries an
//     explicit `acs-predicate: config-check` waiver plus a git-tracking check.
//   - Flaky-shape rules observed: no `./...` sweep, no whole-package `go test`
//     inside a predicate, no wall-clock bound, no literal PID, no bare `git`
//     (every git call is `git -C`), no load generator. One `go build
//     ./cmd/evolve`, done once for the whole package.
//
// `.evolve/ledger.jsonl` and `.evolve/ledger-rebaseline.json` are BOTH gitignored
// (`.gitignore:35 .evolve/*`) — they are runtime state, so the cycle-93
// "assert git-tracked too" rule deliberately applies only to the doc target
// (005), never to the ledger artifacts.
```

### `go/acs/cycle1435/predicates_test.go:66` — above `const (`

```text
// Prefix pin for 004, measured live at RED time on 2026-08-11:
//
//	head -n 114400 .evolve/ledger.jsonl | shasum -a 256
//	  -> b7088c02441445f50ce8cf8d48dfff7d74663429e450dabc401f566c1a291c43
//	head -n 114400 .evolve/ledger.jsonl | wc -c  -> 35510006
//
// The break verify reports (line 114368) lies inside this prefix, so the pin
// covers the damaged region rebaseline must PRESERVE rather than rewrite.
```

### `go/acs/cycle1435/predicates_test.go:94` — above `func stateRoot(t *testing.T) string {`

```text
// stateRoot resolves the STATE root: the ACS suite exports EVOLVE_PROJECT_ROOT
// pointing at MAIN even when predicates run from a worktree (issue #12), because
// `.evolve/` runtime data lives on main. Falls back to the repo root.
```

### `go/acs/cycle1435/predicates_test.go:345` — above `func TestC1435_005_TipWitnessDocumented(t *testing.T) {`

```text
// TestC1435_005_TipWitnessDocumented covers task ledger-tip-witness-doc.
//
// acs-predicate: config-check — the criterion IS documentation content
// ("the architecture doc states the tip-witness semantics"), so there is no
// system to invoke; the waiver is declared per the cycle-85 rule rather than
// dressing a grep up as behavior. It is strengthened two ways: the doc must
// name the whole re-derivation chain (not one magic token), and it must be
// git-tracked (cycle-93: a gitignored doc silently vanishes at ship).
```
