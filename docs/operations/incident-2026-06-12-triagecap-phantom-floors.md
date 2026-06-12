# Investigation: Cycle 301 FAIL — triagecap phantom package floors

**Date:** 2026-06-12 (soak #2, batch `evolve-batch-w4soak2-20260612.log`, cycle 4 of 6)
**Verdict:** cycle-level failure in phase `triage` — review gate rejected the deliverable after 2 corrections
**Classification:** loop defect (gate bug), NOT an agent failure. The triage agent's output was correct all three times.
**Inbox item:** `.evolve/inbox/2026-06-12T12-41-27Z-triagecap-phantom-package-floors.json` (HIGH)
**Investigator:** operator session (soak babysit), live during the failure
**Resolution:** ✅ FIXED interactively 2026-06-12 eve — `fix(triagecap)` commit on main: contract
metadata stripped before package matching (`metadataFieldRE`), prose-collider packages match only
slash-qualified (`pathOnlyPkgs`); both incident shapes replay-pinned (cycle-301 verbatim report
counts 2, cycle-298 bullet counts 1, original cycle-283 overpacking pin preserved at 12).
Throughput window rebaselined (poisoned K=4 entry cleared). Cycle 302 hit the identical deadlock
(7 counted floors; its triage had self-selected this very fix from the inbox before the gate killed
it) — batch stopped by operator after 302, fixed first per owner directive.

---

## Executive summary

The R9.2 triage capacity clamp (`go/internal/triagecap`, `EVOLVE_TRIAGE_CAP_GATE=enforce` default)
counted cycle 301's honest 2-coverage-bullet triage as **6 committed floors** against a cap of 5 and
blocked it. The overcount is deterministic: the counter's known-package vocabulary is built from Go
directory **basenames**, which include `evidence` (go/internal/core/evidence) and `scout`
(go/internal/phases/scout) — and the triage bullet contract **requires** every item to carry
`evidence=...` and `source=scout` metadata. Every floor-bearing bullet therefore gains +2 phantom
floors from its own mandatory formatting. The correction ladder could never converge — the agent
cannot remove contract-mandated tokens — so both corrections re-counted 6 and the cycle failed.

A second, compounding defect: the throughput window the cap is derived from is itself
phantom-poisoned (see §5). K=4 was never a real observation; true observed throughput was 1.

## 1. Timeline (local CST, from log + state.json)

| Time | Event |
|---|---|
| 18:13 | Soak #2 launched (6 cycles). Cycles 298/299/300 PASS and ship. |
| ~20:2x | Cycle 301 triage commits 2 coverage bullets (clihealth, ledger-seal). |
| 20:3x | `[triage-cap] ... 6 committed coverage floors exceed the capacity cap 5 ... BLOCK` (initial). Orchestrator logs `would-salvage misplaced deliverable` (EVOLVE_PHASE_RECOVERY=shadow — log-only). |
| 20:3x | Correction 1/2 re-dispatch → triage re-emits conformant 2-bullet report → counted 6 → BLOCK. |
| 20:37 | Correction 2/2 re-dispatch → triage re-emits again (final on-disk report, mtime 20:37) → counted 6 → BLOCK. |
| 20:43 | `cycle 301: cycle level failure in phase triage: review gate: phase "triage" deliverable rejected after 2 correction(s)`. Worktree `.evolve/worktrees/cycle-301` preserved. Session reap killed 6. Two `failedApproaches` entries recorded (FAIL `cycle-mid-execution-fail` retrospected, + `integrity-breach` bookkeeping). |
| 20:43+ | Loop CONTINUED to cycle 302 (scout dispatched). Batch not halted. |
| 20:41 | Operator filed inbox HIGH `triagecap-phantom-package-floors` (amended 20:47 with window-poisoning finding). |

## 2. Root cause

`CountCommittedFloors` (`go/internal/triagecap/floors.go:59`) counts, for each floor-bearing
`## top_n` bullet (must mention coverage/floor AND a percentage), **one floor per distinct known
package whose name appears as a whole token** in the bullet text (`mentionedPackages`,
floors.go:100; token regex `[A-Za-z0-9_-]+`, floors.go:96).

`KnownPackages` (floors.go:119) builds the vocabulary from **directory basenames** under
`go/internal` and `go/cmd` that contain .go files. This vocabulary includes basenames that collide
with the triage report's own mandatory metadata vocabulary and with ordinary English prose:

- `evidence` — go/internal/core/evidence. Every bullet must contain `evidence=...` → token `evidence`.
- `scout` — go/internal/phases/scout. Every bullet must contain `source=scout` → token `scout`.
- `paths` — matched the prose phrase "error paths" in cycle 298's bullet (see §5).

So a bullet naming exactly one real package counts as 3 floors (1 real + evidence + scout).

## 3. The cycle-301 arithmetic (verified by regex-exact simulation)

Final rejected report (`triage-report-final-rejected.md` in the salvage dir; 2 bullets, both
floor-bearing because they cite "76.2%" / "50% / 52.2%" coverage):

| Bullet | Real pkg | Phantom matches | Counted |
|---|---|---|---|
| clihealth-coverage-boost (evidence=go/internal/clihealth/clihealth.go:48–95) | clihealth | evidence, scout | 3 |
| ledger-seal-coverage (evidence=go/internal/adapters/ledger/seal.go:161–416) | ledger | evidence, scout | 3 |
| **Total** | **2 true** | **+4 phantom** | **6 > cap 5 → BLOCK** |

The simulation replicated floors.go's regexes byte-for-byte against the live report and reproduced
TOTAL=6 exactly.

## 4. Why the correction ladder could not converge

The correction directive tells triage to keep ≤5 floors in `## top_n`. Triage already had only 2
items; the 4 excess "floors" are tokens the bullet contract itself mandates. Any conformant
re-emission re-counts 6. The ladder burned both retries on a fixed point and correctly (per its own
design: "a miscalibrated clamp costs corrections, not a bricked loop" — reviewer.go:29) failed the
cycle rather than looping forever. The ladder's bounded-retry design worked; the clamp's counter is
the defect.

Note: triage COULD have converged by committing only 1 coverage bullet (3 ≤ 5) or by stripping the
percentages from its evidence text (making bullets non-floor-bearing) — but nothing in the
correction directive tells it the phantom-token mechanism, and gaming the percentage out of the
evidence would degrade the report. The directive is unactionable as written for this failure shape.

## 5. Window poisoning (second finding)

`state.json:triageThroughput` = `[{"cycle":298,"floors":4}]` → K=4 → cap=ceil(1.25×4)=5.

Cycle 298's triage had one floor-bearing bullet (`gc-coverage-boost`, cites "88.8% → ≥95%").
Simulation: counted **4** = 1 real (`gc`) + 3 phantoms (`evidence`, `scout`, and `paths` matching
the prose "error paths"). True observed throughput: **1 floor**. K and the cap are fabrications of
the same bug that then used them to reject cycle 301.

Cycles 299/300 escaped only because their bullets cited test counts, not percentage targets — not
floor-bearing, counted 0 (also why they recorded nothing to the window; `Record` no-ops on
floors≤0). The bug was latent until a triage agent wrote precise coverage evidence.

## 6. Impact and residual risk

- Cycle 301: FAIL, no ship. Its selected work (clihealth + ledger-seal coverage) moved to
  `## deferred` paths and remains valid backlog. Worktree preserved (no work lost — triage-only).
- Remaining batch (302–303): any triage committing ≥2 floor-bearing coverage bullets hits the same
  wall. ≤1 coverage bullet passes (3 ≤ 5). A bullet fixing this very defect is safe: no percentage
  target → not floor-bearing → counted 0.
- All future coverage-flavored cycles are throttled to ~1 coverage bullet per cycle until fixed.

## 7. Remediation (filed as inbox HIGH, fix directions)

1. **(strongest) Path-qualified matching** — count a package only when mentioned as
   `go/internal/<pkg>` / `<pkg>/file.go`-style path, not as a bare token. Kills contract-field AND
   prose collisions (`paths`, `core`, `config`, `policy` are all English words).
2. Strip `key=value` metadata fields (`evidence=`, `source=`, `priority=`, `defer_reason=`) from
   bullet text before tokenizing.
3. Blocklist contract vocabulary (`evidence`, `scout`, `source`) from the candidate set (weakest —
   prose collisions remain).

Plus, regardless of direction: **rebaseline or clear `state.json:triageThroughput`** — the healed
counter must not be judged against the poisoned K=4.

TDD pin: cycle-301's verbatim 2-bullet report must count 2; cycle-298's bullet must count 1;
a genuinely overpacked report must still count high.

## 7b. Auto-retrospective concurrence

The loop's own retrospective (`.evolve/runs/cycle-301/retrospective-report.md`) fired and
independently reached the same conclusion: "the clamp built to prevent overpacking (cycles
280/282/283) now over-fires on correctly-sized [commits] ... a **different and new** root cause —
the clamp's [counter]". It did not identify the phantom-token mechanism or the window poisoning
(this document and the inbox item carry those), but its attribution is correct — no
misattribution this time (contrast: cycle-286 retro blamed plan limits for a tmux-server death).

