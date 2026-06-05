# Micro-Phase Catalog — Atomic, Advisor-Composable Development Phases

**Date:** 2026-06-05
**Status:** Approved design; implementation queued as evolve-loop cycle goals (3 waves)
**Research basis:** [knowledge-base/research/micro-phase-catalog-research-2026-06-05.md](../../knowledge-base/research/micro-phase-catalog-research-2026-06-05.md) (2-round web research: 106-agent adversarially-verified deep-research + 3 targeted gap-fill agents)
**Builds on:** ADR-0024 (phase advisor), ADR-0028 (user-defined phases), ADR-0035 (spec-derived contracts), [user-defined-phases.md](user-defined-phases.md), [missing-development-phases-2026-06-03](../../knowledge-base/research/missing-development-phases-2026-06-03.md)

---

## 1. Problem

The advisor composes each cycle from a catalog of ~15 built-ins + 4 user phases. The catalog has near-zero coverage for bugfix-, refactor-, security-depth-, and release-specific responsibilities, and several built-ins are monolithic. Result: every goal type gets roughly the same pipeline shape. The research shows the leading systems (Agentless, AlphaCodium, AgentCoder, Aider) win precisely by decomposing into single-responsibility phases — and that the *phases* are stable across systems while the *control flow between them* is the configurable dimension. That control-flow dimension is exactly what the advisor owns.

**Decision (user-confirmed): additive-first.** All new phases are `optional: true` user phases (`.evolve/phases/<name>/`, zero Go, ADR-0035 path). The spine (`scout → build → audit → ship`) and integrity floor are untouched. Spine decomposition is documented in §6 as a future ADR candidate only.

## 2. Design principles

1. **One responsibility per phase.** ≤2 input artifacts, 1 report artifact + namespaced signals, ONE classify rule. If the phase description needs "and", split it.
2. **LLM judges, tooling gates** (Core Rule 5; EGPS alignment). Detection/enumeration phases are LLM judgment. Gate phases wrap deterministic tooling (benchstat, mutation runner, deadcode, test runner) executed by the LLM agent via the bridge, emitting hard signals. All phases are `kind: llm` (native/command reserved) — deterministic gates are LLM-orchestrated tool runs, the proven `perf-profile` pattern.
3. **Routing triggers scope cost.** Every phase has `insert_when`; heavy phases are diff-scoped (Google incremental-mutation lesson); conditional phases gate on preconditions (SBFL ⇒ tests exist).
4. **Generated tests are signals, not oracles.** `test-amplification` / `reproduce-bug` outputs feed the tdd/audit gates; they never carry independent ship authority (LLM test suites are documented-unreliable).
5. **Anti-bias isolation.** A test-designing agent must not see the implementation (AgentCoder); preserves the existing builder≠auditor cross-family floor philosophy.
6. **Advisor composes, kernel clamps.** Goal-type recipes live in the router persona as guidance; `ClampPlanToFloor` remains the non-bypassable safety net.

## 3. The catalog — 17 phases in 3 waves

Sketches use v4 phase-descriptor vocabulary; transcribe to `.evolve/phases/<name>/phase.json` + `agent.md` at implementation time and validate with `evolve phases validate <name>`.

### Naming rule (added 2026-06-05)

- **Phase names are `<object>-<action>`**: the thing examined, then the operation on it (`smell-scan`, `mutation-gate`, `dependency-audit`). A name must answer "what does this phase look at, and what does it do about it" without reading the spec. Grandfathered outlier: `reproduce-bug` (shipped action-object; do not rename shipped phases for cosmetics).
- **Every phase declares a one-line CORE VALUE** — the single risk it removes — maintained in the router persona's "Phase Catalog — Core Values" table (`agents/evolve-router.md`), which is what the advisor justifies selections against. When the `user-phase-persona-resolution` fix lands, this moves into a `description` field in `phase.json` (schema + `phasespec.PhaseSpec` + catalog-card surfacing) so the value line is machine-carried, not persona-maintained.
- **Work-item / batch names follow the same rule**: deliverable-describing, not sequence-describing (`phases-quality-gates`, not `micro-phase-wave-2`).

### Wave 1 — goal-defining phases (`phases-goal-defining`) — ✅ SHIPPED cycle 217 (`a354d85`)

