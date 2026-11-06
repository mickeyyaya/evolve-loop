# Cycle 2195 Dossier

**Goal:** Wave 157.

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

Wave 156 facts (run 20261105T005830Z): 3 of 9 cycles shipped.
- Cycle 2172: final verdict FAIL. Termination: cycle aborted in phase retro (abnormal-exit epilogue) cause=spine gate: next=ship blocked — a mandatory predecessor's handoff artifact is missing (clean absence, fail-closed; cycle-N class). Last phase: aborted. It did not ship.
- Cycle 2173: final verdict PASS. It shipped.
- Cycle 2174: no final verdict. Last phase: audit. It did not ship. Quota pauses: 1.
- Cycle 2175: final verdict FAIL. Termination: cycle aborted in phase retro (abnormal-exit epilogue) cause=spine gate: next=ship blocked — a mandatory predecessor's handoff artifact is missing (clean absence, fail-closed; cycle-N class). Last phase: aborted. It did not ship.
- Cycle 2176: final verdict PASS. It shipped.
- Cycle 2177: final verdict FAIL. Termination: cycle aborted in phase retro (abnormal-exit epilogue) cause=spine gate: next=ship blocked — a mandatory predecessor's handoff artifact is missing (clean absence, fail-closed; cycle-N class). Last phase: aborted. It did not ship.
- Cycle 2178: final verdict PASS. It shipped.
- Cycle 2179: no final verdict. Last phase: audit. It did not ship. Quota pauses: 1.
- Cycle 2180: final verdict FAIL. Termination: cycle aborted in phase retro (abnormal-exit epilogue) cause=spine gate: next=ship blocked — a mandatory predecessor's handoff artifact is missing (clean absence, fail-closed; cycle-N class). Last phase: aborted. It did not ship.
- Pull requests merged at this boundary: none.

Earlier waves:
- Wave 153 (run 20261104T002909Z): 0 of 1 cycles shipped.
- Wave 154 (run 20261104T032203Z): 0 of 9 cycles shipped.
- Wave 155 (run 20261104T141300Z): 0 of 11 cycles shipped.
**Final verdict:** FAIL
**Run ID:** 01M6QW26BMZZ44MF6Y0EWMSX3Y
**Committed:** `model-currency-c5-launch-model-verification-remainder`

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | PASS | 1m50s |  |
| triage | plan | PASS | 48s |  |
| tdd | plan | PASS | 4m2s |  |
| build | build | PASS | 5m47s |  |
| test-amplification | evaluate | PASS | 1m32s |  |
| audit | evaluate | WARN | 9m40s |  |
| build | build | FAIL | 3m54s |  |
| build | build | PASS | 37s |  |
| audit | evaluate | WARN | 5m5s |  |
| ship | control | FAIL | 34s |  |
| audit | evaluate | WARN | 5m43s |  |
| ship | control | FAIL | 36s |  |
| audit | evaluate | WARN | 8m15s |  |
| ship | control | FAIL | 35s |  |

## Timing

**Total:** 48m58s across 14 phases (1 retried) · **Longest:** audit 9m40s

| Archetype | Wall-clock |
|-----------|------------|
| build | 10m19s |
| control | 1m45s |
| evaluate | 30m14s |
| plan | 6m39s |

## Defects

- **audit-fail** (HIGH): cycle did not pass audit; see audit-report.md + acs-verdict.json — fix: address the audit findings recorded for this cycle


## Failure

**Fingerprint:** `ship|unknown|91656ee0ca16` · **Class:** unknown

- cycle aborted in phase ship (abnormal-exit epilogue) cause=…ted the push of 5243eb97ee28be13f9b92d07bd7e07498bf2e248 because origin/main diverged; Audit must run again on the new base (ship does not


## Carryover

- **address-audit-findings** (high): resolve the audit findings that failed cycle 2195

