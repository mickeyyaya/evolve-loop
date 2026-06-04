# ADR-0035: Unified Phase Descriptor — Spec-Derived Deliverable Contracts

- Status: Accepted
- Date: 2026-06-04
- Extends: ADR-0034 (unified deliverable contract + self-check), the v4 "Lego pipeline" (`phasespec.PhaseSpec` + `.evolve/phases/<name>/phase.json` overlays), ADR-0024 (dynamic phase routing / PhaseAdvisor)

## Context

The v4 Lego pipeline already made phases declarative: a phase is a `phasespec.PhaseSpec`
(registry entry or operator overlay) plus a persona (`agents/evolve-<name>.md`) and a dispatch
profile (`.evolve/profiles/<role>.json`). The advisor can select, skip, insert, and mint phases
at runtime.

But **one phase attribute was still hardcoded in Go**: the *deliverable contract* — WHERE the
phase writes, WHAT shape, and the required sections/verdict (`phasecontract/contract_registry.go`,
a hardcoded `map[string]Contract` of 8 built-in entries). `phasecontract.For(name)` returned a
miss for any user/minted phase. Two consequences for a config-only phase:

1. **No host-side well-formedness verification** — `deliverable.Verify` returned a fail-open
   "no contract registered" error, so the contract gate never checked a user phase's report.
2. **No exact-output-path prompt injection** — `bridge.injectContract` only injected the
   Deliverable Contract block (the ADR-0034 fix) for phases in the hardcoded map, so user/minted
   phases regressed to the exact "agent infers its own output path" failure ADR-0034 cured.

So "add a phase with no code change" was true for *routing* but false for the *contract* — the
last code-bound attribute.

## Decision

**Derive the deliverable contract from the phase spec.** Every `Contract` field is computable
from a `PhaseSpec`:

| Contract field | Derived from | Rule |
|---|---|---|
| `ArtifactName` | `outputs.files[0]` (basename) | else `<name>-report.md` |
| `Kind` | artifact extension | `.json` → JSON, else Markdown |
| `Sections` | `classify.require_sections` | tolerant `## <s>` headings |
| `Verdicts` | `archetype==evaluate` **and** `classify.verdict_on_pass` | opt-in only; else `nil` |
| `RequiredKeys` | — | `nil` (tolerant JSON) |
| `WriteTarget` | — | `workspace` |
| `AgentName` | `agent` | else `evolve-<name>` |

Implemented as `phasecontract.FromSpec(spec)` plus a `Resolver` seam:

- `BuiltinResolver` — the hardcoded map only (byte-identical legacy path).
- `CatalogResolver{builtin, lookup}` — built-ins **win** (override), then `FromSpec` for any name
  the lookup (`Catalog.Get`) knows. So a user phase can never weaken a spine phase's contract.

`deliverable.Verify`/`VerifyWith`, `deliverable.Reviewer` (host gate), `bridge.injectContract`,
and `evolve phase verify` all resolve through the SAME catalog-backed resolver built once from the
orchestrator's merged catalog — the self-check and the host gate cannot drift (the ADR-0034
invariant, now extended to user phases).

**Anti-Goodhart, preserved:** derivation is WELL-FORMEDNESS ONLY (location, kind, sections).
`Verdicts` is opt-in (`nil` by default) so a user phase never silently acquires a verdict gate it
cannot satisfy (the cycle-192 false-FAIL class). Semantic correctness stays the auditor's job.

**Companion runtime-composition fixes (same change set):**

- **Early-exit edge.** The state machine had no `scout→end` edge, so the advisor could insert and
  mint phases but never truly *remove* the tail (end a no-ship convergence cycle early). Added
  guarded `scout/triage→end` edges + `StateMachine.CanTerminateEarly(from, shipPlanned)`: the
  semantic authority. The invariant it defends — **early-exit is ONLY ever a no-ship convergence;
  a ship-intended cycle can never terminate early** — keeps audit-before-ship non-bypassable
  (`enforceNext` consults it before taking the edge; `planRunsShip` gates on the clamped plan).
- **Schema + lint.** `docs/architecture/phase-descriptor.schema.json` documents the unified
  descriptor; `evolve phase lint <name>` reports what the runtime will derive (reusing
  `DiscoverUserSpecs`→`ValidateUserSpec`→`FromSpec`). Lint is **fail-open** — warnings only,
  always exit 0 (enforcement lives in `ValidateUserSpec` + the contract gate).

## Notes on the invariants (for future editors)

- **Early-exit has deliberately one-layered defense.** The `scout/triage→end` edges are
  structurally legal in the transition table, and `CanTerminateEarly` is the *sole* semantic gate
  — there is intentionally NO `SpineSatisfiedUpTo` backstop on this path (unlike ship, which has
  both the clamp and the spine gate). That asymmetry is correct: early-exit is the *absence* of
  work (a no-ship convergence), not the bypass of a gate. Do not "harden" it by adding a spine
  check — that would break legitimate no-ship cycles. The safety comes from `planRunsShip` being
  the SAME predicate (`entry exists ∧ Run`, absent ⟹ false) the floor's `planRuns(out,"ship")`
  uses, so no plan shape can both run ship and permit early-exit.
- **Resolution is by PHASE NAME, not AgentName.** All three consumers resolve a contract by the
  phase identity: the host gate by `in.Phase`, the self-check by the positional phase arg, and the
  bridge by `req.Agent` — which for a user phase is the bare phase name (`runner.go` sets
  `Agent: PhaseName()`, e.g. `"adversarial-review"`, not `"evolve-adversarial-review"`). The
  derived `Contract.AgentName` (`evolve-<name>`) is display/metadata only and is never the
  resolution key, so the built-in-vs-derived AgentName convention split is not a resolution hazard.
- **Strict schema, lenient runtime.** `phase-descriptor.schema.json` sets
  `additionalProperties:false` and is intentionally STRICTER than the runtime parser (tolerant,
  ignores unknown fields, fail-open). The schema serves `evolve phase lint` as a developer aid; a
  lint warning is never a load-time block.

## Consequences

- A phase is now **fully definable in declarative files** — registry entry or
  `.evolve/phases/<name>/phase.json` + persona + profile — including its contract. Adding or
  updating a phase needs **zero Go changes**. Proven by `adversarial-review` and `perf-profile`,
  added as config-only (WS-C) and verified by `usercatalog_research_test.go`.
- The advisor can compose freely at runtime — select / skip / **insert (reacting to prior-phase
  signals via `insert_when`)** / mint / **early-exit** — within the unchanged trust kernel
  (floor / clamp / `SpineSatisfiedUpTo` / gates / ledger remain the deterministic disposer).
- Built-in contracts stay authoritative; the hardcoded map is the override layer, not the only
  source. No behavior change for built-in phases (back-compat tests pin this).

## Alternatives considered

- **A separate `.evolve/contracts.json` file.** Rejected: a second source of truth that would
  drift from the phase.json it describes. The contract is a *projection* of the spec, not
  independent data — deriving it eliminates the drift class entirely.
- **Migrate built-ins to derive too.** Deferred: the built-ins carry hand-tuned tolerant heading
  allow-lists (template-drift history, cycle-192). Keeping them as an authoritative override is
  safer than regenerating them; revisit once the templates stabilize.