#### `fault-localization` (plan)
- **Responsibility:** Hierarchical narrowing for bugfix cycles: repo-tree → suspicious files → declaration skeleton → element → edit locations. No fixing.
- **Inputs:** scout-report.md (issue description). **Outputs:** `fault-localization-report.md`; signals `fault.locations_count`, `fault.confidence`.
- **Routing:** `insert_when: [{field: "scout.goal_type", op: "==", value: "bugfix"}]`, `after: "triage"`.
- **Classify:** require sections `["Suspect Ranking", "Edit Locations"]`; fail_if_empty.
- **Evidence:** Agentless (arXiv 2407.01489) — 32% SWE-bench Lite @ $0.70/issue with this exact phase first.

#### `reproduce-bug` (evaluate)
- **Responsibility:** Produce a FAIL_TO_PASS reproduction test/script that demonstrably fails on the current tree, BEFORE any patch.
- **Inputs:** scout-report.md, fault-localization-report.md. **Outputs:** `reproduce-bug-report.md` + repro test file path; signals `repro.failing` (must be true), `repro.test_path`.
- **Routing:** `insert_when: [{field: "scout.goal_type", op: "==", value: "bugfix"}]`, `after: "fault-localization"` (falls back to after triage).
- **Classify:** require `["Reproduction", "Verification"]`; `fail_if_signal: {"repro.failing": "==false"}` — a repro that doesn't fail is a failed phase.
- **Evidence:** TestPrune (2510.18270): +9.4–12.9% relative resolution; SWE-Tester (2601.13713); SWE-agent/OpenHands reproduce-first. **Highest-leverage single addition.**

