# Scope disease: four costumes of one selection gap

**Period:** 2026-07-01 → 2026-08-04 · **Status:** in-flight (all four instances closed; the class-level answer is test-impact selection — see [Regression TIA](2026-08-regression-tia.md))
**Primary artifacts:** ADR-0069, commit `c37dc324`, PR #399 `8a45a27f`, PR #402 `10f46fe9`, PR #404 `83d76e3d`, PR #405 `44e7a937`, PR #407 `1713c046`, `.github/workflows/go.yml`

## Problem

An autonomous cycle verifies its work at **changed scope** — the packages its
diff touched, in an isolated worktree. Main's CI verifies the **whole repo**,
including repo-wide contract tests that scan surfaces (export naming, on-disk
config catalogs, cross-package invariants) that a diff can break *without
touching the guarded package's files*. Anything living in that gap passes every
per-cycle gate, ships, and turns main red — where it blocks every PR, and where
a running batch keeps shipping on top of it because each subsequent cycle's
gates are also changed-scope and also never select the red package.

ADR-0069 named the structure in July: *"The pattern is not a bug in any one
cycle — it is a structural gap between the cycle's verification scope and CI's
gate scope"* (docs/architecture/adr/0069-cycle-ci-parity-audit-gate.md). Over
five weeks the same disease surfaced in four different costumes, and then a
fifth, meaner variant: for two config-dir surfaces, **CI itself was
scope-selected** — the go workflow's path filters excluded them, so a breaking
change minted *no run at all* and green PR checks were vacuous.

## Context & evidence

### Costume (a) — the apicover exports class

Origin: cycles 426/430/436 shipped unnamed exported symbols that per-cycle
audit green-lit and repo-wide `apicover -enforce` caught on main, forcing
manual salvage each time (table in ADR-0069). ADR-0069 (2026-07-01) added
deterministic CI-parity gates to the audit phase.

Recurrence, 2026-07-20: the cycle-960 ship `504bc913` added
`carryforward_filter.go` + `prune_superseded_orphans.go` exporting three
symbols (`CarryforwardCandidateLandable`, `PruneSupersededOrphans`,
`OrphanVerdict`) with zero naming tests. Per the fixing commit: *"The per-cycle
audit's changed-scope apicover green-lit the ship, but the repo-wide `go`
workflow enforces per-package hard-fail, so main went RED ('UNCOVERED: 3') —
blocking every PR"* (commit `c37dc324`, FIX 2). Even with parity gates
installed, the *scoping* of the parity gate (touched∩enforced packages) left a
hole.

### Costume (b) — the stub coherence class

2026-08-02: during pre-launch cleanup, ship's staging sweep carried the
previously-untracked `.evolve/phases/gate-wiring-proof/` +
`.evolve/profiles/gate-wiring-proof.json` into commit `6880559a`. Tracked,
they appear in every CI checkout and fail two repo-wide coherence tests — a
profile with no persona (dead config) and an optional phase without SELECT
metadata. Main's go workflow went red and *"every loop cycle shipped on top
inherited it"* (PR #399, commit `8a45a27f`). The commit message owns a
misdiagnosis: the red was initially attributed to a demoted contract gate —
*"that was WRONG — the demotion cost nothing, my commit did."* The tests scan
**disk**, which is why they never failed on the runtime plane, where the stubs
legitimately live untracked. (The mint-and-sweep mechanism itself is a separate
class — see [Runtime-minted config stubs](2026-08-minted-stub-class.md).)

### Costume (c) — the signal-parity keystone

2026-08-03, 09:44: cycle-1250's ship `99a82b3c` injected
`sig.Generic["run_dir.artifact_bytes"] = dirSize(workspace)` unconditionally
into `router.Digest` (diff: `go/internal/router/digest.go` + two test files).
The routingtest keystone `TestSignalSpec_DualRenderingAgree` — *"the pure
rendering MUST equal what Digest extracts"* — *"can NEVER pass against a
runtime-computed value the fixture-driven pure rendering cannot know — every
fixture diverges by exactly that one Generic key, on both platforms"* (revert
commit `10f46fe9`). Main's go workflow was red from `99a82b3c` through
`cf7e74aa` — **5 commits** — *"and the running batch kept shipping on top
because the per-cycle gates ran CHANGED-SCOPE and never selected routingtest —
the same per-cycle-vs-repo-wide scope disease as the apicover class, third
costume this week"* (ibid.). The change was confined to `internal/router`;
`internal/routingtest` imports it and holds the invariant — a **reverse
dependency** that forward-only changed-scope derivation cannot see.

### Costume (d) — the phase-catalog metadata guard