## 8. Evidence inventory

Salvage dir = `.evolve/operator-salvage/cycle-301-triagecap-phantom-floors/` (raw snapshots,
not committed).

| Artifact | Location |
|---|---|
| Final rejected triage report (correction 2/2) | salvage dir (below) `triage-report-final-rejected.md` |
| Log excerpt (all 3 BLOCKs + corrections + failure line) | salvage dir `log-excerpt.txt` |
| failedApproaches entries for 301 | salvage dir `failed-approaches-301.json` |
| Poisoned throughput window | salvage dir `triage-throughput-window.json` |
| Full phase artifacts incl. triage-events/interactions ndjson (all 3 attempts) | `.evolve/runs/cycle-301/` |
| Preserved cycle worktree | `.evolve/worktrees/cycle-301/` (until `evolve cycle reset`) |
| Counter source | `go/internal/triagecap/floors.go:59` (CountCommittedFloors), `:96` (tokenRE), `:119` (KnownPackages) |
| Gate seam | `go/internal/triagecap/reviewer.go:74` (Review), window math `window.go` (K, Cap, Record) |
| Colliding packages | `go/internal/core/evidence`, `go/internal/phases/scout` |
| Inbox item (HIGH, amended) | `.evolve/inbox/2026-06-12T12-41-27Z-triagecap-phantom-package-floors.json` |

## 9. Post-batch follow-ups

- [x] Investigation moved to docs/operations/ and committed with the fix.
- [x] Fixed interactively (TDD, dual-review, gated /commit); triageThroughput cleared.
- [ ] Reclaim cycle-301/302/303 worktrees once evidence is no longer needed.
- [ ] Soak verdict accounting: 301 = infrastructure FAIL (gate defect), distinct from work-quality
  FAIL — relevant when judging the ×3 clean-batch bar for R8.5.
