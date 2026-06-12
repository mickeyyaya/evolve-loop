# ADR-0046: Gate Epistemics, Declarative Commitments, and Loop Self-Deploy

- **Status:** Partially Implemented (Layer 0 + blocker-solo + Layer 1 [cycles 304/305] + Layer 2 demotion [interactive, after cycles 306/307 both failed building it — see below]; Layer 3 designed only)
- **Date:** 2026-06-12
- **Driver:** soak-#2 cycles 301/302 bricked by the R9.2 triage capacity clamp
  ([incident](../../operations/incident-2026-06-12-triagecap-phantom-floors.md))
- **Relates to:** ADR-0034/0035 (deliverable contracts), ADR-0039 (failure floor),
  ADR-0044/0045 (phase recovery / corrective interaction), R9 (triage capacity)

## Problem

The R9.2 capacity clamp counted committed coverage floors by regex token-matching over free
triage prose against Go package basenames. The triage bullet contract's own mandatory
`evidence=...`/`source=scout` fields collided with real packages (`core/evidence`,
`phases/scout`), and ordinary prose ("safety-critical paths") collided with `internal/paths`.
An honest 2-bullet commitment counted as 6 floors against a cap of 5; the correction directive
("keep ≤5 floors") was unsatisfiable because the agent can neither see nor reproduce the gate's
arithmetic and cannot remove contract-mandated tokens. Two consecutive cycles burned both
corrections and failed. The throughput window calibrating the cap was fed by the same broken
counter (K=4 recorded; true throughput 1). Cycle 302 carried the gate's own fix as task 1 and
was killed by the gate before build ran. No cycle could repair the running batch: gates execute
in the loop process, which cannot replace its own binary.

The regex bug is fixed (`fix(triagecap)` `0f0b1ff9`, replay-pinned). This ADR addresses the
five structural causes the bug exposed, so the *class* cannot recur.

## Decision (layered; ordered by leverage)

### Layer 0 — point fix (DONE, `0f0b1ff9`)

`metadataFieldRE` strips contract metadata before matching; `pathOnlyPkgs` requires
slash-qualification for prose-collider basenames; production-vocabulary replay pins.

### Blocker-solo triage rule (DONE, config-only)

`agents/evolve-triage.md` Core Principle 5: an item fixing the deterministic
gate/infrastructure defect that failed the previous cycle is committed **alone** in `top_n`,
enforced by a final-check (the inbox priority enum stays `HIGH|MEDIUM|LOW` — the trigger is
evidence-based, not a new priority value the ingestion validator would reject). An infrastructure fix must not be killable by the
infrastructure it repairs; minimizing the gate surface between defect and fix is a triage
responsibility.

### Layer 1 — declarative floor commitment (eliminates the class)

