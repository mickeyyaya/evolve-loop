---
name: cycle-82-ship-refused
description: Post-mortem for two ship-refused events in cycle 82 on 2026-05-18
metadata:
  type: incident
  cycle: 82
  date: 2026-05-18
  severity: HIGH
---

> **Incident post-mortem** — Two `ship-refused` abnormal events during cycle 82 on 2026-05-18.
> Event 1: contract gap (fixable). Event 2: expected defensive behavior (no fix needed).

## TLDR

**Synopsis:** Cycle 82 triggered two ship-refused events. Event 1 failed because `--class trivial` requires `cycle_size_estimate="trivial"` in `cycle-state.json` but the field was blank — no auto-population fallback exists. Event 2 failed because HEAD moved between audit bind and ship — this is correct, expected behavior enforced since v8.13.0.

**Key points:**
- Event 1 is a contract gap: triage output must set `cycle_size_estimate=trivial` for `--class trivial` to work
- Event 2 is expected: the audit-binding invariant correctly detects HEAD movement and refuses
- Remediation for Event 1: set `cycle_size_estimate` manually via `cycle-state.sh set cycle_size_estimate trivial`

**Non-goals:** Changes to ship.sh audit verification logic; changes to `--class cycle` behavior.

## Table of Contents

1. [What Happened](#what-happened)
2. [Research](#research)
3. [Root Cause](#root-cause)
4. [Fix](#fix)
5. [Lessons Learned](#lessons-learned)
6. [References](#references)

---

## What Happened

Two `ship-refused` events were appended to `abnormal-events.jsonl` during cycle 82 on 2026-05-18:

| # | Timestamp | Error message |
|---|-----------|---------------|
| 1 | 20:23:43Z | `ship --class trivial requires cycle_size_estimate='trivial' in cycle-state.json (got: '')` |
| 2 | 20:38:49Z | `git HEAD has moved since audit (audited=1b85426 current=97717b3) — re-run Auditor` |

Both events halted the ship phase with exit code 2 (integrity failure). The cycle did not ship on its first attempts. Ship eventually succeeded after manual remediation.

---

## Research

**Event 1 — trivial class precondition:**

`ship.sh:L319` reads `cycle_size_estimate` from `cycle-state.json` via `cycle-state.sh get cycle_size_estimate`. This field is mirrored from `triage-decision.md` inside `phase-gate.sh:gate_discover_to_triage` (L464). In cycle 82, triage either:
- Was not run, OR
- Did not produce `triage-decision.md` with `cycle_size_estimate: trivial`, OR
- The mirror step silently warned and continued without writing the field

The `--class trivial` path (v10.6.0) performs a hard-fail if `cycle_size_estimate` is not exactly `"trivial"`. There is no auto-population fallback. There is also no guard in `run-cycle.sh` that warns early when `--class trivial` is intended but the precondition field is unset.

**Event 2 — HEAD-moved since audit:**

`ship.sh` reads the latest audit ledger entry to extract the SHA that the Auditor bound its verdict to. It then compares that SHA against the current `git rev-parse HEAD`. If they differ, ship is refused. This invariant has been in place since v8.13.0. It is intentional: if HEAD moved after the audit (e.g., a manual commit, a rebase, a prior ship attempt that partially committed), the audit is no longer valid for the current tree.

In cycle 82, the audited SHA was `1b85426`. By the time the second ship attempt ran, HEAD was at `97717b3`, indicating something had committed to the branch between the audit and the second ship attempt.

---

## Root Cause

**Event 1:** Contract gap — the `--class trivial` path hard-fails on a blank `cycle_size_estimate` field with no ergonomic remediation path surfaced in the error message or usage header. The operator must know to run `cycle-state.sh set cycle_size_estimate trivial` manually, but this is not documented at the point of failure.

**Event 2:** Expected behavior — the audit-binding invariant (v8.13.0+) correctly detected that HEAD had moved since the Auditor ran. This is defensive, correct behavior. The first ship attempt likely succeeded partially (or a prior manual commit landed), advancing HEAD. The remediation hint in the error message is correct: re-run the Auditor on the new state before shipping.

---

## Fix

**Event 1 — ship.sh usage header improvement (applied this cycle):**

Added a remediation hint to the `--class trivial` entry in the usage header (L34–36) clarifying that `cycle_size_estimate` must be set and how to set it:

```
# Remediation: if cycle_size_estimate is blank, run:
#   bash legacy/scripts/lifecycle/cycle-state.sh set cycle_size_estimate trivial
```

No runtime behavior change. The hard-fail on blank `cycle_size_estimate` is intentional (contract enforcement). The fix improves discoverability of the remediation path.

**Event 2 — no fix needed:**

The audit-binding invariant is correct. Operators who encounter this error should follow the displayed remediation hint: re-run the Auditor subagent against the current tree, then re-run ship. No structural change to `ship.sh` is warranted.

**Optional future improvement (not this cycle):**
When `cycle_size_estimate` is blank and `--class trivial` is passed, auto-populate from `triage-decision.md` if it exists and indicates trivial — but only if `triage-decision.md` is present and unambiguous. This is a LOW-risk ergonomic enhancement, scoped to a future cycle.

---

## Lessons Learned

1. **Error messages should include the fix command.** The Event 1 error string contained the contract description but not the remediation command. Operators shouldn't need to read source to fix a precondition failure.

2. **The trivial class is triage-dependent.** `--class trivial` cannot be used ad-hoc without running triage first. This dependency is implicit — it should be explicit in the usage header.

3. **Event 2 is a known, healthy invariant.** HEAD-moved-since-audit is one of the most important integrity checks in ship.sh. It should never be bypassed. Operators seeing this error should re-run the Auditor, not seek a bypass.

4. **Abnormal events are diagnostic gold.** The `abnormal-events.jsonl` format with timestamps, source phase, and severity made this post-mortem straightforward. The format works.

---

## References

- `legacy/scripts/lifecycle/ship.sh` — L319 (trivial class precondition check), L86-97 (integrity_fail + abnormal event write)
- `legacy/scripts/lifecycle/phase-gate.sh` — `gate_discover_to_triage` L464 (cycle_size_estimate mirror)
- `legacy/scripts/lifecycle/cycle-state.sh` — `set` subcommand for manual field override
- `docs/architecture/` — ship-gate architecture (v8.13.0 audit-binding contract)
- `CHANGELOG.md` — v10.6.0 (`--class trivial` introduction)
- Abnormal events log: `.evolve/runs/cycle-82/abnormal-events.jsonl`
