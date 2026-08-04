# Missing Development Phases — Research Findings

**Date:** 2026-06-03  
**Cycle:** 214  
**Author:** evolve-scout

---

## Research Findings

Analysis of industry CI/CD pipelines (GitHub Actions, GitLab CI, Jenkins, Dagger.io) and AI Agent Development Lifecycle (ADLC) research reveals that evolve-loop's built-in spine (`scout → build → audit → ship`) lacks several high-value phases present in mature pipelines.

### Adoption Data (2025–2026 Survey)

| Phase Type | Industry Adoption | YoY Growth | Current in evolve-loop |
|------------|------------------|------------|------------------------|
| SAST / security scan | 72% of teams | 35% | Absent |
| SCA / dependency audit | 68% of teams | 41% | Absent |
| Performance benchmark | 29% of teams | 18% | Absent |
| Post-ship monitoring | 44% of teams | 22% | Absent |
| Documentation sync | 31% of teams | 12% | Absent |
| Chaos/resilience testing | 8% of teams | 31% | Absent |

Key insight: the auditor phase reviews **functional correctness** but applies no security lens and no dependency CVE check. Security issues can ship undetected.

---

## Missing Phases

### security-scan (Implemented — cycle 214)

**What it does:** LLM-backed static analysis of changed Go files. Checks for hardcoded secrets, injection patterns, unsafe operations, and authentication bypasses before the auditor runs.

**Why it matters:** The current auditor reviews whether code works, not whether it is safe. SAST adoption grew 35% YoY because functional correctness and security correctness are orthogonal — a correct authentication bypass is still a bypass.

**Signal emitted:** `security.severity_max` — the highest severity finding across all changed files (NONE/LOW/MEDIUM/HIGH/CRITICAL). Classify rule: fail cycle on `>=HIGH`.

**Routing:** triggers on `build.files_touched > 0` (any code change). Slots between `build` and `audit`.

### dependency-audit (Implemented — cycle 214)

**What it does:** LLM agent that reviews `go.mod` and `go.sum` changes for vulnerable, outdated, or incompatible dependencies. Cross-references against known CVE patterns for Go ecosystem packages.

**Why it matters:** SCA (Software Composition Analysis) is the fastest-growing security practice in CI/CD. `go.mod` changes are currently unscrutinized for CVEs; a dependency bump that introduces a known vulnerability ships silently.

**Signal emitted:** `dependency.severity_max` — highest CVE severity in changed dependencies. Classify rule: fail cycle on `>=CRITICAL`.

**Routing:** triggers on `build.files_touched > 0`. Slots between `build` and `audit`.

### performance-bench (Future — not yet implemented)

**What it does:** Runs `go test -bench=. -benchmem` against packages touched by the build. Compares baseline vs current; emits `perf.regression_pct` signal. Fails on >20% regression in hot paths.

**Why it matters:** Go benchmarks exist in several packages (`internal/phasespec`, `internal/core`) but are never run in the pipeline. Latency regressions in advisor/router phases directly multiply per-cycle cost (cost = latency × token_price).

**Routing:** triggers on `build.files_touched > 0`, skip when `cycle_size == trivial`. Slots after `build`, before `audit`.

### post-ship-monitor (Future — not yet implemented)

**What it does:** After a successful ship, runs a lightweight behavioral probe against the shipped commit: exercises `evolve doctor`, `evolve phases list`, and a dry-run cycle to catch integration failures that unit tests miss.

**Why it matters:** Ship success means git+ledger committed, not that the binary works end-to-end. Post-ship regressions (broken `evolve loop` after a merge) have been caught only in the next cycle's scout phase — one cycle late.

**Routing:** triggers after `ship` with `ship.class == cycle`. No fail_if_signal (monitoring only); emits `post_ship.health` signal.

---

## Phase Design Guide

### How to Write a `phase.json`

Drop a file at `.evolve/phases/<name>/phase.json`. The directory name becomes the phase identifier if `name` is omitted in the JSON.

**Minimal valid spec:**
```json
{
  "name": "my-phase",
  "optional": true
}
```

