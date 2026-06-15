# ADR-0050: Modularization, Dedup, and Unified Phase I/O — Campaign Charter

- **Status:** Accepted — campaign charter. Tracks the multi-PR program executed from plan `happy-petting-wreath.md`. Each slice lands as its own small, TDD'd, dual-reviewed, ship-gated PR with a decision-log line; this ADR's status advances to **Implemented** in Phase 6. No code-behavior change ships outside an explicit flag (`EVOLVE_PHASE_IO`, default `off`).
- **Date:** 2026-06-15
- **Driver:** an operator request to *"modularize, dedup, and make every basic component an independent dependency-injected module unit-tested in isolation with 100% public-API coverage; re-architect phase I/O into a unified isolated envelope so phases are true pipes-and-filters; keep the trust kernel byte-identical."*
- **Evidence:** plan `happy-petting-wreath.md` (the executable slice list) + `docs/architecture/audit-2026-06-15-package-map.md` (the package inventory / dependency graph / dedup register produced in Phase 0.3).
- **Relates to:** ADR-0034 (deliverable contract + `EVOLVE_CONTRACT_GATE`), ADR-0035 (unified phase descriptor — `PhaseRequest`/`PhaseResponse`), ADR-0044/0045 (the `shadow→advisory→enforce` rollout pattern this campaign reuses), ADR-0049 (concurrent multi-cycle — the `RunID` identity this envelope must carry), PRs #98/#99 (verbatim Lego-splits), PRs #100/#101 (`orchestrator.go` 3,555→546 LOC decomposition + `RunCycle`→`cycleRun` method object, shipped with characterization tests).

## Problem

`evolve-loop`'s Go codebase (~110 internal packages) is dominated by two hubs — `core` (~14.7K LOC, 201 packages import it) and `bridge` (~14.2K LOC). The orchestrator god-file is already decomposed (PRs #100/#101), but three structural problems remain:

1. **Phases are not isolated filters.** The `PhaseRequest`/`PhaseResponse` envelope exists (`go/internal/core/phase.go`) but its `UpstreamSignals`/`Signals` fields are **dead**: phases instead read prior artifacts ad-hoc off disk, share a mutable `Context` dict, and share one worktree per cycle. Phases are mutually coupled, hard to test in isolation, risky to change.
2. **Duplication on the leaves.** Git `exec.Command`, env-var parsing, and `fmt.Fprintf(os.Stderr, …)` logging are open-coded across many packages instead of routed through shared leaf utilities — so they cannot be faked in tests and drift independently.
3. **Sub-100% public-API coverage.** Exported symbols lack direct edge-case/concurrency tests; safety-critical concurrency seams (ledger chain, quota guard) have *zero* stress tests; line coverage is reported but the public-API surface is not enforced.

All of this must change **without ever altering the trust kernel** — the ship-gate, the ledger hash-chain, and the EGPS `red_count==0` gate must stay byte-for-byte identical.

## Decisions (the four operator decisions driving the campaign)

| # | Decision | Resolution | Rationale |
|---|----------|-----------|-----------|
| D1 | **Scope & order** | Full architecture audit FIRST (Phase 0.3), then refactor in dependency order **leaf→core**, organized as distinct phases. Hub files (`core`, `bridge`, `ship`) are touched only after their leaf dependencies are adopted — so no file is edited twice. | Minimizes blast radius; each PR stays <~400 LOC and reviewable (P9 of the principle table). |
| D2 | **Coverage target** | **100% of the public-API surface** — every exported symbol gets direct edge-case tests, enforced by a new `cmd/apicover` tool. Line coverage is *reported*, not forced to 100%. | Public API is the contract; a two-signal check (named-in-test AST ∧ `>0%` in `go tool cover`) catches false-greens that a raw line-coverage number hides. |
| D3 | **Phase I/O** | **Full re-architecture** of `PhaseRequest`/`PhaseResponse` into a unified, isolated, typed phase envelope (new leaf `internal/phaseio`), flag-gated `off→shadow→advisory→enforce`. | Turns imperatively-coordinated phases into independent pipes-and-filters (P4); the gate makes every step reversible and provable against the old path (P6/P8). |
| D4 | **Landing** | **Incremental small PRs to main**, each TDD'd (RED first), dual-reviewed, ship-gated, with an ADR/decision-log line + CHANGELOG entry per slice. | The repo's own ADR-0044/0045 precedent; Core Rules 10/12. The ledger hash-chain already binds each commit; per-slice PRs keep the audit trail granular. |

## Guiding principles (with external grounding)

| # | Principle | Source |
|---|-----------|--------|
| P1 | Deep modules, narrow interfaces (1–3 methods); accept interfaces, return structs. | Ousterhout, *A Philosophy of Software Design*; repo `rules/golang`. |
| P2 | Group by dependency, not by kind; isolate every external dependency in its own package; domain types depend on nothing. | Ben Johnson, *Standard Package Layout*. |
| P3 | Dependency Rule: dependencies point inward; core depends on ports, never on adapters. | Hexagonal / Ports-and-Adapters (Cockburn). |
| P4 | Pipeline stages are independent, stateless, aware only of their I/O schema; one stage's output is the next's input. | Azure *Pipes and Filters*; POSA. |
| P5 | Each stage gets sufficient context, is tolerant of unused fields, decides its own error propagation, is idempotent on retry. | Azure *Pipes and Filters*. |
| P6 | Refactor under a safety net: characterization/golden tests capture current behavior first; create seams; refactor while re-running the net. | Feathers, *Working Effectively with Legacy Code*. |
| P7 | TDD red→green→refactor for every change; tests verify intent, table-driven, behavior-named. | Beck, *TDD*; GOOS; repo `go/docs/testing.md`; Core Rule 9. |
| P8 | Smallest reversible steps; gated rollout; loud failure. | Repo ADR-0034/0044/0045; Core Rules 10/12. |

