# Pipeline-Hardening Campaign: 2026-08-16 → 2026-08-17

**What this is:** the complete issue → approach → result record of the weekend console campaign that ran alongside batches 20260816a–20260817b. Companion analysis (per-cycle FAIL taxonomy + ranked plan): [2026-08-17-failure-rate-review-1481-1503.md](2026-08-17-failure-rate-review-1481-1503.md).
**Scoreboard:** 4 pipeline PRs merged (#468 #469 #470 #471) + 1 docs PR open (#472, boundary-held) · 3 lane ships (`1a8e4972`, `4f133efe`, `7ad61417`) · 2 multi-attempt lane bodies salvaged/landed · 4 batch halts, each root-caused to completion before resume · ~12 inbox items filed or extended, every one with live evidence · 2 items parked with executable salvage recipes.

---

## 1. Issue → approach → result

| # | Issue (live evidence) | Approach taken | Result |
|---|---|---|---|
| 1 | **One stray `}` halted a batch** (cycle-1478): audit sentinel = valid 2.8KB WARN JSON + one byte; whole-string parse rejected; prose fallback off at enforce → 3 gate blocks on UNCHANGED bytes (mtime proof) → circuit-open → verdict-incoherence halt while prose, sentinel, and 226 green predicates agreed | Console-first TDD: leading-value `json.Decoder` bounded by the comment capture; the REAL artifact checked in as fixture; gate-level enforce pin; adversarial review APPROVE-W-N | **#468** (`4a805658`). Zero `bad_verdict` blocks in every subsequent batch. Correction-staleness half deliberately split out → item 0.87→0.90 |
| 2 | **Parked scope re-dispatched anyway** (cycle-1487, burn 3): inbox deletion shipped, yet the wave adopted the scope from continuation-registry — and re-registered itself | Read `registry.go`, released bindings under the flock sidecar convention, preserved pointers into item files; filed the class with planner-seam prescription | Burn loop stopped. `park-consume-releases-continuation-binding` 0.86→0.89. Later hardened by discoveries #6/#8/#10 |
| 3 | **Self-authored unsatisfiable predicate** (cycle-1488): `FileContains` used for an ABSENCE assertion — red on every tree; `FileNotContains` existed, its doc even names the anti-idiom (cycle-352) | Filed authoring-lint item; later repaired in the #470 salvage (one-line primitive swap; cascade greened the 1492 meta-predicate) | Landed in **#470**. Lint item 0.8→0.85 (authoring-time shift-left) |
| 4 | **Manufactured FAIL on green work** (cycle-1493): test-amplification phase wrote a `$`-anchored `-run` meta-test (guaranteed `[no tests to run]`) into `go/acs/`; + closure-claim gate false-positive ("fail-closed" compound, path-shaped cycle ref) | Console-first: closure matcher carve-outs red-first on the live line-36 bytes (review HIGH: strong rung widened `verified[ -]closed`); amplification persona placement/`$`-anchor/logged-retraction rules; EGPS-attribution routed to the DESIGNED path (anti-gaming constraint documented, not rushed) | **#469** (`c7903671`). Both matcher classes dead; `phase-authored-red-tests-poison-egps` 0.87 design task queued |
| 5 | **WARN-ship consumption gap** (cycle-1494): acs WARN with 0 reds SHIPPED real work, but #466's PASS-only gate left the tracked item pickable — landed work re-offerable | Consumed console-side at boundary; filed the gate-condition fix (key on the LANDING, not the verdict) | `warn-ship-consumption-gap` 0.82→0.84; no re-pick of sleep-time occurred |
| 6 | **Registry is a first-class lane SOURCE** (cycle-1497): a CONSUMED scope dispatched with no adoption event and no carryover entry — the wave planner mints lanes from registry bindings directly | Verified all three stores empirically; released the binding post-drain; rewrote the 0.86 item's fix to put the guard at the PLANNER's registry read (primary) with adoption as belt | Class pinned at its true seam; four-burn evidence chain in the item |
| 7 | **4-attempt feature 90%-stuck** (verdict-cache, cycles 1485/1488/1492/1495): each retry green on its own work, anchored by the inherited broken predicate | **Console salvage recipe**: fresh branch off main + `cherry-pick -n <snapshot>` + repair + full bar + adversarial review — which found a REAL Put-site operand bug 3 lane audits missed (projectRoot HEAD vs worktree base under fleet concurrency) | **#470** (`e36604e7`): ~1,300 lines of lane work landed; write-side fail-closed; red-first sibling-advance trio; item consumed-as-landed |
| 8 | **Line-locality closure false-RED** (cycle-1502): 165-green/0-red cycle FAILed because a WARN summary line didn't re-cite what the report's own per-id disposition table proved | Lineage-scoped demotion: disposition-covered lineage demotes misses PER LINE (≥1 named cycle, all ∈ lineage). Adversarial review BLOCKED my draft twice — ref-less claims and empty-ledger vouching were laundering leaks — both remedies red-first among 6 tests | **#471** (`3bb59ba5`). Third closure-claim false-RED generation dead; the gate's anti-laundering floor intact by proof |
| 9 | **Registry aliasing** (cycles 1501/1503): the SAME scope bound under two names (`carryover-lane-…` + renamed `lane-…`) — retirement under one name leaves the alias armed | Released both under flock; noted cycle-1498's shipped `retire-consumed-fleet-alias` as the in-tree class fix to verify | Aliases dead this scope; alias-retirement verification rides the salvage |
| 10 | **THIRD dispatch store demonstrated live** (cycle-1505, 2026-08-17): scope parked (inbox) + released (registry) — dispatched anyway from carryoverTodos | Let the cycle drain (mid-flight interference is worse); retire the scope from the carryover store under the state flock at drain; the parked retirement mechanism itself is the class kill | In progress at time of writing; the three-store thesis now has one live demonstration per store |
| 11 | **Correction loop can't fix a one-line error** (cycle-1499): unused import survived THREE corrections | Extended the freshness item: corrections must embed the failing tool output VERBATIM + content-hash freshness (2026 literature: typed diagnosis-before-recovery; same-agent retry without new evidence is weak) | `contract-correction-stale-artifact-freshness` 0.87→0.90 — ranked the cheapest large win |
| 12 | **Five-burn retirement item** (cycles 1486/1493/1496/1500/1503): convergent progress each attempt — mechanism → REAL H1 security finding (`VerifiableBy` executed unclamped = command-injection surface, traced to the ITEM's own prescription) → production wiring uncovered | Parked with an executable salvage recipe (coverage gap `cmd_loop_wave.go:323-405`; harden `validateRetirementCommand` against `;`/`&&`/`||`; keep the clamp); all registry aliases released | Ready for a deliberate console landing — the highest-leverage single fix in the queue |

## 2. Meta-findings (what the weekend actually taught)

1. **Retirement must be transactional across ALL dispatch stores.** Inbox (#466), carryovers, and the continuation registry (including aliases) each independently re-dispatch; every leak was found by burning a lane. One retirement seam, three stores, one landing.
2. **Gates fail safe here** — every defect produced false FAILs, never false PASSes, and each false-RED class died with the live artifact as its regression fixture. The expensive part was diagnosis latency, not damage.
3. **The adversarial review layer is carrying real weight**: it BLOCKed my own drafts three times this weekend (verified-closed evasion; ref-less demotion; empty-ledger vouch) and found a fleet-concurrency operand bug three lane audits missed. Cross-agent correction > self-correction, exactly as the 2026 literature predicts.
4. **Console salvage beats blind retry past ~3 burns.** Verdict-cache: 4 lane attempts stuck, 1 console hour landed it. The retirement item is queued for the same treatment.
5. **Prompt guidance does not reach schedulers.** "Do-not-re-bundle" notes were re-bundled; annotations are for agents, structure is for planners — fixes must land in typed state (registry entries, gates), not prose.

## 3. Reprioritization (applied 2026-08-17, per this review)

| Item | Old → New | Why |
|---|---|---|
| `contract-correction-stale-artifact-freshness` (+output-fidelity) | 0.87 → **0.90** | Cheapest large win; touches every correction across every phase |
| `park-consume-releases-continuation-binding` (planner seam) | 0.86 → **0.89** | Kills the largest waste class at its true source |
| `multi-slug-lane-scope-reconciliation` | 0.87 (hold) | Deterministic FAIL on every bundle; structural split at mint |
| `phase-authored-red-tests-poison-egps` | 0.87 (hold, DESIGN path) | Ship-floor change — must not be rushed |
| `continuation-findings-anchor-first` | 0.78 → **0.86** | Two lanes re-derived green work around a ten-line repair |
| `acs-absence-primitive-and-unsatisfiability-lint` | 0.80 → **0.85** | Authoring-time shift-left for the unsatisfiable-contract family |
| `warn-ship-consumption-gap` | 0.82 → **0.84** | Landed-work re-offer window |
| `posthoc-diff-anchor-no-op-cycle` | 0.80 (hold) | Disclosure honesty; 3 live occurrences |
| `same-subsystem-wave-scheduling` | 0.68 (hold) | Real but rarer |
| **Parked for console salvage** (not weight-queued): `carryover-lane-retirement-verifiableby` (5 burns, recipe ready), `minted-phase-verdict-contract-unsatisfiable`, `context-fill-telemetry-and-cap` | — | Salvage-before-requeue; recipes in the parked files |

## 4. Verification

All PR numbers, merge SHAs, cycle numbers, and gate outputs in this document trace to: the batch logs (`.evolve/logs/batch-2026081{6,7}*.log`), per-cycle run dirs (`.evolve/runs/cycle-14*/`), archived escalation dossiers (`pipeline-escalation.resolved-pr4*.json`), the merged PRs' own test suites (each false-RED class pinned by its live artifact), and the failure-review ledger's §5 evidence map.
