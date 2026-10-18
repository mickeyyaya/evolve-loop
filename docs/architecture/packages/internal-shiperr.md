# internal/shiperr

## Purpose

`internal/shiperr` is the structured ship-to-orchestrator error protocol. The ship phase is a pure executor. It verifies the process is right, then commits, fast-forward merges or pushes. When it cannot execute, it returns a `ShipError` with a precise code, a severity class, the stage that failed and a debug map. The orchestrator decides what follows: route to the debugger, re-run an upstream phase, or block. Ship never rejects a cycle's changes, because audit already decided that.

## Design

- **Zero-dependency leaf.** The ship phase builds the error and `core` matches it with `errors.As`. Both import this package, so the protocol type cannot form an import cycle. `core` re-exports the symbols through aliases, so existing call sites do not change.
- **Classes.** `transient` means a retry may succeed. `precondition` means a stale or missing upstream fact that a re-run can re-establish. `integrity` means a real breach, such as tampering or tree drift, and the debugger blocks it unless it can prove the change safe. `config` means an operator error. Every class still goes to the orchestrator's resolver.
- **Distinct codes.** A failure that a router must treat differently has its own code, not an alias of a broader one. Examples: a fleet fast-forward divergence is transient, because a clean rebase and re-audit fixes it. A fleet rebase conflict is an integrity blocker. A manifest refusal is not a retryable git stage failure. A repo-contract red is not a test-infrastructure death, which is retry-safe.
- **Debug map.** `NewShipError` keeps an odd trailing key with an empty value, because a lost diagnostic key is worse than a blank one. `DebugString` sorts the keys, so ledger output is deterministic.
- **Landing conflicts.** `IsLandingConflict` is true for the signal codes of `GIT_FLEET_REBASE_NEEDED`, `GIT_FLEET_REBASE_CONFLICT` and `AUDIT_BINDING_HEAD_MOVED`. `internal/wave` uses it for the conflict measure, and the oversight of the orchestrator uses it to mark a lane that rebased. The three codes live only here (`TestIsLandingConflict_IsTrueForTheThreeRecoveryCodesOnly`).
- **Signal projection.** Each code projects into the Signal Center as `SHIP_<code>`. The integrity class is an incident, and every other class is a warning. `SignalDebugKeys` is the whitelist of debug keys that reach the signal line. The rest stays in `ship-error.json`.
- **Rewritten codes.** The key `original_code` carries the code that a host rewrote before recovery. It is on the whitelist, so the signal line keeps it. Pinned by `TestShipErr_StepKeyAndSignalDebugKeys`.
- **Source-derived tests.** The vocabulary tests parse `shiperr.go` for the declared constants. A new code or class therefore cannot go undocumented or unregistered.

## Invariants

- Each wire string is exact, because ledger entries and the debugger key off it. Pinned by `TestVocabularyWireStrings`.
- The codes that a router must tell apart do not collapse into one value. Pinned by `TestCodeRepoContractInfra_Vocab`, `TestCodeRepoContractGate_Vocab`, `TestCodeManifestGate_DistinctFromEntireVocabulary` and `TestManifestGateCode_WireStringAndDistinctness`.
- A `ShipError` survives wrapping and renders one stable line. Pinned by `TestAsShipError` and `TestNewShipError_Error`.
- An odd debug key is kept with an empty value. Pinned by `TestNewShipError_OddKV`.
- The debug string is deterministic. Pinned by `TestDebugString_Sorted`.
- Every declared code has a signal doc, and every declared class has a severity row. Pinned by `signalcodes_test.go`.
