# internal/phases/ship/landing

## Purpose

`internal/phases/ship/landing` is the fleet landing of the ship phase. It runs the fast-forward merge of a cycle branch, the push with its inline repair, the post-push head read and the ship-binding witness. It is a leaf under the ship phase. The host composes it and keeps the preflight, the verification read and its own log lines.

## Design

- **Leaf imports.** The package imports the standard library and three named internal packages. It never imports `internal/core` or the ship phase. `importgraph_test.go` declares this, and the compiler is the cycle guard.
- **Injected context.** The leaf does not read the environment. The host passes the fleet flag as a bool, the phase name as a string, and the run identity through `WithRun`. Source scans pin both rules.
- **Streams read at use.** `New` takes the git port and a streams accessor. The accessor is read at every use, because the host sets its streams after construction. A nil git port panics at first use.
- **Null objects.** A nil signal accessor, a nil center and a nil log sink are no-ops. `SignalsWired` reports if a center is reached.
- **Errors.** Every error the leaf returns is built at one site, `fail`, with the landing step stamped into the debug map. The divergence class depends on the fleet flag. Under a fleet, a diverged merge is transient and a peer's rebase recovers it. Otherwise it is a precondition that the repair ladder may fix.
- **Push repair.** The inline repair never rebases and never force-pushes. The once-per-run guard and the dry-run skip both arrive in the request, because the host owns them. Origin divergence is reclassified as `needs-reaudit`.
- **Post-push head.** A failed head read after the push is a warning, not an error. The push has already landed.
- **Binding witness.** `WriteBinding` writes `ship-binding.json` atomically through a shared dossier type, so a field rename is a compile error. A write failure is returned as an error, because the callers must still see it. The push has already landed at that point.
- **Limits.** `limits_test.go` enforces the clean-code limits: every function under 50 lines, nesting depth at most four, every file under 800 lines.

## Invariants

- The leaf never imports core or the ship phase. Pinned by `importgraph_test.go`.
- The leaf never writes stderr and never reads the environment. Pinned by `TestLeaf_NoHandWrittenStderrOrEnvReads`.
- Every whitelisted debug key is spelled through its shiperr constant. Pinned by `TestLeaf_SpellsTheWhitelistedDebugKeysThroughShiperr`.
- The landing reproduces the host's goldens for every scenario it owns. Pinned by `TestPush_ArgvAndLogSequencesMatchTheGoldens`.
- A green landing emits no signal. A fault emits its code once, and no event is terminal. Pinned by `TestLanding_NeverEmitsShipErrorAndHappyPathsAreSilent`.
- The repair outcome strings are fixed. Pinned by the postship tests and the `acs/cycle752` pin.
- `--dry-run` still performs the push. Only the repair is skipped, under a dry run or after the once-guard is spent. Pinned by `TestPush_OnceGuardAndDryRunSkipTheRepair`.
