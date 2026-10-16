# internal/failureadapter

## Purpose

`internal/failureadapter` is the deterministic failure-adaptation kernel. `Decide` is a pure function over the `failedApproaches` entries of `state.json`. It returns a `Decision`: an `Action`, an optional `Verdict`, environment to set, and `Evidence` counts. The orchestrator follows the `Decision` verbatim. `ListPendingByClass` lists the entries that are still inside their retention window. The package ports `scripts/failure/failure-adapter.sh`.

## Design

- **No I/O.** The package takes no process, file or clock dependency, except `Options.Now`. `Options.Now` defaults to `time.Now`, and tests override it. The caller reads the state and passes the entries in.
- **Retention is per classification.** `AgeOutSeconds` maps each `Classification` to its window: one hour, one day, seven days, thirty days, or effectively never. An unknown classification gets one day.
- **An entry is kept in three cases** (`isNonExpired`):
  - an explicit `expiresAt` in the future;
  - both `expiresAt` and `recordedAt` missing (a truly legacy entry);
  - `expiresAt` missing but `recordedAt` present, which gets a one-day TTL.
- **Unparseable times follow the bash `try ... catch`.** An unparseable `expiresAt` counts as still in the future. An unparseable `recordedAt` counts as 1970, so the entry is expired.
- **The rules run in a fixed order. The first match wins.**
  1. Any intent-rejected entry.
  2. Infrastructure-systemic.
  3. Two or more distinct cycles with code-audit-fail.
  4. Two or more distinct cycles with code-build-fail.
  5. A tail streak of three or more infra-transient entries.
  6. One or more infra-transient entries. Strict mode retries with fallback. Other modes merge into `set_env` and proceed.
  
  No match proceeds.
- **Recurrence counts distinct cycles, not entries.** One cycle that fails twice is not a recurring failure. The tail streak walks the entries in reverse. It stops at the first entry that is not infra-transient (or the legacy `infrastructure`).
- **The systemic summary is cut to 200 bytes.** This matches `head -c 200` in the bash source.

## Invariants

- **The bash source stays canonical until the v11.0 cutover.** Both implementations must give the same `Decision` for the same input. This keeps the trust kernel host-CLI-agnostic. The tests cite the bash line that each rule replicates.
- **No block means no verdict.** A PROCEED leaves `VerdictForBlock` at `VerdictNone`. A strict-mode block carries a concrete verdict. `TestDecision_VerdictNoneOnProceed` pins this.
- **`ListPendingByClass` keeps the input order.** It uses the same expiry rule as `Decide`. The count in `evolve guard list-audit-fails` therefore matches the kernel's count.
