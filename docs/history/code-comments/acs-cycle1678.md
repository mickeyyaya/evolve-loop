# Comment history: `acs/cycle1678`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1678/predicates_test.go:3` — above `package cycle1678`

```text
// Package cycle1678 materializes the acceptance criteria of the TWO inbox
// items this fleet lane committed (lane-scope.json todo_ids ∩
// triage-report.md ## top_n) — and nothing else (R9.3):
//
//	inbox-console-worklist-view  (priority M, weight 0.70, code)
//	ledger-verify-seal-anchor    (priority M, weight 0.70, code)
//
// The lane's third scoped id, kb-graph-projector, was triage-DEFERRED and gets
// ZERO predicates here (a predicate gating deferred work starves the committed
// task — cycle-280).
//
// # The two gaps
//
// inbox-console-worklist-view (ADR-0074 F8). Routing authority is already
// typed plumbing: inboxbatch.PartitionConsole splits the backlog into
// lane-dispatchable and console-routed (operator-owned) with one reason per
// routed item, triage consumes it (triage.go inboxBatchesSection), and
// inboxmover.Claim refuses a console-routed draw with exit 3. The OPERATOR's
// own command does not: cmd_inbox.go runs Classify over EVERY loaded item, so
// `evolve inbox batches` presents operator-owned work as a selectable lane
// batch with no reason and no separation. Measured on this worktree before the
// change, a mixed fixture renders:
//
//	3 items -> 2 batches
//	- batch 1 (weight 0.96; campaign camp-x): operator-work, lane-work
//	- batch 2 (weight 0.92; no shared signal): role-gate-fix
//
// — both console-routed items inside the selectable listing. That is the RED.
//
// ledger-verify-seal-anchor. The work is CARRIED in this worktree by the
// cycle-1677 continuation snapshot (ADR-0076) and is absent from main, so this
// cycle is the one that ships it. `go/acs/cycle1677` is not in the regression
// set and therefore does not run this cycle; re-binding the criteria here is
// the only way THIS cycle's gate proves the ledger behaviour it merges.
//
// # AC map (1:1 with test-report.md ## AC-Materialization)
//
//	A1  evolve inbox output separates console-routed items with reasons
//	    (route field + protected-files derivation)          → 001, 002, 003, 011
//	A2  console_routed is a TERMINAL bucket — an excluded item is
//	    accounted once, never re-deferred per cycle         → 004
//	A3  go test -race on the touched package passes         → 005
//	B1  break → seal → valid tail verifies with an informational
//	    sealed-prefix note; a break AFTER the last seal is BROKEN → 006, 007, 008
//	B2  the live repository ledger reports its post-seal status → 009
//	B3  go test -race ./internal/adapters/ledger passes      → 010
//
// # Adversarial axes (skills/adversarial-testing §6)
//
// NEGATIVE. 002 is the anti-no-op killer for the inbox half: an implementation
// that prints a console header unconditionally, or that sweeps EVERY item out
// of the lane listing, greens 001 and fails 002. 007 is the killer for the
// ledger half: an implementation that unconditionally prints a sealed-prefix
// line and exits 0 greens 006/009 and fails 007. 004's third arm refuses a
// blanket-refusing claim floor.
//
// EDGE/OOD. 003 covers the empty inbox, an unknown argument, and a --max with
// no value. 008 covers a chain with no anchor at all. 007 covers a tail forged
// one line past the anchor.
//
// SEMANTIC. Separation (001), non-separation when nothing is routed (002),
// argument/empty behaviour (003), terminal-bucket enforcement (004), race
// cleanliness (005, 010), sealed-prefix provenance (006), refusal (007),
// derivation-not-literal (008), live corpus + read-only-ness (009), and
// text/JSON path parity (011) — ten distinct behaviours, not one restated.
//
// # Flaky-shape contract
//
// No `./...` sweep and no banned suite: the only nested `go test` invocations
// are ONE named package each (./internal/inboxbatch, ./internal/adapters/ledger
// — measured 1.4s and 14.4s on this worktree), never ./internal/core or
// ./cmd/evolve. No wall-clock bounds; the one contended read (the LIVE ledger
// in 009) is a stat-stable snapshot with retry, which is a STATE poll. No
// literal PIDs. No bare `git` or `go` resolving cwd — every invocation is
// -C anchored. No un-reaped load generators. The CLI is built ONCE in TestMain
// and every assertion runs that binary.
//
// # Reachability probe (cycle-644 rule)
//
// This package imports only pkg/acsassert and the standard library — a leaf
// that pins no import edge and names no internal symbol. The Builder is free
// to choose each seam's shape (a new render helper, a partition-aware Config,
// a second method); the frozen contract is the CLI's observable output, which
// is the surface both items' acceptance criteria are written against.
```

### `go/acs/cycle1678/predicates_test.go:107` — above `var (`

```text
// ---------------------------------------------------------------------------
// Harness: the real CLI, built once (the cycle-1648/1659/1666/1677 TestMain
// shape).
// ---------------------------------------------------------------------------
```

### `go/acs/cycle1678/predicates_test.go:203` — above `protectedPath = "go/internal/guards/role.go"`

```text
// protectedPath is a ProtectedSurfaceManifest member (the cycle-1036 burn:
// the role gate is a surface a lane structurally cannot write). If the
// manifest ever drops it, 004's second arm fails loudly — that is the pin,
// asserted through a production caller rather than by importing guards,
// which keeps this package a leaf.
```

### `go/acs/cycle1678/predicates_test.go:908` — above `func TestC1678_011_JSONPathCarriesTheSamePartitionAsTheTextPath(t *testing.T) {`

```text
// TestC1678_011_JSONPathCarriesTheSamePartitionAsTheTextPath pins the OTHER
// production path through `evolve inbox batches`. `--json` is a separate
// branch in runInbox (cmd_inbox.go), and a partition wired only into the text
// renderer would leave every machine consumer reading operator-owned work as
// dispatchable — the wired-into-one-path-only defect (#373).
//
// The assertion is on the partitioner's REASONS rather than on a JSON schema,
// so the Builder stays free to shape the document: today the raw item fields
// (`"route": "console-manual"`, the files entry) are present but PartitionConsole's
// reasons are not, which is exactly the difference between dumping items and
// reporting a routing decision.
```