2026-08-04: cycle-1262's scout window minted an empty
`.evolve/phases/ship-stage-hygiene-check/phase.json`; ship's whole-tree bind
swept it into tracking; the repo-wide catalog guard
`TestPhaseCatalog_OptionalPhasesHaveSelectMetadata` went red on main on both
platforms *"while per-cycle changed-scope never selected internal/phasespec
(scope-disease costume #4 — filed as TIA enforce-flip evidence)"* (PR #404,
commit `83d76e3d`).

### The CI-selection meta-gap

Both config-dir costumes exposed a second-order instance: `go.yml`'s path
filters covered `go/**`, `skills/**`, `agents/**` — not `.evolve/phases/**` or
`.evolve/profiles/**`. So cycle-1262's breaking change *"broke main invisibly
and #404's fix merged with the guard never re-run on CI (its PR checks skipped
the matrix for the same reason)"* (PR #405, commit `44e7a937`). Identically for
profiles: *"tracked stubs broke the v22.13.0 release suite invisibly and #406's
un-tracking fix ALSO minted no run — the profiles twin of the phases gap #405
closed the same day"* (PR #407, commit `1713c046`). Green PR checks on those
PRs were vacuous for the guard in question: no run was ever minted. The filters
now carry self-documenting comments on main, e.g.:

> `'.evolve/phases/**'  # TestPhaseCatalog_* (go/internal/phasespec) scans the
> on-disk phase catalog; a phases-only change broke main invisibly (2026-08-04:
> cycle-1262 minted a metadata-less stub, #404 fixed it, and NEITHER triggered
> this workflow).` — `.github/workflows/go.yml` (origin/main `0c7500c3`)

## Approaches considered

- **Run the whole-repo CI suite in every cycle.** Rejected from the start:
  cost under fleet load, and the whole-suite contention-flake surface is its
  own established failure family (the cycle 858–862 class was fixed
  specifically by scoping the integration tier *down* to touched packages,
  `f09ecfc4`). Scope disease cannot be cured by un-scoping everything.
- **Targeted deterministic CI-parity gates in the audit phase** (ADR-0069's
  choice): run the *exact* whole-repo CI command for known-dangerous classes
  (vet, acs-durable, apicover on touched∩enforced). Real but reactive — each
  costume revealed the next surface the parity set didn't cover (960's
  apicover scoping hole, coherence tests, routingtest, phasespec).
- **Per-instance remedies with the guard's own prescription.** For (d), the
  guard demanded *"metadata-not-allowlist"* — padding the guard's allowlist was
  explicitly rejected in favor of honest SELECT metadata (#404). For (c),
  fix-forward was rejected in favor of a surgical revert because the parity is
  *structurally unsatisfiable* — no test tweak is honest; the telemetry intent
  was refiled with its framework constraint spelled out (*"an
  orchestrator-runtime signal must enter through the sanctioned dual-rendering
  path … never an unconditional Digest-side injection"*, `10f46fe9`; refiled as
  `artifact-bytes-signal-dual-rendering` 0.8 in queue commit `809fd3f8`).
- **Reverse-dependency test-impact selection** — derive the affected set as the
  importer *closure* of the changed set, then select tests over it. This is the
  only approach that addresses the class rather than the instance; it became
  `egps-regression-tia-selection` and ADR-0082 (next entry). Its honest
  measurement also bounded it: import-graph selection cannot represent the
  config-dir→test edges of costumes (b) and (d) (ADR-0082, Finding).

## Decision & reasoning

Each instance was fixed console-first at maximum reasoning (per the standing
`pipeline_fixes_console_first` rule), each with a reproduction of the **true CI
condition** rather than an assumed one — a checkout *with* the stubs for (b),
the guard red at `37bc664a` for (d), keystone tests red on the pre-revert tree
for (c). The CI-selection meta-gap was closed by widening the path filters
(#405, #407) with comments that carry the incident so the reason survives in
the file itself, mirroring the existing skills/agents precedent comments. The
class-level answer — selection that follows reverse dependencies and,
eventually, declared impact surfaces — was deliberately routed to its own
workstream instead of being bolted onto any instance fix.

## Implementation

| Fix | Commit / PR | Diff surface | Verification |
|---|---|---|---|
| (a) 3 orphaned exports covered + goal-hash normalize | `c37dc324` | `go/internal/core` (lanescope, orchestrator, naming tests) | repo-wide apicover green; new fail-open tests |
| (b) un-track gate-wiring-proof stubs + gitignore | #399 `8a45a27f` | 2 deletions + `.gitignore` | *"both tests RED with the files present in a checkout, GREEN after — the exact CI condition, reproduced locally rather than assumed"* |
| (c) surgical revert of Digest injection | #402 `10f46fe9` | exactly the 3 injected files | `go test ./internal/routingtest/ ./internal/router/` — both PASS on the revert |
| (d) SELECT metadata for the minted stub | #404 `83d76e3d` | `.evolve/phases/ship-stage-hygiene-check/phase.json` | guard *"RED at 37bc664a -> GREEN with phasespec+phasecoherence"* |
| meta: phases path filter | #405 `44e7a937` | `.github/workflows/go.yml` (+2 lines) | workflow-file change self-triggers the matrix — the green-with-fix signal |
| meta: profiles path filter | #407 `1713c046` | `.github/workflows/go.yml` (+2 lines) | same; provided the green signal for the v22.13.1 release |

The batch-integrity review's follow-up table (§3,
docs/operations/batch-integrity-review-2026-08-04.md) re-checked #399, #402,
#404, #405, #407 for design conformance and TDD evidence; for #405/#407 it
records the honest shape of the proof: *"The gap itself was the failing test
(no run minted on a breaking change); post-merge matrix runs on main = the
regression proof."*

## Results (measured)

- Costume (c) held main red for **5 commits** (`99a82b3c`→`cf7e74aa`,
  ~09:44–17:45 on 2026-08-03) with the live batch shipping on top throughout
  (`10f46fe9`).
- Costume (b) held main red from 2026-08-02 until #399 (2026-08-03 08:40),
  inherited by every intervening cycle ship (`8a45a27f`).
- The profiles variant cost one release: v22.13.0 (`f3548a49`) failed its
  release suite and was auto-demoted to prerelease; v22.13.1 (`97b05149`)
  shipped the same day with the #406/#407 fixes (detail in the
  [minted-stub entry](2026-08-minted-stub-class.md)).
