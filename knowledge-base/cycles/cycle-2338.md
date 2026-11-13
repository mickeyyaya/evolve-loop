# Cycle 2338 Dossier

**Goal:** Wave 167.

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

Wave 166 facts (run 20261111T162732Z): 11 of 16 cycles shipped.
- Cycle 2320: final verdict PASS. It shipped.
- Cycle 2321: final verdict PASS. It shipped.
- Cycle 2322: final verdict PASS. It shipped.
- Cycle 2323: final verdict PASS. It shipped.
- Cycle 2324: final verdict PASS. It shipped.
- Cycle 2325: final verdict PASS. It shipped.
- Cycle 2326: final verdict PASS. It shipped.
- Cycle 2327: final verdict PASS. It shipped.
- Cycle 2328: final verdict PASS. It shipped.
- Cycle 2329: final verdict PASS. It shipped.
- Cycle 2330: final verdict PASS. It shipped.
- Cycle 2331: final verdict FAIL. Termination: cycle aborted in phase retro (abnormal-exit epilogue) cause=…ting helper instead of a copy (docs/architecture/test-system.md §6.4, §6.5):   acs/cycle-N/predicates_test.go:goDir = acs/cycle-N/predicates_test.go:moduleDir. Last phase: aborted. It did not ship.
- Cycle 2332: final verdict FAIL. Last phase: retro. It did not ship.
- Cycle 2333: final verdict PASS. It shipped.
- Cycle 2334: final verdict FAIL. Termination: cycle aborted in phase retro (abnormal-exit epilogue) teardown=submit_wedged: …00s extends_used=0 max_extends=6 last_review=pause liveness=unknown progressed=false busy=false transient=false detector_error="": core: bridge artifact timeout. Last phase: aborted. It did not ship.
- Cycle 2335: final verdict FAIL. Last phase: memo. It did not ship.
- Pull requests merged at this boundary: none.

Earlier waves:
- Wave 163 (run 20261109T204337Z): 14 of 16 cycles shipped.
- Wave 164 (run 20261110T083435Z): 8 of 16 cycles shipped.
- Wave 165 (run 20261111T013325Z): 14 of 32 cycles shipped.
**Final verdict:** FAIL
**Run ID:** 01M793F1CR7W3C0TRCT9YEJGJQ
**Committed:** `flake-ship-cancel-wait-reports-a-clean-exit`

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | FAIL | 1m10s |  |
| retro | control | PASS | 2m36s |  |

## Timing

**Total:** 3m46s across 2 phases (0 retried) · **Longest:** retro 2m36s

| Archetype | Wall-clock |
|-----------|------------|
| control | 2m36s |
| plan | 1m10s |

## Defects

- **audit-fail** (HIGH): cycle did not pass audit; see audit-report.md + acs-verdict.json — fix: address the audit findings recorded for this cycle


## Failure

**Fingerprint:** `scout|infra-error|c7d89db3f2ab` · **Class:** infra-error

- phase scout: scout: bridge: bridge: launch exit=87: [agy-claude-tmux] session killed: evolve-bridge-agy-r01M793F1-c2338-scout-pid42468-n5-1794528311


## Carryover

- **address-audit-findings** (high): resolve the audit findings that failed cycle 2338

