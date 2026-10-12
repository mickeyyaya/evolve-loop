# internal/loopchain

## Purpose

`internal/loopchain` is the loop's chain engine (decomposition unit 13, ADR-0103). It has two parts. `Driver` is the outer batch-chaining loop. At every boundary it decides if another batch may start. After each batch it decides how the exit code moves the chain.

`Refresher` is the boundary binary refresh. It rebuilds and re-execs when the running binary has fallen behind HEAD.

Both take their process collaborators explicitly, so the leaf stays core-free. The host's package-level test seams are projected into them at call time. Design: [decomposition unit 13](../decomposition/13-loopwave.md).
## Design

- `StartDecision` is pure. It decides if batch n may start. `ContinueDecision` is also pure. It maps a finished batch's exit code onto the next move.
- The refresh runs the guards, then rebuild and re-pin, then arm and exec. Every refusal degrades to one `LOOP_BOUNDARY_REFRESH_SKIPPED` and `refreshed=false`.
- `Result` is both the run's outcome and the chain's stdout wire schema. The host marshals it as is.
- Failure modes are `loop.warning` WARN and `loop.halt` INCIDENT under module `loop`. The `[chain]` report lines are INFO-shaped and stay on the warn writer.
- The on-disk names are the re-exec breaker marker and the boundary-refresh audit trail, `RefreshLogEntry`.

## Invariants

- Precedence at a boundary: an unreadable inbox stops the loop loudly. Invalid items are named before the stop decision, so the operator sees them even on the boundary that ends the chain. The operator brake is resolved once and gates the side-effecting refresh.
- A refresh that fires is terminal, because the new process resumes the chain. The pure start decision runs last. Inside `StartDecision` the order is brake, then a drained inbox after at least one batch, then the runaway cap.
- A drained inbox is a CONTINUE condition, not a START condition. The `n > 0` scope guarantees at least one batch. Opting into chaining is therefore never weaker than the pre-chain contract. The drained-inbox exit never widens the cap, and a non-positive cap runs nothing. There is no cap+1.
- `ContinueDecision`: rc 0 (clean) and rc 3 (completed with absorbed failures) continue, because both ran to completion. rc 5 is the quota-pause contract. The batch saw every family exhausted, so a relaunch will burn the next batch into the same drained families. The chain defers with its checkpoint intact. Any other code is a fatal batch outcome and propagates unchanged.
- The fleet width is recorded per batch and read fresh. Fleet width is a hard operator commitment. A chain that silently narrows lanes looks healthy in every other signal. The operator may widen it mid-chain. The chain must never narrow it.
- The Refresher runs only between batches, so the loop body is single-threaded per boundary and needs no lock. It never halts the loop. A degrade keeps the current binary running.
- The Center is flushed before the exec, because a successful exec never returns.
- A nil dependency is a programming error. It panics at first use.
- The re-pin is provenance-gated. An empty running commit (an unstamped dev binary) is unverifiable, so the re-pin returns false. A stripped or tampered binary cannot authorize itself, and there is no sentinel substitute. A successful rebuild is not a verification.
- The Makefile stamps a 12-character short SHA, but `git rev-parse HEAD` returns the full SHA. `merge-base --is-ancestor` treats equal commits as ancestors. The stamp is therefore resolved to its full SHA first. Otherwise an up-to-date binary will always look ahead.
- The rebuilt artifact is `<root>/go/bin/evolve`, not `os.Args[0]`. `os.Args[0]` is the stale running image that the refresh exists to escape.
- The breaker marker is consulted for the running commit. An absent, unreadable or corrupt marker means no prior try. The breaker must never stop a legitimate first refresh.
- The audit trail is best-effort. A failed append must not halt the chain, because the pin has already moved. Reading the last entry returns nil when the result is clean. A missing, empty or unreadable file must not turn a successful refresh into a failure.
- The inbox count takes only the root-level `*.json` files that parse as an inbox item. An item is a JSON object with a non-empty `id`. Lifecycle subdirectories are not pending work. A root-level file that is not an item is named in `skipped` and not counted. Counting it will hold pending above zero forever and burn the chain into `max_batches` with nothing consumed.
- A missing inbox means zero pending. Any other read error stops the chain loudly. The item check is deliberately shallow, because a stricter check will silently drop items that real consumers accept.
- `BrakeEngaged` is the presence of the `loop-stop` file. It stops a batch at its next wave boundary and a chain at its next batch boundary.
- `Result.Exit` is the process exit code and never appears on the wire. `BoundaryRefresh` is omitted on every ordinary stop.
- The exit codes 3 and 5 are the named constants `exitCompletedWithAbsorbed` and `exitQuotaPause`.

## Findings

- A chain stop reason is a wire string (`chain_*`). The seven values form the summary's vocabulary. Renaming one breaks the consumers of the chain's stdout.
