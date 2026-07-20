# ADR-0073 — Cross-lane mint registry: tree-diff guard exemption for verified advisor mints

- **Status:** Accepted (2026-07-20)
- **Refines:** the tree-diff guard (Workstream B / R5–R9 classifier vocabulary in `internal/core/cyclerun_review.go` + `leak_recovery.go`); complements ADR-0072 (system-failure floor) and the fleet shared-tree architecture.
- **Deciders:** operator ("pipeline-first" standing directive + approval of per-cycle-isolation direction) + session (Variant A2 refinement, adversarial-security hardening).

## Context — cycle-967: a PASS scout aborted for another lane's sanctioned write

Fleet lanes run `evolve cycle run` against the **shared** project root. The advisor mint
(`phaseregistrar.Registrar`, wired in `cmd_cycle.go`) persists a minted phase's spec to the shared
`.evolve/phases/<name>/phase.json` — a **protected deliverable prefix** (`evolveDeliverablePrefixes`,
`leak_recovery.go`). The per-phase tree-diff guard diffs that same shared tree against a **per-lane**
baseline, so lane-970's mint of `.evolve/phases/gate-wiring-proof/phase.json`, landing during
lane-967's scout, was charged to 967's PASS scout → false cycle abort. Forensics (2026-07-20 sweep)
confirmed this cross-lane attribution end-to-end, corroborated by the orphaned
`.evolve/profiles/gate-wiring-proof.json` mint residue.

## Rejected alternatives

1. **Blanket exemption of `.evolve/phases/` for non-source-writer phases** — exactly the
   deliverable-leak loophole `TestIsScoutEvalMaterialization` pins against
   (`{scout, ".evolve/phases/x/phase.json", false}`): any phase could smuggle a phase config.
2. **A1 — literal per-cycle mint isolation** (move `PhasesDir` per-cycle): entangles the READ side —
   the bridge resolves a minted phase's profile off ONE global root shared with built-ins
   (`paths.go` / `EVOLVE_PROFILES_DIR_OVERRIDE`); moving the write alone breaks dispatch, and a
   per-cycle search path spreads into `cmd_cycle`, bridge resolution, and phaseinventory roots.

## Decision — Variant A2: a shared registry of minted names, verified by content

Keep mints in the shared root (dispatch untouched). Make the **guard** distinguish mint
infrastructure from a real leak:

- **`internal/mintregistry`** (new leaf; apicover-enforced at creation): registry at
  `<projectRoot>/.evolve/active-mints.json`. `Append` is flock-serialized (sidecar-lock convention,
  `flock.WithPathLock`), TTL-prunes (6 h) and replaces re-minted names (bounded file);
  `ActiveNames` is a lock-free read (atomic-rename ⇒ never torn) that fails **safe** — corrupt ⇒
  error + empty set; `QuarantineCorrupt` renames a corrupt registry aside under the lock (re-reads
  first) so a corruption outage is bounded to one check.
- **Registrar** (`phaseregistrar.Register`): appends the minted name **after** the clamp,
  **before** persisting files — register-before-persist; the reverse order recreates the race.
  A registry append failure **rejects the mint loudly** (an unregistered mint is an abort landmine
  for every concurrent lane). Wired at the composition root with
  `RegistryPath: mintregistry.Path(projectRoot)` — projectRoot-anchored (not evolveDir) because the
  guard only sees writes under `ProjectRoot`; write and read must agree on that axis.
- **Guard** (`cyclerun_review.go` leak filter): a third OR'd classifier, `isActiveMintPhasePath`,
  exempts a leaked path IFF it is one of the **exactly two** paths a mint writes —
  `.evolve/phases/<name>` (bare dir entry) or `.evolve/phases/<name>/phase.json` — for a
  registered, TTL-fresh, **content-verified** name. `verifiedActiveMints` re-runs the registrar's
  spec-side clamp on the on-disk file (parses as `PhaseSpec`, name==dir, `optional:true`,
  `ValidateUserSpec` clean; kebab name checked *before* any FS access so a forged name can never
  traverse). Registry read error ⇒ **ABNORMAL** log + quarantine + exemption disabled for that
  check (guard stays armed — the fail-safe direction).

## Security posture (adversarial review, 2026-07-20)

Threat model: a rogue phase agent with shell access and the OS sandbox **off** (nested-macOS
fallback). Review finding: the registry file is unauthenticated — such an agent can forge entries
directly. **Provenance is unattainable in that mode**: every candidate anchor (registry, ledger
chain, state.json) is plain on-disk state an FS-writing attacker can rewrite, and there is no
secret channel. The enforced invariant is therefore **clamp parity**, not provenance:

> A forged registry entry can never exempt anything a sanctioned advisor mint could not already
> carry. The advisor is itself an LLM; its mints are precisely "clamped optional user specs."
> Anything unclamped — garbage, non-optional, name-mismatched, invalid-kind, companion payload
> files, novel unregistered names — still aborts loudly (pinned by
> `TestGuardAbortsForgedRegistryEntryWithUnclampedSpec`,
> `TestGuardAbortsCompanionPayloadUnderRegisteredMint`,
> `TestGuardStillAbortsUnregisteredPhaseConfigLeak`, `TestGuardStillAbortsExpiredMintPhaseConfigLeak`).

**Accepted residual** (documented, queued — not silently shipped):

1. A clamp-parity forged spec suppresses the abort *signal* the guard previously raised once per
   smuggle. The abort was only ever a signal: pre-fix the leaked file **persisted after the abort**,
   was discovered/loaded by the next cycle (`phasespec.DiscoverUserSpecsFromRoots`), and staged by a
   later ship's `git add -A` regardless. The load path never re-runs the registrar clamp
   (`ValidateUserSpecWithCatalog` checks the floor but not envelope/`WritesSource` normalization) —
   a **pre-existing** hole for any on-disk spec however it arrived. → queued pipeline-band item:
   *load-time registrar-equivalent clamp for discovered user specs*.
2. Ship's main-tree `git add -A` staging surface (stages any residue, forged or benign) —
   pre-existing; → queued pipeline-band item.
3. Dispatch-side forgery via `.evolve/profiles/<name>.json` was **never** guarded (profiles are
   runtime state, exempt long before this change) — the spec-side guard is and remains the only
   content control at this boundary.

## Consequences

- Fleet lanes no longer false-abort on each other's mints (composed pin:
  `TestGuardExemptsConcurrentLaneMintedPhaseConfig`); the 967 class is closed.
- The residual cross-lane case — two concurrent **source-writer** phases and leak-recovery's
  relocation of another lane's artifacts — remains queued as
  `treediff-guard-worktree-phase-crosslane-residual` (Option B, fleet-lane-owned-paths registry).
- TTL means an idle registry decays to zero exemptions; an actively-minting batch refreshes
  entries on every mint. A corrupt registry self-heals via quarantine instead of disabling the
  exemption fleet-wide until the next mint.
