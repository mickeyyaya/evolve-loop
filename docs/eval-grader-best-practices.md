> **Audience**: Builder agents writing eval graders for `.evolve/evals/*.md`
> **Status**: Normative — enforced at `gate_discover_to_build` and `gate_build_to_audit` (WARN below 0.7 kill rate)

# Eval Grader Best Practices

## Table of Contents

1. [Why Tautological Graders Break Everything](#1-why-tautological-graders-break-everything)
2. [Grader Level Taxonomy](#2-grader-level-taxonomy)
3. [Level 3.5 — Control-Flow Structural (Recommended)](#3-level-35--control-flow-structural-recommended)
4. [Level 4 — End-to-End Behavioral](#4-level-4--end-to-end-behavioral)
5. [Anti-Patterns to Avoid](#5-anti-patterns-to-avoid)
6. [Mutation Resistance Self-Test](#6-mutation-resistance-self-test)
7. [Quick Reference: Pattern Templates](#7-quick-reference-pattern-templates)

---

## 1. Why Tautological Graders Break Everything

A grader that passes after the source-under-test is mutated does not test behavior — it tests for string presence. This is structurally equivalent to writing `assert True` in a unit test.

Consequence: the Auditor receives a PASS verdict from the eval runner, ships the cycle, and the mutation survives in production. The bug is invisible until it causes an incident.

Evolve-loop cycles 102–111 (reward-hacking class) were all rooted in this failure mode. `mutate-eval.sh` was introduced specifically to catch it. Its threshold (≥0.7 kill rate at the gate, ≥0.8 canonical target) is the project's main defense.

---

## 2. Grader Level Taxonomy

| Level | Name | Example | Kill Rate | Use When |
|-------|------|---------|-----------|---------|
| 0 | No-op | `echo ok` | 0% | Never — blocked by `eval-quality-check.sh` |
| 1 | Source-presence | `grep -q "FLAG" script.sh` | 10–30% | Only for existence checks (file must exist) |
| 2 | Output-file check | `grep -q "result" output.log` | 40–60% | When you have a real output artifact |
| 3 | Execution-based | `jq -e '.field == true' file.json` | 60–80% | Multi-field structural validation |
| **3.5** | **Control-flow structural** | `awk '/FLAG.*==.*1/{p=1} p && /dispatch/{f=1} END{exit(!f)}' script.sh` | **65–80%** | **Default target for evolve-loop evals** |
| 4 | End-to-end behavioral | Phase-gate invocation with minimal workspace | 80–90% | Critical wiring checks only |

**Target**: Level 3.5 or higher for every grader. Level 1 (`grep -q`) is acceptable only for file-existence checks where the whole assertion is "does this file exist."

---

## 3. Level 3.5 — Control-Flow Structural (Recommended)

Level 3.5 uses `awk` to verify that a flag **guards a non-trivial body** containing the correct dispatch call. This survives the four mutation classes that kill Level 1:

| Mutation class | Level 1 survives? | Level 3.5 survives? |
|---------------|-------------------|---------------------|
| Variable rename (partial match) | Yes — substring still present | No — awk match fails |
| Comment-out the guarded body | Yes — flag string still in file | No — body no longer follows flag |
| Flag name typo (`FLAGZ`) | Yes — original string still present | No — awk pattern does not match `FLAGZ` |
| Body removal (empty guard) | Yes | No — `found` never set |

### Template: flag guards dispatch call

```bash
awk '/ENV_FLAG.*==.*1/{p=1} p && /dispatch-call/{found=1} END{exit(!found)}' path/to/script.sh
```

Replace `ENV_FLAG` with the exact env var name and `dispatch-call` with the function or binary name that should appear in the guarded body.

### Template: flag in correct section of a markdown file

```bash
awk '/^#/{section=$0} /KEY_PHRASE/{if(section ~ /Section Name/){found=1}} END{exit(!found)}' path/to/file.md
```

This ensures the phrase appears in the correct section heading, not anywhere in the file.

### Template: multi-field structural validation with jq

```bash
jq -e 'has("field1") and has("field2") and (.model_tier_default | test("^(sonnet|haiku|opus)$"))' file.json
```

This is Level 3 but higher-signal than a single-field `jq -e '.field == value'` because removing any required field kills it.

---

## 4. Level 4 — End-to-End Behavioral

Level 4 invokes the actual kernel script being tested with a minimal contrived state and asserts on exit code or stdout pattern. Reserve for critical wiring checks where the cost is justified.

### Example: phase-gate invocable (dry-run path)

```bash
phase-gate.sh --version 2>/dev/null || grep -q "phase-gate" scripts/lifecycle/phase-gate.sh
```

For a real end-to-end check, construct a minimal `$WORKSPACE` with a synthetic artifact and invoke the gate function. This requires a test fixture and is expensive — use only when Level 3.5 is insufficient.

---

## 5. Anti-Patterns to Avoid

### Anti-pattern 1: Absolute paths

```bash
# BAD — non-portable, fails in worktrees and CI
grep -q "FLAG" /Users/alice/ai/claude/evolve-loop/scripts/lifecycle/phase-gate.sh

# GOOD — relative, works everywhere
grep -q "FLAG" scripts/lifecycle/phase-gate.sh
```

### Anti-pattern 2: Pure source-presence for behavioral claims

```bash
# BAD — this passes even if the flag exists but the dispatch body is commented out
grep -q "EVOLVE_AUDIT_ADVISORY_REVIEW" scripts/lifecycle/phase-gate.sh

# GOOD — verifies the flag guards the actual dispatch call
awk '/EVOLVE_AUDIT_ADVISORY_REVIEW.*=.*"1"/{p=1} p && /audit-advisory-review.sh/{found=1} END{exit(!found)}' scripts/lifecycle/phase-gate.sh
```

### Anti-pattern 3: Fixture-based acceptance checks that don't match real artifact format

```bash
# BAD — synthetic fixture uses inline format that real artifacts don't use
echo "Verdict: SHIPPED" | grep -qiE 'Verdict[[:space:]]*:[[:space:]]*(SHIPPED)'

# GOOD — use a real historical artifact to verify extraction logic
awk '/^##[[:space:]]+Verdict/{f=1;next} f && NF{print tolower($0);exit}' \
    .evolve/runs/cycle-17/orchestrator-report.md | grep -qiE 'shipped'
```

---

## 6. Mutation Resistance Self-Test

Before submitting an eval, run `mutate-eval.sh` against it:

```bash
bash scripts/verification/mutate-eval.sh .evolve/evals/your-eval.md --threshold 0.7
```

Exit 0 = kill rate ≥ 0.7 (gate passes). Exit 1 = tautological (gate warns). Exit 2 = anomaly.

The gate at `gate_build_to_audit` runs this automatically on every new eval file created in the build phase. A kill rate below 0.7 emits WARN to the Auditor; below 0.7 after the WARN-only rollout period, the gate will escalate to FAIL.

---

## 7. Quick Reference: Pattern Templates

```bash
# File existence (Level 1 — acceptable only for "file must exist" assertions)
test -f path/to/file.ext

# Multi-field JSON structural validation (Level 3)
jq -e 'has("f1") and has("f2") and (.tier | test("^(a|b|c)$"))' file.json

# Boolean invariants (Level 3)
jq -e '.enabled == true and .read_only == true' file.json

# Flag guards dispatch body (Level 3.5)
awk '/ENV_FLAG.*==.*1/{p=1} p && /dispatch/{found=1} END{exit(!found)}' script.sh

# Keyword in correct section of markdown (Level 3.5)
awk '/^#/{section=$0} /PHRASE/{if(section ~ /Target Section/){found=1}} END{exit(!found)}' file.md

# Named routing in case statement (Level 3.5)
awk '/role-name\)/{p=1} p && /\.json/{found=1} END{exit(!found)}' dispatch.sh

# Count-based (better than pure grep-q — requires non-zero occurrences)
grep -c "KEYWORD" script.sh | awk '{exit($1<1)}'
```