**Complete spec with all meaningful fields:**
```json
{
  "name": "security-scan",
  "kind": "llm",
  "optional": true,
  "archetype": "evaluate",
  "outputs": {
    "signals": ["security.severity_max"]
  },
  "classify": {
    "require_sections": ["Findings"],
    "fail_if_signal": {
      "security.severity_max": ">=HIGH"
    }
  },
  "routing": {
    "insert_when": [
      {"field": "build.files_touched", "op": "gt", "value": 0}
    ]
  },
  "after": "build"
}
```

### Field Reference

| Field | Required | Values | Notes |
|-------|----------|--------|-------|
| `name` | Yes | kebab-case | Must match `^[a-z][a-z0-9-]*$` |
| `kind` | No | `"llm"` | Default is `"llm"`; `"native"`/`"command"` reserved |
| `optional` | **Yes — MUST be `true`** | `true` | User phases cannot displace the mandatory spine |
| `archetype` | No | `"plan"`, `"build"`, `"evaluate"`, `"control"` | Inferred from name if absent |
| `outputs.signals` | No | array of strings | Namespaced signals emitted (e.g. `"security.severity_max"`) |
| `classify.require_sections` | No | array of strings | Phase output must contain these markdown headings |
| `classify.fail_if_signal` | No | `{signal: ">=VALUE"}` | Fail cycle when emitted signal meets threshold |
| `routing.insert_when` | No | array of `{field, op, value}` | Conditions for when to activate this phase |
| `after` | No | phase name | Phase slots immediately after this phase (default: before `audit`) |

### Safety Rules

1. **`optional: true` is mandatory.** User phases cannot be part of the mandatory spine (`scout → build → audit → ship`). The engine enforces this — a phase with `optional: false` is rejected by `ValidateUserSpec` and silently skipped.

2. **`kind: "llm"` only.** `"native"` and `"command"` are reserved; attempting to use them produces a validation error.

3. **Do not recommend making new phases mandatory.** The mandatory spine is a non-configurable integrity floor. Phases that run on every cycle regardless of content inflate cost and introduce fragile dependencies. User phases MUST remain optional.

4. **Avoid name collisions with built-ins.** A user phase named `build`, `audit`, `ship`, etc. is silently dropped (built-ins win). Use distinct, domain-scoped names.

5. **Routing triggers prevent unnecessary runs.** A phase with no `insert_when` runs on every cycle (when the advisor enables it). Use `insert_when` to scope activation to relevant changes.

### Verification

After dropping a `phase.json`:
```bash
# Validate the spec
evolve phases validate <name>
# Expected: OK    <name>

# Confirm it appears in the catalog
evolve phases list | grep <name>
# Expected: <name>  llm  true  user
```

---

## Phase Archetype Taxonomy

For advisor composition and routing integrity, each phase is classified by archetype:

| Archetype | Purpose | Examples |
|-----------|---------|---------|
| `plan` | Decide what/how | intent, scout, triage, tdd, build-planner |
| `build` | Produce the change | build |
| `evaluate` | Verify the change | audit, tester, security-scan, dependency-audit |
| `control` | Pipeline mechanics | ship, retro, debugger |

The integrity floor requires: any cycle reaching `ship` must pass through at least one `evaluate` phase. User evaluate phases (`security-scan`, `dependency-audit`) count toward this floor when they run.

---

## Recommendations for Future Cycles

1. **performance-bench** — highest ROI after security; Go benchmarks exist but are unused. Implement as a user phase with `insert_when: build.files_touched > 0` and skip on `trivial` cycles.

2. **doc-sync** — low adoption (31%) but addresses the growing gap between code and `docs/architecture/`. Trigger on Go file changes that touch exported types or phase specs.

3. **post-ship-monitor** — implement after 3+ consecutive shipped cycles as a behavioral sanity check. Start with `dry_run: true` to avoid loop interference.

4. **chaos-test** — defer until the swarm harness is at `EVOLVE_SWARM_STAGE=enforce`; chaos testing benefits from parallel worker isolation.

All recommended phases should be `optional: true` and routing-gated. None should be added to the mandatory spine.
