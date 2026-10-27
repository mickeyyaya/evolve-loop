# internal/continuation

## Purpose

`internal/continuation` is the one schema of the continuation binding (ADR-0076 slice C). The binding lets a later cycle resume the preserved, snapshot-committed work of a failed cycle. A cold restart is the alternative.

Three consumers share the `Continuation` type, so the schema cannot drift between writer and readers:

- The orchestrator writes the per-cycle manifest at the preserve decision.
- The inbox mover stamps released items from the manifest, in the same transaction as the release.
- The batch loader parses the stamp back off the item.

Work resumes and grades do not. A continuation carries no verdict state, and every phase runs again on adoption.

## Design

- **`Continuation` binds preserved work to an item.** `SnapshotSHA` is an immutable commit on `Branch`. Adoption checks against it, never against mutable dirty state.
- **The manifest is a per-cycle workspace artifact.** `WriteManifest` saves it atomically and `ReadManifest` loads it. `ManifestName` is exported. The audit teardown hold, the citation self-cite denylist and the defect ledger name the file through its owner. None of them uses a literal of its own.
- **The registry is the second identity class.** A lane whose scope came from the wave planner has no inbox claim and no item file to stamp. The root-owned `RegistryPath` file maps the lane-scope todo id to the same `Continuation` value as the manifest. A registry-only field or a forked format brings back the drift that this package prevents.
- **`WriteRegistryEntry` is a locked read and write.** The registry is one map that every lane of a fleet wave shares, at the shared project root. A whole-file overwrite orphans the binding of every other lane. An unserialized read and write loses a binding when two lanes read the same pre-state. Their shared `.tmp` publishes also interleave.
- **The flock sidecar serializes the write.** `flock.WithPathLock` serializes goroutines and processes. `writeRegistryEntryLocked` and `publishRegistryLocked` require the caller to hold that lock. The write path and the delete path share the one publish. A new try of the same scope replaces its own entry, so the latest preserved work wins. An empty scope id is never a key.
- **The registry has a release half.** `DeleteRegistryEntry` releases a binding without a condition. An absent scope is a clean no-op, and an empty scope id is rejected, as on the write path. Without it, entries never expired. A binding whose snapshot had landed, or whose worktree was reaped, kept the out-of-band check of the defect-ledger gate armed. Every later lane of that scope met it.
- **A conditional release holds one lock.** `DeleteRegistryEntryIfCycle` checks the ancestor cycle and deletes under one lock hold. A read and a delete under two locks lose the concurrent rebind of a sibling lane. Only the orchestrator calls the delete path. Lane agents have no write path to the root-owned registry, and this keeps the anti-tamper property of the gate.
- **Reads separate "nothing" from "broken".** A missing manifest is `(Continuation{}, false)`. An absent registry, an unknown scope and a blank scope id are clean misses. `ListRegistryEntries` returns an empty map for an absent registry, so the operator surface works on a healthy project. A present file that does not parse is an error everywhere. Schema drift must be loud and never a silent fresh start.
- **Host paths are redacted before they reach a commit.** `RedactHostPaths` collapses the home directory of the operator to `~` in `Worktree` and `FindingsPath`. Consumed inbox items are tracked, and they reach a public remote. `SnapshotSHA`, `BaseSHA` and `Branch` stay as they are, because salvage resolves work from them. An unresolvable home is a no-op, because redaction must never block a release.
- **`AppendReleased` is the one released shape.** Ship's in-commit consume, `evolve inbox consume` and the consumed-corpus reconciler all write it, and it redacts. The salvage pointer cannot drift between routes.
- **`RepairFirst` names the reds that a continuation inherits** ([failure-rate review §4.4](../../incidents/2026-08-17-failure-rate-review-1481-1503.md)). It reads the `acs-verdict.json` of the prior workspace. It keeps each red result that is not synthetic and whose `go/acs/<pkg>` directory is still in the adopted worktree. Adoption archives the ancestor's own package, which then does not run, so it is not listed. Each line names the `_test.go` file that declares the test, or the package directory. The evidence excerpt of the red is not copied, because agent text must stay inside the data fence.

  The directive has two classes. A red with `is_regression` or `is_red_team` is a shipped predicate: the directive says to fix the code and never to edit, skip or retract it. Only another red is a predicate of this task, which the builder may repair or retract. Pinned by `TestRepairFirst_ARedRegressionPredicateIsFixedInTheCodeNeverRetracted`.

  A package name outside `[A-Za-z0-9_-]` is skipped, so a verdict entry cannot point outside `go/acs/`. An absent verdict is no directive, and an unreadable verdict or predicate file is an error. In cycles 1492 and 1495 two lanes rebuilt green work around one inherited red predicate. The findings described the red, but did not order its repair. That red was in the ancestor's own package, and the adoption archive (f8955a32) now removes that shape. The directive serves the reds that are left: mostly shipped predicates that the prior change breaks.
- **The release gate lives here.** `RequireOperatorAuthority` is the operator gate for every path that can drop a binding. The defect-ledger gate reads a binding as anti-tamper evidence. The phase guard denies in-process Agent dispatch, but it does not see a Bash call of the evolve binary. Without a gate, any process that can run Bash can erase the lineage of a live scope.
- **The gate is in the package, not in `cmd/evolve`.** A later in-process caller cannot route around the gate that the CLI honours.
- **Only the parameter authorizes.** `-operator` is the one authorization. X18d removed the environment fallback `EVOLVE_OPERATOR_CONFIRM` (ADR-0156 I-16), so no exported value of the environment can erase a binding. The error that the gate returns is the refusal text of the CLI, so the message and the gate cannot drift.

## Invariants

- **Only `-operator` authorizes.** `TestRequireOperatorAuthority` pins it: the gate passes with the parameter, and it refuses without it, also when `EVOLVE_OPERATOR_CONFIRM=1` is in the environment.
- **One manifest file name.** `TestManifestName_IsTheFileTheManifestRoundTripsThrough` pins it.
- **One schema.** The registry stores `Continuation` as it is, with the field names of the manifest. `TestRegistry_StoresTheContinuationSchemaVerbatim` pins it.
- **A second scope does not overwrite the first, and concurrent lanes keep their bindings.** `TestRegistry_SecondScopeDoesNotClobberFirst` and `TestRegistry_ConcurrentLaneWritesDoNotLoseBindings` pin it.
- **Absence is a clean miss and corruption is loud.** `TestRegistry_MissingFileAndUnknownScopeAreCleanMiss` and `TestRegistry_CorruptFileIsLoudError` pin it.
- **A rebind between the read and the delete survives.** `TestDeleteRegistryEntryIfCycle_ReleasesOnlyTheNamedAncestor` pins it.
- **Redaction collapses host paths and keeps the salvage refs byte-identical.** `TestRedactHostPaths` pins it.
- **No registry is an empty list.** `TestListRegistryEntries` pins it.

## Findings

- None.