Triage **declares** committed floors structurally in the triage artifact's JSON companion
(`"floors": ["clihealth", "ledger"]` — the companion already exists; `postship.go:91`). The
capacity clamp counts declarations exactly — no prose parsing in the judging path. The fixed
heuristic counter demotes from judge to **cross-examiner**: prose-vs-declaration divergence
yields a correction that is *always satisfiable* ("your bullets discuss gc but floors omits
it"), because the agent owns the declaration. The throughput window records declared+shipped
floors (ground truth), ending self-referential calibration. `DeferredFloorPackages` and the
R9.3 evalgate floor-binding consume the same declarations (deferred/dropped floors declared per
section), retiring the remaining prose scrapes.

This is single-source-with-projection applied to commitment counting: today the floor count has
two sources (agent intent implicit in prose; the gate's lossy reconstruction); the declaration
makes it one source with the heuristic as a projection check. Lying in declarations games only
a planning clamp, is flagged by the cross-check, and the builder's actual overrun feeds the
window honestly.

### Layer 2 — epistemic gate classes + identical-rejection instinct (contains all future gate bugs)

> **Implementation status (2026-06-13):** the demotion core is LIVE for the one production
> heuristic gate (`internal/triagecap/demotion.go`: `ReasonTemplateHash` digit-run-length
> normalization — jitter-insensitive, magnitude-sensitive per the cycle-306 lesson;
> `ShouldDemote` consecutive-pair check with one-cycle scope; auto-filed idempotent inbox
> defect; consulted INSIDE `NewReviewer` at rejection time so composition-root wiring cannot
> be forgotten — the cycle-307 lesson). The generic GateClass registry is DEFERRED until a
> second heuristic gate exists (YAGNI; documented in demotion.go's header). The self-check
> CLI admission rule shipped in cycle 305 (`evolve guard triage-floors`).

1. Deliverable reviewers are classified at registration: **fact-gate** (verifies ground truth:
   red_count, attestation hashes, artifact presence) vs **heuristic-gate** (estimates from
   lossy input: floor counts, classifier verdicts over prose). The integrity floor
   (`ship ⇒ build ∧ audit ∧ (tdd unless trivial)`) is fact-class and **never demotable**.
2. A heuristic gate rejecting with a **byte-identical reason template across 2 consecutive
   cycles** is treated as a gate defect, not a work defect (real overpacking varies across
   cycles; identical rejections are a determinism artifact): the gate runs **shadow for the
   next cycle only**, an inbox HIGH is auto-filed, and the would-block WARN rides the artifact
   so the audit sees it. Implemented as a new signature class in the existing
   faillearn/instinct machinery (the S5 fatal-signature pattern) — two `failedApproaches`
   entries with the same gate-signature hash trigger the demotion. One-cycle scope plus the
   auto-filed defect keeps "fail twice and the gate gives up" ungameable.
3. **Gate admission rules** (apply to every enforce-mode reviewer):
   - it must expose its exact check as an agent-runnable CLI (`evolve guard <check> <artifact>`,
     extending `evolve phase verify`), and correction directives must cite that command —
     ladders are convergeable only over surfaces the agent can self-check;
   - heuristic gates must ship with a golden corpus harvested from **production** runs —
     artifacts AND environmental inputs (the real `KnownPackages` output, not a curated
     fixture). The R9.2 bug shipped because `knownPkgsFixture` omitted the colliding basenames;
     the replay pins passed trivially against a vocabulary that lied about production.

### Layer 3 — loop self-deploy: binary re-exec at cycle boundary (the true hotfix)

At each cycle boundary: if the tracked `go/evolve` differs from the running binary's baked-in
identity (`version.commit`), boot-smoke the new binary (`evolve doctor boot` path); on success,
`exec` into it resuming the batch (checkpoint/resume infra, hardened in soak #6); on smoke
failure, keep the current binary and file a defect. Gated `EVOLVE_SELF_DEPLOY=off|shadow|enforce`
(shadow logs would-exec), one generation per cycle (no exec loops), SELF_SHA re-pin on exec.
Cycle ships already commit the rebuilt binary (e.g. cycle 298), so with Layers 2+3 this incident
self-heals end-to-end with zero operator: 301 fails → instinct demotes the gate to shadow →
302 ships the fix it had already self-selected (solo, per the blocker rule) → boundary re-exec →
303 runs the fixed gate at enforce. A dedicated "hotfix phase" is rejected: hotfix capability is
emergent from blocker-solo + demotion + self-deploy, and a hotfix phase without re-exec cannot
affect the running batch (gates run in-process).

## Alternatives considered

- **Dedicated hotfix phase** — rejected: without re-exec it ships fixes the running batch never
  executes; with re-exec it duplicates what triage + failure floor + self-deploy already give.
- **Path-qualified-only package matching** (no declarations) — rejected as the terminal state:
  it breaks the cycle-283 overpacking pin (bare-name prose mentions are intended signal) and
  keeps the heuristic as judge; kept only as the Layer-0 mitigation for `pathOnlyPkgs`.
- **Blocklist contract vocabulary** — weakest Layer-0 variant; prose collisions remain.
- **EVOLVE_TRIAGE_CAP_GATE=shadow globally** — discards real overpacking protection (cycles
  280/282/283) to suppress one bug.

## Consequences

- Layer 1 changes the triage contract surface (persona + JSON companion schema + clamp + window
  recorder + evalgate consumer) — medium slice, highly testable, good cycle goal.
- Layer 2 rides existing instinct machinery; the admission rules are cheap and mostly process.
- Layer 3 touches the loop core and needs its own design pass + soak; natural home is the CE
  fleet-supervisor wave (worker restart on new binary is native there).
- Sequencing: nothing blocks soak #3 (Layer 0 unbricked it). Layer 1 → Layer 2 → Layer 3.

## Evidence

[Incident report](../../operations/incident-2026-06-12-triagecap-phantom-floors.md) ·
fix `0f0b1ff9` + chore `f1dc17a8` (CI green) · raw forensics
`.evolve/operator-salvage/cycle-301-triagecap-phantom-floors/` · inbox items
`declarative-floor-commitment`, `heuristic-gate-demotion-instinct`, `loop-binary-self-deploy`.