## Target architecture (end state)

### Unified phase envelope — new leaf `internal/phaseio` (imports only `phasespec`)
- **`PhaseInput`** — identity/roots (`Cycle, RunID, GoalHash, ProjectRoot, Workspace, Worktree, Phase, PreviousPhase`); **sealed** `Env`/`Spec`; **typed** `Upstream Handoffs` (replaces ad-hoc disk reads); **sealed** `CycleInputs` (goal/strategy/commitMessage/fleetScope/challengeToken — getters only); separate typed `*ErrorContext` and `*CorrectionState` channels (replace the mutable `Context` injections + the "## Correction" markdown blob); `WorktreeWritable bool`.
- **`PhaseOutput`** — current `PhaseResponse` fields + mandatory structured `Verdict` + `*FailureBlock`; namespaced `Signals`; recorded `WorktreeTreeSHA` (the seam for future per-phase-worktree isolation).
- **`Handoffs`** — typed per-phase views (`Scout/Triage/Build/Audit`) + `Generic(key)` plane + `Degraded()` read-miss list; built by **reusing `router.Digest`** (single on-disk-shape authority — no second reader).
- **Canonical on-disk envelope** `<Workspace>/handoff-<phase>.json` wrapping today's exact per-phase `payload` bytes plus promoted top-level `verdict`/`signals`/`failure` (Postel-compatible; `digest.go` reads `payload.*` with fallback to flat).

### Modularization end state
- **Leaf utilities (P2):** new `internal/gitexec` (isolate the git CLI); completed adoption of `internal/envchain` (typed env registry), `internal/paths` (`.evolve` layout), `internal/log` (unified logger + `Console` sink); optional `internal/jsonio`.
- **God-files (P1):** `orchestrator.go` already split (PRs #100/#101); remaining >500-LOC files (`phase_advisor.go` 708, `cyclerun.go` 490, …) kept under the 800-LOC ceiling; oversized `bridge/driver_*.go` Lego-split if >800.
- **Test infrastructure:** `cmd/apicover` public-API-coverage tool; `fixtures.StressN` concurrency helper; per-module Definition-of-Done.

## Rollout & trust-kernel invariants

- **One flag** `EVOLVE_PHASE_IO` with stages `off` (default) → `shadow` (golden-equivalence) → `advisory` (old path wins, compared) → `enforce`; circuit-breaker auto-demotes `enforce→advisory` after N blocks. Rollback at any stage = `EVOLVE_PHASE_IO=off` (or prior release tag after cutover).
- **Byte-identity guarantees** (each gated on a dedicated test before it lands):
  1. With `EVOLVE_PHASE_IO=off`, the dispatch path is byte-identical to pre-change.
  2. The ledger only *adds* files (`handoff-*.json`, shadow files); it never rewrites a ledger line. The phase→role binding (`audit→auditor`, `build→builder`) keeps the exact role vocabulary ship depends on, proven by `TestRecordPhaseBinding_*_ByteIdenticalTo*`.
  3. The EGPS gate's `red_count==0` logic is unchanged; only its *input source* moves (audit migrated LAST, disk-read authoritative under advisory, equivalence-proven before enforce).

## Alternatives considered

1. **Big-bang rewrite of the phase contract.** Rejected — violates P6/P8; no way to prove equivalence; one bad assumption corrupts the live loop.
2. **Force 100% line coverage.** Rejected — line coverage rewards test theatre; public-API coverage with a two-signal false-green check targets the actual contract (D2).
3. **Split god-files by kind (all handlers together, all types together).** Rejected by P2 — group by dependency; verbatim Lego-splits along cohesion lines (the PR #98/#99 pattern) instead.
4. **Skip the audit doc, refactor opportunistically.** Rejected — D1 mandates the full audit first so every later slice is traceable to a measured fan-in/fan-out and dedup count.

## Consequences

- **Positive:** every basic component becomes an independently unit-testable DI module with 100% public-API coverage; phases become isolated typed filters; duplication collapses onto shared leaves; the whole program is reversible (one flag) and traceable (one decision log + one ledger chain).
- **Negative / risk:** the phaseio track (Phase 3) is multi-PR and the riskiest; partial completion leaves the loop on the old path (the safe default, `off`). Mitigated by leaf→core ordering, the gate, and the byte-identity tests above.
- **Trust kernel preserved throughout:** ship-gate, ledger hash-chain, and EGPS gate stay byte-identical; `go/test/trustkernel/` + `evolve acs suite` (red_count==0) + `evolve ledger verify` gate every PR.

## Decision-log & traceability

Every slice appends one row to `docs/architecture/decision-log-modularization.md` (rows batched into a per-phase docs slice — Phase 0 → commit `56456200`, Phase 1 → PR #112 / commit `64095c18` — that lands separately from the row-less code slices, since per-slice appends collide on the shared table across the parallel code PRs), references this ADR ID, and carries its `apicover` + `-race` evidence in the per-module Definition-of-Done. The full detailed contract lives in `docs/superpowers/specs/2026-06-15-modularization-unified-phase-io-design.md`.
