# ACS cycle allocation recovery floor

Date: 2026-09-12

Branch: `fix/acs-cycle-allocation-floor`

Worktree: `dev/acs-cycle-allocation-floor`

## Problem and impact

Fresh-cycle allocation trusted only two mutable fields in `.evolve/state.json`:
the highest completed cycle and the highest leased cycle. That is sufficient
while the state file and repository history always move together. It is unsafe
after a clean clone, state restoration, or state reset when the source tree
already contains historical `go/acs/cycle<N>` packages.

With empty state and a tracked `go/acs/cycle1`, the allocator returned cycle 1.
The TDD profile is intentionally allowed to write the current cycle's ACS
package, so the new run could modify the historical package instead of creating
a new one. The existing polluted-workspace archive does not protect source
files; it runs after allocation and covers `.evolve/runs/cycle-N`, not
`go/acs/cycleN`.

This is an identity-allocation defect. Tightening a model prompt would leave the
host capable of minting an already-occupied identity and would depend on every
model noticing the collision.

## Recovery contract

Fresh allocation now uses three monotonic inputs:

1. `lastCycleNumber`, the highest completed cycle in state;
2. `lastAllocatedCycleNumber`, the highest lease already minted in state;
3. the highest occupied canonical `cycle<N>` entry in the Go module's `acs/`
   directory.

The next identity is one greater than their maximum. The source floor is
discovered before any lease, workspace, worktree, or model dispatch. For the
production storage path, that floor is combined with both state counters inside
the existing serialized `UpdateState` operation. Concurrent allocators may read
the same source floor, but the second update sees the first allocator's persisted
lease and advances again.

The checkout used for that first scan can be stale. Production worktree creation
fetches the upstream integration ref after allocation, so it can provision a
commit containing a cycle package that was absent locally. A second, exact-path
check therefore runs against the created worktree. If the selected `cycle<N>` is
occupied, setup returns an error before cycle-state persistence or phase
dispatch. The error reports and preserves the worktree: `Create` may have reused
an existing recovery location, and its interface does not grant deletion
ownership. The lease remains burned, so the next fresh run advances. This keeps
the collision check outside resume, which must retain the interrupted run's
identity.

Legacy storage adapters without `UpdateState` still advance above the greater of
the completed-state counter and source floor. Their pre-existing lack of
cross-process serialization is unchanged.

Resume does not use this path. It retains the cycle and run identity recorded by
the interrupted run, which is required for artifact and ledger continuity.

## Component boundaries

`internal/acssuite` owns parsing and inventory of ACS package identities because
it already owns the canonical `./acs/cycle<N>` spelling. Its scanner accepts
only positive canonical decimal names (`cycle1`, `cycle42`), ignores unrelated
or noncanonical entries, and treats any occupied canonical name as reserved.
Counting a file or symlink is deliberate: creating a directory at that same path
would still collide.

`internal/codequality.ResolveModuleDir` is the checked resolver for root-level Go
modules versus this repository's nested `go/` module. It distinguishes a missing
`go/` directory from inspection failures and recognizes a nested module by its
`go/go.mod` file. That marker matters after provisioning: guard setup creates a
support-only `go/bin` directory even for root-level modules, and directory
existence alone would redirect the ACS check away from root-level `acs/`.
Existing formatting callers retain the compatibility helper `ModuleDir`;
allocation and collision checks use the checked form and fail closed.

`internal/core/cycle_source.go` composes those two owners into the local floor and
provisioned-source checks. Core retains responsibility for state allocation,
worktree preservation, and error context. No second state writer or lock was
introduced.

Missing ACS trees return floor zero and preserve state-only allocation.
Inventory errors fail closed before state mutation because proceeding would
reintroduce the overwrite risk. A maximum integer in either source or persisted
state produces a cycle-number-exhausted error instead of wrapping negative. An
inspection failure after provisioning occurs after the lease, but still before
cycle-state persistence and model dispatch; it preserves and reports the
worktree with the full source context.

## TDD evidence

The first orchestration regression created empty state beside an occupied
`go/acs/cycle41`. Before production changes, `newCycleRun` allocated and
persisted cycle 1; the assertion required cycle 42.

The ACS inventory tests were then written before their implementation. They
initially failed to compile because `HighestCyclePackageNumber` did not exist.
They define canonical parsing, missing-tree behavior, occupied non-directory
paths, malformed and overflowing names, and read-error propagation.

After the scanner passed in isolation, the original orchestration regression
remained RED. Wiring the floor into the lease turned it GREEN. Architecture
review then identified integer exhaustion as an uncovered boundary. Two new
tests failed by assertion because source and persisted `MaxInt` values wrapped
without error. A bounded increment turned them GREEN. A 16-goroutine race test
starting from source floor 100 verifies unique leases 101 through 116.

The architecture review found that `gitWorktree.Create` fetches upstream after
the local source scan. An integration-tier real-Git RED test held a root-module
clone at cycle 41 while a bare remote advanced to cycle 42; the old path
provisioned cycle 42 and returned success. Its first guard revision was also RED
because worktree setup synthesizes `go/bin`, which a directory-only resolver
mistook for a nested module. The module-marker resolver and provisioned-source
guard turn the case GREEN. The test verifies no cycle state is written, the
colliding worktree is preserved and reported, and lease 42 remains burned. Git
fixture commands have bounded contexts and isolated signing, hook, global
configuration, system configuration, and terminal-prompt settings.

A separate RED symlink-loop regression showed that module discovery had hidden
`go/` inspection errors and continued through allocation. The checked resolver
now stops that path before a lease or worktree. A post-provision inspection-error
test verifies the same fail-closed boundary while preserving a possibly reused
worktree. Parser fixtures include both a bare `cycle` name and an all-digit value
too large for `int`.

## Verification and review plan

The final tree is gated in this order:

1. ACS inventory and Core allocation tests, including `-race` on concurrent
   floor-aware allocation;
2. complete `internal/acssuite` and `internal/core` package suites;
3. `gofmt -l .`, `go vet ./...`, and `go test -count=1 ./...`;
4. architecture, Go-test, general code-review, and simplification reviews over
   the staged diff;
5. the repository-native tree-bound commit gate, PR CI on Ubuntu and macOS, and
   exact-head merge.

## Scope limits

The local floor plus provisioned-source check prevents a fresh run from writing
an already occupied Go ACS cycle path. It does not make historical predicates
immutable, and it does not coordinate independent repositories that deliberately
use separate `.evolve` state stores. The post-fetch check covers the exact
selected identity; it is a collision guard rather than a second allocator. A
rejected setup is not resumable because no cycle state was initialized; its
reported worktree is retained only for inspection and explicit reclamation.
Legacy bash cycle predicates are retired and are not used as another allocation
authority.
