# ACS Predicate Quality Gate — Four-Layer Defense

> Design document for the predicate-quality four-layer defense, activated in cycle-86.
> See [CHANGELOG.md](../../CHANGELOG.md) for version history.

## Problem: Tautological Predicates

ACS predicates (`acs/cycle-N/*.sh`) can be trivially-tautological. A grep-only predicate like:

```bash
grep -q "Predicate quality review" agents/evolve-auditor.md ; exit $?
```

always passes if the string is present, regardless of whether the implementation is correct. The predicate tests string presence, not behavioral correctness. Mutation testing cannot kill it — any mutation that preserves the string passes regardless of actual system behavior.

## Four-Layer Defense (activated cycle-86)

### Layer 1 — Author Separation (cycle-85)

| Role | Responsibility |
|---|---|
| TDD-Engineer | Writes behavioral predicates BEFORE Builder implements |
| Builder | Implements to pass predicates — cannot author predicates |

**Mechanism:** `EVOLVE_TEST_PHASE_ENABLED=1` (default-on) forces TDD-Engineer phase between Triage and Build. Builder profile denies predicate authorship.

**Reference:** `.evolve/profiles/tdd-engineer.json`, `.evolve/profiles/builder.json`

### Layer 2 — Static Linter (`predicate-quality-c2-linter`)

**Script:** `legacy/scripts/verification/lint-acs-predicates.sh`

**Classification rules:**

| Classification | Criteria |
|---|---|
| `BEHAVIORAL` | Uses subprocess invocations (`$(...)`, backtick) or arithmetic/jq/awk/wc |
| `GREP_ONLY` | `grep -q` calls with no subprocess invocations (count > 0, subprocess_count == 0) |

**Gate hook:** `gate_build_to_audit` in `legacy/scripts/lifecycle/phase-gate.sh` runs the linter on every predicate in `acs/cycle-N/`. Any GREP_ONLY predicate **blocks the gate** (exit 1). Opt-out not available — this is a hard gate.

**Usage:**
```bash
# Classify predicates with explanation
bash legacy/scripts/verification/lint-acs-predicates.sh --predicates-dir acs/cycle-N --explain

# Exit codes: 0 = all behavioral, 1 = grep-only detected
```

**Test suite:** `tests/verification/test-lint-acs-predicates.sh` — 7 FAIL fixtures (grep-only patterns), 2 PASS fixtures (behavioral). Run: `bash tests/verification/test-lint-acs-predicates.sh`.

### Layer 3 — Auditor Review (`predicate-quality-c3-auditor-review`)

**Agent:** `agents/evolve-auditor.md` — **Predicate quality review** section

The Auditor classifies every predicate as `behavioral` / `grep-only` / `mixed` and:
- Raises **CRITICAL** defect for each un-waived grep-only predicate
- Raises **HIGH** warning for each mixed predicate (needs human verification)

**acs-verdict.json schema extension:**

```json
{
  "verdict": "PASS",
  "red_count": 0,
  "predicate_quality": {
    "per_predicate": [
      {
        "path": "acs/cycle-86/pred-lint-acs-exists.sh",
        "classification": "behavioral",
        "has_subprocess_invocation": true,
        "waived": false
      }
    ],
    "summary": {
      "behavioral_count": 5,
      "grep_only_count": 0,
      "mixed_count": 0,
      "blocking_count": 0
    }
  }
}
```

`blocking_count > 0` forces `verdict = "FAIL"` regardless of predicate exit codes.

**Inspect post-cycle:**
```bash
jq '.predicate_quality.summary' .evolve/runs/cycle-N/acs-verdict.json
```

### Layer 4 — Activation and Promotion (`predicate-quality-c4-activate-and-promote`)

| Change | Before (cycle-85) | After (cycle-86) |
|---|---|---|
| `EVOLVE_TEST_PHASE_ENABLED` | `0` (opt-in) | `1` (default-on) |
| Mutation gate `gate_discover_to_build` | WARN-only at kill_rate < 0.7 | **FAIL-gate** at kill_rate < 0.7 |
| Orchestrator phase flow | Scout → Triage → Builder | Scout → Triage → **TDD-Engineer** → Builder |
| Lint gate `gate_build_to_audit` | Not present | **FAIL-gate** on grep-only predicates |

**Mutation gate opt-out:** Set `EVOLVE_MUTATION_GATE_STRICT=0` to revert to WARN-only. Not recommended for production.

## Lifecycle

```
TDD-Engineer (writes behavioral predicates)
    ↓
Builder (implements; cannot author predicates)
    ↓
gate_build_to_audit: lint-acs-predicates.sh FAIL if grep-only
    ↓
Auditor: classifies, emits predicate_quality block, CRITICAL on grep-only
    ↓
acs-verdict.json: blocking_count > 0 → FAIL
    ↓
gate_discover_to_build (next cycle): mutation gate FAIL at kill_rate < 0.7
```

## References

- `legacy/scripts/verification/lint-acs-predicates.sh` — Layer 2 linter
- `tests/verification/test-lint-acs-predicates.sh` — Layer 2 test suite
- `legacy/scripts/verification/mutate-eval.sh` — mutation testing (grep_only_check pre-flight)
- `legacy/scripts/lifecycle/phase-gate.sh` — gate_build_to_audit, gate_discover_to_build
- `agents/evolve-auditor.md` — Predicate quality review section (Layer 3)
- `agents/evolve-orchestrator.md` — EGPS Tester Phase section (Layer 4)
- `.evolve/profiles/tdd-engineer.json` — Layer 1 author separation
- `.evolve/profiles/builder.json` — Builder predicate-authorship denial
