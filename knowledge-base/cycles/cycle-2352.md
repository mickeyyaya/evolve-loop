# Cycle 2352 Dossier

**Goal:** Wave 170.

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

Wave 169 facts (run 20261113T084906Z): 1 of 4 cycles shipped.
- Cycle 2347: final verdict SKIPPED_UNKNOWN. Termination: triage-empty-commitment. Last phase: triage. It did not ship.
- Cycle 2348: final verdict FAIL. Termination: cycle aborted in phase retro (abnormal-exit epilogue) cause=…/mickeyyaya/evolve-loop/go/internal/core runs 3 times; the cycle-N suite must vet it once FAIL FAIL github.com/mickeyyaya/evolve-loop/go/acs/cycle-N 2.185s FAIL. Last phase: aborted. It did not ship.
- Cycle 2349: final verdict FAIL. Termination: cycle aborted in phase retro (abnormal-exit epilogue) cause=… helper instead of a copy (docs/architecture/test-system.md §6.4, §6.5):   acs/cycle-N/predicates_test.go:moduleDir = acs/cycle-N/predicates_test.go:moduleDir. Last phase: aborted. It did not ship.
- Cycle 2350: final verdict PASS. It shipped.
- Pull requests merged at this boundary: none.

Earlier waves:
- Wave 166 (run 20261111T162732Z): 11 of 16 cycles shipped.
- Wave 167 (run 20261112T235603Z): 1 of 4 cycles shipped.
- Wave 168 (run 20261113T054547Z): 3 of 4 cycles shipped.
**Final verdict:** FAIL
**Run ID:** 01M7A58HCKC03M6YCS780Z9KZ1
**Committed:** `rebuild-pr-1254-acs-compile`

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | PASS | 59s |  |
| triage | plan | PASS | 36s |  |
| tdd | plan | PASS | 3m16s |  |
| build | build | PASS | 14m29s |  |
| tdd | plan | PASS | 2m58s |  |
| build | build | PASS | 14m48s |  |
| audit | evaluate | FAIL | 1h2m36s |  |
| tdd | plan | PASS | 9m1s |  |
| build | build | PASS | 33m58s |  |
| audit | evaluate | FAIL | 2m36s |  |
| retro | control | PASS | 1m26s |  |

## Timing

**Total:** 2h26m42s across 11 phases (0 retried) · **Longest:** audit 1h2m36s

| Archetype | Wall-clock |
|-----------|------------|
| build | 1h3m15s |
| control | 1m26s |
| evaluate | 1h5m12s |
| plan | 16m49s |

## Defects

- **audit-fail** (HIGH): cycle did not pass audit; see audit-report.md + acs-verdict.json — fix: address the audit findings recorded for this cycle


## Failure

**Fingerprint:** `audit|verdict-fail|28a4fbe54bf6` · **Class:** verdict-fail

- phase audit verdict FAIL routed to retro (agent-graded; see the audit report artifact) defect=H2: acceptance 'every acs package compiles again' undelivered: 10 acs packages (go/acs-vet-baseline.txt:1-


## Carryover

- **address-audit-findings** (high): resolve the audit findings that failed cycle 2352

