# Binary lag: the loop's fixes landed on main while the loop kept executing the past
**Period:** 2026-08-04 → 2026-08-05 · **Status:** shipped (boot self-heal)
**Primary artifacts:** `go/cmd/evolve/cmd_loop_boot_refresh.go` (+ tests) · cycle-1309 halt + consumed P0 · queue item `auto-refresh-binary-at-boundary` (0.94)

## Problem
The loop ships fixes to `main`, but the running process executes the binary it
booted with. Every landed pipeline fix is inert until an operator manually
rebuilds, repins, and relaunches. The gap between "fixed in the repo" and
"fixed in execution" — binary lag — silently converts already-solved defects
back into live failures.

## Context & evidence
Measured in one overnight window (2026-08-05, batch cycles 1287–1310):
- The sentinel tail-anchor parser fix landed at cycle-1301 (`64f8620e`,
  autonomous). Cycles 1302–1309 kept running the old first-match parser.
- The `audit-warn-prescriptions-unenforced` lane — whose subject matter forces
  its reviews to quote sentinel examples — burned three full cycles
  (1298/1308/1309) as parser-distortion casualties and then **halted the
  entire batch** via three identical `audit|gate-block` fingerprints
  (cycle-1309 P0, consumed 2026-08-05 with the narrative).
- The contract-block CLI escalation, breaker-neutral salvage, and demotion
  ledger sat inert for a day after landing for the same reason.
- Every refresh in the window was manual: `make -C go build` +
  `evolve reset-sha -operator` + relaunch — deterministic operator toil
  (three times in 24h), exactly the class Rule 5 says belongs in code.
- Throughput cost in the overnight deep-dive: ~5 of ~20 cycles + one batch
  halt attributable to the class.

## Approaches considered
1. **Chain-boundary refresh only** (rebuild between batches in
   `--until-inbox-empty` mode) — rejected as primary: this deployment runs
   discrete batches relaunched by the operator, so the boundary hook would
   never fire here; boot is the point every run passes through.
2. **Halt-loud on staleness** (refuse to boot a stale binary, print the
   remedy) — rejected: converts toil into a different toil; the remedy is
   deterministic, so executing it beats demanding it.
3. **Hot code reload / plugin architecture** — rejected outright:
   massive complexity for a problem a rebuild+re-exec solves.
4. **Boot-time detect → rebuild → re-exec** — adopted. The re-exec'd process
   then runs the *existing* boot machinery (auto-repin, recovery, preflight)
   on the fresh binary, so no new SHA-integrity path is invented.

## Decision & reasoning
Detection compares the binary's linker-stamped build commit
(`version.Commit()`) against plane `HEAD`, and rebuilds **only when the delta
touches `go/`** — the one tree the binary embeds (skills/, agents/, `.evolve/`
are disk-read at runtime; a docs-only advance never warrants a rebuild).
The rebuild goes through `make -C go build` so the ldflags stamp stays owned
by exactly one place. Re-exec replaces the process at the same executable
path with the same argv, plus a marker env (`EVOLVE_BOOT_BINARY_REFRESHED`)
capping the self-heal at one attempt per boot chain — a rebuild that does not
change the stamp can never re-exec forever. **Every step is fail-open**: an
empty stamp, unresolvable HEAD, unknown delta, failed rebuild, or failed exec
WARNs and boots the old binary — a stale batch is yesterday's status quo; a
bricked loop is strictly worse.

### The adversarial review BLOCK — and why it was the most valuable step

The first implementation was **refuted in review** (the #400 pattern, caught
pre-merge): the rebuild changes the on-disk binary hash, and the very next
boot step — the within-version SELF_SHA classifier, whose state is the
*normal* live-plane state — reads an unreconciled pin as TAMPERING,
deliberately refuses to auto-repin, and HALTS pre-scout. The "fail-open" heal
would have converted "stale but running" into "halted until an operator runs
reset-sha" — the precise anti-goal — and the wiring test was structurally
blind to it because it faked recovery. Six further findings: marker existence
semantics broke legitimate re-heals in chain mode; --resume never refreshes;
non-idempotent boot work repeats in the healed child; the exec target was
never verified against the rebuild output; a concurrent-rebuild race; and the
test suite's blindness to the killer interaction.

Revision, all findings: the refresh now reconciles the pin through the SAME
provenance-gated primitive the across-version heal uses (attemptBootRepin →
phaseintegrity.RepinIfDrifted — the rebuilt stamp is HEAD-ancestral by
construction, so the anti-tamper gate is not weakened for foreign binaries),
with a real-primitive test asserting `expected_ship_sha` equals the rebuilt
file's hash before exec and a decline-path test proving an unverifiable
binary is never exec'd; the marker carries the healed-to HEAD (value
semantics — refuse only a same-target no-op rebuild); the exec target is
verified against `go/bin/evolve` BEFORE rebuilding; resume exclusion and the
bounded double-prune are documented intent in the file header.

