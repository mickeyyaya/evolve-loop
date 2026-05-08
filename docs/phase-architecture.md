# Phase Architecture — Deep Dive

> Full-detail walkthrough of every phase in an evolve-loop cycle. Pipeline diagram + per-phase: who runs, when, inputs, what it checks/does, outputs, model selection, version-specific behavior. Companion to [release-protocol.md](release-protocol.md) and [architecture/tri-layer.md](architecture/tri-layer.md).

## Pipeline diagram

```
/evolve-loop CLI argv: [cycles] [strategy] [goal]
       │
       ▼
evolve-loop-dispatch.sh  (privileged shell, NOT a subagent)
  - environment preflight  (sandbox, worktree base, nested-Claude)
  - fail loop circuit-breaker (5 same-cycle iterations -> abort)
       │  per cycle:
       ▼
run-cycle.sh  (privileged shell)
  - provision worktree at $TMPDIR/evolve-loop/<hash>/cycle-N
  - initialize cycle-state.json at phase=calibrate
  - spawn orchestrator subagent (profile-restricted)
       │  orchestrator sequences (every transition through phase-gate hook):
       ▼
  Phase 0     CALIBRATE      -> benchmark dimensions
  Phase 0b    INTENT         -> structure goal (v8.19.1+ always-on)
  Phase 1/2   RESEARCH/SCOUT -> discover tasks + write evals
              (phase-gate transition; ledger entry with prev_hash v8.37+)
  Phase 3     BUILD          -> Builder edits in worktree
              (phase-gate transition)
  Phase 4     AUDIT          -> Auditor verdict (PASS|WARN|FAIL)
              (verdict-driven branch:)
  Phase 5a      PASS  -> ship.sh (worktree-aware v8.43+)
  Phase 5b      WARN  -> record + ship.sh (v8.35.0+ fluent)
  Phase 5c      FAIL  -> record-failure-to-state.sh + retrospective
  Phase 6     LEARN          -> retrospective extracts lessons
  Phase 7     META           -> self-improvement (every 5 cycles)
       │
       ▼
orchestrator-report.md -> cycle-state cleared -> ledger entry written
```

Each transition between phases passes through `scripts/guards/phase-gate-precondition.sh`, which reads `cycle-state.json` and refuses subagent dispatches that violate the Scout to Builder to Auditor sequence. Each subagent invocation is wrapped by `subagent-run.sh` which:

1. Reads the phase agent's profile (`.evolve/profiles/<role>.json`)
2. Resolves model (default tier from profile, with v8.35.0 adaptive downgrade for trivial diffs)
3. Generates a per-invocation challenge token
4. Captures `git_head` + `tree_state_sha` (cycle binding — v8.13.0)
5. Spawns `claude -p` (or platform adapter) with profile-scoped tool permissions
6. On exit: computes artifact SHA256, writes ledger entry with `prev_hash` (v8.37.0 hash chain) and updates `.evolve/ledger.tip`

## Phase 0 — Calibrate

