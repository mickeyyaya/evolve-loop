# Cycle 2070 Dossier

**Goal:** Wave 145.

Work the highest-weight lane-eligible inbox items from start to end: claim, tdd, build, audit and ship. Keep full phase integrity.

Every cycle must:
- commit to a claimed inbox item (an empty top_n must not run the spine);
- bind its audit to the changes tree that it ships;
- disposition its own defect ledger and the inherited ids;
- record a terminal outcome on every exit;
- never change a protected control-plane file with a tool;
- leave no stale worktree.

Routing:
- An item whose declared fix surface is a protected file is console work. A pipeline-integrity item that the operator did not route to the lanes is also console work.
- An item that has `route: lane` is lane work. ADR-0099 document deliverables are lane work.
- A card on a protected surface is a route, not a verdict. The cycle ends as planned no-work, and the item goes to the console.

Craft:
- New and changed code has no comments. Names, types and tests give the intent.
- Write each new document and each new log text in ASD-STE100.
- A git-backed test fixture uses internal/gittest. It never uses a raw `git init`.
- A lane never changes go/internal/sizeratchet/offenders.json or the deadline of a gate.

Evidence:
- To know what a cycle did, read the cycle state and the signals (`evolve wave status`). Do not read the text log.

The measure of this wave is the number of shipped cycles. Keep each cycle shipping.

Wave 144 facts (run 20261027T071914Z): 3 of 5 cycles shipped.
- Cycle 2053: final verdict PASS. It shipped.
- Cycle 2054: final verdict FAIL. Termination: cycle aborted in phase retro (abnormal-exit epilogue) cause=…plain material path go/internal/routingtest/engine.go   Explanation Documentation: Changed Areas does not explain material path go/internal/routingtest/fakes.go. Last phase: aborted. It did not ship.
- Cycle 2055: final verdict FAIL. Termination: cycle aborted in phase retro (abnormal-exit epilogue) cause=…/history.go:24 internal/usageprobe.appendHistory 72.2% (13/18)     REFUSED internal/usageprobe/windows.go:31 internal/usageprobe.RecordObservation 90.9% (10/11). Last phase: aborted. It did not ship.
- Cycle 2056: final verdict PASS. It shipped.
- Cycle 2057: final verdict PASS. It shipped.
- Pull requests merged at this boundary: none.

Earlier waves:
- Wave 141 (run 20261026T115530Z): 4 of 6 cycles shipped.
- Wave 142 (run 20261026T182320Z): 1 of 2 cycles shipped.
- Wave 143 (run 20261027T021525Z): 9 of 16 cycles shipped.
**Final verdict:** FAIL
**Run ID:** 01M5YSW6TP1GG8KM32VF9FTFB7
**Committed:** `bridge-sandbox-contract-private-tmux-flake`

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | PASS | 51s |  |
| triage | plan | PASS | 37s |  |
| fault-localization | plan | PASS | 1m4s |  |
| tdd | plan | PASS | 5m19s |  |
| build | build | FAIL | 3m19s |  |
| retro | control | PASS | 2m7s |  |

## Timing

**Total:** 13m17s across 6 phases (0 retried) · **Longest:** tdd 5m19s

| Archetype | Wall-clock |
|-----------|------------|
| build | 3m19s |
| control | 2m7s |
| plan | 7m51s |

## Defects

- **audit-fail** (HIGH): cycle did not pass audit; see audit-report.md + acs-verdict.json — fix: address the audit findings recorded for this cycle


## Failure

**Fingerprint:** `build|infra-error|3e842daff928` · **Class:** infra-error

- review gate: phase "build" deliverable rejected after 1 correction(s): build handoff floor: 1 deterministic check failure(s) — fix these exactly before handoff: the tests of the changed packages fai


## Carryover

- **address-audit-findings** (high): resolve the audit findings that failed cycle 2070

