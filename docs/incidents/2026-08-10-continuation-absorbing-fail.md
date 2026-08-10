# Incident 2026-08-10: continuation absorbing-FAIL + minted-phase naked dispatch (batches 3–5)

**Severity:** P0 (two batch halts in one day) · **Cycles:** 1407–1424 across batches 3–4
**Fixed by:** #428 (`3ea44496`), #429 (`e8c03f02`); operator ops batch #430 (`238ac210`)
**Related:** [2026-08-09 zero-ship batch](2026-08-09-zero-ship-batch.md) · [ADR-0084](../architecture/adr/0084-gate-integrity-invariants.md) · [ADR-0085](../architecture/adr/0085-continuation-registry-release.md)

## What happened

Batch 3 (post-ADR-0084 hardening) shipped 2/7 cycles, then was operator-halted
under the new 2-zero-ship rule (cycles 1421 FAIL, 1422 interrupted mid-audit,
1423 FAIL). A three-agent investigation (ship-gate audit / disposition-contract
audit / CI classification, each returning file:line hypotheses) found the ship
gate provably GREEN-capable and CI 29/30 green — the stall was bookkeeping.
Batch 4 then halted on its FIRST wave with a self-classified infra-systemic
failure (cycle 1424). Batch 5, carrying both fixes, resumed shipping.

## Root causes

1. **Continuation-registry absorbing-FAIL state** (cycles 1412, 1418). The
   root-owned `continuation-registry.json` had NO delete API — 84 immortal
   entries, ~60 pointing at reaped worktrees or GC'd runs. When adoption
   declines a stale snapshot (`validateContinuation` rejection,
   `continuation_stamp.go`) it provisions a fresh worktree WITHOUT a workspace
   manifest — and the defect-ledger gate's out-of-band anti-tamper check
   (`defect_ledger.go:497-504`, the cycle-1285 shield) blocks unconditionally
   on registry-binding-without-manifest. Every scope that ever preserved work
   became a permanent audit auto-FAIL: the adopter and the gate disagreed
   about the same binding, and nothing an agent authored could un-block it.
2. **Minted-phase naked dispatch** (cycle 1424). `Adapter.injectContract`
   passed prompts through unchanged when the contract resolver missed — but
   the engine polls `ArtifactPath` regardless. A minted phase
   (`defect-disposition-ledger`) was dispatched with a 2,159-byte prompt
   carrying no "## Deliverable Contract" and no "DELIVERABLE PATH:" footer;
   the agent, never told where to write, produced nothing; 600s
   artifact-timeout (exit 81) → SYSTEM halt. The hardened diagnostics (#413 /
   cycle-1409 landings) named the missing sections precisely — the halt's own
   evidence was the diagnosis.
3. **(Surfaced during the approved ledger re-anchor, still open)** ledger
   appends are not chain-safe under fleet concurrency: `prev_hash` breaks
   recur ~per cycle from mid-July (55 sequential epoch-anchors greened the
   runtime plane; the console plane's ~180+ dense breaks were left at their
   documented state), and `evolve ledger anchor <seq>` can bind BACKWARD when
   sibling entries share seq numbers. Queued as
   `ledger-fleet-concurrency-chain` (0.9) — fleet-safe appends + a native
   one-shot `ledger rebaseline`.

## Fixes

| Fix | Where | Mechanism |
|---|---|---|
| Registry release on adoption decline | #428, `internal/continuation/registry.go` + `internal/core/continuation_stamp.go` | `DeleteRegistryEntry` + single-lock `DeleteRegistryEntryIfCycle` (check-and-delete under ONE flock hold — the review-blocked TOCTOU is pinned by test); orchestrator-side `releaseDeclinedBinding` in the decline branch; the gate's anti-tamper block is byte-identical — agent-side manifest deletion still blocks |
| Minted-path disclosure | #429, `internal/adapters/bridge/bridge.go` | Resolver miss + non-empty artifactPath ⇒ synthesized minimal contract, `RenderContractFooter` ONLY (the full tail embeds an `evolve phase verify <agent>` self-check that is guaranteed exit 10 for a miss — an impossible instruction, review-blocked and negatively pinned) |
| Retro write-order directive | #428, `agents/evolve-retrospective.md` | disposition.json BEFORE the final report write — interim for the plan-Phase-B deliverable SSOT (86/88 recent retros lost the completion race; ordering provably wins) |

## Regression coverage

| Failure mode | Pinned by |
|---|---|
| Declined binding outlives its lineage (absorbing FAIL) | `core/continuation_decline_release_test.go` |
| Conditional release races a sibling rebind (TOCTOU) | `continuation/registry_delete_test.go::TestDeleteRegistryEntryIfCycle_ReleasesOnlyTheNamedAncestor` |
| Registry delete semantics (absent no-op, empty rejected, named-scope only) | `continuation/registry_delete_test.go` |
| Naked minted dispatch (path never disclosed) | `adapters/bridge/bridge_contract_minted_test.go` |
| Synthesized footer smuggles impossible instructions | same file, negative pins (`phase verify`, sentinel/self-check absence) |
| Gate anti-tamper unchanged | pre-existing `phases/audit` cycle-1285 pins (all green, untouched) |

## Lessons

1. **Two components sharing a record must share one liveness decision.** The
   adopter declined bindings the gate kept enforcing — releases now happen at
   the decline site (orchestrator-side, agent-unreachable). (ADR-0085.)
2. **An engine that polls a path must disclose that path.** Any dispatch with
   a pollable artifact gets at least the footer, resolver hit or miss.
3. **Never render an instruction the recipient cannot satisfy** — the first
   fix draft embedded a self-check that is guaranteed exit 10 for exactly the
   agents receiving it; adversarial review caught it (as it caught a TOCTOU
   in #428) — two BLOCK verdicts, both real, both pinned.
4. **State surgery has a stop-line.** Iterative ledger anchoring regressed on
   shared seqs and desynced tip sidecars; the console plane was restored
   byte-exactly and the proper mechanism queued rather than hand-patched.
5. The operator guardrails (2-zero-ship halt, per-cycle ship reporting,
   gate-proof-before-code) drove both halts to root cause in hours — they are
   now repo policy (CLAUDE.md, #430) and enforced in-loop (`consecutive-failures`
   breaker, #423).
