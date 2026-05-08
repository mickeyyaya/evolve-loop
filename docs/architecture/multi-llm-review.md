> **Version**: v8.51.1 — produced by evolve-loop cycle 2 (extended for v8.54.0 cross-CLI consensus auditor)
> **v8.54.0 Update**: Axis C (cross-CLI consensus) is now operationally available. See [Cross-CLI Consensus Auditor (v8.54.0)](#cross-cli-consensus-auditor-v8540) below.
> **Status**: AUTHORITATIVE — supersedes any prior informal notes on multi-LLM design.
> **Scope**: Per-phase CLI assignment (Axis A). Cross-CLI consensus fan-out (Axis C) is noted but explicitly deferred.

# Multi-LLM Architecture Review

## Table of Contents

1. [Executive Summary](#executive-summary)
2. [Three Axes of Multi-LLM Support](#three-axes-of-multi-llm-support)
3. [Design Strengths (What v8.51.0 Gets Right)](#design-strengths)
4. [Gap Analysis](#gap-analysis)
   - [GAP-1: All profiles hardcoded to `cli: "claude"` (BLOCKING)](#gap-1-all-profiles-hardcoded)
   - [GAP-2: No operator UX for per-phase CLI (HIGH)](#gap-2-no-operator-ux)
   - [GAP-3: Fan-out ledger entries lack `quality_tier` (HIGH)](#gap-3-fanout-ledger-missing-tier)
   - [GAP-4: No cycle-level quality_tier composition (HIGH)](#gap-4-no-cycle-level-composition)
   - [GAP-5: No mixed-CLI automated test coverage (HIGH — CLOSED THIS CYCLE)](#gap-5-no-mixed-cli-test)
   - [GAP-6: Composition logic documented but not shipped (MED — CLOSED THIS CYCLE)](#gap-6-compose-script-missing)
   - [GAP-7: Degraded mode profile permissions are soft (MED)](#gap-7-soft-permissions)
   - [GAP-8: Cross-CLI consensus fan-out unarchitected (LOW / FUTURE)](#gap-8-cross-cli-consensus)
5. [What This Cycle Shipped](#what-this-cycle-shipped)
6. [Deferred to v8.52.0+](#deferred-to-v8520)
7. [Architectural Position (Ultrathink)](#architectural-position-ultrathink)
8. [Manual Profile Configuration Procedure](#manual-profile-configuration)
9. [References](#references)

---

## Executive Summary

> **The multi-LLM swarm design is architecturally sound but operationally unverified and incomplete as of v8.51.0.**

The dispatch plumbing works — `subagent-run.sh` reads `profile.cli` and routes to the correct adapter (`scripts/cli_adapters/{claude,gemini,codex}.sh`). The capability tier model (full/hybrid/degraded/none) is principled and correct. Kernel guarantees (Tier-1 hooks: role-gate, ship-gate, phase-gate-precondition, ledger SHA chain) are CLI-independent and robust.

What was missing:

| Missing piece | Gap ID | Status after this cycle |
|---|---|---|
| Proof that mixed-CLI dispatch actually routes correctly | GAP-5 | **CLOSED** — `multi-cli-cycle-test.sh` provides regression gate |
| Capability composition logic (min-tier across phases) | GAP-6 | **CLOSED** — `_capability-compose.sh` ships |
| All profiles = `cli: "claude"` (no mixed config in production) | GAP-1 | **OPEN** — manual procedure documented below |
| No operator UX for per-phase CLI | GAP-2 | **DEFERRED** to v8.52.0 |
| Fan-out ledger entries lack `quality_tier` | GAP-3 | **OPEN** — schema extension deferred |
| No cycle-level `quality_tier` in cycle-state / orchestrator-report | GAP-4 | **OPEN** — deferred |
| Degraded mode tool permissions are soft | GAP-7 | **ACKNOWLEDGED** — existing mitigations adequate |
| Cross-CLI consensus fan-out (Axis C) | GAP-8 | **DEFERRED** — architecturally interesting but out of scope |

---

## Three Axes of Multi-LLM Support

The phrase "multi-LLM architecture" has three distinct interpretations in the evolve-loop context:

| Axis | Description | Status |
|---|---|---|
| **(A) Per-phase CLI** | Scout=Claude, Builder=Codex, Auditor=Gemini within one cycle | PRIMARY SCOPE — design exists (v8.51.0), now test-verified (this cycle) |
| **(B) Per-phase model tier** | Haiku/Sonnet/Opus per role within one CLI | SHIPPED — v8.35.0 |
| **(C) Cross-CLI consensus fan-out** | Auditor fans out to 3 CLIs and votes; break same-vendor sycophancy | FUTURE — architecturally powerful but undesigned; noted below |

This review focuses on Axis A.

---

## Design Strengths

| Strength | Evidence |
|---|---|
| Dispatch plumbing is correct | `subagent-run.sh:590` reads `profile.cli`; adapter selected correctly for any declared value |
| Capability model is principled | Five dimensions, four tiers, probe-based runtime resolution — sound design |
| Graceful degradation is safe | v7.9.0 Gemini Forgery defenses + Tier-1 kernel hooks make degraded mode structurally safe |
| Adapter contract is uniform | `cross-cli-parity-test.sh` verifies all 3 adapters maintain the same ENV var contract |
| `quality_tier` per `agent_subprocess` is captured | Ledger entries for single dispatch include `quality_tier` (v8.51.0) |
| Tier-1 kernel hooks are CLI-independent | `role-gate`, `ship-gate`, `phase-gate-precondition`, ledger SHA chain operate on bash commands, not on adapter dispatch |

---

## Gap Analysis

### GAP-1: All profiles hardcoded to `cli: "claude"` {#gap-1-all-profiles-hardcoded}

**Severity**: BLOCKING (for any real multi-CLI cycle)

**Evidence**: All 10 profiles in `.evolve/profiles/` declare `"cli": "claude"`. No production cycle has ever run with a non-Claude CLI for any phase.

**Root cause**: The operator-facing UX layer was never shipped (v8.52.0 roadmap). Manually editing `.evolve/profiles/*.json` is the only path, and it is fragile and undiscoverable.

**Impact**: The multi-CLI design has no operational test bed. It is a design intent, not a running configuration.

**Remediation**:
1. Document the manual procedure (this document, §Manual Profile Configuration).
2. `multi-cli-cycle-test.sh` provides the regression gate — future changes to `subagent-run.sh` routing will be caught.
3. Full operator UX: deferred to v8.52.0 (`--phase-cli scout=gemini,builder=claude` flag).

**Note**: `.evolve/profiles/` lives in `PROJECT_ROOT` (writable by operator), not `PLUGIN_ROOT` (read-only install). So manual profile editing is supported even in plugin install mode.

---

### GAP-2: No operator UX for per-phase CLI selection {#gap-2-no-operator-ux}

**Severity**: HIGH

**Evidence**: `platform-compatibility.md:120` documents the v8.52.0 roadmap: "e.g., Scout=Claude (broad codebase scan), Builder=Codex (focused implementation), Auditor=Gemini (independent perspective)." This flag was never shipped.

**Impact**: Operators who want to use the multi-CLI design must hand-edit `.evolve/profiles/`. The UX is fragile: a typo in `"cli"` causes the dispatch to fail at runtime with an "adapter not executable" error.

**Remediation (this cycle)**: Document the manual procedure (§Manual Profile Configuration). Runtime validation in `subagent-run.sh --validate-profile` catches typos before execution.

**v8.52.0 target**: `EVOLVE_PHASE_CLI` env-var map or `--phase-cli` flag that patches profiles at invocation time without permanent file edits.

---

### GAP-3: Fan-out `agent_fanout` ledger entries lack `quality_tier` {#gap-3-fanout-ledger-missing-tier}

**Severity**: HIGH

**Evidence**: Ledger entries with `"kind": "agent_fanout"` (written by `_write_fanout_ledger_entry` in `subagent-run.sh:872`) have no `quality_tier` field. The `agent_subprocess` format (single dispatch) includes it; the `agent_fanout` format does not.

```json
{"kind": "agent_fanout", "worker_count": 3, "workers": [...], "exit_code": 0, ...}
```

**Impact**: In a mixed-CLI fan-out (e.g., scout fan-out workers on different CLIs), there is no way to attribute quality degradation to a specific worker from the ledger alone. Post-hoc forensics require reading per-worker stdout logs.

**Remediation**: Extend `_write_fanout_ledger_entry` in `subagent-run.sh` to:
1. Accept per-worker quality tiers from the `results.tsv` (workers write their tier to a `.quality` sidecar).
2. Compute the composite (lowest) tier using `_capability-compose.sh`.
3. Add `"quality_tier"` to the `agent_fanout` JSON entry.

**Deferred to v8.52.0** — requires schema coordination with ledger verification tooling.

---

### GAP-4: No cycle-level `quality_tier` composition {#gap-4-no-cycle-level-composition}

**Severity**: HIGH

**Evidence**: `platform-compatibility.md:122` states "per-phase capability tiers will compose at the cycle level." `ship.sh`, `cycle-state.sh`, and `orchestrator-report.md` have zero references to `quality_tier`.

**Impact**: A cycle where Scout=full, Builder=hybrid, Auditor=degraded records nothing about its degraded status at the summary level. Operators cannot answer "what was this cycle's quality tier?" without reading individual ledger entries.

**Remediation**:
1. `_capability-compose.sh` now ships (this cycle) — the tool for min-tier aggregation exists.
2. Extend `orchestrator-report.md` generation to call `_capability-compose.sh` on per-phase tiers.
3. Write the result to `cycle-state.json:cycle_quality_tier` via `cycle-state.sh`.

**Deferred to v8.52.0** — orchestrator prompt changes + cycle-state schema extension.

---

### GAP-5: No mixed-CLI automated test coverage {#gap-5-no-mixed-cli-test}

**Severity**: HIGH — **CLOSED THIS CYCLE**

**Evidence (pre-cycle)**: `swarm-architecture-test.sh` (232 lines) had zero mixed-CLI dispatch tests. The multi-CLI dispatch routing was an untested hypothesis.

**Fix shipped**: `scripts/tests/multi-cli-cycle-test.sh`

- 13 assertions covering scout→gemini, builder→claude, auditor→codex routing
- Verifies negative cases (gemini does not leak to claude-adapter)
- Tests DEGRADED mode activation (gemini without claude binary)
- Tests `_capability-compose.sh` correctness (5 assertions)
- No real LLM invocations — uses `VALIDATE_ONLY=1` + `EVOLVE_TESTING=1` test seams
- Bash 3.2 compatible

Run: `bash scripts/tests/multi-cli-cycle-test.sh`
Expected: `13 PASS, 0 FAIL`

---

### GAP-6: Composition logic documented but not shipped {#gap-6-compose-script-missing}

**Severity**: MED — **CLOSED THIS CYCLE**

**Evidence (pre-cycle)**: The "lowest tier wins" rule was stated in `platform-compatibility.md` and implemented WITHIN `_capability-check.sh` for per-adapter dimension aggregation. No `_capability-compose.sh` existed for cross-adapter/cross-phase composition.

**Fix shipped**: `scripts/cli_adapters/_capability-compose.sh`

```bash
bash scripts/cli_adapters/_capability-compose.sh full hybrid degraded  # → degraded
bash scripts/cli_adapters/_capability-compose.sh full hybrid           # → hybrid
bash scripts/cli_adapters/_capability-compose.sh full                  # → full
```

Bash 3.2 compatible. Mirrors the `mode_rank` / `rank_to_mode` pattern already in `_capability-check.sh`.

---

### GAP-7: Degraded mode profile permissions are soft {#gap-7-soft-permissions}

**Severity**: MED

**Evidence**: When `profile_permissions: none` (gemini/codex degraded), the calling LLM session has no `--allowedTools`/`--disallowedTools` enforcement. An auditor in degraded mode could theoretically call the `Edit` tool.

**Existing mitigations**:
- Anti-forgery prompt inoculation in SKILL.md (structural resistance to tool misuse)
- Post-hoc artifact verification (`subagent-run.sh verify_artifact`)
- Tier-1 kernel hooks fire on bash commands regardless of adapter mode
- Gemini Forgery defenses (v7.9.0+): artifact content checks, git diff substance gate, state.json checksum, .sh write protection

**Position**: These mitigations are adequate for the degraded mode use case. No new structural fix is proposed. The tradeoff is explicitly acknowledged: degraded mode trades subprocess isolation for pipeline continuity.

**Operator guidance**: If subprocess isolation is required (production deployments), use `--require-full` or `EVOLVE_GEMINI_REQUIRE_FULL=1` to block degraded-mode cycles. See `docs/architecture/platform-compatibility.md`.

---

### GAP-8: Cross-CLI consensus fan-out unarchitected {#gap-8-cross-cli-consensus}

**Severity**: LOW / FUTURE

**Description** (Axis C): Fan-out the Auditor phase to 3 different CLIs (e.g., Claude Auditor + Gemini Auditor + Codex Auditor), then aggregate verdicts via majority vote or consensus threshold. This would break same-vendor LLM sycophancy — an auditor from the same vendor as the builder may be systematically more lenient.

**Why this matters architecturally**: Axis A (per-phase sequential CLI assignment) gives diverse perspectives across phases but not within a phase. A Builder-Claude + Auditor-Gemini setup is still one-vs-one. Axis C gives N independent auditors from different vendors — much stronger independence signal.

**Why this is deferred**: The fan-out aggregator (`aggregator.sh`) handles `verdict` merge mode (majority vote on PASS/FAIL/WARN strings), but the per-worker quality tier problem (GAP-3) would compound: 3 workers across 3 CLIs, each with different tiers. The design requires GAP-3 to be closed first. It also requires fan-out to be enabled by default (currently `EVOLVE_FANOUT_ENABLED=0`).

**Recommendation**: Dedicate a future cycle to Axis C design after GAP-3 and GAP-4 are closed.

---

## What This Cycle Shipped

| Artifact | Path | Purpose |
|---|---|---|
| Multi-CLI dispatch test | `scripts/tests/multi-cli-cycle-test.sh` | Regression gate for GAP-5: proves routing works |
| Capability composition script | `scripts/cli_adapters/_capability-compose.sh` | Closes GAP-6: min-tier aggregation across phases |
| This architecture review | `docs/architecture/multi-llm-review.md` | Closes GAP-2 partially: manual procedure documented |

Test run evidence:
```
bash scripts/tests/multi-cli-cycle-test.sh
Results: 13 PASS, 0 FAIL
PASS — multi-CLI dispatch routing verified
```

---

## Deferred to v8.52.0+

| Item | Gap | Rationale |
|---|---|---|
| `--phase-cli scout=gemini,builder=claude` operator flag | GAP-2 | Full feature work; requires env-var injection into profile loading |
| Fan-out ledger `quality_tier` field | GAP-3 | Requires sidecar protocol between worker and parent + schema coordination |
| `cycle-state.json:cycle_quality_tier` + orchestrator-report.md integration | GAP-4 | Orchestrator persona change + cycle-state schema extension |
| Cross-CLI consensus Auditor fan-out (Axis C) | GAP-8 | Novel design; depends on GAP-3 being closed first |

Items not deferred — explicitly out of scope:
- New CLI adapter (Copilot, Cursor): not mentioned in user goal
- Relaxing Tier-1 trust kernel: non-negotiable per CLAUDE.md
- Multi-model-tier changes (Haiku/Sonnet/Opus): already settled v8.35.0

---

## Architectural Position (Ultrathink)

**Core question**: Is the v8.51.0 design sound, or does it require redesign?

**Answer: The design is sound. What was missing is verification closure and composition closure.**

The adapter → dispatch → quality_tier pipeline is correctly layered. The kernel guarantees are CLI-independent and robust. The capability tier model (full/hybrid/degraded/none) correctly captures the tradeoff space. The five capability dimensions (subprocess_isolation, budget_cap, sandbox, profile_permissions, challenge_token) and their probe-based resolution are a solid, principled design.

The pre-cycle problem was structural: the design claimed multi-CLI capability but had no executable evidence that it worked. `multi-cli-cycle-test.sh` closes that gap. The result is 13 passing assertions across three CLIs, negative-case verification, and DEGRADED mode behavior validation — run in under 2 seconds with no external dependencies.

**The deeper architectural concern** is about what "multi-LLM independence" actually means. Simply assigning different CLIs to sequential phases (Scout=Gemini, Builder=Claude, Auditor=Codex) creates diversity but not independence — the Builder does not independently evaluate the Scout's findings, and the Auditor operates downstream with full context from both prior phases. True cross-LLM independence requires parallel execution, not sequential execution. This is what Axis C addresses: N Auditors from N vendors independently evaluate the same Build artifact, then a consensus gate aggregates their verdicts.

For Axis A (per-phase sequential CLI assignment), the current design is architecturally complete minus the operator UX layer (v8.52.0). The missing pieces were verification infrastructure and composition utilities — both shipped this cycle.

**The recommendation**: proceed with v8.52.0 operator UX work (GAP-2 UX flag, GAP-3 ledger schema, GAP-4 cycle-state field) on the basis of this verified architecture. Plan a dedicated cycle for Axis C after GAP-3 is closed.

---

## Manual Profile Configuration Procedure

Until v8.52.0 ships the `--phase-cli` flag, operators can configure per-phase CLI assignment by editing `.evolve/profiles/*.json` in the project root.

**Step 1**: Identify which phases to assign to which CLI.

```
Scout  = Gemini  (broad codebase scan; independent perspective from Claude)
Builder = Claude  (full caps: subprocess isolation, budget cap, sandbox)
Auditor = Gemini  (independent audit perspective; hybrid mode if claude on PATH)
```

**Step 2**: Edit the relevant profiles in `<project-root>/.evolve/profiles/`.

```bash
# Example: assign scout to gemini
jq '.cli = "gemini"' .evolve/profiles/scout.json > .evolve/profiles/scout.json.tmp
mv .evolve/profiles/scout.json.tmp .evolve/profiles/scout.json
```

**Step 3**: Validate the profile resolves to the expected adapter.

```bash
bash scripts/dispatch/subagent-run.sh --validate-profile scout
# Expected stderr: [gemini-adapter] ...
```

**Step 4**: Optionally probe the capability tier.

```bash
bash scripts/cli_adapters/_capability-check.sh gemini --human
# Adapter: gemini
# Quality tier: hybrid  (if claude on PATH)
#            or degraded (if no claude)
```

**Step 5**: For a multi-CLI cycle, compose the expected cycle quality tier.

```bash
bash scripts/cli_adapters/_capability-compose.sh hybrid full hybrid
# Output: hybrid
```

> **HYBRID Mode Warning**: Setting `"cli": "gemini"` (or `"cli": "codex"`) in a profile when `claude` is on PATH does NOT invoke Gemini's intelligence — `gemini.sh` delegates to `claude.sh`, which runs `claude -p` under the hood. You are running Claude, not Gemini. The "gemini" tier name refers to the adapter shim, not to the Gemini LLM. To get actual Gemini intelligence in audit cycles, use API-based adapters (v8.52.0+ roadmap); they are not yet available in this release.

**Caveats**:
- `.evolve/profiles/` is writable even in plugin-install mode (it lives in PROJECT_ROOT, not PLUGIN_ROOT).
- Profile changes persist across cycles; restore to `"cli": "claude"` after testing unless you intend a permanent change.
- The gemini/codex adapters in DEGRADED mode (no claude binary on PATH) do not make LLM calls themselves — the calling LLM writes the artifact directly. Only use DEGRADED mode when you understand the reduced isolation implications.

---

## Cross-CLI Consensus Auditor (v8.54.0) {#cross-cli-consensus-auditor-v8540}

> **Status**: SHIPPED — operator-facing opt-in via `--consensus-audit` flag or `EVOLVE_CONSENSUS_AUDIT=1` env var.

v8.54.0 wires the v8.53.0 `cross-cli-vote` merge mode into the Auditor phase as opt-in cross-CLI consensus. This is the architectural answer to **same-vendor sycophancy** — the failure mode where an Auditor running on the same LLM family as the Builder agrees uncritically because they share training data biases and reasoning conventions.

### When to use

- High-risk releases where a false-positive PASS verdict would be expensive to roll back
- Cycles touching authentication, payments, kernel hooks, or other security-sensitive surfaces
- Releases that deviate significantly from prior shipped patterns (where the Auditor's reasonable-baseline assumption is least valid)
- Forensic cycles after a near-miss incident (cycle 102-111 reward hacking, Gemini Forgery 2026-03-23)

NOT recommended for routine cycles — the cost is roughly N× the standard audit budget (N = number of voters), and the marginal value over a single Adversarial Auditor (default-on, Opus auditing Sonnet's builds) is small for low-risk diffs.

### Consensus protocol: MAJORITY-PASS with FAIL-VETO

The aggregator's `cross-cli-vote` merge mode applies these rules:

| Vote pattern | Consensus | rc | Rationale |
|---|---|---|---|
| All N return PASS | PASS | 0 | Unanimous; ship |
| ≥ ceil(N/2) PASS, no FAIL | PASS | 0 | Quorum met without veto |
| Any FAIL | **FAIL** | **1** | Veto rule: any one CLI smelled blood |
| < ceil(N/2) PASS, no FAIL | WARN | 0 | Below quorum; ships per fluent default unless `EVOLVE_STRICT_AUDIT=1` |

The veto rule is the central insight: a single dissenting CLI is more informative than M agreeing CLIs, because agreement could be sycophancy. The protocol biases toward false negatives (block ship when uncertain) over false positives (ship when divergence exists).

### Profile schema

The Auditor profile (`.evolve/profiles/auditor.json`) declares its consensus configuration:

```jsonc
{
  "name": "auditor",
  "cli": "claude",                       // default for non-consensus mode
  "model_tier_default": "opus",
  "consensus": {
    "enabled": false,                    // opt-in default; set via env or flag
    "cli_voters": ["claude", "gemini", "codex"],
    "quorum": 2,                         // ceil(N/2) by default
    "require_min_tier": "hybrid"         // exclude voters below this capability tier
  },
  "parallel_subtasks": [...]             // semantic dimensions, unchanged
}
```

`require_min_tier` filters voters by their adapter capability tier (full/hybrid/degraded/none from v8.51.0 capability framework). A degraded-mode adapter cannot reliably participate in consensus because its trust kernel is reduced; the default `hybrid` minimum keeps consensus voters at production-grade isolation.

### Operator UX

Three opt-in surfaces:

```bash
# Flag form (preferred — explicit per-invocation)
/evolve-loop --consensus-audit 1 polish "audit hardening"

# Env var form (CI-friendly, persistent across multiple dispatches)
EVOLVE_CONSENSUS_AUDIT=1 bash scripts/dispatch/evolve-loop-dispatch.sh 1 polish "..."

# Profile-side persistence (always-on for this auditor profile)
jq '.consensus.enabled = true' .evolve/profiles/auditor.json > .evolve/profiles/auditor.json.tmp     && mv .evolve/profiles/auditor.json.tmp .evolve/profiles/auditor.json
```

When enabled:
- Orchestrator dispatches the audit phase via `scripts/dispatch/consensus-dispatch.sh` instead of `subagent-run.sh dispatch-parallel auditor`
- Each `cli_voters` entry runs the full audit prompt under that CLI (parallel via `fanout-dispatch.sh`)
- Per-voter artifacts at `.evolve/runs/cycle-N/consensus-workers/<cli>-audit.md`
- Aggregated verdict at `.evolve/runs/cycle-N/audit-report.md` (replaces what standard auditor would produce)
- Ledger entry kind: `agent_fanout` with workers list and consensus quality_tier

### Cost note

Consensus audit costs ≈ N × standard audit. With cli_voters=[claude, gemini, codex] and the Auditor's default Opus tier (~$1.10/audit), a single consensus audit is ~$3.30. Operators should not enable `consensus.enabled: true` profile-side without explicit budgetary intent.

The `quality_tier` per-CLI annotation in fan-out ledger entries (v8.52.0) makes per-vendor cost auditable post-hoc.

### Defense-in-depth linkage

This pattern responds to two prior incidents documented elsewhere:

- **Gemini Forgery (2026-03-23)** — same-vendor reasoning produced internally-consistent but fabricated artifacts. A cross-CLI Auditor would have surfaced the discrepancy (different vendors trained on different corpora produce divergent outputs on novel content; agreement signals truth more than within-vendor agreement does).
- **Cycle 102-111 reward hacking** — the Auditor and Builder were both Sonnet, so the Auditor accepted tautological evals the Builder authored. A cross-CLI consensus would have given non-Sonnet voters a chance to spot the tautology.

Neither of these incidents is fully solved by consensus auditing alone — kernel hooks, mutation testing, and audit-binding remain the primary defenses — but consensus closes the residual same-vendor-sycophancy gap.

### Roadmap

- v8.54.x — orchestrator integration: when `EVOLVE_CONSENSUS_AUDIT=1`, the orchestrator subagent prompt routes to consensus-dispatch.sh automatically
- v8.55.0 — cost surfacing: per-CLI cost breakdown in orchestrator-report.md
- v8.56.0 — `consensus.required` field at the cycle level (force consensus for cycles touching specific paths, e.g., kernel hooks)

---

## References

| Source | Relevant section |
|---|---|
| `docs/architecture/platform-compatibility.md` | Capability model definition; per-CLI tier table; v8.52.0 roadmap |
| `scripts/dispatch/subagent-run.sh:590` | Profile.cli authoritative dispatch |
| `scripts/cli_adapters/_capability-check.sh` | Per-adapter capability resolution; quality_tier aggregation |
| `scripts/cli_adapters/_capability-compose.sh` | Cross-phase tier composition (new this cycle) |
| `scripts/tests/multi-cli-cycle-test.sh` | Mixed-CLI dispatch regression gate (new this cycle) |
| `scripts/dispatch/subagent-run.sh:872` | `_write_fanout_ledger_entry` (lacks quality_tier — GAP-3) |
| `scripts/tests/swarm-architecture-test.sh` | Tri-layer wiring tests (does not cover mixed-CLI dispatch) |
| `docs/incidents/gemini-forgery.md` | Why Tier-1 defenses operate at pipeline layer, not adapter layer |
| `docs/architecture/tri-layer.md` | Orchestration pattern taxonomy (Patterns 1-5) |
| `.evolve/profiles/*.json` | All profiles — confirmed 100% `"cli": "claude"` pre-cycle |
