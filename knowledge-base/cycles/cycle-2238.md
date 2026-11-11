# Cycle 2238 Dossier

**Goal:** Wave 162.

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

Wave 161 facts (run 20261108T161932Z): 9 of 14 cycles shipped.
- Cycle 2216: final verdict PASS. It shipped. Quota pauses: 1.
- Cycle 2219: final verdict PASS. It shipped. Quota pauses: 1.
- Cycle 2220: final verdict FAIL. Last phase: end. It did not ship. Quota pauses: 1.
- Cycle 2223: final verdict PASS. It shipped. Quota pauses: 1.
- Cycle 2225: final verdict PASS. It shipped.
- Cycle 2226: no final verdict. Last phase: audit. It did not ship. Login walls: 1.
- Cycle 2227: final verdict PASS. It shipped.
- Cycle 2228: final verdict PASS. It shipped.
- Cycle 2229: final verdict PASS. It shipped.
- Cycle 2230: final verdict PASS. It shipped.
- Cycle 2231: no final verdict. Last phase: audit. It did not ship. Login walls: 1.
- Cycle 2232: no final verdict. Last phase: audit. It did not ship. Login walls: 1.
- Cycle 2233: final verdict SKIPPED_UNKNOWN. Termination: triage-empty-commitment. Last phase: triage. It did not ship.
- Cycle 2234: final verdict PASS. It shipped.
- Pull requests merged at this boundary: none.

Operator notes for wave 162:
- Seat token set; claude seats run isolated on it (fix for the 8-24 h logout). Main CI fixed (#1373). gc plan #1380 queued as gc-legacy-gap-x1..x11. Cycles 2226 and 2170 stopped on operator request; 2232 lost its lane checkpoint to a console stop and restarts from its item.

Earlier waves:
- Wave 158 (run 20261106T093235Z): 2 of 8 cycles shipped.
- Wave 159 (run 20261106T154410Z): 0 of 4 cycles shipped.
- Wave 160 (run 20261107T090007Z): 9 of 16 cycles shipped.
**Final verdict:** FAIL
**Run ID:** 01M6ZCY4KQVXGT4JJ882KCN9T2
**Committed:** `gc-legacy-gap-x10-gc-runs-when-a-cycle-seals`

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | PASS | 9m17s |  |
| triage | plan | PASS | 3m39s |  |
| tdd | plan | PASS | 5m11s |  |
| build | build | FAIL | 2h8m25s |  |
| retro | control | PASS | 1m55s |  |

## Timing

**Total:** 2h28m27s across 5 phases (1 retried) · **Longest:** build 2h8m25s

| Archetype | Wall-clock |
|-----------|------------|
| build | 2h8m25s |
| control | 1m55s |
| plan | 18m8s |

## Defects

- **audit-fail** (HIGH): cycle did not pass audit; see audit-report.md + acs-verdict.json — fix: address the audit findings recorded for this cycle


## Failure

**Fingerprint:** `build|infra-error|ae4cbae9fd8b` · **Class:** infra-error

- phase build: build: bridge: bridge: launch exit=81: artifact-timeout: cause=review_pause reason="agent hung: stalled for 3 consecutive busy intervals — fast-fail before 6-interval backstop" phase=bu


## Carryover

- **address-audit-findings** (high): resolve the audit findings that failed cycle 2238

