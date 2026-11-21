# Cycle 2585 Dossier

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
**Run ID:** 01M7YKCJDPHZK0HZK4VZ5HJ7WF
**Committed:** `system-halt-drains-the-pool`

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | PASS | 41s |  |
| triage | plan | PASS | 41s |  |
| tdd | plan | PASS | 39s |  |
| build | build | PASS | 21m26s |  |
| audit | evaluate | FAIL | 5m56s |  |
| build | build | PASS | 48m22s |  |
| audit | evaluate | FAIL | 2m33s |  |
| retro | control | PASS | 1m23s |  |

## Timing

**Total:** 1h21m41s across 8 phases (0 retried) · **Longest:** build 48m22s

| Archetype | Wall-clock |
|-----------|------------|
| build | 1h9m49s |
| control | 1m23s |
| evaluate | 8m29s |
| plan | 2m0s |

## Defects

- **audit-fail** (HIGH): cycle did not pass audit; see audit-report.md + acs-verdict.json — fix: address the audit findings recorded for this cycle


## Failure

**Fingerprint:** `audit|verdict-fail|ac639195d500` · **Class:** verdict-fail

- the integration tier (`go test -tags integration`) reported 4 offender(s) locally. NOTE — host/CI parity gap: this host HAS tmux and CI runners do not, so every test guarded by requireTmux runs here
- verdict-conflict: auditor narrative=WARN but 1 deterministic gate(s) forced FAIL [integration-tier gate] — the gate outranks the narrative (ship policy unchanged); both readings are recorded so the 


## System Failure

**Category:** infra-systemic
**Level:** system
**Evidence:** orchestrator-classified infra-systemic: failure-dossier.json audit_declared.class=infrastructure-systemic; audit-fail-reason.json verdict-conflict names only the integration-tier gate; integration-tier.log has 4 'panic: test timed out after 10m0s' on go/cmd/evolve and 0 '--- FAIL' lines; acs-verdict.json red=0; same fingerprint ac639195d500 as cycle 2576
**Halt:** true

## Carryover

- **address-audit-findings** (high): resolve the audit findings that failed cycle 2585

