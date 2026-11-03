# internal/prefixqueue

> Decision records: [ADR-0078](../adr/0078-fleet-landing-prefix-queue.md) (the prefix-queue landing composer) and [ADR-0147](../adr/0147-the-wave-orchestrator-owns-integration-and-decides-when-the-wave-reaches-main.md) (the wave orchestrator owns integration). The design notes, the invariants and the findings of the queue are in [internal-fleet.md](internal-fleet.md).

## Purpose

`internal/prefixqueue` holds the prefix queue of the landing composer: `PrefixQueue`, `LaneCandidate` and `RiskTier`. The package imports only `sync`, so each package can import it.

## Design

- **A leaf package.** The code was in `internal/fleet` until 2026-11-03. The compose engine of `internal/integration` (ADR-0147 I2) uses `ResolveCulprit`. `integration` cannot import `fleet`, because `fleet` imports `guards`, `guards` imports `core` and `core` imports `integration`. So the queue moved to this package.
- **The aliases in `fleet`.** `fleet` keeps `type PrefixQueue = prefixqueue.PrefixQueue`, the other types, the tier constants and `NewPrefixQueue`. So `phases/ship` and the predicates of cycles 975, 981 and 982 compile with no change. `TestPrefixQueueAliases_AreThePrefixQueuePackage` pins the aliases.
- **`LandingMode` stays in `fleet`.** The landing mode is the policy of the fleet ship, and the queue does not read it.

## Invariants

- Each exported method takes the mutex. Pinned by `TestPrefixQueue_ConcurrentEnqueue_NoLostWork` and by `go/acs/cycle982`, which reads this file for `sync.Mutex`.
- After `ResolveCulprit`, `verify(landed)` holds. Pinned by `TestPrefixQueue_ResolveCulpritTrimsARedUnionOfGreenGroups`.
- A lane that shares a file with the open group starts a new group. Pinned by `TestPrefixQueue_ASharedFileStartsANewGroup`.
