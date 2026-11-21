# Cycle 2587 Dossier

**Goal:** Wave 185.

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

Wave 184 facts (run 20261120T204107Z): 5 of 8 cycles shipped.
- Cycle 2563: final verdict PASS. It shipped.
- Cycle 2574: final verdict PASS. It shipped.
- Cycle 2575: final verdict PASS. It shipped.
- Cycle 2576: final verdict FAIL. Last phase: memo. It did not ship.
- Cycle 2577: final verdict FAIL. Termination: cycle aborted in phase retro (abnormal-exit epilogue) cause=…directories): .git  fatal: not a git repository (or any of the parent directories): .git fatal: not a git repository (or any of the parent directories): .git…. Last phase: aborted. It did not ship.
- Cycle 2578: final verdict PASS. It shipped.
- Cycle 2579: final verdict FAIL. Last phase: memo. It did not ship.
- Cycle 2580: final verdict PASS. It shipped.
- Pull requests merged at this boundary: none.

Earlier waves:
- Wave 181 (run 20261119T004314Z): 22 of 23 cycles shipped.
- Wave 182 (run 20261119T213950Z): 7 of 11 cycles shipped.
- Wave 183 (run 20261120T094708Z): 9 of 15 cycles shipped.
**Final verdict:** FAIL
**Run ID:** 01M7YKCJE99V3Q23WBH9EYGTS3
**Committed:** `main-ci-red-is-polled-while-the-wave-runs`

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | PASS | 39s |  |
| triage | plan | PASS | 40s |  |
| fault-localization | plan | PASS | 1m19s |  |
| tdd | plan | PASS | 2m37s |  |
| build | build | PASS | 26m57s |  |
| audit | evaluate | FAIL | 13m5s |  |
| tdd | plan | PASS | 5m55s |  |
| build | build | FAIL | 26m7s |  |
| retro | control | PASS | 5m3s |  |

## Timing

**Total:** 1h22m22s across 9 phases (0 retried) · **Longest:** build 26m57s

| Archetype | Wall-clock |
|-----------|------------|
| build | 53m4s |
| control | 5m3s |
| evaluate | 13m5s |
| plan | 11m10s |

## Defects

- **audit-fail** (HIGH): cycle did not pass audit; see audit-report.md + acs-verdict.json — fix: address the audit findings recorded for this cycle


## Failure

**Fingerprint:** `audit|verdict-fail|4db8b1c2d912` · **Class:** verdict-fail

- the integration tier (`go test -tags integration`) reported 3 offender(s) locally. NOTE — host/CI parity gap: this host HAS tmux and CI runners do not, so every test guarded by requireTmux runs here
- verdict-conflict: auditor narrative=WARN but 1 deterministic gate(s) forced FAIL [integration-tier gate] — the gate outranks the narrative (ship policy unchanged); both readings are recorded so the 


## Carryover

- **address-audit-findings** (high): resolve the audit findings that failed cycle 2587

