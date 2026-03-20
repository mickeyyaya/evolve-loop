# Evolve Loop — Annotated Showcase

This is a walkthrough of a single evolve-loop cycle from start to finish, with every signal, exchange, and output annotated. The fictional scenario: a mid-sized TypeScript project where the loop is on cycle 8 using the `innovate` strategy.

By the end you will see how the Scout's bandit/novelty/crossover machinery selects a task, how Builder and Auditor communicate via mailbox, how the Builder writes its retrospective, how an instinct is extracted in Phase 5, and how the Operator composes a next-cycle brief and session narrative.

---

## Phase 1 — DISCOVER (Scout)

The Scout runs in `incremental` mode (cycle 2+). It reads `changedFiles` from `git diff HEAD~1 --name-only`, checks the operator brief from the previous cycle, scans the mailbox, and then selects tasks.

### Step 1 — Operator Brief (from previous cycle)

Before touching the codebase, Scout reads `workspace/next-cycle-brief.json`:

```json
{
  "cycle": 7,
  "weakestDimension": "novelty",
  "recommendedStrategy": "innovate",
  "taskTypeBoosts": ["feature", "refactor"],
  "avoidAreas": ["src/auth/", "agents/evolve-operator.md"],
  "rationale": "Last 3 cycles were quality/hardening work. novelty dimension scored 0.31 (floor: 0.40). Push exploratory features to rebalance fitness vector."
}
```

Scout overrides the ambient `balanced` strategy with `innovate`, boosts `feature` and `refactor` task types by +1 priority, and marks `src/auth/` as a no-touch zone — the same treatment given to stagnation patterns.

### Step 2 — Bandit Task Selection

The Scout maintains a multi-armed bandit over task types. After reading `evalHistory` from `state.json`:

```
banditArms:
  feature:   wins=14  pulls=18  ucb=0.91  ← highest UCB — selected
  bugfix:    wins=6   pulls=9   ucb=0.77
  security:  wins=3   pulls=4   ucb=0.71
  refactor:  wins=5   pulls=8   ucb=0.68
```

The UCB formula rewards both historical success rate and exploration of under-pulled arms. `feature` wins here: high win rate AND a `+1 taskTypeBoosts` signal from the operator brief.

### Step 3 — Novelty Scoring

Scout scores candidate tasks against a novelty index built from `changedFiles` over the last 5 cycles. Files touched often score low; untouched files score high.

```
Candidate: add-async-export-pipeline
  files:  src/export/          → novelty 0.94 (never touched)
  type:   feature               → bandit arm boost
  size:   M-complexity
  noveltyScore: 0.81

Candidate: fix-auth-token-refresh
  files:  src/auth/             → avoidAreas match — SKIP
  noveltyScore: N/A

Candidate: extract-shared-validators
  files:  src/utils/validators  → novelty 0.30 (touched cycle 5, 6)
  type:   refactor
  noveltyScore: 0.30
```

`add-async-export-pipeline` is selected as Task 1. Its novelty score is logged in the decision trace.

### Step 4 — Semantic Crossover

For Task 2, Scout applies crossover: it blends a pattern from a successful past task (`add-streaming-api`, cycle 4) with the current gap (`no progress indicator for long-running exports`). The merged idea becomes `add-export-progress-events` — a feature that takes the streaming backbone from cycle 4 and applies it to the export domain.

The scout-report records:

```yaml
decisionTrace:
  - task: add-async-export-pipeline
    selectionSignals:
      banditArm: feature
      ucbScore: 0.91
      noveltyScore: 0.81
      operatorBriefBoost: +1 (taskTypeBoosts: feature)
    rationale: "High-novelty untouched module + top bandit arm + operator brief alignment."

  - task: add-export-progress-events
    selectionSignals:
      banditArm: feature
      ucbScore: 0.91
      noveltyScore: 0.72
      crossoverSource: "cycle-4/add-streaming-api"
      crossoverTarget: "export progress signaling"
    rationale: "Semantic crossover: reuse streaming pattern from cycle 4 in export domain. Moderate novelty, strong precedent."
```

This decision trace is what the Novelty Critic and future Scout instances read when assessing whether the pipeline is exploring or stagnating.

---

## Phase 2 — BUILD (Builder)

Builder receives the task for `add-async-export-pipeline` and starts with a mailbox check.

### Agent Mailbox Exchange

