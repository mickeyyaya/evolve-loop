# Comment history: `internal/shiperr`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero h2

### `go/internal/shiperr/manifestgate_vocab_test.go:8` — above `func TestManifestGateCode_WireStringAndDistinctness(t *testing.T) {`

```text
// TestManifestGateCode_WireStringAndDistinctness pins the dedicated manifest-gate
// error code (cycle-1064, task dedicated-manifest-gate-error-code). Today the
// enforce-mode manifest gate reuses CodeGitStageFailed, so a ledger/debugger
// consumer cannot tell "the gate refused an out-of-manifest path" from "the
// underlying `git add` shelled out and failed" — and worse, GIT_STAGE_FAILED is
// classified TRANSIENT (apicover_shiperror_test.go), so an INTEGRITY-flavoured
// block inherits a retry-friendly class. Mirrors CodeCommitPrefixGate, the
// sibling gate that already owns a dedicated code.
```

### `go/internal/shiperr/repocontractinfra_vocab_test.go:5` — above `func TestCodeRepoContractInfra_Vocab(t *testing.T) {`

```text
// TestCodeRepoContractInfra_Vocab pins the cycle-1409 infra-classification
// code's wire value and — the load-bearing half — its NON-ALIASING of
// CodeRepoContractGate. The whole point of the split is that a router or
// dashboard can distinguish "a repo-wide guard suite is genuinely RED in this
// lane" (fix your code) from "the scanner pack died twice with no test-level
// failure" (infra fault, safe to re-dispatch). If these two ever collapse onto
// one value, the cycle-1402/1403/1405 false-RED storm is back with no signal.
```

### `go/internal/shiperr/shiperr.go:10` — above `type ShipError struct {`

```text
// ShipError is the structured ship→orchestrator error protocol. The ship
// phase is a PURE EXECUTOR: it verifies the process is right and executes
// (commit / ff-merge / push), but it has NO power to reject a cycle's changes
// — that decision was already made by audit. When ship cannot execute, it
// returns a ShipError carrying a precise Code, a severity Class, the Stage it
// failed at, and a Debug map with enough content to diagnose the problem. The
// orchestrator (not ship) decides what to do with it: route to the debugger
// phase, re-run an upstream phase, or block.
//
// It lives in this zero-dependency leaf (internal/shiperr), imported by BOTH the
// ship phase (which constructs it) and core's orchestrator (which errors.As
// matches it) — so the shared protocol type can never form an import cycle in
// either direction. core re-exports the symbols via aliases for back-compat.
//
// Why a structured envelope: the old boundary collapsed ~50 distinct ship
// failure points into one generic "ship: native exit=N" + joined logs, so the
// orchestrator could not tell a transient push race from a stale audit-binding
// from a real tampering breach. Each is now a distinct Code with its own Class
// and Debug context (cycle-148/150/151 incident family).
```

### `go/internal/shiperr/shiperr.go:96` — above `CodeControlPlaneViolation ShipErrorCode = "CONTROL_PLANE_VIOLATION"`

```text
// verify-class — pipeline control-plane integrity boundary (ADR-0064).
// A --class cycle commit whose diff touches the integrity surface (gates,
// metric SSOT, guards, contract, grading rubrics, hook wiring) — a cycle may
// not edit the gate that grades it. Precondition-class (not retryable); the
// fix is to ship the control-plane change via --class manual instead.
```

### `go/internal/shiperr/shiperr.go:118` — above `CodeGitFleetRebaseNeeded ShipErrorCode = "GIT_FLEET_REBASE_NEEDED"`

```text
// CodeGitFleetRebaseNeeded (ADR-0049 S5b): under fleet mode a ff-merge
// divergence is EXPECTED — a peer cycle moved main while this cycle was
// mid-pipeline. NOT a terminal failure: the cycle must rebase onto the new
// main and re-verify the merged tree (test-the-merged-tree / merge-queue
// pattern) before re-shipping. Transient so the failure floor routes it to
// recovery rather than aborting the cycle.
```

### `go/internal/shiperr/shiperr.go:125` — above `CodeGitFleetRebaseConflict ShipErrorCode = "GIT_FLEET_REBASE_CONFLICT"`

