# EGPS — Execution-Grounded Process Supervision (v10.0.0+)

> Unified architectural pattern that replaces model-claimed verdicts with sandbox exit codes. Subsumes the 5 gaming signals diagnosed in cycles 30–39 (AC-by-grep, confidence cliff, recurring isolation breach, self-referential tautological-eval, carryover work-shifting). Research basis: [knowledge-base/research/execution-grounded-process-supervision-2026.md](../../knowledge-base/research/execution-grounded-process-supervision-2026.md).

## One sentence

**Stop letting any model — Builder, Auditor, or judge — report whether the work is done; let the sandbox's exit code be whether the work is done.**

## Verdict contract change (BREAKING)

| Pre-v10 | v10.0.0+ |
|---|---|
| `audit-report.md` declares `Verdict: PASS\|WARN\|FAIL` + `confidence: 0.0–1.0` | `acs-verdict.json` declares `verdict: PASS\|FAIL` (binary, no scalar) |
| Auditor narrative drives ship/no-ship | `acs-verdict.json:red_count == 0` IS the ship gate |
| WARN ships by default (fluent posture) | No WARN level exists; any RED predicate = FAIL |

## Predicate format

Every acceptance criterion compiles to an executable bash script at `acs/cycle-N/{NNN}-{slug}.sh`:

```bash
#!/usr/bin/env bash
#
# AC-ID:         cycle-N-NNN
# Description:   one-line summary
# Evidence:      pointer (file:line OR commit-SHA OR test-name)
# Author:        agent role (builder/tester/auditor) + persona
# Created:       ISO-8601 timestamp
# Acceptance-of: link back to the build-report.md AC line/token

# Run the actual code path being claimed. NOT grep-on-source.
output=$(bash scripts/my-feature.sh --test-mode)
[ "$output" = "expected" ] && exit 0
exit 1
```

**Exit 0 = GREEN (criterion met). Non-zero = RED (criterion violated; cycle FAILS).**

## Banned patterns inside predicates

Enforced by `scripts/verification/validate-predicate.sh`:

1. `grep -q "..." somefile; exit $?` as the only check — presence ≠ execution
2. `echo "PASS"; exit 0` with no real work — tautology
3. `curl`, `wget`, `gh api/pr/release` — hermetic-determinism requirement
4. `sleep` >= 2s — predicates must be fast
5. Writes to `/etc`, `/var`, `/usr`, `/home`, `/tmp` outside the predicate's own scratch dir
6. Missing metadata headers (`AC-ID`, `Description`, `Evidence`, `Author`, `Created`, `Acceptance-of`)

## Verdict computation