`workspace/agent-mailbox.md` at the start of Phase 2:

```markdown
## Message #1
- from: scout
- to: builder
- cycle: 8
- re: add-async-export-pipeline

src/export/ has never been touched by the loop. No existing tests.
Recommend creating src/export/__tests__/ alongside implementation.
Blast radius: LOW — no other module imports from src/export/ yet.
```

Builder reads this, notes the missing test directory, and proceeds. After implementation it posts:

```markdown
## Message #2
- from: builder
- to: auditor
- cycle: 8
- re: add-async-export-pipeline

Implementation complete. Created 3 new files:
  src/export/asyncExport.ts        — async queue + worker pool
  src/export/exportEventEmitter.ts — typed EventEmitter wrapper
  src/export/__tests__/asyncExport.test.ts

No existing tests were modified. Coverage for src/export/: 87%.
Watch: asyncExport.ts uses a dynamic import() for the PDF renderer.
      Node < 18.12 does not support top-level await in ESM — pinned minimum in package.json.
```

### Builder Retrospective Notes

After shipping, Builder writes `workspace/builder-notes.md`:

```markdown
# Builder Notes — Cycle 8

## Task: add-async-export-pipeline

### File Fragility
- src/export/asyncExport.ts: Worker pool size is hardcoded at 4. If a future
  task adds configurable concurrency, this is the single source of truth to
  update — and it must also update the test mock in __tests__/asyncExport.test.ts.
- package.json: engines.node floor raised to >=18.12.0. Any future task touching
  Node version constraints must check this is still compatible with the CI matrix.

### Approach Surprises
- dynamic import() required a tsconfig.json module change from "CommonJS" to "ESNodeNext".
  This was not anticipated. The change is safe but could affect other modules that rely
  on require()-style imports — auditor should verify.

### Recommendations for Scout
- src/export/ is now established. Future tasks here can be M-complexity; the
  module boundary is clear.
- tsconfig.json is now a hotspot watch: two tasks in 2 cycles have touched it.
  Future scout should flag it as blast-radius: MEDIUM.
```

---

## Phase 3 — AUDIT (Auditor)

Auditor reads Builder's mailbox message, runs eval graders, and posts back:

```markdown
## Message #3
- from: auditor
- to: builder
- cycle: 8
- re: add-async-export-pipeline
- severity: WARN

tsconfig moduleResolution change accepted — verified no require() calls in affected paths.
One finding: exportEventEmitter.ts exports a default but the consumer uses a named import.
This will throw at runtime. Fix before SHIP.

Eval graders: 4/5 PASS. 1 FAIL: grep for "EventEmitter" in test file — 0 matches found.
```