```text
// CodeGitFleetRebaseConflict (ADR-0049 G13a): the fleet rebase onto main hit a
// genuine MERGE CONFLICT — two concurrent cycles edited the same lines, which
// the advisor's disjoint-file partition should have kept apart. Unlike
// RebaseNeeded (a clean replay → re-audit), a conflict cannot be re-audited
// away, so it is an INTEGRITY-class blocker routed to the debugger for triage
// (recommend sequential retry / partition split), not a silent abort.
```

### `go/internal/shiperr/shiperr.go:134` — above `CodeManifestGate ShipErrorCode = "MANIFEST_GATE"`

```text
// CodeManifestGate (cycle-1064): the ship-bind manifest gate refused to
// commit path(s) no build/TDD report declared — the cross-lane untracked-leak
// guard firing under enforce. Dedicated rather than reusing
// CodeGitStageFailed (a TRANSIENT, retry-friendly `git add` failure) so the
// ledger/debugger can tell an integrity-flavoured BLOCK from a flaky stage:
// re-running audit can never re-establish the precondition, only re-declaring
// the path in a phase report can. Mirrors CodeCommitPrefixGate, the sibling
// ship-local gate that already owns a code.
```

### `go/internal/shiperr/shiperr.go:143` — above `CodeRepoContractGate ShipErrorCode = "REPO_CONTRACT_GATE"`

```text
// CodeRepoContractGate (2026-08-05): the ship-time repo-contract scanner
// pack found a repo-wide guard suite RED in the lane worktree — pushing
// would turn main red (the week's four scope-disease landings). Distinct
// code so the ledger/debugger can tell a contract block from git failures.
```

### `go/internal/shiperr/shiperr.go:148` — above `CodeRepoContractInfra  ShipErrorCode = "REPO_CONTRACT_INFRA"`

```text
// CodeRepoContractInfra (cycle-1409): the repo-contract scanner pack exited
// nonzero TWICE without a single test-level failure event — the toolchain
// died (build-cache contention, module fetch, OOM kill), the guard suites
// did not say "your code is broken". Deliberately NOT an alias of
// CodeRepoContractGate: before this split, `cmd.Run()`'s bare exit error was
// wrapped as a contract RED, so an infra flake blocked three audit-green
// ships (cycles 1402/1403/1405 — the preserved worktree re-ran 4/4 GREEN on
// the identical tree). The router/debugger need to tell "fix your code" from
// "safe to re-dispatch"; precondition-class because a re-dispatch can
// re-establish it, unlike a genuine contract violation.
```

### `go/internal/shiperr/signalcodes.go:3` — above `import (`

```text
// signalcodes.go — ADR-0101 S2: the ship vocabulary in the Signal Center's
// code space. Projection, not copy: SignalCode(c) is "SHIP_" + c, and every
// code is registered under the ship module with a one-line doc so
// docs/architecture/signal-codes.md and the ship.error events share ONE
// vocabulary. AllCodes is the sorted list the registration walks; the vocab
// test pins each constant's wire string, this table pins that each has a doc.
```

### `go/internal/shiperr/signalcodes.go:79` — above `const (`

```text
// The Debug keys the ONE ship.error producer projects onto the signal line
// (ADR-0103 unit 07), each spelled ONCE so a producer cannot respell one out
// of the projection (the field would still reach ship-error.json but vanish
// from the ship.error event). Homed here, the leaf both the ship phase and
// core import, so the landing stamps them and core reads them without either
// importing the other. StepKey names the landing step an error was built in:
// "integrate" for the ff-merge, "push" for the push and its inline repair,
// "binding" for the ship-binding witness.
```

### `go/internal/shiperr/signalcodes_test.go:3` — above `import (`

```text
// signalcodes_test.go — ADR-0101 S2: every ShipErrorCode is projected, not
// copied, into the Signal Center's code space as SHIP_<code>, registered under
// the ship module with a one-line doc so docs/architecture/signal-codes.md and
// the ship.error events share ONE vocabulary.
```

### `go/internal/shiperr/signaldebug_test.go:8` — above `func TestShipErr_StepKeyAndSignalDebugKeys(t *testing.T) {`

```text
// ADR-0103 unit 07 — the step vocabulary's key, the five other whitelisted
// Debug keys and the whitelist the ONE ship.error producer (core's
// emitShipError) projects into Event.Fields. All live here, the leaf the ship
// phase and core already import, so the landing stamps the keys and core
// reads them without importing the landing — and every key has ONE spelling
// (a producer respelling a key would otherwise still write ship-error.json
// while the field silently vanished from the signal line).
```
