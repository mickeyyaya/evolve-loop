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
4. **Generated tests are signals, not oracles.** `test-amplification` / `bug-reproduction` outputs feed the tdd/audit gates; they never carry independent ship authority (LLM test suites are documented-unreliable).
5. **Anti-bias isolation.** A test-designing agent must not see the implementation (AgentCoder); preserves the existing builder≠auditor cross-family floor philosophy.
6. **Advisor composes, kernel clamps.** Goal-type recipes live in the router persona as guidance; `ClampPlanToFloor` remains the non-bypassable safety net.

## 3. The catalog — 17 phases in 3 waves

Sketches use v4 phase-descriptor vocabulary; transcribe to `.evolve/phases/<name>/phase.json` + `agent.md` at implementation time and validate with `evolve phases validate <name>`.

### Naming rule (added 2026-06-05; unified 2026-06-06)

Two tiers — **the shape of a name encodes the phase's tier**:

1. **Single-word names are the reserved core-pipeline vocabulary**: `intent`, `scout`, `triage`, `tdd`, `build`, `tester`, `audit`, `ship`, `retrospective`, `memo`. This set is CLOSED — no new single-word phase names, ever. (They are wired into the trust kernel, 53 ACS predicates, and the ledger; renaming them is a migration, not a cleanup.)
2. **Every optional / advisor-selectable / user / minted phase is `<object>-<action>`**: the thing examined, then the operation on it — `smell-scan`, `mutation-gate`, `dependency-audit`, `bug-reproduction` (renamed from `reproduce-bug` 2026-06-06 to fit the rule; signal namespace `repro.*` kept — signals are namespaces, not phase names). The action may be a nominal (`-localization`, `-amplification`, `-reproduction`) when the short verb is ambiguous. A name must answer "what does this phase look at, and what does it do about it" without reading the spec.
3. **Grandfathered Go-wired exceptions** (violate tier 2's shape but cost a Go migration to fix): `tester` (single-word but optional → would be `integration-test`), `build-planner` (role-noun → would be `build-plan`). Do not copy their shapes for new phases.
4. **Every phase declares a one-line CORE VALUE** — the single risk it removes — maintained in the router persona's "Phase Catalog — Core Values" table (`agents/evolve-router.md`). When the `user-phase-persona-resolution` fix lands, this moves into a `description` field in `phase.json` (schema + `phasespec.PhaseSpec` + catalog-card surfacing) so the value line is machine-carried; the same fix adds a naming lint (`evolve phases validate`: optional phases must match `^[a-z]+(-[a-z]+)+$`).
5. **Work-item / batch names follow the same spirit**: deliverable-describing, not sequence-describing (`phases-quality-gates`, not `micro-phase-wave-2`).

### Wave 1 — goal-defining phases (`phases-goal-defining`) — ✅ SHIPPED cycle 217 (`a354d85`)

#### `fault-localization` (plan)
- **Responsibility:** Hierarchical narrowing for bugfix cycles: repo-tree → suspicious files → declaration skeleton → element → edit locations. No fixing.
- **Inputs:** scout-report.md (issue description). **Outputs:** `fault-localization-report.md`; signals `fault.locations_count`, `fault.confidence`.
- **Routing:** `insert_when: [{field: "scout.goal_type", op: "==", value: "bugfix"}]`, `after: "triage"`.
- **Classify:** require sections `["Suspect Ranking", "Edit Locations"]`; fail_if_empty.
- **Evidence:** Agentless (arXiv 2407.01489) — 32% SWE-bench Lite @ $0.70/issue with this exact phase first.

#### `bug-reproduction` (evaluate)
- **Responsibility:** Produce a FAIL_TO_PASS reproduction test/script that demonstrably fails on the current tree, BEFORE any patch.
- **Inputs:** scout-report.md, fault-localization-report.md. **Outputs:** `bug-reproduction-report.md` + repro test file path; signals `repro.failing` (must be true), `repro.test_path`.
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
| bugfix | fault-localization → bug-reproduction → [tdd, build] → (regression via existing tdd/audit) |
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

## 8. Wave 4 — adversarial-pipeline phases (2026-06-14, skills-derived)

Source: a research pass over the most-starred community agent-skill catalogs (agentskills.media → `alirezarezvani/claude-skills`, `VoltAgent/awesome-agent-skills`, `anthropics/skills`, the openaitoolshub "349 ranked" list) cross-referenced against existing coverage. Translates the highest-value, currently-uncovered skill classes into **15 new optional `evaluate` gates** so the loop hardens *any* request type. All are `optional:true` user phases (4-file scaffold, zero spine change); each is an independent skeptic that may BLOCK on CRITICAL but is advisory + clamped to the integrity floor.

**The live recipes + per-phase core values are the SSOT in [`agents/evolve-router.md`](../../agents/evolve-router.md)** (Goal-Type Recipes table + Phase Catalog — Core Values). This section is the design record (what + why + which skill it derives from); it does not restate the recipes.

| Phase (archetype) | Goal type | Derived from / risk removed |
|---|---|---|
| `premise-challenge` (evaluate) | feature/all | Brainstorming / Systematic Debugging — falsifies the premise before build (Core Rules 1–3 as a gate) |
| `coverage-gate` (evaluate) | feature/all | Test-coverage review — changed-line coverage regression (complements mutation-gate = strength) |
| `secret-leak-scan` (evaluate) | security/all | Security scanning — hardcoded-secret/entropy scan of the added diff |
| `flake-rerun-scan` (evaluate) | bugfix/all | Test-quality review — re-run touched tests for non-determinism |
| `race-condition-scan` (evaluate) | concurrency *(new)* | concurrency-patterns / go-review — `go test -race` + goroutine-leak gate |
| `authz-gap-scan` (evaluate) | security | auth-authz-patterns — RBAC/ABAC/object-level/JWT/session gaps |
| `compat-surface-check` (evaluate) | api-design *(new)* | API-contract review — apidiff of realized public surface vs prior release |
| `contract-fuzz-probe` (evaluate) | api-design *(new)* | data-validation-schema — boundary validation (not just non-crashing) |
| `migration-safety-check` (evaluate) | data-migration *(new)* | database-migrations — reversibility/idempotency of schema/state migrations |
| `telemetry-coverage-check` (evaluate) | observability *(new)* | observability-patterns — logs/metrics/traces on new paths before ship |
| `license-provenance-audit` (evaluate) | supply-chain *(new)* | dependency/supply-chain — license + SLSA/SBOM provenance (CVE-complement) |
| `prompt-regression-eval` (evaluate) | agent-instruction *(new)* | agent-self-evaluation — behavioral-rubric regression on persona/skill edits |
| `accessibility-audit` (evaluate) | accessibility *(new)* | frontend-a11y — WCAG 2.1/2.2 AA conformance |
| `frontend-design-review` (evaluate) | frontend-ui *(new)* | frontend-design — UI quality/design-system conformance |
| `locale-format-check` (evaluate) | i18n *(new)* | i18n-l10n-patterns — string externalization + locale-aware formatting |

**New signal namespaces:** `premise.*`, `coverage.*`, `leak.*`, `flake.*`, `race.*`, `authz.*`, `compat.*`, `boundary.*`, `migration.*`, `telemetry.*`, `license.*`, `prompteval.*`, `a11y.*`, `uidesign.*`, `i18n.*` (checked against §4.3 — no collisions).

**New goal types** added to the router recipe table *and* the closed `knownCategories` vocabulary (`go/internal/phasespec/validate.go`) so `.evolve/phase-inventory.json` `category_index` groups them: `concurrency`, `api-design`, `data-migration`, `observability`, `supply-chain`, `agent-instruction`, `accessibility`, `frontend-ui`, `i18n`. Routing uses the proven `insert_when: scout.goal_type == "<type>"` string-equality mechanism (free-form classifier string — no Go enum). The four broad gates (`premise-challenge`, `coverage-gate`, `secret-leak-scan`, `flake-rerun-scan`) trigger on generic signals (`build.files_touched`/`build.diff_loc`/`scout.cycle_size`) so they harden most cycles; the advisor still proposes and `ClampPlanToFloor` caps total insertions at `max_optional_insertions` (6).

**Out of scope this wave (deferred):** `string-extraction-audit` (folded into `locale-format-check`); a `plan`-archetype design counterpart for each gate (the gates verify; existing `architecture-design`/`api-contract-design`/`threat-model` cover forward design). **→ The design-counterpart deferral is closed by Wave 5 below.**

## 9. Wave 5 — coverage expansion + plan/evaluate design pairing (2026-06-14, skills-derived)

Source: a second research pass over the most-starred community agent-skill catalogs (agentskills.media and the catalogs it aggregates) cross-referenced against the post-Wave-4 coverage. Wave 4 hardened *verification* (15 evaluate gates); Wave 5 does two things: (a) extends adversarial coverage into the **highest-star skill classes still uncovered** — database/query, caching, fault-tolerance/resilience, message-delivery semantics, error-handling, container/infra config, type-design, and stream/batch data integrity — with **9 new `evaluate` gates**; and (b) closes the Wave-4 deferral by adding **5 `plan`-archetype design counterparts** so the advisor gets both halves of the loop for a risk class: a forward *design* phase (propose before build) paired with an *evaluate* gate (verify after build). All 14 are `optional:true` user phases (the same 4-file scaffold, zero spine change).

**The live recipes + per-phase core values remain the SSOT in [`agents/evolve-router.md`](../../agents/evolve-router.md)** (Goal-Type Recipes table + Phase Catalog — Core Values). This section is the design record.

| Phase (archetype) | Goal type | Derived from / risk removed |
|---|---|---|
| `query-performance-scan` (evaluate) | database *(new)* | database-review-patterns / postgres-patterns — N+1, missing index, full scan, unbounded result |
| `cache-strategy-scan` (evaluate) | caching *(new)* | caching-strategies — invalidation correctness, stampede/TTL, stale-read, cache-aside race |
| `resilience-gap-scan` (evaluate) | resilience *(new)* | microservices-resilience-patterns — missing timeout/retry/circuit-breaker/bulkhead on external calls |
| `idempotency-check` (evaluate) | messaging *(new)* | message-queue-patterns / batch-job-patterns — at-least-once dedup / exactly-once / safe replay |
| `error-handling-scan` (evaluate) | bugfix/all *(broad)* | error-handling-patterns / silent-failure-hunter — swallowed errors, ignored returns, silent fallback |
| `container-hardening-scan` (evaluate) | infrastructure *(new)* | container-kubernetes-patterns — Dockerfile/k8s insecure defaults (root/:latest/limits/privileged/secrets) |
| `cicd-pipeline-audit` (evaluate) | infrastructure *(new)* | cicd-pipeline-patterns — unpinned actions, pull_request_target risk, over-privileged token, secret-to-log |
| `type-safety-audit` (evaluate) | refactor/all *(broad)* | type-system-patterns / type-design-analyzer — any/unchecked-cast escape hatches, boundary invariants |
| `data-integrity-check` (evaluate) | data-pipeline *(new)* | data-pipeline-patterns — schema drift, null/dedup, out-of-order/late data, partial-write boundaries |
| `resilience-design` (plan) | resilience *(new)* | microservices-resilience-patterns — forward fault-tolerance design (pairs with resilience-gap-scan) |
| `data-model-design` (plan) | database *(new)* | database-review / DDD — schema/index/access-pattern design (data-layer peer of api-contract-design) |
| `caching-strategy-design` (plan) | caching *(new)* | caching-strategies — cache pattern/key/invalidation/TTL design (pairs with cache-strategy-scan) |
| `observability-design` (plan) | observability | observability-patterns — instrumentation/SLO/alert design (pairs with telemetry-coverage-check) |
| `rollout-plan` (plan) | release | feature-flags-progressive-delivery / deployment-patterns — canary/blue-green + flag/kill-switch + rollback triggers |

**New signal namespaces:** `query.*`, `cache.*`, `resilience.*`, `idempotency.*`, `errhandling.*`, `container.*`, `cicd.*`, `typesafety.*`, `dataintegrity.*`, `resiliencedesign.*`, `datamodel.*`, `cachedesign.*`, `obsdesign.*`, `rollout.*` (checked against §4.3 + the Wave-4 set — no collisions; design/gate pairs use distinct namespaces, e.g. `cache.*` vs `cachedesign.*`).

**New goal types** added to the router recipe table *and* the closed `knownCategories` vocabulary (`go/internal/phasespec/validate.go`): `database`, `caching`, `resilience`, `messaging`, `infrastructure`, `data-pipeline`. (`observability` and `release` already existed and gain a `plan` design phase.) Routing uses the proven `insert_when: scout.goal_type == "<type>"` string-equality mechanism; the two broad gates (`error-handling-scan`, `type-safety-audit`) trigger on `build.diff_loc >= 50` so they harden most cycles. `ClampPlanToFloor` still caps total insertions at `max_optional_insertions`.

**Plan/evaluate pairing (the architectural point):** for caching, resilience, and observability the advisor can now insert the design phase *before* build and the gate *after* — the design declares the contract (invalidation triggers, fallback strategy, SLOs) and the gate blocks if the implementation violates it. `data-model-design`'s after-side is covered by `query-performance-scan` (which verifies the access paths and indexes it designs) plus `migration-safety-check`; `rollout-plan` is forward-only, with `post-ship-monitor` covering the after-side.

**Out of scope this wave (deferred):** dedicated gates for `data-model-design` and `rollout-plan`; a broader infra fix to stop committing the host `project_root` path in generated `.evolve/phase-inventory.json` (pre-existing, carried from Wave 4).