**Persona**: orchestrator (no separate Calibrate agent; the orchestrator's first action).
**When**: ONCE per `/evolve-loop` invocation (not per cycle), at the start of cycle 1.
**Purpose**: Compute the project's `projectBenchmark` across 8 dimensions so Scout has objective signal for task selection.

### What Calibrate measures

| Dimension | Automated probe | LLM probe |
|---|---|---|
| `documentationCompleteness` | Count `*.md` files, README presence, docstring coverage | Subjective rating of clarity |
| `specificationConsistency` | Schema mismatches, type errors | Internal contradictions |
| `defensiveDesign` | Try/catch density, input validation, schema-validation calls | Risk-coverage assessment |
| `evalInfrastructure` | Test count, eval file count, mutation-test capability | Coverage believability |
| `modularity` | Avg file size, cyclomatic complexity, coupling metrics | Organizational coherence |
| `schemaHygiene` | JSON schema validation, type-system completeness | Field-naming consistency |
| `conventionAdherence` | Linter pass rate, formatter consistency | Style alignment with stated norms |
| `featureCoverage` | Implementation vs spec audit | Gap analysis |

Each dimension gets a `composite` score (0-1) blending automated and LLM probes. The result writes to `state.json:projectBenchmark`.

### Why this matters

- **Scout's task selection** weights by `1 - composite` for each dimension (low scores = priority surface)
- **Mastery gating**: `state.json:mastery.level` (`novice` to `apprentice` to `journeyman` to `master`) advances on consecutive successes
- **Regression detection**: each cycle's `fitnessScore` compares against `benchHist[-1]`; a drop triggers `fitnessRegression` flag

### Phase output

- `state.json:projectBenchmark` updated
- `phase0-calibrate.md` workspace artifact (audit trail)
- Ledger entry: `kind=calibrate`

### Cost

Typically ~$0.30-0.80 for a fresh calibration. Skipped on cycles 2+ within the same `/evolve-loop` invocation (cached).

---

## Phase 0b — Intent (v8.19.1+)

**Persona**: `agents/evolve-intent.md` — "Intent Architect" (model: tier-1 / Opus default).
**When**: Always-on for the `/evolve-loop` slash command path. Before Scout. Skipped only on legacy `bash run-cycle.sh` invocations without `EVOLVE_REQUIRE_INTENT=1`.
**Purpose**: Convert the user's vague goal into a structured `intent.md` so Scout doesn't have to infer.

### Why Intent exists

Quoting the persona file's framing:
- 56% of real-world user instructions are missing key information (arXiv 2409.00557)
- Production agents typically achieve only 25% prompt fidelity
- Karpathy named "wrong assumptions running uncaught" as the #1 failure mode in agentic coding

The cycle 25 incident (2026-04-29 audit) showed the cost of skipping this phase: a vague user goal "improve the UI" produced 25 cycles of work in the wrong direction. Intent prevents that class of failure by making assumptions explicit.

### What Intent produces — `intent.md` schema

```yaml
---
awn_class: IMKI | IMR | IwE | IBTC | CLEAR
goal: <restated goal in 1-2 sentences>
non_goals: [<what NOT to attempt>]
constraints: [<hard constraints — must be met>]
interfaces: [<files/modules/APIs the change will touch>]
acceptance_checks:
  - check: <verifiable criterion>
    how_verified: <programmatic|manual>
assumptions: [<what Intent ASSUMED while structuring>]
challenged_premises:
  - premise: <a premise Intent identified in user's goal>
    challenge: <why it might be wrong>
    resolution: <how to proceed despite uncertainty>
risk_level: low | medium | high
---
```

### The Ask-when-Needed (AwN) classifier

| Class | Meaning |
|---|---|
| **IMKI** (Intent Misses Key Information) | Goal incomplete but completable by inference; Intent fills gaps |
| **IMR** (Intent Mostly Right) | Goal clear; minor structuring needed |
| **IwE** (Intent with Errors) | Goal has factual errors; Intent corrects |
| **IBTC** (Intent Beyond This Cycle) | Goal too large; Intent narrows scope, flags out-of-scope as `non_goals` |
| **CLEAR** | Goal fully specified; minimal structuring |

The `failure-adapter.sh` rule "intent-rejected" fires only on `IBTC` after operator escalation, blocking the cycle.

### The mandatory >=1 challenged_premise rule

Every intent.md MUST include at least one `challenged_premises` entry. Even a CLEAR goal has at least one assumption worth surfacing. This is the **anti-sycophancy mechanism** at the structuring layer — Intent isn't allowed to just nod along; it must flag at least one thing.

If Intent emits zero challenged_premises, `phase-gate-precondition.sh:gate_calibrate_to_intent` rejects the artifact and the cycle aborts.

### What Intent MUST NOT do

- NOT plan implementation steps (Scout's job)
- NOT design APIs (Builder's job)
- NOT decide model routing (kernel's job)
- NOT skip the AwN classifier or challenged_premises rule

### Phase output

- `.evolve/runs/cycle-N/intent.md` (artifact)
- Ledger entry with artifact_sha256

### Cost

~$0.40-1.00 per cycle. Always uses tier-1 (Opus) since structuring quality matters more than speed.

---

## Phase 1 / 2 — Research + Discover (Scout)

**Persona**: `agents/evolve-scout.md` — model tier-2 (Sonnet) default, tier-1 on cycle 1 or when explicit goal given.
**When**: After Intent. Before Builder.
**Purpose**: Scan the codebase, identify the highest-leverage task(s) for this cycle, write evals (acceptance graders) Builder will be evaluated against.

### Mode-based discovery

Scout has three modes determined by cycle context:

| Mode | Trigger | Behavior |
|---|---|---|
| `full` | Cycle 1 of any invocation | Read ALL project docs, full codebase scan, generate `project-digest.md` |
| `incremental` | Cycle 2+ same invocation | Read `project-digest.md` cached, scan only `git diff HEAD~1 --name-only` |
| `convergence-confirmation` | `state.stagnation.nothingToDoCount == 1` | Confirm there's genuinely no work; if confirmed, exit cycle cleanly |

### Goal handling

- **If goal provided** (most common): focus discovery on goal-relevant areas. Scan only those. Research only those approaches.
- **If goal null** (autonomous): broad discovery — assess all 8 benchmark dimensions, scan full codebase, pick highest-impact work.

### Strategy handling

The CLI argv's strategy keyword shapes Scout's discovery scope:

| Strategy | Scout focus |
|---|---|
| `balanced` | All dimensions, no preference |
| `innovate` | Feature-coverage gap, new capabilities |
| `harden` | Stability, defensive design, eval infrastructure |
| `repair` | Bug fixes, broken tests, no new features |
| `ultrathink` | Complex refactors; tier-1 model forced |
| `autoresearch` | Hypothesis testing; embraces failure for diversity |

### Scout's responsibilities

1. **Read instinctSummary** — apply successful patterns, avoid known anti-patterns
2. **Online research (if needed)** — invoke `online-researcher.md` protocol via WebSearch + WebFetch; save Knowledge Capsules to `.evolve/research/`
3. **Task selection** — pick 1-3 tasks ranked by `composite_score = (1 - benchmark_dimension_score) × strategy_weight × leverage_estimate`
4. **Write evals** — for each task, write `.evolve/evals/<task-slug>.md` with:
   - **Code Graders**: bash commands that must exit 0
   - **Regression Evals**: full test suite must pass
   - **E2E Graders** (UI tasks): Playwright assertions
   - **LLM Graders**: subjective rubric scored by Auditor
5. **Mutation-testing pre-flight** (v8.20+): runs `scripts/verification/mutate-eval.sh` against new evals; kill rate < 0.8 -> flagged as tautological

### Pattern-3 fan-out (v8.23+, opt-in)

When `EVOLVE_FANOUT_SCOUT=1`, Scout fans into 3 sub-scouts:
- `scout-codebase` — internal scan only
- `scout-research` — external research only
- `scout-evals` — eval design only

The aggregator merges their findings into a unified `scout-report.md`. Default-off because fan-out 3x a single Scout's cost.

### Phase output

- `.evolve/runs/cycle-N/scout-report.md` — task list + recommended skills + research links
- `.evolve/evals/<task>.md` — eval definitions per task
- `project-digest.md` (cycle 1 only) — codebase fingerprint
- Ledger entry

### Cost

Typically $0.50-2.50/cycle. Cycle 1 (full mode + research) is most expensive.

---

## Phase 3 — Build (Builder)

**Persona**: `agents/evolve-builder.md` — model tier-2 (Sonnet) default, tier-1 (Opus) for M+5-files diffs or retry >= 2.
**When**: After Scout. Before Auditor.
**Purpose**: Implement the task(s) Scout selected. Edit files in the per-cycle worktree (NOT main repo). Self-verify (run tests). Commit in worktree branch.

### Worktree isolation (mandatory)

Builder's profile (`.evolve/profiles/builder.json`) restricts:
- `add_dir: ["{worktree_path}"]` — only the cycle's worktree directory
- `sandbox.write_subpaths: ["{worktree_path}", ".evolve/runs/cycle-*"]`
- `disallowed_tools` includes `Bash(git push:*)`, interpreter-execution patterns (`python3 -c`, `perl -e`, `bash -c`, etc.), and direct edits to `.evolve/state.json`, `.evolve/ledger.jsonl`, agent files, scripts, etc.

If Builder is somehow spawned in the main worktree (e.g., misconfiguration), Step 0 of its workflow self-aborts:
```bash
MAIN_WORKTREE=$(git worktree list --porcelain | head -1 | sed 's/worktree //')
[ "$MAIN_WORKTREE" = "$(pwd)" ] && { echo "FATAL: Builder in main worktree"; exit 1; }
```

### Build workflow (8 steps)

| Step | Action |
|---|---|
| **0** | Verify worktree isolation (above) |
| **1** | Read `instinctSummary` + check `.evolve/genes/` for matching action recipes (confidence >= 0.6) |
| **2** | Read task + eval graders from scout-report and `.evolve/evals/` |
| **2.5** | Online research if needed |
| **2.7** | Skill consultation if `task.recommendedSkills` is non-empty |
| **3** | Design (chain-of-thought required): files to change, implementation order, risks, alternatives, evidence |
| **4** | Implement |
| **4.5** | E2E test generation (UI tasks): invoke `everything-claude-code:e2e-testing` skill, run Playwright, iterate implementation (NOT test) |
| **5** | Self-verify: run task's eval graders + regression tests |
| **6** | Commit in worktree (`git add -A; git commit -m "<type>: <desc> [worktree-build]"`) |
| **7** | Write `build-report.md` with: tasks completed, files changed, test results, eval grader outcomes, applied instincts, skills invoked |

### v8.43.0+ Builder-to-main bridge

**Pre-v8.43.0 bug**: Builder's commits stayed on the worktree's `evolve/cycle-N` branch. ship.sh ran from main repo cwd, saw a clean tree, exited with nothing committed. Cycles produced correct work that never reached production. Real-world cost: $18.92 across 5 wasted cycles.

**v8.43.0 fix**: ship.sh now explicitly handles the worktree path. When `--class cycle` and `cycle-state.active_worktree` is set:
1. `cd $active_worktree`; `git add -A`; `git commit` ON the cycle branch
2. `cd` main; `git merge --ff-only $cycle_branch`
3. `git push origin main`

If the cycle branch has divergent history (Builder rebased or did weird things), `--ff-only` fails and ship refuses — operator must investigate.

### Phase output

- Code changes committed to `evolve/cycle-N` branch in the worktree
- `.evolve/runs/cycle-N/build-report.md` — narrative + file list + test results
- Ledger entry

### Cost

$0.50-3.00/cycle depending on diff complexity. Tier-1 upgrade kicks in for retries (audit FAIL -> Builder re-runs at higher tier).

---

## Phase 4 — Audit (Auditor)

**Persona**: `agents/evolve-auditor.md` — model tier-1 (Opus) default; v8.35.0 auto-downgrades to Sonnet for trivial diffs (<=3 files, <=100 lines, no security paths).
**When**: After Builder. Before Ship.
**Purpose**: Cross-check Builder's narrative against ground truth. Issue PASS/WARN/FAIL verdict.

### What Auditor checks (Single-Pass Review Checklist)

#### A. Code Quality
Pattern adherence, naming, error handling, defensive design. NOT style — focuses on bug-class issues (resource leaks, off-by-one, missing input validation).

#### B. Security
Secret leakage, injection patterns, sandbox boundary violations. Force-upgrades to Opus when changed paths match security regex (`auth|crypto|payment|secret|\.env|password|token`).

#### B2. Hallucination Detection
Does build-report cite specific files that don't exist? Does it claim line-counts that don't match actual diff? **v8.34.0's actual-diff footer** records the real file list in commit message — Auditor cross-checks Builder's narrative against ground truth.

#### C. Pipeline Integrity
- All claimed phases produced their artifacts?
- Challenge tokens match (artifact has the per-invocation token Builder was issued)?
- No bypass markers (`EVOLVE_BYPASS_*`) appearing in build evidence?

#### D. Eval Integrity and Rigor
- Builder didn't WEAKEN graders to pass (eval tampering — CRITICAL)
- Test files weren't ALTERED to make them pass without fixing implementation
- Mutation-test kill rate >= 0.8 (catches tautological evals)

#### D2. Step-Confidence Cross-Validation
Each numbered build step requires evidence. "I ran `npm test`" with no output excerpt -> suspicious. With excerpt -> verifiable.

#### D5. E2E Grounding (UI/browser tasks only)
Frontend tasks require Playwright screenshots / DOM snapshots, not just "I tested it visually".

### Verdict semantics

| Verdict | Meaning | Downstream effect |
|---|---|---|
| **PASS** | All checks passed. Ship without reservation. | ship.sh commits + pushes; lastCycleNumber advances |
| **WARN** | Findings present, not blocking. | **Ships by default** (v8.28.0 ship.sh, v8.35.0 orchestrator). Recorded as `code-audit-warn` (low severity, 1d age-out). `EVOLVE_STRICT_AUDIT=1` reverts to legacy block-on-WARN. |
| **FAIL** | Real defect, refuse to ship. | Orchestrator skips ship; `record-failure-to-state.sh` appends `code-audit-fail` (high severity, 30d age-out). Triggers Retrospective. |

ship.sh parses verdict via regex: accepts `Verdict: PASS|WARN|FAIL` (case-insensitive) OR markdown-bold heading form `## Verdict\n**PASS**`. v8.30.0 added dual-verdict detection — if BOTH PASS and FAIL appear, ship refuses ("auditor produced inconsistent artifact").

### Adversarial Auditor framing (default-on)

The runner prepends an "ADVERSARIAL AUDIT MODE" framing requiring positive evidence for PASS verdicts. Auditor model defaults to Opus (different family from Builder's Sonnet) to break same-model-judge sycophancy. Disable only with `ADVERSARIAL_AUDIT=0`.

### Pattern-3 fan-out (v8.23.0+, opt-in)

When `EVOLVE_FANOUT_AUDITOR=1`, audit fans into 4 sub-auditors:

| Sub-auditor | Focus |
|---|---|
| `audit-eval-replay` | Re-runs every eval via `verify-eval.sh` |
| `audit-lint` | shellcheck/jsonlint/typecheck on changed files |
| `audit-regression` | Full `run-all-regression-tests.sh` |
| `audit-build-quality` | Lock-file integrity, no committed secrets, no large binaries |

Aggregator's rule: ANY FAIL -> aggregate FAIL.

### Cycle binding — Auditor's structural anchor role

This is what makes Auditor's verdict **unforgeable post-hoc**:

```json
{
  "role": "auditor",
  "artifact_path": "/path/to/audit-report.md",
  "artifact_sha256": "<hash of the file>",
  "git_head": "<HEAD when auditor saw the code>",
  "tree_state_sha": "<sha256 of git diff HEAD>",
  "challenge_token": "<random per-invocation>",
  "entry_seq": N,
  "prev_hash": "<sha of previous ledger entry>"
}
```

When ship.sh runs, it:
1. Reads the LATEST auditor ledger entry
2. Verifies audit-report.md still has the recorded SHA (didn't get rewritten post-audit)
3. Verifies current `git_head` matches recorded (cycle binding)
4. Verifies current `tree_state_sha` matches (no modifications between audit and ship)
5. Verifies the verdict text in audit-report

ANY mismatch -> `INTEGRITY-FAIL`, ship refused.

### Phase output

- `.evolve/runs/cycle-N/audit-report.md` — verdict + per-eval results + findings
- Ledger entry with `artifact_sha256` + cycle binding

### Cost

$0.40-2.50/cycle. v8.35.0 adaptive tiering brings trivial-diff audits to ~$0.50; complex/security audits stay at ~$2.50.

---

## Phase 5 — Ship/Record

**Two paths depending on Audit verdict.**

### 5a. PASS or WARN -> ship.sh

**Script**: `scripts/lifecycle/ship.sh` — privileged shell, NOT a subagent. Allowlisted as the canonical entry point by `ship-gate.sh`.
**Invoked by**: Orchestrator with `ship.sh "<commit-message>"`.

#### ship.sh flow (v8.43.0)

1. **Parse `--class`** (cycle | manual | release; default cycle)
2. **Self-SHA verification** (TOFU) — ensures ship.sh wasn't tampered with
3. **Class-aware verification** — for `cycle`, requires recent auditor PASS; for `manual`, interactive y/N; for `release`, skips audit (release-pipeline only)
4. **Cycle binding check** — current `git_head` + `tree_state_sha` must match audit-report's recorded values
5. **Worktree-aware ship (v8.43.0+)**:
   - Read `cycle-state.active_worktree`
   - `cd $active_worktree`; `git add -A`; commit on cycle branch
   - `cd` main; `git merge --ff-only evolve/cycle-N`
   - `git push origin main`
6. **Append actual-diff footer (v8.34.0)** — commit message gets a `## Actual diff` block listing files modified + line counts
7. **Advance `state.json:lastCycleNumber`** (v8.34.0+) — closes the "counter never advances" deadlock
8. **Optional GitHub release** (when `EVOLVE_SHIP_RELEASE_NOTES` set, used by `release-pipeline.sh`)

#### Operator overrides

| Env var | Effect |
|---|---|
| `EVOLVE_STRICT_AUDIT=1` | Block ship on WARN (revert to pre-v8.28.0) |
| `EVOLVE_BYPASS_SHIP_VERIFY=1` | Skip self-SHA pin (legacy compat; deprecated -> use `--class manual`) |
| `EVOLVE_BYPASS_SHIP_GATE=1` | Bypass the kernel hook entirely (emergency only; logged WARN) |
| `EVOLVE_SHIP_AUTO_CONFIRM=1` | Skip interactive y/N for `--class manual` (CI mode) |

### 5b. FAIL -> record-failure + retrospective

**Script**: `scripts/failure/record-failure-to-state.sh` (privileged shell).
**Invoked by**: Orchestrator with `record-failure-to-state.sh $WORKSPACE FAIL`.

The script appends to `state.json:failedApproaches[]` with the cycle's verdict + classification + expiresAt + audit binding.

The classification taxonomy (`scripts/failure/failure-classifications.sh`):

| Classification | Severity | Age-out | Retry policy |
|---|---|---|---|
| `infrastructure-transient` | low | 1d | yes |
| `infrastructure-systemic` | high | 7d | needs-operator |
| `intent-malformed` | low | 1d | yes |
| `intent-rejected` | terminal | never | no |
| `code-build-fail` | high | 30d | needs-operator |
| `code-audit-fail` | high | 30d | needs-operator |
| `code-audit-warn` | low | 1d | yes |
| `ship-gate-config` | low | 1d | yes |
| `human-abort` | low | 1h | yes |

The next cycle's `failure-adapter.sh` reads non-expired entries, applies decision rules, and emits a deterministic JSON the orchestrator follows verbatim (`PROCEED` | `RETRY-WITH-FALLBACK` | `BLOCK-CODE` | `BLOCK-OPERATOR-ACTION`).

### Phase output (5a path)

- A new commit on `origin/main` (via worktree-aware ff-merge)
- Updated `state.json:lastCycleNumber`
- Updated `.evolve/ledger.tip` and `.evolve/ledger.jsonl` (v8.37 hash chain)

### Phase output (5b path)

- New entry in `state.json:failedApproaches[]`
- Worktree branch `evolve/cycle-N` cleaned up (run-cycle.sh's EXIT trap)
- Retrospective phase triggered

---

## Phase 6 — Learn (Retrospective)

**Persona**: `agents/evolve-retrospective.md` — read-only everywhere except its own artifacts + `.evolve/instincts/lessons/*.yaml`.
**When**: After audit FAIL or WARN (NOT on PASS — successful cycles emit a lighter "what went well" version via the Learn skill, not a full retrospective).
**Purpose**: Extract a reusable lesson from the failure so future cycles don't repeat it.

### Core principle: the retrospective IS the lesson

A retrospective that says "audit failed because of D1, D2, D3 defects" is **a status report, not a retrospective**. A retrospective answers:

- **What was the underlying assumption that turned out to be wrong?** (Not "we wrote bad code" — the deeper assumption.)
- **What signal could have surfaced this earlier?** (Earlier in the cycle, ideally before Builder ran.)
- **What guardrail would prevent the same class of failure?** (Often a new test, a new instinct, a new auditor probe, or a process change — not just "write better code.")
- **Has this happened before?** (If `priorLessons` shows >=2 prior failures with the same `errorCategory`, this is a **systemic issue**.)

### Lesson YAML schema

Each retrospective produces a lesson file at `.evolve/instincts/lessons/<id>.yaml`:

```yaml
id: inst-L042  # monotonic, orchestrator-suggested
errorCategory: <e.g., "shell-parser-substring-matching">
description: |
  <reusable lesson — agent-readable, not session-specific>
preventiveAction: |
  <concrete check or pattern future agents should apply>
trigger:
  - <pattern that signals this class of failure>
contradicts: [inst-007]  # optional: prior instincts this supersedes
relatedInstincts: [inst-019, inst-031]  # optional: cross-links
confidence: 0.7
firstSeen: 2026-05-08T...
lastSeen: 2026-05-08T...
seenCount: 1
```

### Adversarial honesty about contradictions

If the failure suggests an existing instinct was wrong, the retrospective MUST flag it. The orchestrator doesn't auto-prune contradicted instincts, but flagging enables a downstream `prune` step.

### One lesson per root cause, not one per defect

Three HIGH defects (D1, D2, D3) all stemming from the same root cause produce **one** lesson. Genuinely different root causes get separate lessons cross-linked via `relatedInstincts`.

### How lessons reach future cycles

- `state.json:instinctSummary[]` is the trimmed-for-context list of recent lessons
- Scout reads `instinctSummary` and applies known patterns
- Builder reads `instinctSummary` and reads matching `.evolve/genes/` action recipes
- Auditor reads `instinctSummary` to know what to specifically check for

### Phase output

- `.evolve/runs/cycle-N/retrospective-report.md` — narrative
- `.evolve/runs/cycle-N/handoff-retrospective.json` — structured handoff
- `.evolve/instincts/lessons/<id>.yaml` — the persistent lesson (RUNTIME source for Scout/Builder/Auditor)
- Updated `state.json:instinctSummary` (orchestrator merges via `merge-lesson-into-state.sh`)
- `.evolve/audit-investigations/<date>-cycle-<N>-<verdict>-<slug>/` (v8.46.0+) — operator-facing investigation dir

### v8.46.0+ audit-investigations folder

For human-readable failure review, every FAIL/WARN cycle now also gets a durable investigation directory at `.evolve/audit-investigations/<YYYY-MM-DD>-cycle-<N>-<VERDICT>-<slug>/`:

```
2026-05-08-cycle-1-FAIL-spec-mismatch/
├── investigation.md     # narrative root-cause analysis (= retrospective-report.md)
├── improvements.md       # concrete improvement suggestions
├── evidence/             # FROZEN snapshots — preserved against runs/cycle-N/ cleanup
│   ├── audit-report.md
│   ├── build-report.md
│   ├── orchestrator-report.md
│   ├── intent.md
│   └── scout-report.md
├── lesson.yaml           # copy of the canonical lesson YAML for at-a-glance review
└── status.json           # {state: open|closed|implemented, ...}
```

Status lifecycle:

| State | Meaning |
|---|---|
| `open` | Investigation written, awaiting operator decision |
| `closed` | Investigated and decided no action needed (intentional WARN, environmental fluke) |
| `implemented` | Improvements actioned. `action_refs[]` lists commit SHAs / PR numbers |

The `INDEX.md` at the directory root is auto-regenerated by `scripts/failure/index-investigations.sh` after each merge. Operator workflow:

1. Read INDEX.md to see open investigations
2. Open the dir, read investigation.md + evidence/ + improvements.md
3. Decide → update status.json → re-index

Lesson YAML in `instincts/lessons/` is the RUNTIME source-of-truth (Scout/Builder/Auditor read it via `state.json:instinctSummary`). The investigation dir is the OPERATOR source-of-truth (humans review, decide, action). Same lesson, two surfaces with different audiences.

The orchestrator's `merge-lesson-into-state.sh` populates this dir automatically as part of the FAIL/WARN paths. See [.evolve/audit-investigations/README.md](../.evolve/audit-investigations/README.md) for the full operator workflow.

---

## Phase 7 — Meta (every 5 cycles, optional)

**Persona**: shared with orchestrator + retrospective patterns.
**When**: Triggered when `state.lastCycleNumber % 5 == 0`. Not always-on.
**Purpose**: Self-improvement of the EVOLVE-LOOP itself — patterns that recur across cycles get codified into kernel hooks, profile changes, or skill prompts.

### What Meta does

1. **Pattern detection across last 5 cycles**:
   - Recurring failure modes -> propose new kernel hook or eval
   - Recurring success patterns -> propose new gene or skill
   - Recurring cost outliers -> propose budget tier adjustment
2. **Proposal generation** in `state.json:proposals[]` — the operator reviews and accepts/rejects
3. **Auto-experiment design** in `state.json:researchAgenda` — autoresearch strategy uses these for future cycles

### Why this is rare

Meta-cycle work is high-leverage but high-risk (it modifies the kernel that all future cycles use). It's gated to every-5-cycles to ensure enough signal accumulates between meta-changes. Operator approval gate prevents runaway self-modification.

### Phase output

- `state.json:proposals[]` updated (operator-reviewable)
- `state.json:researchAgenda` updated (auto-experiment seeds)
- No direct code or kernel changes (those go through normal build/audit/ship cycles, with the meta-output AS the goal)

---

## Cross-cutting concerns

These apply to every phase:

### Kernel hooks (Tier 1)

Three PreToolUse hooks fire at every transition:

| Hook | Watches | Denies |
|---|---|---|
| `phase-gate-precondition.sh` | Bash with `subagent-run.sh` | Out-of-order phases |
| `role-gate.sh` | Edit/Write tool calls | Writes outside the active phase's allowlist |
| `ship-gate.sh` | Bash with git/gh verbs | Anything except `scripts/lifecycle/ship.sh` doing commit/push/release |

These cannot be bypassed during normal operation. Emergency overrides (`EVOLVE_BYPASS_*`) log loudly and are considered CRITICAL violations per CLAUDE.md.

### Tamper-evident ledger (v8.37.0+)

Every phase's subagent invocation records to `.evolve/ledger.jsonl`:

```json
{
  "ts": "...",
  "cycle": 25,
  "role": "scout",
  "kind": "agent_subprocess",
  "model": "sonnet",
  "exit_code": 0,
  "duration_s": "870",
  "artifact_path": "...",
  "artifact_sha256": "...",
  "challenge_token": "...",
  "git_head": "...",
  "tree_state_sha": "...",
  "entry_seq": 56,
  "prev_hash": "868aba98..."
}
```

`prev_hash` chains entries; modifying entry N breaks every entry after. `.evolve/ledger.tip` records the latest entry's full SHA — truncation detection. `bash scripts/observability/verify-ledger-chain.sh` walks the chain and reports breaks (rc=1) or truncation (rc=2).

### Cycle binding (v8.13.0)

At every subagent invocation, `subagent-run.sh:capture_git_state()` captures:
- `git_head` — the commit HEAD when the agent started
- `tree_state_sha` — SHA256 of `git diff HEAD` (working tree state)

These pin the agent's understanding to a specific code state. ship.sh refuses to ship if the current state diverges from what the auditor saw.

### Failure adaptation (v8.22.0+, fluent v8.28.0+)

`scripts/failure/failure-adapter.sh` reads `state.json:failedApproaches[]` and emits a deterministic decision JSON:

```bash
bash scripts/failure/failure-adapter.sh decide --state .evolve/state.json
# {"action": "PROCEED|RETRY-WITH-FALLBACK|BLOCK-CODE|BLOCK-OPERATOR-ACTION",
#  "reason": "...", "set_env": {...}, "verdict_for_block": "..."}
```

The orchestrator follows the action verbatim. Pre-v8.22 this was a markdown rule the orchestrator interpreted (non-deterministic). v8.28.0 made the default fluent — would-have-blocked rules emit awareness instead of hard blocks. `EVOLVE_STRICT_FAILURES=1` reverts to legacy.

### Adaptive cost (v8.33.0 + v8.35.0)

- v8.33.0: cache-friendly prompt order (agent persona prepended verbatim -> hits Anthropic prompt cache, 0.1x cost on subsequent reads in same 5-min TTL window)
- v8.33.0: conditional context blocks (skip empty `recentLedgerEntries`, `recentFailures`, `instinctSummary` headers)
- v8.35.0: adaptive auditor model selection via `scripts/utility/diff-complexity.sh` — Sonnet for trivial diffs, Opus for complex/security

### Three-Tier Strictness Model (v8.24.0+)

| Tier | Mechanism | Default | Auto-adapt? |
|---|---|---|---|
| **1 — Structural integrity** | phase-gate, ledger SHA, role-gate, ship-gate, hash chain | Always on | NEVER |
| **2 — OS isolation** | sandbox-exec/bwrap, per-cycle worktree | On (always) | Worktree path auto-selected per environment; sandbox falls back when nested |
| **3 — Workflow defaults** | intent capture, fan-out, mutation testing, adversarial audit | Opt-in via env flags | N/A |

Tier 1 is non-negotiable. Tier 2 adapts. Tier 3 is operator-controlled.

---

## Reading order for new contributors

If you're learning evolve-loop's architecture, read in this order:

1. [README.md](../README.md) — high-level project overview
2. [AGENTS.md](../AGENTS.md) — cross-CLI invariants (the universal pipeline contract)
3. **This file** — phase-by-phase deep dive
4. [docs/release-protocol.md](release-protocol.md) — vocabulary (push != tag != release != publish != propagate)
5. [docs/architecture/tri-layer.md](architecture/tri-layer.md) — Skill / Persona / Command separation
6. [CLAUDE.md](../CLAUDE.md) — Claude Code-specific runtime + version-by-version notes
7. [docs/release-notes/index.md](release-notes/index.md) — per-version theme index

Each version of evolve-loop encodes hard-won lessons from real failures. The version notes in CLAUDE.md describe the concrete incidents that motivated each structural change. If a section here references "vN.NN.0+" — that's where to find the incident report.