Builder sees the WARN (iteration #2), fixes the named/default export mismatch, adds the missing test assertion, and posts:

```markdown
## Message #4
- from: builder
- to: auditor
- cycle: 8
- re: add-async-export-pipeline (retry)

Fixed: export changed to named. Test updated with EventEmitter assertion.
All 5 eval graders now pass locally.
```

Auditor re-runs graders, all pass. Audit result: PASS.

---

## Phase 5 — LEARN (Instinct Extraction)

After SHIP, the orchestrator runs instinct extraction. It reads build-report.md, builder-notes.md, and audit-report.md together and reasons about durable patterns.

### Extracted Instinct

```yaml
- id: inst-019
  pattern: "dynamic-import-requires-esm-tsconfig"
  description: >
    Using dynamic import() in TypeScript requires "module": "NodeNext" (or "ESNext")
    in tsconfig.json and "type": "module" in package.json. This silently fails at
    compile time when the project was previously CommonJS. Always check tsconfig.json
    module setting before using dynamic import(). Discovered in cycle 8 when
    asyncExport.ts required a module system change that affected the whole project.
  confidence: 0.6
  source: "cycle-8/add-async-export-pipeline"
  type: "convention"
  category: "semantic"
```

The instinct starts at confidence 0.6 (slightly above the default 0.5) because the Builder's surprise note provided a direct causal explanation — the extractor rewarded the specificity. Next time a task touches `import()` or `tsconfig.json`, Scout will surface this instinct and Builder will know to check `module` settings upfront.

---

## Operator Brief and Session Narrative

At the end of Phase 5, the Operator runs its health assessment and composes two outputs.

### Next-Cycle Brief (machine-readable)

Written to `workspace/next-cycle-brief.json` for Scout to read next cycle:

```json
{
  "cycle": 8,
  "weakestDimension": "quality",
  "recommendedStrategy": "harden",
  "taskTypeBoosts": ["bugfix", "test"],
  "avoidAreas": ["tsconfig.json", "package.json"],
  "rationale": "Two novelty-forward features shipped. quality dimension dipped to 0.44 (floor: 0.45) — one audit iteration required. Suggest a harden cycle to consolidate coverage before the next feature push."
}
```

The Scout for cycle 9 will read this before scanning the codebase, override its strategy to `harden`, and apply a `+1` boost to `bugfix` and `test` tasks. The feedback loop is closed: Operator fitness data flows programmatically into Scout's task selection.

### Session Narrative (human-readable)

Appended to `workspace/operator-log.md`:

```markdown
## Session Narrative

Cycle 8 explored src/export/ for the first time — a module that had been untouched
across all prior cycles. The Scout's novelty scorer and bandit arm converged on the
same choice: a feature task in genuinely new territory. The implementation required
an unexpected TypeScript module system change (CommonJS to NodeNext), which the
Auditor caught via a named/default export mismatch before ship. One retry resolved
it cleanly.

The surprising moment was tsconfig.json becoming a blast-radius concern mid-build —
something neither Scout nor Builder anticipated because the module system had never
been relevant before. The extracted instinct (inst-019) captures this for future cycles.

The loop learned something structural this cycle, not just incremental. Quality dipped
slightly as a result of the exploratory push, which is the expected tradeoff when
novelty is the weakest MAP-Elites dimension. Cycle 9 will harden the gains.
```

This narrative is the "what just happened" layer that makes a multi-cycle run legible to a newcomer reading the logs. It synthesizes the clinical fitness vectors into a story with cause, effect, and forward intent.

---

## The Full Picture

Here is how each feature contributed to this one cycle:

| Feature | Role in this cycle |
|---|---|
| Bandit task selection | Ranked `feature` arm first; shaped Task 1 and Task 2 picks |
| Novelty scorer | Identified `src/export/` as high-novelty territory |
| Semantic crossover | Generated Task 2 by blending cycle 4 streaming pattern with export domain |
| Decision trace | Logged all three signals per task for Novelty Critic and future Scouts |
| Operator next-cycle brief | Read at Phase 1 start; overrode strategy and boosted task types |
| Agent mailbox | Carried Scout hints to Builder; Builder findings to Auditor; Auditor WARN back to Builder |
| Builder retrospective | Documented tsconfig fragility and the dynamic-import surprise |
| Instinct extraction | Captured `dynamic-import-requires-esm-tsconfig` at confidence 0.6 |
| Session narrative | Synthesized the cycle's story for human readers |
| MAP-Elites fitness | Detected quality dip; fed `weakestDimension` into next-cycle brief |

Each feature is independently useful. Together they form a closed loop: Scout selects with data, Builder builds with context, Auditor gates with precision, and Operator feeds the findings back to Scout for the next cycle.

---

## Running Your Own Cycle

```bash
# Start autonomous mode
/evolve-loop

# Goal-directed innovation push (like this cycle)
/evolve-loop 3 innovate add async export support

# After a cycle like this one, harden automatically
/evolve-loop 1 harden
```

See [architecture.md](docs/architecture.md) for a deep dive into data flow, and [instincts.md](docs/instincts.md) for the instinct extraction and graduation system.

---

## Writing Domain Walkthrough

The evolve-loop is not limited to code. Here's how the same pipeline works for a **writing project** — a technical blog maintained as a collection of Markdown articles.

### Setup: domain.json

```json
{
  "domain": "writing",
  "evalMode": "rubric",
  "shipMechanism": "file-save",
  "buildIsolation": "file-copy"
}
```

This tells the orchestrator: use LLM rubric grading instead of bash exit codes, save files instead of git commit/push, and copy files for isolation instead of git worktrees.

### Scout discovers a task

The Scout scans the writing project and finds:
- `articles/api-design.md` has a "T-O-D-O: add rate limiting section" placeholder
- No article covers the new v2 API endpoints announced last month

It selects: **"Add rate limiting section to api-design.md"** (S-complexity, feature).

### Eval definition: Rubric Grader (not bash)

Instead of `grep -q "rate limit" articles/api-design.md`, the eval uses a rubric:

```markdown
# Eval: add-rate-limiting-section

## Rubric Grader
- Criterion: "Completeness — does the section cover rate limit headers, quotas, and retry guidance?"
  - 0: Section missing or placeholder only
  - 25: Mentions rate limiting but missing key details
  - 50: Covers basics but missing retry guidance
  - 75: Covers headers, quotas, and retry — minor gaps
  - 100: Comprehensive coverage with examples
- Model: tier-3
- Threshold: average score >= 60 to pass

## Coverage Check
- Required: ["Rate Limit Headers", "Quota Tiers", "Retry-After"]
- Threshold: 100%
```

### Builder implements

The Builder copies `articles/api-design.md` to a temp directory (file-copy isolation — no git worktree needed), writes the new section, and self-verifies against the rubric.

### Ship mechanism: file-save (not git)

Instead of `git commit && git push`, the orchestrator:
1. Saves the updated file in place
2. Logs the change in the ledger
3. No git operations needed — the project may not even have a `.git` directory

### What stays the same

Everything else is identical to the coding walkthrough above:
- Instinct extraction still runs in Phase 5
- The Operator still checks for stalls and quality trends
- Bandit task selection still tracks which task types succeed
- The benchmark eval still scores project quality (dimensions may differ)

The domain adapter swaps only the 4 touch points. The learning loop itself is universal.

---

## Research Domain Walkthrough

Here's how the evolve-loop works for a **research project** — a collection of findings documents with cited sources investigating a technical topic.

### Setup: domain.json

```json
{
  "domain": "research",
  "evalMode": "hybrid",
  "shipMechanism": "file-save",
  "buildIsolation": "file-copy"
}
```

Research uses `hybrid` eval mode: bash checks for structural requirements + LLM groundedness checks for factual accuracy. File-copy isolation since there's no git repository.

### Scout discovers a task

The Scout scans the research project and finds:
- `findings/llm-scaling.md` has 5 claims but only 2 cite sources
- The research question "What are the cost tradeoffs of multi-agent vs single-agent?" has no findings document

It selects: **"Add source citations to llm-scaling.md"** (S-complexity, stability).

### Eval definition: Groundedness + Coverage (not bash)

```markdown
# Eval: add-scaling-citations

## Groundedness Check
- Input: findings/llm-scaling.md + sources in references/
- Model: tier-2
- Check: "For each factual claim, identify the supporting source. Flag claims with no source."
- Threshold: >= 80% of claims grounded to pass

## Coverage Check
- Required: ["Parameter scaling laws", "Training compute costs", "Inference latency"]
- Threshold: 100%

## Code Graders (structural checks)
- `grep -c '\[.*\]' findings/llm-scaling.md | awk '{exit ($1 < 4)}'` → at least 4 citations
```

The hybrid approach: bash verifies citation count (deterministic), LLM verifies citation accuracy (requires reasoning).

### Build isolation: file-copy (not worktree)

```bash
COPY_DIR=$(mktemp -d)/evolve-build-cycle-8-add-scaling-citations
cp -rp . "$COPY_DIR" && rm -rf "$COPY_DIR/.evolve"
# Builder works in $COPY_DIR, edits findings/llm-scaling.md
# After pass, diff and apply changes back to main directory
```

No git operations needed. The Builder works on an isolated copy, and changes are merged back via file diff.

### Ship mechanism: file-save

```json
{"ts":"2026-03-19T10:00:00Z","cycle":8,"role":"orchestrator","type":"ship","data":{"mechanism":"file-save","files":["findings/llm-scaling.md"]}}
```

The updated file is saved in place. A backup goes to `.evolve/history/cycle-8/output/`. No git commit, no push — the research project lives as local files.

### Benchmark dimensions: research-specific

Instead of "Modularity" and "Convention Adherence", the benchmark evaluates:
- **Claim Accuracy** — are factual claims supported by cited sources?
- **Source Coverage** — are all research questions addressed with findings?
- **Methodology Rigor** — is the research approach systematic and reproducible?
- **Citation Hygiene** — are citations formatted consistently with accessible sources?

### What stays the same

The research walkthrough shares 100% of the pipeline infrastructure with coding:
- Scout discovers tasks and writes eval definitions
- Builder implements in an isolated workspace
- Auditor runs evals (different grader types, same pass/fail gate)
- Operator monitors health, detects stalls, recommends strategy changes
- Instincts are extracted and consolidated identically
- Bandit arms track which research task types succeed
