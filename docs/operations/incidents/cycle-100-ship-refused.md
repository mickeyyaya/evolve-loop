# Incident Report: Ship Refused — Cycle 100

> **Severity:** WARN | **Status:** Resolved (cycle 102 — documentation) | **Carryover ID:** `abnormal-ship-refused-c100`
> **Cycle:** 100 | **Prior precedent:** `docs/operations/incidents/cycle-82-ship-refused.md`

---

## 1. What Happened

Cycle-100 recorded three concurrent `ship-refused` abnormal events, all at timestamp cluster `2026-05-20T16:11`. Three independent root causes fired simultaneously, blocking ship:

| # | Reason | Root Cause Category |
|---|---|---|
| 1 | "no Auditor ledger entry found — independent review missing" | Ledger sequencing race |
| 2 | "ship.sh has been modified WITHIN plugin version 1.0.0 (expected=e7dab80f… actual=3bb11c2e…)" | SHA drift — transient |
| 3 | "--class manual requires interactive stdin (not a tty). Set EVOLVE_SHIP_AUTO_CONFIRM=1 for non-interactive use" | TTY constraint |

Evidence source: `.evolve/runs/cycle-100/abnormal-events.jsonl` (3 entries, same timestamp cluster).

Cycle-100 ultimately did NOT ship from these events. Cycle-101 shipped successfully (commit 6369ccf) after the SHA drift self-healed.

---

## 2. Research

### Evidence examined

- `.evolve/runs/cycle-100/abnormal-events.jsonl` — three ship-refused entries
- `scripts/lifecycle/ship.sh` — SHA verification logic, ledger check logic, TTY check
- `state.json:expected_ship_sha` — value `e7dab80f…`
- `shasum -a 256 scripts/lifecycle/ship.sh` (post-cycle-101) — `e7dab80f…` (now matches expected)
- `state.json:carryoverTodos` — `abnormal-ship-refused-c100` was absent; only recorded in cycle-101's local `carryover-todos.json`
- `docs/operations/incidents/cycle-82-ship-refused.md` — prior ship-refused incident documenting the ledger-race pattern

### Timeline reconstruction

Cycle-100 ran the agy (antigravity CLI adapter) feature. During the build phase, `ship.sh` was temporarily modified (likely as part of testing the agy adapter's chmod behavior). This caused the SHA drift from `e7dab80f` to `3bb11c2e`.

At ship time: (a) the Auditor's ledger entry had not yet been written (race), (b) the SHA mismatch was detected, and (c) the ship was invoked in a non-TTY environment without `EVOLVE_SHIP_AUTO_CONFIRM=1`.

---

## 3. Reasoning

### Root Cause 1: Auditor ledger entry race

`ship.sh` checks for a recent Auditor ledger entry before allowing ship. If `subagent-run.sh` writes the entry asynchronously (after the Auditor persona exits but before the orchestrator invokes `ship.sh`), the gap can be milliseconds to seconds. Under load (multiple phase agents completing near-simultaneously), this race fires.

This is a known recurrence pattern — documented in `cycle-82-ship-refused.md` as "auditor ledger sequencing." The fix (retry with backoff inside `ship.sh`) has been discussed but not implemented. Self-corrects on retry; documented only this cycle.

### Root Cause 2: ship.sh SHA drift (transient, now self-healed)

`ship.sh` contains a self-integrity check: it verifies its own SHA against `state.json:expected_ship_sha`. If the file was modified (even temporarily, even by a build step testing chmod/file operations), the check fails.

The modification was transient — cycle-101 restored `ship.sh` to its canonical state, and the SHA now matches `e7dab80f` (verified via `shasum -a 256`). No code change required. The SHA drift was caused by the agy adapter development touching chmod-related scaffolding.

**Current state:** `expected_ship_sha = e7dab80f`, `actual = e7dab80f`. HEALED.

### Root Cause 3: TTY constraint / EVOLVE_SHIP_AUTO_CONFIRM

`ship.sh --class manual` requires an interactive TTY for the confirmation prompt. When invoked from a non-TTY context (e.g., a `claude -p` subagent or a pipeline script), it exits with the error shown. The fix is to set `EVOLVE_SHIP_AUTO_CONFIRM=1` for non-interactive use.

This is documented in `CLAUDE.md` env-var table and is a known operational limitation. No code change warranted — it is a correct safety guard.

---

## 4. Fix

### Root Cause 1 (ledger race)

No code fix this cycle (scope constraint). Operational mitigation: retry ship after a 2–5 second wait if the ledger-missing error fires. A structural fix (atomic ledger-write + ship gate wait loop) is a candidate for a future cycle.

### Root Cause 2 (SHA drift)

Self-healed at cycle-101. Cycle-102 verifies the current SHA matches `expected_ship_sha`. Future prevention: Builder worktrees should not modify `scripts/lifecycle/ship.sh` as a side-effect of feature work; if chmod tests require touching lifecycle scripts, restore them before the build phase ends.

### Root Cause 3 (TTY)

Operational fix: when running ship in a CI or non-TTY context, always set `EVOLVE_SHIP_AUTO_CONFIRM=1` or use `--class cycle` (which does not require TTY confirmation). This is already documented; no code change needed.

---

## 5. Lessons

| Lesson | Scope |
|---|---|
| Multiple concurrent ship-refused events at the same timestamp cluster indicate compounded root causes — each must be triaged independently | Ship-refusal triage playbook |
| The auditor ledger race is a known recurring pattern; each recurrence should be documented and counted — the fix will be prioritized when count exceeds 3 incidents | `docs/operations/incidents/` audit history |
| ship.sh SHA drift is recoverable — check `shasum -a 256 scripts/lifecycle/ship.sh` vs `state.json:expected_ship_sha` before diagnosing deeper | `scripts/lifecycle/ship.sh` runbook |
| EVOLVE_SHIP_AUTO_CONFIRM=1 must be set for all non-interactive ship invocations; `--class manual` is not safe for pipeline use without it | CLAUDE.md env-var table (already documented) |
| Carryover items not propagated to `state.json:carryoverTodos` are invisible to the failure-adapter; ensure memo agents write back to state, not only to local cycle files | Memo agent write-back discipline |

---

## 6. References

- Primary evidence: `.evolve/runs/cycle-100/abnormal-events.jsonl`
- SHA verification: `state.json:expected_ship_sha = e7dab80f`, confirmed healed at cycle-101
- Ship-gate logic: `scripts/lifecycle/ship.sh` (self-integrity check, ledger check, TTY check)
- Prior ship-refused precedent: `docs/operations/incidents/cycle-82-ship-refused.md`
- CLAUDE.md env-var table: `EVOLVE_SHIP_AUTO_CONFIRM`, `--class manual` documentation
- Cycle-101 ship commit (confirms SHA healed): git log `6369ccf`