#### `behavior-baseline` (evaluate) + `behavior-compare` (evaluate, gate)
- **Responsibility (pair):** Characterization/golden-master safety net straddling the edit. `behavior-baseline` captures pre-refactor observable behavior (targeted test outputs / CLI transcripts) before build; `behavior-compare` re-runs and diffs after build, failing on unexplained drift.
- **Inputs:** baseline: scout-report.md; compare: build-report.md + baseline artifact. **Outputs:** `behavior-baseline.md` / `behavior-compare-report.md`; signals `behavior.preserved` (bool), `behavior.delta_count`.
- **Routing:** both `insert_when: [{field: "scout.goal_type", op: "==", value: "refactor"}]`; baseline `after: "tdd"`, compare `after: "build"`.
- **Classify (compare):** require `["Comparison", "Verdict"]`; `fail_if_signal: {"behavior.preserved": "==false"}`.
- **Evidence:** Feathers characterization tests; DiffKemp (ICST'21); RefactoringMiner 3.0 (TOSEM).

#### `smell-scan` (evaluate)
- **Responsibility:** Detect + rank code smells in the target module/diff (Fowler taxonomy + intent-level smells). Never fixes.
- **Inputs:** scout-report.md. **Outputs:** `smell-scan-report.md` (smell, `file:line`, severity, suggested refactoring); signals `smell.count`, `smell.blocker_count`.
- **Routing:** `insert_when: [{field: "scout.goal_type", op: "==", value: "refactor"}]`, `after: "triage"`.
- **Classify:** require `["Findings"]`; fail_if_empty.
- **Evidence:** SonarQube >85K orgs; LLM smell benchmarks (2504.16027, 2601.09873) — LLMs catch intent-level smells, so LLM-detect + deterministic-gate split.

#### `threat-model` (plan)
- **Responsibility:** Lightweight STRIDE pass over changed security-relevant surfaces (auth/authz, data handling, network boundary, subprocess/exec); emit threats × mitigations.
- **Inputs:** scout-report.md (planned surfaces). **Outputs:** `threat-model.md`; signals `threat.count`, `threat.severity_max`.
- **Routing:** `insert_when: [{field: "scout.security_relevant", op: "==", value: true}]` (advisor-selectable for security goals), `after: "triage"`.
- **Classify:** require `["Threats", "Mitigations"]`; `fail_if_signal: {"threat.severity_max": ">=CRITICAL"}` (unmitigated critical threat blocks the plan).
- **Evidence:** Microsoft SDL/STRIDE; STRIDE GPT; ThreatModeling-LLM (2411.17058); TMAC per-PR practice.

#### `test-amplification` (evaluate)
- **Responsibility:** Adversarial test generation by an agent that has NOT seen the implementation (reads contract/spec only): basic + edge + large-scale inputs. Tests are proposals feeding the audit gate.
- **Inputs:** tdd-contract.md (spec side), build-report.md (file list only, not diffs). **Outputs:** `test-amplification-report.md` + proposed test files; signals `amplify.tests_added`, `amplify.failures_found`.
- **Routing:** `insert_when: [{field: "build.files_touched", op: "gt", value: 0}, {field: "scout.cycle_size", op: "!=", value: "trivial"}]`, `after: "build"`.
- **Classify:** require `["Generated Tests", "Results"]`; fail_if_empty. Signal `amplify.failures_found > 0` routes the existing content-routed `tester` phase.
- **Evidence:** AgentCoder anti-bias test designer (2312.13010); AlphaCodium AI-test phase (2401.08500).

### Wave 2 — deterministic-tooling gates (`phases-quality-gates`)

#### `mutation-gate` (evaluate, gate)
- **Responsibility:** Diff-scoped mutation testing of changed packages; mutation score gates test-suite strength.
- **Outputs:** `mutation-gate-report.md`; signals `mutation.score`, `mutation.survivors`. **Classify:** `fail_if_signal: {"mutation.score": "<60"}` (tune from shadow data).
- **Routing:** tests changed or `amplify.tests_added > 0` (validates generated tests — the answer to "who watches the generated tests").
- **Evidence:** Google TSE 2021 (>24K devs; incremental = feasibility); 93%-coverage/34%-mutation-score failure case.

#### `benchmark-gate` (evaluate, gate) — *upgrade of existing `perf-profile`*
- **Responsibility:** Statistical benchmark comparison vs a stored baseline (benchstat-style, N samples, p-value), not single-run eyeballing. BLOCK on significant regression beyond environment-calibrated threshold (~7% on noisy runners; CodSpeed evidence).
- **Outputs:** signals `perf.regression_pct`, `perf.significant` (bool). **Classify:** `fail_if_signal: {"perf.significant": "==true"}`.
- **Routing:** benchmarked packages touched; skip trivial cycles.

#### `fuzz-probe` (evaluate)
- **Responsibility:** Short-budget Go-native fuzz (`go test -fuzz=. -fuzztime=60s`) of changed parser/input-handling functions; write crashers to corpus.
- **Outputs:** signals `fuzz.crashers`, `fuzz.coverage_new`. **Classify:** `fail_if_signal: {"fuzz.crashers": ">0"}`.
- **Routing:** changed files match parser/decode/unmarshal surfaces (advisor judgment + path heuristic).
- **Evidence:** OSS-Fuzz (>13K vulns/~1K projects); ClusterFuzzLite per-PR model; OSS-Fuzz-Gen for LLM harness generation (+29% coverage).

#### `cleanup-sweep` (evaluate)
- **Responsibility:** Reachability-based dead-code/unused-dependency detection (`x/tools/cmd/deadcode` RTA, `go mod tidy -diff`). Detection only; removal is a build task in a later cycle gated by behavior-lock.
- **Outputs:** signals `deadcode.symbols`, `deadcode.unused_deps`.
- **Routing:** `goal_type == refactor` or periodic hygiene cycle.

#### `rollback-plan` (control)
- **Responsibility:** Pre-ship revert-readiness: declare revert mechanism (git-revert/flag-off), blast radius, known-good version; verify revert command works.
- **Outputs:** `rollback-plan.md`; signal `rollback.ready` (bool). **Classify:** `fail_if_signal: {"rollback.ready": "==false"}`.
- **Routing:** `ship.class == cycle` with high-risk change (releases, migration-touching, >N files).
- **Evidence:** Google SRE release engineering; DORA change-failure-rate (<5% elite).

### Wave 3 — release / feature / memory (`phases-release-and-memory`)

#### `changelog-sync` (control)
- **Responsibility:** Verify CHANGELOG/release-notes drift vs shipped commits (conventional-commit derivation); deterministic tooling wrapped, minimal LLM.
- **Outputs:** signal `changelog.drift_count`. **Routing:** after ship, or pre-release cycles.
- **Evidence:** conventional commits / semantic-release / release-please ecosystem.

#### `post-ship-monitor` (control)
- **Responsibility:** Behavioral probe of the shipped tree (`evolve doctor`, `evolve phases list`, dry-run) to catch integration failures one cycle earlier. Monitoring only; emits `post_ship.health`.
- **Routing:** after `ship` with `ship.class == cycle`. (Already specified in missing-development-phases-2026-06-03; 44% industry adoption.)

#### `api-contract-design` (plan)
- **Responsibility:** Contract-first interface design (Go interfaces / CLI surface / JSON schema) authored before build for new API surfaces; the contract artifact becomes a tdd/audit input.
- **Outputs:** `api-contract.md`; signal `contract.surfaces`. **Routing:** `goal_type == feature` with new exported surface.
- **Evidence:** design-first/OpenAPI practice; Pact CDCT for the verify side.

#### `context-condense` (control)
- **Responsibility:** Summarize + prune long per-cycle artifacts (run dir) for downstream phases when cumulative artifact size exceeds threshold; keeps phase prompts within context budget.
- **Outputs:** condensed digest artifact; signal `condense.ratio`. **Routing:** run-dir artifact bytes > threshold.
- **Evidence:** OpenHands condenser (~2× cost cut, no perf loss, arXiv 2511.03690).

#### Enhancements (catalog cards / persona updates, not new phases)
- **`problem-reflection` / `solution-ranking`** (AlphaCodium): fold into `spec-verify` and `architecture-design` catalog cards — instruct: restate the problem, enumerate 2-3 candidate approaches, rank by correctness/simplicity/robustness.
- **Corpus-level `lesson-extract`** (ExpeL): retro/memo enhancement — periodically mine *across* cycle retrospectives into generalizable rules (the instinct/KB system is the storage layer).
- **Build-internal patch sampling + majority vote** (Agentless phase 2/3): builder-persona research note, not a phase.

## 4. Advisor integration

### 4.1 Goal-type recipes (add to router persona `agents/evolve-router.md`)

The advisor classifies the cycle goal (classify-then-route — Anthropic/LangGraph canonical pattern) and composes from the recipe row, dropping phases whose `insert_when` doesn't fire:

| Goal type | Recipe (optional insertions around the mandatory spine) |
|---|---|
| bugfix | fault-localization → reproduce-bug → [tdd, build] → (regression via existing tdd/audit) |
| feature | problem-reflection (spec-verify card) → api-contract-design → [tdd, build] → test-amplification → tester |
| refactor | smell-scan → behavior-baseline → [build] → behavior-compare → mutation-gate → cleanup-sweep |
| security | threat-model → [tdd, build] → security-scan + dependency-audit (existing) → fuzz-probe |
| performance | benchmark baseline capture → [build] → benchmark-gate |
| release | rollback-plan → changelog-sync → [ship] → post-ship-monitor |
| docs / trivial | spine only (no insertions) |

Recipes are guidance, not law: the advisor may mix rows (e.g. a security-relevant refactor takes threat-model + behavior-lock), and `ClampPlanToFloor` clamps everything.

### 4.2 Config change

`phase-registry.json` config block: raise `max_optional_insertions` **4 → 6** (refactor recipe needs 6). Config-only.

### 4.3 Signal namespace conventions

New namespaces: `fault.*`, `repro.*`, `behavior.*`, `smell.*`, `threat.*`, `amplify.*`, `mutation.*`, `fuzz.*`, `deadcode.*`, `rollback.*`, `changelog.*`, `condense.*` (existing: `security.*`, `dependency.*`, `perf.*`, `post_ship.*`).

## 5. Implementation plan (queued as evolve-loop goals)

Each wave = one or more autonomous cycles. Per phase: `.evolve/phases/<name>/phase.json` + `agent.md`, `evolve phases validate <name>` green, shadow-run before classify rules are tightened. Wave 1 also adds the recipe table to the router persona + the `max_optional_insertions` bump. Queued in `.evolve/state.json:carryoverTodos[]`.

## 6. Future wave (documented only — NOT in scope): spine decomposition

A later ADR may decompose built-in monoliths, requiring Go/registry/floor changes:

- **scout** → `codebase-scan` + `web-research` + `eval-design` (already sub-agents inside the scout skill; promotion would make them advisor-routable individually — e.g. skip web-research on carryover cycles).
- **audit** → `correctness-audit` (floor evaluate) + advisor-selectable lenses (style, safety, docs) — partially superseded by the user evaluate phases above, which already act as audit lenses.
- **build** → optional `patch-candidates`/`patch-rank` sampling stages (Agentless model) — likely better as builder-internal strategy than as phases.
- **risk-gate** (OpenHands per-action SecurityAnalyzer) — a per-tool-call kernel hook, not a phase; belongs in the sandbox/trust-kernel layer if pursued.

Floor implications (≥1 evaluate before ship; build∧audit ordering) must be re-proven if the spine ever splits; until then, additive user phases deliver most of the flexibility at zero floor risk.

## 7. Verification

- Per phase at implementation: `evolve phases validate <name>` OK; `evolve phases list` shows it; one shadow cycle with the phase routed but `fail_if_signal` relaxed; then tighten.
- Catalog-level: a bugfix-goal cycle's routing-plan.json shows the bugfix recipe phases selected; a docs-goal cycle shows spine-only (cost control works both ways).
- Advisor honors recipes: compare `routing-plan.json` justifications against goal classification across ≥3 goal types.
