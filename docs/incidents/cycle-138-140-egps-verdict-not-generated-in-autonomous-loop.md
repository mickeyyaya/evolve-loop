# Incident: cycle-138→140 — EGPS verdict (`acs-verdict.json`) never generated in the autonomous loop → audit structurally forced to FAIL → no clean ship

> **Window:** 2026-05-29 · **Status:** ROOT CAUSE VERIFIED, fix approved (audit phase generates the verdict), NOT yet implemented. · **Severity:** HIGH — blocks the "two clean sequential cycles" standing goal; every autonomous cycle's audit is forced to FAIL regardless of actual quality.
> Companion to [cycle-124-137 challenge-token incident](cycle-124-137-param-mapping-and-challenge-token-single-source.md) and the [regression coverage index](REGRESSION-COVERAGE-INDEX.md).

## 1. Executive summary

| Field | Value |
|---|---|
| **Symptom** | cycles 138 & 139 ran all 7 phases with substantively clean audits (auditor wrote `## Verdict`/`**PASS**`, predicates green), yet neither produced a formal `ship`. 138 shipped only by luck (build committed inline); 139 did not ship at all (`FinalVerdict: SKIPPED_UNKNOWN`, work lost with the worktree). |
| **Root cause** | The audit phase's EGPS gate requires `<workspace>/acs-verdict.json` with `red_count == 0` to PASS, and treats a **missing** file as FAIL **by explicit design** (`go/internal/phases/audit/audit.go:11`). But **nothing in the autonomous `evolve loop` generates `acs-verdict.json`** — it is only produced by the operator/CI command `evolve acs suite` (ADR-0025). So every autonomous cycle's audit is structurally forced to FAIL. |
| **Why it stayed hidden** | The audit-report's text verdict is genuinely `PASS`, so logs/reports *look* clean. The forced-FAIL happens silently inside `Classify` on the missing-file branch, then the state machine routes `audit → retro` instead of `audit → ship`. |
| **Approved fix** | The **audit phase generates `acs-verdict.json`** by running the ACS suite (`acssuite.Run` + `WriteVerdict`) when the file is absent, before reading it — honoring a pre-staged file if present. Makes the autonomous cycle self-contained. |

## 2. Evidence (verified from sealed forensics)

- `cycle-139.reset-20260529T032017.../audit-report.md` → `## Verdict` then `**PASS**` (regex `extractAuditVerdict` returns `"PASS"` — confirmed by probe test).
- `cycle-139.reset-.../acs-verdict.json` → **MISSING**.
- cycle-138 (`SHIPPED_VIA_BUILD`) → also **MISSING** `acs-verdict.json`.
- `VerifyCycle(138)` and `VerifyCycle(139)` against the real ledger → `OK=true, missing=[]` (Bug A's ledger-vocabulary fix works; this is NOT a verify false-negative).
- Older cycles that DO have `acs-verdict.json` (cycle-90/97/101/106) were run via the operator `evolve acs suite` path, not pure `evolve loop`.

## 3. The mechanism

```
audit.Classify(artifact):
  verdict = extractAuditVerdict(report)         # "PASS" (report is genuinely clean)
  redCount, err = readRedCount(workspace/acs-verdict.json)
  if err != nil:                                # ← FILE MISSING in autonomous loop
      verdict = FAIL                            #   forced FAIL, regardless of report
  ...
StateMachine.Next(PhaseAudit, FAIL) → PhaseRetro   # never PhaseShip
```

So: clean report + missing EGPS file ⇒ forced FAIL ⇒ `audit→retro` ⇒ no ship.

### 138 vs 139 — why only one "shipped"

| | acs-verdict.json | audit (forced) | build committed inline? | outcome |
|---|---|---|---|---|
| 138 | missing | FAIL | **yes** (HEAD moved) | `SHIPPED_VIA_BUILD` (work survived by luck) |
| 139 | missing | FAIL | no | `SKIPPED_UNKNOWN` (work lost with worktree) |

The two cycles differed only by whether the build phase happened to commit inline — not by any quality signal. Neither ran the formal `ship` phase.

## 4. Approved fix (NOT yet implemented)

**Owner: the audit phase.** In `audit.Classify` (or a pre-Classify hook), when `<workspace>/acs-verdict.json` is absent, run the ACS suite and write the verdict before reading it:

```go
// pseudo
if _, err := os.Stat(verdictPath); os.IsNotExist(err) {
    v, _ := acssuite.Run(acssuite.Options{Root: rootForCycle(req), Cycle: req.Cycle})
    _, _ = acssuite.WriteVerdict(req.EvolveDir, v)   // writes runs/cycle-N/acs-verdict.json
}
redCount, acsErr := readRedCount(verdictPath)        // now present
```

- `acssuite.Run` already has an injectable `Exec` seam → unit-testable without real shell.
- A pre-staged `acs-verdict.json` (operator/CI) is honored (only generate when absent).
- `Root` must be the per-cycle worktree (where this cycle's `acs/cycle-N/*.sh` predicates live), falling back to project root.
- **Regression test:** a cycle with `acs/cycle-N/` predicates and no pre-staged verdict → audit generates it → `red_count==0` → verdict PASS → state machine routes `audit→ship`.

### Decision record
Approved approach (operator, 2026-05-29): **"Audit phase generates it"** over (b) runner-generates-pre-audit or (c) tolerate-absence. Rationale: keeps the EGPS gate computed from the cycle's real predicates (non-gameable floor preserved), and makes the autonomous cycle self-contained rather than depending on an externally pre-staged file.

## 5. Lessons

1. **A gate that requires an artifact nothing produces is a structural deadlock, not a quality signal.** The EGPS design (ADR-0025) assumed `evolve acs suite` runs; the pure Go `evolve loop` path silently never invokes it. Same class as the cycle-137 ledger-vocabulary drift: a contract assumed by one component, not satisfied by the path that actually runs.
2. **"Looks clean" ≠ "shipped clean."** The audit *report* said PASS; the *gate* forced FAIL on a missing file; the cycle then shipped-or-not based on inline-build luck. Always check the terminal outcome (`FinalVerdict` + did HEAD move), not just the phase reports.
3. **Disambiguated outcome labels earned their keep.** The v12.2 `SHIPPED_VIA_BUILD` vs `SKIPPED_UNKNOWN` split is exactly what made the 138-vs-139 difference legible — without it both would have read as a bare `SKIPPED`.

## 6. References

- Code: `go/internal/phases/audit/audit.go` (EGPS gate, `extractAuditVerdict`, `readRedCount`), `go/internal/acssuite/acssuite.go` (`Run`, `WriteVerdict`), `go/internal/core/statemachine.go:96` (audit→ship/retro).
- ADR: [0025-acs-suite-runner-and-red-team.md](../architecture/adr/0025-acs-suite-runner-and-red-team.md) (the `evolve acs suite` host-side generator).
- EGPS design: [egps-v10.md](../architecture/egps-v10.md).
- Sealed forensics: `.evolve/runs/cycle-138.reset-*`, `.evolve/runs/cycle-139.reset-*`.
- Backlog row: gap #0 in [REGRESSION-COVERAGE-INDEX.md](REGRESSION-COVERAGE-INDEX.md).
