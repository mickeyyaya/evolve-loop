# FAIL analysis & pass-rate levers — 37 post-fix cycles (2026-07-21)

Scope: every FAIL across batches 1–3 after the v22.5.0-era pipeline fixes
(cycles 974–1009; 24 PASS / 13 FAIL ≈ 65%). Zero false-REDs and zero
system-misclassifications *except* the S4 gap (1001's audit said SYSTEM-class,
cycle recorded `SystemFailure: null`).

## The 13 FAILs, clustered

| Cluster | Cycles | Share | Cure (queue item, weight) |
|---|---|---|---|
| **A. Shared-state disease** (RMW lost-write + worktree→canonical propagation) | 992, 994, 995, 999, 1000, 1001 | 6/13 | `statejson-stalerevision-cas-lost-write` 0.96 · `statejson-worktree-canonical-propagation` 0.95 |
| **B. Structural mismatch: live-state tasks in worktree builders** | 984 (+ overlaps A: 992/994/999/1000) | 1–5/13 | `operator-state-task-archetype-native-apply` 0.93 (NEW) |
| **C. Builder handed off known-broken work** | 983, 1007(part), 1008, 1009 | 4/13 | `build-selfcheck-hard-gate-at-deliverable-contract` 0.94 (NEW) |
| **D. Correct rejections (keep!)** | 975 inert-API, 991 premise-BLOCK | 2/13 | none — this is the pipeline working |
| **E. All-or-nothing on monotonic work** | 992 (101 verified prunes discarded) | 1/13 | `monotonic-partial-progress-landing` 0.90 (NEW) |

## The three decisive findings

1. **Smoking gun (C):** cycle-1008's `build-selfcheck.json` records `./cmd/evolve`
   FAILING at handoff — the builder ran the check, saw red, and handed off anyway;
   983 never ran it. The artifact and the ACS backstop both exist; the missing
   piece is one contract check at the E2 deliverable seam (red selfcheck ⇒ reject
   build deliverable ⇒ in-phase correction ladder). Converts a ~12.5M-token FAIL
   cycle into a ~1–2M in-phase iteration.
2. **The warning existed before the incident (A):** the adversarial reviewer
   flagged the state RMW race as F1 MEDIUM in cycle-992 *before* it destroyed
   data in 1001 — and nothing escalated it into the queue. Cross-cycle synthesis
   of review findings is the same gap `lesson_to_action_gap` names.
3. **No within-batch learning (S5):** after 999 failed on the state defect, the
   very next wave drew two more same-family tasks (0/2 wave); the telemetry item
   burned 3 attempts rediscovering the same `./cmd/evolve` breakage.
   `failedApproaches` recorded everything ("would-have-blocked: BLOCK-CODE ×8")
   but fluent mode only advises and triage doesn't consult it for *selection*.
   → `adr0072-s5-task-quarantine` escalated to 0.92; retro must also write
   defect lists + preconditions back INTO the driving item.

## Projected pass rate if the levers land

A (−6) + C (−3~4) + B residual (−1) + E (−1) ⇒ FAILs reduce to the healthy
floor of correct rejections (D) + genuinely new defects: **~90% expected**,
with the remaining FAILs being ones we *want* (premise blocks, inert-API
refusals, honest integrity refusals like 1001's builder declining to fabricate).

## Ordering note

C (0.94) before everything else on ROI: one contract check, existing machinery,
prevents the most common self-inflicted class. A (0.96/0.95) is the correctness
prerequisite for any live-state work and un-quarantines 7 items. B/E redesign
the task classes that structurally cannot pass today.