- After #405/#407, a `.evolve/phases/**` or `.evolve/profiles/**` change mints
  the go matrix on both push and pull_request (`.github/workflows/go.yml` on
  main). No recurrence data yet post-fix; the measure of success is that the
  *next* config-dir break fails a PR check instead of main.

### Interim class fix (2026-08-05): the ship-time repo-contract scanner pack

Costume #5 (incident-postmortem, cycle-1313) proved the class outruns
point-fixes, and TIA-enforce is gated on impact-surface manifests that do not
exist yet. The interim class fix landed console-side: the SHIP phase now runs
the four incident-proven repo-wide guard packages (phasespec, profiles,
phasecoherence, routingtest) in the lane worktree BEFORE bind/push
(`internal/phases/ship/repocontract.go`, dial
`gates.repo_contract_gate`, default **enforce** — a deliberate deviation from
shadow-first, justified because the pack is existing deterministic FP≈0 repo
tests whose breakage IS a red main). A RED pack fails the lane's ship with the
dedicated `REPO_CONTRACT_GATE` code instead of redding main and storming the
operator's CI email. All five costumes would have been caught in-lane by this
gate. Wiring pinned against the cycle-1064 silently-unwired trap.

## Retrospective — what we learned

- **Per-cycle green is a statement about the changed scope, not about main.**
  Every costume shipped through honest, working gates that were looking at the
  wrong set. The disease is *selection*, and instance fixes only re-cover the
  surface that just burned.
- **Reverse dependencies are the recurring blind spot.** (c) is the purest
  form: the guard lives in a package that imports the changed one. Forward
  changed-scope can never see it; only importer-closure selection can
  (ADR-0082's `changedpkgs.ImporterClosure` exists because of this).
- **CI path filters are themselves a selection mechanism, with the same
  disease.** The meta-gap made PR checks vacuously green — "CI green" claims
  must confirm a run was actually *minted* for the surface in question.
- **Repo-wide guards over non-Go surfaces (config catalogs) escape the import
  graph entirely** — ADR-0082's honest measurement made this structural: 2 of
  the 3 incident classes that were filed as "TIA enforce-flip evidence" are
  unrepresentable in an import-graph-only selector, which is why filter
  widening + declared impact surfaces is the current best answer, not enforce.
- Recorded discrepancy, per the no-invention rule: the session memory ledger
  attributes the apicover recurrence to "cycle-962"; the fixing commit
  `c37dc324` attributes the offending ship to cycle-960 (`504bc913`). This
  entry follows the commit.

## Links

- ADR-0069 — docs/architecture/adr/0069-cycle-ci-parity-audit-gate.md
- ADR-0082 — docs/architecture/adr/0082-regression-test-impact-selection-shadow.md
- docs/operations/batch-integrity-review-2026-08-04.md (§3 follow-up table)
- Sibling entries: [Regression TIA](2026-08-regression-tia.md) (the class
  answer), [Runtime-minted config stubs](2026-08-minted-stub-class.md)
  (costumes b/d's mint mechanism),
  [Releases v22.11–v22.13.1](2026-08-release-engineering.md),
  [Batch integrity review](2026-08-batch-integrity-review.md)
- Queue items: `egps-regression-tia-selection` (`.evolve/inbox/2026-07-30T09-00-00Z-…`),
  `phase-mint-carries-select-metadata`, `artifact-bytes-signal-dual-rendering`