`scripts/lifecycle/run-acs-suite.sh <cycle>` runs:
- All `acs/cycle-N/*.sh` (this cycle's predicates)
- All `acs/regression-suite/cycle-*/*.sh` (every prior cycle's accumulated predicates)

Output: `acs-verdict.json` with schema:

```json
{
  "schema_version": "1.0",
  "cycle": 40,
  "predicate_suite": { "this_cycle_count": 8, "regression_suite_count": 47, "total": 55 },
  "results": [
    { "ac_id": "cycle-40-001", "predicate": "acs/cycle-40/001-foo.sh", "exit_code": 0, "result": "green", "duration_ms": 234, "is_regression": false },
    { "ac_id": "cycle-32-001", "predicate": "acs/regression-suite/cycle-32/001-bar.sh", "exit_code": 1, "result": "red", "duration_ms": 89, "is_regression": true, "evidence_excerpt": "..." }
  ],
  "green_count": 54,
  "red_count": 1,
  "red_ids": ["cycle-32-001"],
  "verdict": "FAIL",
  "ship_eligible": false
}
```

## ship-gate integration

`scripts/lifecycle/ship.sh` (cycle-class commits only) gates on `acs-verdict.json`:

1. If file present AND `red_count > 0` → ship-gate FAILS with `integrity_fail`
2. If file present AND `red_count == 0` → ship proceeds, log `OK: EGPS predicate suite verdict=PASS`
3. If file absent → existing fluent-posture audit-report.md verdict check applies (bootstrap)

`--class manual` and `--class release` bypass the gate (operator overrides).

## Lifecycle

```
Builder phase → writes acs/cycle-N/*.sh predicates alongside the build artifacts
              ↓
Auditor phase → runs validate-predicate.sh on each; writes acs-verdict.json via run-acs-suite.sh
              ↓
Ship phase   → ship-gate enforces acs-verdict.json red_count == 0
              ↓
Post-ship    → promote-acs-to-regression.sh moves acs/cycle-N/ → acs/regression-suite/cycle-N/
              ↓
Next cycle   → must keep all regression-suite predicates GREEN + add its own
```

## Why this subsumes the 5 gaming signals

| Gaming signal | How EGPS structurally eliminates it |
|---|---|
| AC-by-grep | Each AC = executable script; grep-only banned by validate-predicate.sh |
| 0.78–0.87 confidence cliff | No scalar — verdict is exit-code AND across all predicates |
| Recurring same defect | regression-suite/ accumulates every prior AC; recurrence = structural FAIL |
| Tautological-eval irony | validate-predicate.sh banned patterns + mutation-gate FAIL on kill-rate < 0.8 (v10.1+) |
| Carryover work-shifting | Deferred HIGH = open predicate file; ship-gate refuses while it's RED |

## Bootstrap (v10.0.0 first cycle)

Cycles 1–39 do NOT have `acs/cycle-N/*.sh`. The first v10 cycle (40+):
- Starts with empty `acs/regression-suite/`
- Builder writes `acs/cycle-40/*.sh` for new ACs
- Auditor runs `run-acs-suite.sh` → produces verdict
- ship-gate enforces
- Post-ship, predicates promote to `acs/regression-suite/cycle-40/`

Cycles 41+ accumulate the regression suite organically. Backfill of cycles 30–39 is **out of scope** for v10.0.0 (operator value judgment; can be added in v10.1).

## Files

| File | Role |
|---|---|
| `scripts/lib/acs-schema.sh` | Shared constants (banned patterns, exit codes, JSON schema) |
| `scripts/verification/validate-predicate.sh` | Lint predicates for banned patterns |
| `scripts/lifecycle/run-acs-suite.sh` | Run predicates, emit acs-verdict.json |
| `scripts/utility/promote-acs-to-regression.sh` | Post-ship: move predicates to regression-suite/ |
| `scripts/tests/acs-suite-test.sh` | 24 assertions covering schema, validator, runner, promotion |
| `scripts/lifecycle/ship.sh` | Gates on acs-verdict.json (v10.0.0 addition) |

## Out of scope (deferred to follow-on minor releases)

- v10.1: Persona updates (Builder writes predicates, Auditor verifies; Orchestrator runs run-acs-suite)
- v10.1: Backfill predicates for cycles 30–39 (optional)
- v10.2: Mutation-gate promoted from advisory WARN to gating FAIL
- v10.2: Meta ACH 2025 equivalent-mutant detection
- v10.3: Dedicated `evolve-tester` persona separate from Builder

## Research citations (short list)

Full bibliography in `knowledge-base/research/execution-grounded-process-supervision-2026.md`. Most directly load-bearing:

- [Skalse et al. NeurIPS 2022](https://arxiv.org/abs/2209.13085) — the impossibility result for unhackable scalar proxies
- [Lightman et al. OpenAI 2023, PRM800K](https://arxiv.org/abs/2305.20050) — process supervision beats outcome supervision
- [Sherlock 2025](https://arxiv.org/pdf/2511.00330) — removing execution grounding inflates spurious repairs by 131.7%
- [SWE-bench harness](https://github.com/SWE-bench/SWE-bench) — production proof: verdict = `FAIL_TO_PASS ∧ no PASS_TO_PASS regression`
- [Lilian Weng 2024](https://lilianweng.github.io/posts/2024-11-28-reward-hacking/) — surveys 9 point-mitigations, concludes **none works in isolation**
