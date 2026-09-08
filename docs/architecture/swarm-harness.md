# Swarm harness — supported contract

Reader swarm and the single-runner shadow path are supported. Live writer swarm is unsupported: its integration tree, acceptance authority, and worker guard dependencies have not been connected into the full cycle lifecycle.

## Modes

| Configuration | Reader phase | Writer phase |
|---|---|---|
| `swarm.stage=shadow` (default) | Delegate to ordinary phase runner | Delegate to ordinary phase runner |
| `swarm.stage=advisory` | Run ordinary phase and collect reader observations | Reject a usable writer swarm plan before dispatch |
| `swarm.stage=enforce` | Synthesize worker reports | Reject a usable writer swarm plan before dispatch |

No plan or a plan that collapses to a single worker uses the ordinary runner. The configuration comes from `.evolve/policy.json`; old `EVOLVE_SWARM_STAGE` instructions are historical.

The rejection names the missing authority and recommends shadow or fleet lanes. It occurs before the ordinary writer or workers perform side effects. It must not silently report a merge as a completed build.

## Why writer activation is deferred

The merge helper supports acceptance and conflict callbacks, but those are not supplied by the production decorator. Its separate integration worktree also does not become the cycle's authoritative worktree, and provisioning can omit pending TDD work. Helper-level tests do not establish end-to-end shipping semantics.

Activation requires a dedicated change proving pending TDD work plus worker changes reach the exact tree Audit and Ship validate, with cancellation, conflicts, invalid acceptance, resume, and cleanup covered. Prefer retaining the existing authoritative cycle worktree. Automatic conflict redispatch is optional scope, not a guarantee to invent from this historical design.

Fleet parallelism uses independent complete cycles and its own landing coordination; it is distinct from splitting one phase into writer workers.

Regression: `go/internal/phases/swarmrunner/writer_support_integration_test.go`; existing reader and shadow tests preserve supported behavior. See [current runtime contract](current-runtime-contract.md) and [the archived v4 claim](../private/research/archived-2026-09-09/architecture-swarm-harness.md).