A third forcing function then improved the design again: the flag-ceiling
gate (`TestRegistry_FlagCeiling`, target zero flags) refused the marker's
registry row — so the env-var marker was retired for a **consume-once file**
(`.evolve/boot-refresh-marker`), which simultaneously satisfies the ceiling
AND eliminates the darwin duplicate-env first-wins hazard (re-review N1) at
the root instead of working around it. Three independent guards — the
adversarial reviewer, the SELF_SHA classifier's design, and the flag ceiling
— each made the mechanism structurally better than the version I first wrote.

## Implementation
`cmd_loop_boot_refresh.go`, called in `runLoop` immediately before
`bootRecoverFn` (a stale binary should not run recovery logic either). All
side effects behind package-var seams (`bootRefreshHeadFn`,
`…SourceDeltaFn`, `…RebuildFn`, `…ExecFn`), mirroring the existing
boot-recovery seam pattern. TDD red-first: twelve tests — seven behavioral
(no-op when current; rebuild-then-exec order + marker; docs-only skip;
unverifiable stamp; averted refresh loop; rebuild/exec fail-open), the
policy-off dial, real-git adapter semantics (short-SHA ranges, go/ pathspec),
the REAL-primitive pin-reconcile + decline-path pair (the review's minimum
bar), non-plane-executable refusal, value-marker re-heal, and a wiring proof
through the **live `runLoop` path**.

## Results (measured)
At ship time: unit + wiring proofs green; live validation pending the next
boot after a go/-touching landing — expected observable:
`[loop] boot-refresh: binary <stamp> is behind HEAD <head> with a go/ delta —
rebuilding and re-exec'ing`. The success metric is negative: zero future
halts whose fingerprint class was already fixed on main, and zero manual
rebuild-repin-relaunch sequences in the operator log.

## Retrospective — what we learned
- **"Landed" has two meanings and the gap between them is a failure class.**
  §3.9 already forbids calling a mechanism "active" from lane labels; this
  incident adds the runtime half: a fix is not DONE until the executing
  binary contains it. Chronicle/ledger entries for pipeline fixes should
  record the *activation* boot, not just the merge SHA.
- **The breaker was right twice.** Both cycle-1286 and cycle-1309 halts were
  honest signals of operational gaps (push-strand topology; binary lag) —
  the canned "forged verdict" P0 text was wrong both times, which is why
  root-cause text must derive from the artifacts (§3.9).
- **Fail-open at the seam level is not fail-open at the system level.** Every
  unit edge WARN'd and continued, yet the composed system still bricked —
  because the failure fired in the re-exec'd child, past the point of no
  return, in a component the wiring test faked. The killer-interaction test
  (real repin primitive + real state pin) is the shape that catches this
  class; seam-faked wiring proofs alone cannot.
- Remaining scope (tracked on `auto-refresh-binary-at-boundary`, re-scoped
  after this landing): the chain-boundary hook for `--until-inbox-empty`
  deployments, and surfacing refresh events in the dossier/loop summary.

## Follow-up landings (cycle-1353)

Two gaps this chronicle left open are now closed in code:

- **`chain-boundary-binary-refresh-stop`** — the "concurrent double-launch"
  bullet in `cmd_loop_boot_refresh.go`'s DOCUMENTED INTENT block was an
  *accepted risk* resting on "simultaneous loop launches are already excluded
  operationally". That assumption was stale: the fleet runs N≥1 concurrent
  lanes off ONE shared `go/bin/evolve`, so a boot-time rebuild+re-exec can
  swap the binary out from under a live batch — the standing rule is "NEVER
  rebuild the plane binary mid-batch". `bootRefreshFleetLaneFn` now ENFORCES
  it: a fresh per-run `.lease` (`internal/runlease`, the same heartbeat gc
  reads for liveness) anywhere under `<EvolveDir>/runs/` stops the refresh
  before rebuild or exec. Fail-open is preserved and extended — a lease check
  that *errors* also stops the refresh, because an unproven-idle plane is not
  a safe one. The two conditions WARN with distinguishable text ("a concurrent
  fleet lane is active" vs "fleet-lane check unverifiable") so an operator
  scanning stderr can tell *known unsafe* from *unknown*.
- **`chain-summary-refresh-event-field`** — "surfacing refresh events" now has
  its first half. The cycle-start model-catalog refresh (`planCycle`,
  `internal/core/cyclerun.go`) was silent on success and stderr-only on
  failure; it now appends a `catalog_refresh` ledger entry mirroring the
  `operator_directives` stamp six lines below it — `Action` is `ok`/`failed`,
  `Message` carries the resolved `catalog.refresh_stage` (wired from
  `policy.CatalogConfig().RefreshStage` at the composition root via
  `core.WithCatalogRefreshStage`). The `refresh_stage=shadow` soak reads a
  queryable trail instead of scrollback. Not covered: a distinct `skipped`
  outcome — the injected refresher's `func(ctx) error` contract returns `nil`
  for both a TTL-skip and a real refresh, so distinguishing them needs a
  wider signature (deferred).

## Links
[2026-08-batch-integrity-review.md](2026-08-batch-integrity-review.md) (§3.9) ·
[2026-08-push-strand.md](2026-08-push-strand.md) (the sibling operational-gap
halt) · queue: `auto-refresh-binary-at-boundary`, `consumption-rides-landing-ship`,
`codex-repl-boot-strikes` (the other overnight throughput causes)
