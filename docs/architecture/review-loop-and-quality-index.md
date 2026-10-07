# Review loop and quality index

> ADR-0124 (amended 2026-10-07) · plan: [code-review-phase-2026-10.md](../plans/code-review-phase-2026-10.md).
> Written in the Issue / Gap / Solution shape.
>
> **Operator directives (2026-10-07):**
> - *"code/output simplifier should be done within the build related phases, there should be review phase done after build and before audit to provide the suggestion and feedback to build phase agent to make the deliverables more robust and more efficient and effective. The review phase should include all different review skills, e.g. architecture review, performance, debugability, concurrent, ...etc. The goal of review should continue to provide the feedback to build phase agent until all the concerns and defects are resolved. Then the deliverables will send to the audit phase final evaluation. The audit should focus on evaluating from different perspecitives shared with review phase agent to learn if the reviewed and fixed output is qualified for all index score."*
> - *"the review phase agent should justify what reviews are required for the changes done by build phase agent. It should also read the previous agent output/deliverables if needed to help reasoning what review needs to be done."*
>
> **Terms used throughout:**
> - **Quality index:** the fixed set of ten dimensions every change is judged on (§2).
> - **Dimension:** one axis of the index, scored 1–5 or N/A.
> - **Review Plan:** the reviewer's per-dimension statement of what to review, how deeply, and why (§3).
> - **Finding:** one reviewed defect with a suggested fix (§4).
> - **Round:** one completed `code-review` dispatch. Round 1 reviews the whole diff; later rounds are delta re-reviews.
> - **Disposition:** the build's answer to one finding id: FIXED with evidence, or DEFERRED with a reason.
> - **Threshold severity:** `workflow.findings_repair.code-review.threshold` (default MEDIUM). Findings at or above it follow the strict rules (§5.2, §5.6); findings below it follow the LOW rules.
> - **Unresolved mass:** the single number the loop must drive down (§5.3).
> - **Judge:** a phase whose findings can earn a repair round: `audit` and `code-review`.
> - **Review digest:** the kernel-written summary of the review that the audit reads instead of the raw review report (§6.1).
> - **Correction rung:** the deliverable gate's contract-correction ladder. A report that fails its contract is re-dispatched with the violation list, a bounded number of times, before the phase FAILs.
> - **Phase A / phase B:** phase A is the lead reviewer with the full index; phase B adds data-driven specialists (§8).
> - **Shadow / enforce:** the rollout stages (§9).
> - **C0 / C0c:** the staged agent-config lane (`dev/cl-agentcfg`). C0 preloads `engineering-craft` and `code-review-simplify` into every source-writing dispatch; C0c is its self-review part, the builder's and TDD engineer's `## Self-Review` block.
> - **L2:** the routing lane (`dev/cl-routingl2`), which owns the CLI routing table.
> - **Console:** the operator's interactive session that designs and lands lanes. In the decision tables, "console" means the console proposed the decision and the operator approved it with the plan.
>
> Paths are relative to `go/internal/` unless they start with a top-level directory.

## Issue

The audit was the loop's only code reviewer and also its ship gate. As a result:

- **The builder heard about defects too late.** A defect reached the builder only after the audit rejected the work, as verdict prose recovered through the retry envelope (ADR-0093/0096). The ADR-0096 ship-rate study shows repair converges poorly once feedback arrives as a verdict.
- **One judgment carried every lens.** A single judgment had to cover correctness, architecture, tests, performance, concurrency, debuggability and security. The auditor persona grew to 23,956 bytes, against an 18,102-byte ACS baseline.
- **The best review in the system ran outside the pipeline.** The console's review chain (author self-pass, simplifier, architecture and Go reviewers, fix round, delta re-review) found real defects on every lane reviewed on 2026-10-06/07:
  - a rule living in four places that disagreed;
  - an ownership check applied at one of its two sites;
  - an exemption stated twice;
  - surviving mutants on changed lines.

  None of these were visible at design time, and each needed more than one round to converge.
- **The ten single-lens evaluate phases fired only on advisor judgment.** perf-profile, race-condition-scan, error-handling-scan and the others are inserted ad hoc, so most code cycles got none of them.

## Gap

1. **No independent review between build and audit.** The author (C0c's self-simplify and self-review) and the gate (audit) were the only voices.
2. **No loop.** Nothing returned concrete, suggested fixes to the builder and checked them again before the gate.
3. **No shared definition of "qualified".** The reviewer's lenses, the audit's checklist and the lens phases each scored on their own vocabulary, so the gate could not ask "is every dimension good enough?" and review could not aim at the gate's bar.
4. **Review scope was never justified.** Nobody recorded which lenses a change needs and why, so coverage could silently drift between "everything, shallowly" and "whatever the advisor thought of".

## Solution

```
                     ┌───────────────── fix round n (build, seeded brief) ◀──────────────┐
                     ▼                                                                    │ granted
build ──▶ code-review round n ──▶ decide ──┬─ granted (enforce only) ──────────────────────┘
          (Review Plan, Findings,          │
           Scores; Disposition Review      ├─ resolved ───────────────┐
           from round 2)                   ├─ stop: budget |          ├──▶ other evaluate phases ──▶ audit ──▶ qualify ──┬─ ship
                                           │   no-progress |          │                              (own Scores,        │
                                           │   oscillation |          │                               adjudication)      └─ audit-repair
                                           │   ledger fault ──────────┤                                                     (existing path; a
                                           └─ shadow (decision        │                                                      build re-entry may
                                               signalled, nothing ────┘                                                      run a delta round)
                                               granted)
```

The pieces:

- **The quality index (§2)** is the contract that both the review loop and the audit use.
- **`code-review` (§3–§4)** reviews the build's diff. It plans its review per dimension, reports findings with suggested fixes, and scores the full index.
- **The loop (§5)** returns findings to build until they are resolved, with deterministic stops so the cost is bounded.
- **The audit (§6)** scores the same index independently and decides qualification. The kernel cross-checks that decision as logic, not format.

### 1. Strategy (why this shape)

Three strategies were weighed. The full table is in the plan §2.

| | S1: one multi-lens reviewer | S2: lens panel every round | **S3 (chosen): one shared index + a lead reviewer + triggered specialists** |
|---|---|---|---|
| Coverage | every lens, shallow | every lens, deep | every dimension through the lead; depth where the diff triggers it |
| Cost per round | 1 deep dispatch | about 8 dispatches | 1 deep dispatch + 0–3 triggered |
| Conflicting advice | reasoned together | the lenses contradict each other with no arbiter | the lead arbitrates |
| Shared with the audit | needs a rubric | needs a rubric | the rubric is the design |

**S3 ships in two phases:**
- **Phase A** is the lead reviewer with the full index.
- **Phase B** adds specialists only where the audit keeps out-scoring the reviewer on a dimension, which shows the lead is too lenient there (§8).

### 2. The quality index

There is one home for the index, and both the loop and the audit read it:
- the skill `skills/quality-index/SKILL.md` (the bars, the N/A rules, the review procedure per dimension and the grammars), plus `COMPACT.md`, the projection the skill overlay injects;
- the Go package `qualityindex`: the vocabulary (`Keys`, `NeverNA`), the threshold resolver (`ResolveThresholds`), the parsers (`ParseScores`, `ParsePlan`), the agreement check (`Agree`) and `Qualifies`.

Both are protected surfaces. A drift test pins the skill's and COMPACT's dimension lists and never-N/A set to `qualityindex.Keys()` and `qualityindex.NeverNA`.

#### 2.1 Dimensions

| Key | Judges | Score 4 (the qualifying bar) | Pulls to ≤3 | N/A allowed when | Review procedure (skill) |
|---|---|---|---|---|---|
| `correctness` | Acceptance met; logic and edge cases | Every acceptance criterion is proven by a test that fails without the change; no known logic defect | An unproven criterion; an unhandled edge case that is reachable | never | the Task Contract's acceptance against the tests; `code-review-simplify` logic pass |
| `architecture` | One home per rule, coupling, boundaries, patterns with their forces | No duplicated belief introduced; dependencies point inward; every new seam or pattern names its force | A second copy of a rule; a leaked layer; a pattern with no force; a function over cap that was touched and did not shrink | never | `architecture-review` rubric |
| `maintainability` | Clean-code limits, naming, dead code, comments | Functions ≤50 lines, ≤3 params where natural, nesting ≤4, files <800; zero non-machine comments; no dead or unused exports | A breached limit; an added comment; a flag argument; dead code | never | `code-review-simplify` Simplification Catalog; `engineering-craft` clean-code limits |
| `test-quality` | Behaviour over surface, red-first, mutation, determinism | Red-first evidence for new behaviour; changed lines have no surviving non-equivalent mutant; deterministic; no mocks of the project's own logic | A surviving mutant; an assertion-free or mock-only test; a sleep- or order-dependent test | never | `golang-test-review` checklist (Go), `tdd-craft` §Reviewing for TDD, `go test -overlay` mutants |
| `robustness` | Error handling, loud failures, recovery rungs, validation at boundaries | No swallowed error; process failures get a recovery rung; inputs are validated at trust boundaries | A swallowed error; a silent degrade; a block on a format-only issue | the diff is pure documentation or data with no code path | the error-handling-scan checklist, folded into quality-index |
| `concurrency` | Races, leaks, lock ordering, context propagation | `-race` clean on the touched packages; every goroutine has an owner and a stop; the context is honoured | A race; a leaked goroutine; a lock-order hazard; an ignored cancellation | the diff touches no goroutine, channel, mutex, atomic, shared mutable state or context-cancellation path | `-race` on the touched packages plus the race-condition-scan checklist, folded into quality-index |
| `performance` | Algorithmic cost, allocations, I/O, hot paths, prompt and token cost of agent-facing text | No new superlinear work on a hot path; no I/O in a loop that can be batched; prompt bytes justified | A measurable regression; an unbounded read; repeated whole-file passes | the diff changes no executed path and no agent-facing text | the perf-profile checklist (and a benchmark at deep), folded into quality-index |
| `debuggability` | Signals, error context, logs, diagnosability | A failure names its cause and inputs; new behaviour emits a registered signal or log where an operator would look | An error with no context; a silent state change; a log with no fields | the diff changes no runtime behaviour | the telemetry-coverage-check checklist, folded into quality-index |
| `security` | Input handling, injection, secrets, sandbox and fences, protected surfaces | No new injection path; no secret in code or logs; fences and the protected manifest are respected | A path traversal; command injection; an unvalidated external input; a fence bypass | the diff touches no trust boundary, external input, credential, subprocess or fence | `security-review-scored` and the security-scan checklist |
| `docs-consistency` | Docs as the primary asset; config, not code; no flags | The package docs, CHANGELOG and plan status are updated; behaviour lives in config where policy says so; no feature flag | A stale doc; a flag; a policy literal in code | never | the docs-floor rules (`docsfloor`), the code-comments convention |

- **Score 5:** exemplary; nothing to improve within scope.
- **Score 2:** a defect a user would hit.
- **Score 1:** harmful or absent (for example, a data-loss path or no tests at all).
- **Never N/A:** exactly `correctness`, `architecture`, `maintainability`, `test-quality` and `docs-consistency` (`qualityindex.NeverNA`). The parser rejects N/A on them.
- **Scope of scoring.** The index applies to a code cycle whose build touched files, the same predicate that pins `code-review` (`deliverable_kind==code && build.files_touched>0`). The audit writes `## Scores`, and the kernel cross-checks it, only on such a cycle (§6.2).

#### 2.2 The Scores grammar

Every review round and every audit writes a `## Scores` section with exactly one line per dimension:

```markdown
## Scores
- correctness: 4 — every acceptance criterion pinned by a red-first test (test-report.md; TestX, TestY)
- concurrency: N/A — the diff adds no goroutine, channel, mutex, atomic or shared state (git diff grep: 0 hits)
- performance: 3 — CR2: readAll on a 47 MB file per call at ledger.go:120; batchable
```

**Parse rules** (`qualityindex.ParseScores`, on the visible lines of `reportdoc.Section`):
- a scored line is `- <key>: <value> <sep> <rationale>`;
- the key is one of §2.1's keys;
- the value is an integer from 1 to 5, or `N/A` (any letter case);
- `<sep>` is an em dash, an en dash or a hyphen with a space on each side (` — `, ` – `, ` - `), split at the first one, as `reportdoc`'s finding headings tolerate;
- the rationale (or N/A reason) must not be empty;
- a score below its dimension's threshold cites at least one finding id (`CR<n>`) whose `Dimension` is that dimension, so a gap always reaches the builder as an actionable finding (checked by the report grammar, §3).

**A malformed section** goes to the correction rung (format, never logic). Malformed means any of:
- a missing dimension;
- a duplicate line;
- an unknown key;
- a never-N/A dimension marked N/A;
- an empty rationale;
- a score out of range;
- a gap that cites no finding on its dimension.

#### 2.3 Qualification

```text
Qualifies(scores, thresholds) -> (ok bool, gaps []Gap)
  for each dimension d in the index:
    s := scores[d]
    case s missing                 -> gaps += {d, reason: "missing"}
    case s == N/A and d.neverNA    -> gaps += {d, reason: "N/A not allowed"}
    case s == N/A                  -> continue
    case s.value < thresholds[d]   -> gaps += {d, score: s.value, threshold: thresholds[d]}
  ok := len(gaps) == 0
```

- **Thresholds** come from `workflow.quality_index.thresholds` (`qualityindex.ResolveThresholds`). The compiled default is **4 for every dimension** (operator Q-D3).
  - `"*"` sets every dimension; a per-key entry then overrides it.
  - A value must be an integer from 1 to 5. An out-of-range value or an unknown key is ignored, with a config warning, and the default stands.
- **One Specification.** `Qualifies` is the only qualification test: the review loop's "resolved" predicate (§5.2), the audit cross-check (§6.2) and the shadow metrics (`REVIEW_FINDINGS.gaps`) all call it.

### 3. The Review Plan: justified scope

**Every round opens with a Review Plan.** This answers the operator's second directive.

**What the reviewer reads.** It reads the build's diff and, where its reasoning needs them, the earlier phases' deliverables. They reach it two ways:
- **In the prompt:** the Task Contract block (the bound inbox item's acceptance and the ACS predicate inventory). `code-review`'s `phase.json` asks for it with `prompt_context: ["goal", "task_contract"]`. The goal is listed because a declared list replaces the evaluate default `["goal"]`. The kernel seeds the block for any phase whose spec requests it, beside the built-in tdd, build and audit.
- **In the workspace** (listed in `phase.json` `inputs.files`, which documents and does not gate):
  - `intent.md` (goal, non-goals, risks), when the intent phase ran;
  - `scout-report.md` (the selected task and its notes);
  - `triage-report.md` and `triage-decision.json`;
  - `test-report.md` (what TDD covered);
  - `build-report.md`, including the builder's own `## Self-Review` (C0c);
  - the cycle's explanation document, at the path `build-report.md`'s `## Explanation Documentation` names, when the explanation contract is active;
  - the eval file `.evolve/evals/<slug>.md` the scout report names;
  - on later rounds, the review digest's history (the previous findings, dispositions and Disposition Reviews; §5.6);
  - on an audit-repair re-entry, the audit's findings in `audit-report.md`.

**Every claim in `build-report.md` is a claim to verify, never evidence.**

```markdown
## Review Plan
- Inputs read: intent.md, scout-report.md#Selected Tasks, test-report.md, build-report.md#Changed Areas, go/internal/x (diff)
- correctness: required (standard) — 3 acceptance criteria in the inbox item; TDD pinned 2 (test-report.md), the third (refusal path) is unpinned
- concurrency: required (deep) — the diff adds a goroutine in usageprobe/evidence.go:96 and a shared cache map
- security: N/A — no trust boundary, external input, subprocess or credential in the diff (git diff --stat: 4 files, all internal/)
- …one line per dimension…
```

**The plan grammar** (`qualityindex.ParsePlan`) has its own line forms; it shares only the key vocabulary and the separator with Scores:
- `- Inputs read: <list>` is **required** and non-empty;
- a required line is `- <key>: required (light|standard|deep) <sep> <justification>`;
- an N/A line is `- <key>: N/A <sep> <reason>`.

**Rules:**
- **Coverage.** Every dimension appears exactly once, and an unknown key is malformed.
- **Depth** names what the round runs:
  - `light`: read and reason;
  - `standard`: also run the touched packages' tests;
  - `deep`: also run probes, `go test -overlay` mutants, `-race` or benchmarks.
- **N/A lines** give a concrete reason tied to the diff, and a never-N/A dimension is never N/A.
- **Scores must agree with the plan** (`qualityindex.Agree`). A dimension the plan marks N/A must also be N/A in Scores, and the reverse.
- **Depth follows risk.** Deep is expected where the diff touches the dimension's risk surface, as in the concurrency line in the example above.
- **The audit may contest an N/A** by scoring that dimension anyway (§6.3).

**Who checks.** The report grammar `code-review-report` (`codereview.ValidateReport`) runs in the deliverable gate, so `evolve phase verify code-review` and the runner's gate both apply it. `phase.json` declares it with `classify.grammars: ["code-review-report"]`, and the deliverable gate dispatches each declared grammar to its registered check. The check:
- parses the Review Plan and the Scores;
- checks their agreement;
- checks that every gap cites a finding on its dimension, against the thresholds the project's policy resolves;
- reports each violation as one `bad_grammar` violation, which the correction rung hands back to the reviewer.

**The report sections** (`phase.json` `classify.require_sections`): `## Review Plan`, `## Findings`, `## Scores` and `## Verdict` are required, and the verdict sentinel ends the report. `## Architecture Review` (architecture-review's three-book audit) is optional. From round 2, `## Disposition Review` is required (Q4). The morning's `## Scope` is replaced by the Review Plan, whose `Inputs read` line and per-dimension depth state the scope.

### 4. Findings and the ledger

#### 4.1 Finding grammar (in `## Findings`)

```markdown
### CR3 (HIGH) — the single-flight leader's cancellation poisons the cache
- Dimension: concurrency
- Location: go/internal/usageprobe/evidence.go:103
- Scenario: the leader's ctx is cancelled mid-query → the cancelled read is cached and served to followers for the TTL
- Evidence: TestProbe_LeaderCancel… fails: follower got "context canceled" (go test -overlay, workspace/probes.txt)
- Fix: run the query on context.WithoutCancel(ctx) bounded by Timeout; never land a read whose error is the caller's ctx.Err()
```

- **Severity** is one of CRITICAL, HIGH, MEDIUM or LOW.
- **Dimension** is a §2.1 key. The morning grammar's `Category` is gone: `correctness` stays, `architecture` stays, `test-quality` stays, and `clean-code` becomes `maintainability`.
- **Fix** is a concrete patch sketch, or for a test gap the test name plus its assertion. A finding without a fix, or with an unknown dimension, is still recorded as written, never dropped (§7), and the persona requires both.
- **Each field is one line.** A longer patch goes in a fenced block below the fields; the row records the `Fix:` line, and the full report stays in the workspace.
- **Parsing** goes through `codereview.Parse` (on `reportdoc.Section`, `Findings` and `Fields`).

#### 4.2 Projection to ledger rows

Each finding becomes a row in `defect-ledger.json`, merged by `defectledger.Append`, the one merge rule the audit's Emit also uses. Dedupe, the 64-row cap and the stand-in row for a cut are each kept per source, so a review row never dedupes, cuts or swallows an audit row:

| Field | Value |
|---|---|
| `id` | `defectledger.ID("code-review" + "\xff" + text)`, the content hash of the source-qualified text; the `\xff` byte cannot appear in a JSON-decoded defect, so an audit row (whose id stays `ID(text)`) never collides with it |
| `text` | `[SEV] dimension location — title \| scenario: … \| evidence: … \| fix: …` (capped by `TextMaxRunes`; the report's positional `CR<n>` is left out) |
| `status` | `OPEN` in enforce; `DEFERRED` (reason = `codereview.ShadowReason`) in shadow |
| `source` | `code-review` (empty means the audit) |
| `round` | the round that raised it |
| `severity` | the finding's severity |
| `dimension` | the finding's dimension (additive, `omitempty`) |

- **The stand-in row for a cut** takes the cut rows' status, reason, source and round, and the highest severity among them, so a cut never hides a CRITICAL, and in shadow never mints an OPEN row. Its text starts with `code-review: `, so it is never the audit's stand-in. The fix brief names how many findings were cut (§5.5).
- **A shadow row is never a lineage.** The reconcile gate's empty-ancestor check and its vouch count only the rows a continuation owes: audit rows, and OPEN rows of any source. An ancestor holding only DEFERRED shadow rows is the empty ancestor, exactly as if the review had not run.
- **Provenance survives inheritance.** When a continuation inherits a code-review row, the reconcile gate carries `source`, `round`, `severity` and `dimension` through, whatever the graded status. An inherited row enters the continuation's audit as an inherited defect (the existing gate). It does not enter the continuation's review round 1, which reviews only the continuation's own diff.

### 5. The loop until resolved

#### 5.1 Per-round state (cycle state, persisted on every path)

```go
type ReviewRepair struct {
    Unresolved        []int             // unresolved mass after each round (§5.3)
    Reopens           map[string]int    // id → times a FIXED claim on it was rejected
    RejectedDeferrals map[string]int    // id → times a DEFERRED reason on it was rejected
    LastScores        map[string]string // dimension → "N" | "N/A", the latest review vector
    TreeAt            map[int]string    // round → worktree checkpoint ref recorded at its dispatch
    Hashes            map[string]string // artifact → sha256 recorded after the latest round (§5.7)
    ProgressBaseline  int               // index into Unresolved the next progress test compares against
    PendingFixRound   bool              // set before the fix build dispatch, cleared by its completion
    Stop              string            // "" | resolved | budget | no-progress | oscillation | ledger
}
```

- **The JSON field** on `CycleState` is `review_repair,omitempty`, so an old checkpoint parses unchanged.
- **The round count has one home:** `codereview.RoundFrom(cs.CompletedPhases)`. The state keeps no `Rounds` counter.
- **Persistence on every path.** The state is written with the cycle state before any dispatch the decision schedules, on the grant, on every stop and in shadow. A resume reads `PendingFixRound`: when it is set, resume dispatches the fix build and never re-decides.
- **The audit-repair counter** (`AuditRepairAttempts`) is unchanged. The generalized repair reads each judge's budget through one accessor, so the audit's behaviour stays byte-identical (golden).

#### 5.2 Resolved (operator Q-D2)

```text
strict(r) := rank(r.severity) ≤ rank(cfg.threshold)        (CRITICAL < HIGH < MEDIUM < LOW in rank order)

resolved(rows, scores) :=
     for every code-review row r:
         strict(r)   ⇒  r.status ∈ {FIXED, DEFERRED, DISPUTED}
         ¬strict(r)  ⇒  r.status ∈ {FIXED, DEFERRED}
  ∧  Qualifies(scores, thresholds).ok
```

**Statuses after a delta round reflect the reviewer's verdict on each claim (§5.6):**
- An ACCEPTed FIXED or DEFERRED claim keeps its status.
- A REJECTed claim on a strict row reverts to OPEN.
- A claim on a non-strict row is never rejected: any well-formed disposition resolves it.
- DISPUTED rows are resolved for the loop's purposes; the audit adjudicates them (§6.4).
- An *unverified* claim (the Disposition Review was exhausted at the correction rung, §5.6) keeps its status for the loop and is listed in the digest for the audit to verify.

#### 5.3 Unresolved mass and the progress rule (operator Q-D1)

```text
weight(CRITICAL)=8  weight(HIGH)=4  weight(MEDIUM)=2  weight(LOW)=1
findingMass  := Σ weight(r.severity) over code-review rows with status OPEN
gapMass      := Σ (threshold[d] − score[d]) over dimensions with a numeric score below threshold
U(n)         := findingMass + gapMass                       (after round n)
progress(n)  := n is the first round since the baseline  or  U(n) < U(baseline-previous)   (strict decrease)
```

- **Order.** U(n) is computed after round n's Disposition Review is applied (§5.6a) and its new rows are appended.
- **LOW findings count while OPEN**, because they must be dispositioned.
- **N/A dimensions add no gap mass.**
- **New findings raised on the fix diff add to U(n).** A fix that trades one HIGH for a new HIGH is therefore not progress.
- **Late findings are bounded.** Round n ≥ 2 raises findings only on the fix diff (the tree since round n−1's checkpoint, §5.1 `TreeAt`). The exception is a CRITICAL or HIGH defect in the original diff, marked `late` in its title, which also counts. A verbatim re-raise of an existing row is deduplicated by `Append` and has no effect; a reopen must go through a REJECT.
- **Every gap cites a finding** (§2.2), so U = 0 with `resolved` false cannot happen.

#### 5.4 The decision after each round (deterministic; no LLM envelope)

```text
decideAfterReview(cs, rows, report, cfg):
  st := cs.ReviewRepair ; n := codereview.RoundFrom(cs.CompletedPhases)
  if the round's rows were not recorded (ledger fault):
      enforce: st.Stop = ledger; REVIEW_SKIPPED{reason: ledger} → continue
      shadow:  signal, nothing more                              → continue
  view := rows                                       ; in shadow, rows whose reason is ShadowReason are read as OPEN
  apply the Disposition Review to view (§5.6a)       ; enforce persists it, shadow only computes it
  st.Unresolved += U(view, report.Scores) ; st.LastScores = report.Scores
  decision :=
      resolved(view, report.Scores)                                   → resolved
      n >= cfg.max_rounds                                             → budget
      !progress(n)                                                    → no-progress
      any id with st.Reopens[id] >= 2 and status(id) == OPEN          → oscillation
      otherwise                                                       → granted
  REVIEW_ROUND{round: n, decision, stage, unresolved, finding_mass, gap_mass, progress}
  if stage == shadow:   would_repair := decision == granted ; persist(st) ; grant nothing ; st.Stop stays "" → continue
  if decision == granted: st.PendingFixRound = true ; persist(st) ; REVIEW_REPAIR_GRANTED → build (fix round n)
  else:                   st.Stop = decision ; persist(st) ; REVIEW_RESOLVED or REVIEW_UNRESOLVED{reason} → continue
```

- **"Continue"** means the remaining evaluate phases in the plan, then the audit.
- **`max_rounds` counts reviews.** With the default of 4 there are at most 4 reviews and 3 fix builds per cycle, across every audit-repair re-entry. `max_rounds = 1` means review-only. The config requires `max_rounds ≥ 1` (an out-of-range value is ignored with a warning).
- **Shadow computes the full decision** as if enforced, over the rows read as OPEN, and signals it. It grants nothing and leaves `Stop` empty, so the metrics measure exactly what enforce would do.
- **Progress across a re-entry.** At an audit-repair re-entry the kernel resets the progress baseline (`ProgressBaseline` = the next index), so the first round after it passes the progress test. Without the reset, a round that ended resolved at U = 0 would make every later round fail the strict-decrease test.
- **Oscillation** tests only ids whose status is OPEN now: an id that was reopened twice and later accepted no longer stops the loop.
- **Routing (the backward edges).**
  - On a grant, `decideAfterReview` sets `cr.scheduledNext = build`. The completed fix build sets `cr.scheduledNext = code-review` (round n+1), which bypasses the `conditional_mandatory` pin.
  - Both edges, `code-review → build` and `build → code-review`, join `decisionOnlyEdge` (`audit_fail_decision.go`), so the routing advisor never proposes them, and they join the registry's `legal_successors`, so `sm.CanTransition` allows them.
  - After an audit-repair build re-entry, the kernel (not the pin) schedules `code-review` when `RoundFrom < max_rounds`. Otherwise it emits `REVIEW_BUDGET_SPENT` and schedules the next phase in the plan, and the digest marks the build's new FIXED claims unverified so the audit verifies them.
  - The resume path schedules from the persisted state (`PendingFixRound`), never from a re-decision.
- **One generalized path:** the decision lives beside `decideAfterAuditFail` in the generalized findings repair, keyed by judge.
- **No new remediation route:** `maybeRemediate` stays deterministic-gates-only (`remediationDenied`). The review loop is a findings-driven repair, never a remediation of a judgment verdict.

#### 5.5 The fix-round brief (build re-dispatch)

`composeReviewRepairBrief(cs)` composes the brief in `composeRepairBrief`'s shape, bounded by its byte budget, and the kernel seeds it into the build dispatch under the context key `CtxKeyReviewRepairFindings` (`review_repair_findings`):

1. **Unresolved findings,** ordered by severity, then dimension. Each carries:
   - id, severity, dimension and location;
   - the scenario and evidence;
   - the **suggested fix**.
2. **Rejected claims,** with the reviewer's reason (§5.6).
3. **Dimension gaps:** dimension, score, threshold, the reviewer's rationale and the finding ids it cites.
4. **The disposition obligation,** with the canonical schema example (`defectledger.DispositionsSchemaExample`).
5. **The scope rule** (from C0c's review: the builder's self-review touched TDD-owned tests):
   - fix through the build's own files;
   - tests that a test-gap finding asks for are the builder's own new tests;
   - TDD-authored tests and `go/acs/**` predicates are read, never edited.

- **Overflow** is summarized with a count, including the findings the ledger's cap cut (§4.2), so a long tail never truncates silently.
- **Precedence with the audit brief.** During a review fix round that follows an audit-repair re-entry, `AuditRepairActive` is still set, so the build prompt carries both briefs. It renders `audit_repair_findings` first, because the gate's findings outrank the reviewer's, then `review_repair_findings`.
- **Per-round archives.** Before each fix dispatch the kernel archives `build-report.md` and the build prompt as `<name>.round<N>` (`phasecontract.RoundArchiveFilename`), the same rule as the audit archives, so every round's claims stay readable.

#### 5.6 Disposition protocol, and the delta re-review's duties

**The build** answers every OPEN id in `defect-dispositions.json`:
- **FIXED** with evidence: `file:line` plus the test, with its red → green or a mutant kill.
- **DEFERRED** with a reason: counter-evidence for why the finding is wrong or out of scope.

A non-strict (below-threshold) row needs either, with any well-formed content.

**The delta re-review (round n ≥ 2)** reads the digest's history (the previous findings, dispositions and Disposition Reviews), the fix diff (the tree since round n−1's checkpoint, passed as `review_base`) and the full diff. It then:
1. **Verifies each FIXED claim on a strict row.** The evidence must exist, and the test must fail without the fix (run it, or probe it).
2. **Judges each DEFERRED reason on a strict row.**
3. **Scans the fix diff** for new defects (new findings, `round = n`; §5.3's late-finding bound).
4. **Re-scores the full vector.**
5. **Updates the Review Plan** if the fix changed scope (for example, a fix that introduced concurrency).

Steps 1 and 2 are written in a `## Disposition Review` section:

```markdown
## Disposition Review
- d3f9c0a1b2c3d4e5f60718293a4b5c6d7: ACCEPT — TestX now fails on the pre-fix tree (probe run in workspace/dispo.txt)
- da71e0b1c2d3e4f5061728394a5b6c7d8: REJECT — the cited test asserts only that a mock was called; the refusal path is still unpinned
- d0b1c2d3e4f5061728394a5b6c7d8e9f: NOTE — LOW row; deferral reason read, no ruling
```

- **One line per id** the build dispositioned since the previous round, using the full `d` + 32-hex id. ACCEPT and REJECT are for strict rows; a non-strict row gets at most a NOTE and is never rejected.
- **A missing or duplicate line is malformed** and goes to the correction rung. It is never read as acceptance. If the rung is exhausted, the claims stand as *unverified*: the loop treats them as dispositioned, and the digest lists them for the audit to verify (§6.1).

#### 5.6a Applying dispositions to same-cycle rows (`applyReviewDispositions`)

The existing reconcile gate grades only rows inherited from an ancestor cycle (`mergeInherited`, `gradeClaim`). Rows raised in the same cycle need their own application, which is new code:

- **After every build dispatch,** `applyReviewDispositions` reads `defect-dispositions.json`. For each OPEN code-review id it applies `gradeClaim`'s table:
  - FIXED needs evidence that resolves through the audit's citation resolver;
  - DEFERRED needs a non-empty reason;
  - anything else stays OPEN, and the id joins `REVIEW_DISPOSITION_MISSING{ids}`.
- **After every delta review,** it applies the Disposition Review:
  - ACCEPT keeps the status.
  - REJECT of a FIXED claim → OPEN, and `Reopens[id]++`.
  - REJECT of a DEFERRED reason → OPEN, and `RejectedDeferrals[id]++`. At 2, the row becomes **DISPUTED** (`defectledger.StatusDisputed`, added with Q4) and `REVIEW_FINDING_DISPUTED` is emitted. A DISPUTED row is no longer re-sent to build; the audit adjudicates it.
  - Non-strict rows never enter `Reopens`, `RejectedDeferrals` or DISPUTED.
- **The audit's cross-check** (§6.2) calls the same function, so the build's last answers are graded the same way the loop graded them.
- **`gradeClaim` refuses DISPUTED as a build claim.** Its default branch already rejects any status other than FIXED and DEFERRED.

#### 5.7 The tamper check (the role fence for the review's own artifacts)

`defect-ledger.json` and `code-review-report.md` are workspace files, and `guards.Role` is a Claude PreToolUse hook (`role.go`). It does not stop a codex or agy builder, or a Bash write. Same-cycle rows have no ancestor to rebuild from. So:

- **After each review round,** the kernel records a sha256 of `defect-ledger.json` and of `code-review-report.md` in `ReviewRepair.Hashes`, and keeps a copy of each under the workspace's `review-snapshots/`.
- **After each build dispatch,** it re-hashes both. On a mismatch it emits `REVIEW_ROW_TAMPERED` (WARN, `fields.artifact`) and restores its copy before `applyReviewDispositions` runs.
- **The build writes only `defect-dispositions.json`** among the review's artifacts.

### 6. The audit: the final evaluation on the same index

#### 6.1 Inputs, and independence

The audit receives:
- the diff and the acceptance and evals (as today);
- **the review digest**, `code-review-digest.md` in the workspace.

**The digest.** The kernel writes it (`writeReviewDigest`) before each audit dispatch, from the ledger, the cycle state and the review report. Its sections:
- `## Review Plan`: the last round's plan, verbatim;
- `## Findings`: every code-review row, with its id, severity, dimension, status and round;
- `## Dispositions`: the build's claims, with the Disposition Review's ruling on each, and the claims that stand **unverified** (the Disposition Review was exhausted at the correction rung, or no review budget remained after a re-entry);
- `## Disputed`: the DISPUTED ids, with the reviewer's rejections and the builder's counter-evidence;
- `## Stop`: the stop reason and the round count.

It deliberately omits the review's Scores.

**Blind scoring.** The auditor persona forbids opening `code-review-report.md` at all; the digest carries everything the audit needs except the Scores. The divergence metric (§6.3) makes anchoring visible: near-zero divergence across many cycles is itself a signal.

**Skills.** The auditor loads `quality-index` and keeps its acceptance, integrity, scope and eval duties. The code-review checklists move out of the persona (into quality-index and architecture-review), which brings it under the 18,102-byte baseline.

#### 6.2 Qualification, cross-checked by the kernel (logic, not format)

On a code cycle whose build touched files (§2.1's scope), the auditor writes its own `## Scores`, plus an `## Adjudication` section for each DISPUTED id:
- `UPHOLD`: the finding stands.
- `OVERTURN`: the deferral is accepted.

```text
strictOpen := code-review rows (after applyReviewDispositions) with status OPEN and strict(r)
qualified  := Qualifies(audit.Scores, thresholds).ok
            ∧ strictOpen is empty
            ∧ every DISPUTED id is adjudicated, and no UPHOLD on a strict row
            ∧ the existing disposition GRADE passes (an unevidenced FIXED is laundering)
            ∧ the existing acceptance, eval and integrity gates pass

if auditor.verdict ∈ {PASS, WARN} and !qualified  →  verdict FAIL; one AUDIT_QUALIFICATION_GAP per cause
if the Scores or Adjudication section is malformed  →  the correction rung (enforce only; §9)
```

- **Shippable verdicts.** `recordAndBranch` acquires the ship window on PASS or WARN, so both are cross-checked.
- **Where it runs.** The cross-check runs in the audit phase's Classify (`phases/audit`), beside the defect-ledger Reconcile gate, so it decides before `recordAndBranch` acquires the ship window.
- **The flip declares a failure class.** A FAIL with no declared class is declined by `computeRetryEnvelope` ("audit declared no failure class"). So on a flip the kernel writes a qualification block, `audit-qualification.json`, `{class: code-audit-fail, defects: [one line per gap, OPEN strict row, upheld id or unruled id]}`, and `auditFailEnvelope` reads it when the auditor's own sentinel carries no failure block. `decideAfterAuditFail` then grants a repair through the `CategoryCodeAuditFail` policy row, like any auditor-declared FAIL.
- **The repair.** The FAIL takes the existing audit-repair path: `decideAfterAuditFail`, then a tdd or build re-entry. After a build re-entry, `code-review` runs a delta round if budget remains (§5.4 routing); otherwise `REVIEW_BUDGET_SPENT`, and the audit re-judges.
- **In shadow** the cross-check computes `would_fail` over the rows read as OPEN (§5.4) and signals it. It never flips the verdict, and a missing or malformed audit Scores or Adjudication section is signalled, never sent to the correction rung, so shadow changes no verdict and adds no dispatch.

#### 6.3 Divergence: learning from disagreement

After every audit that scored the index, the kernel compares the audit's vector with the review's final `LastScores`. A cycle whose review was skipped has no review vector, and no divergence is computed.

```text
for each dimension d:
  diverged if |audit[d] − review[d]| ≥ workflow.quality_index.divergence (2)
           or exactly one of them is N/A
  direction := audit_lower | audit_higher | na_mismatch
  → REVIEW_AUDIT_DIVERGENCE{dimension, review, audit, direction}, appended to the workspace's review-divergence.json
```

- **`direction = audit_lower`** means the reviewer was too lenient on that dimension. Only `audit_lower` counts toward phase B; `na_mismatch` never does.
- **The phase-B trigger** (§8) reads `review-divergence.json` across cycles: a dimension whose audit-lower share exceeds `workflow.review.specialist_trigger.audit_lower_share` (0.3) over the last `window_cycles` (20) code cycles that scored it.
- **Lessons:** the retrospective reads the divergence records into lessons.

#### 6.4 Adjudicating DISPUTED findings

The auditor rules on each DISPUTED id with a reason, after weighing the reviewer's rejection against the builder's counter-evidence:
- **UPHOLD** sets the row to OPEN with reason `upheld by audit`. On a strict row it makes the change unqualified (§6.2).
- **OVERTURN** sets the row to DEFERRED with the auditor's reason.
- **A missing ruling** is malformed and goes to the correction rung. If the rung is exhausted, the missing ruling counts as UPHOLD.
- **At cycle close,** any row still DISPUTED becomes OPEN, so a FAILed cycle's continuation inherits it as owed. The reconcile gate carries a non-OPEN ancestor row verbatim and never owes it, so this closing step is what keeps a dispute from escaping inheritance.

### 7. Recovery rungs and degrade posture

Process failures get a recovery step; only logic blocks.

| Situation | Behaviour |
|---|---|
| No deep-tier capacity (every route walled) | Today a quota wall reaches `pauseForQuota`, which pauses the loop. Q6 adds a branch at that seam for a non-gate phase: a code-review quota wall emits `REVIEW_SKIPPED{reason: capacity}` and the walk continues. A skipped round is not a completed dispatch, so it does not count toward `max_rounds`. The audit still scores the index, so qualification is never waived. |
| Malformed Review Plan, Findings, Scores or Disposition Review | The correction rung (bounded). Once it is exhausted, the phase degrades to SKIPPED+WARN and the walk continues: nothing is recorded, `REVIEW_SKIPPED{reason: malformed}` is emitted at WARN, and the ledger gets a `contract_exhaustion_skip` entry. This is the shape of the existing non-canonical-verdict degrade (`nonFloorExhaustionDegrade`), and it holds for any optional evaluate phase past the ship floor that the routing config does not list as mandatory (`config.mandatory_phases`), on the fresh and the resume ladder alike (`Orchestrator.degradesOnExhaustion`, `degradesRejection`). The same phase's contract blocks never reach the contract gate's persisted breaker (`ReviewInput.BreakerExempt`: no count, no reset, no demotion). A malformed review therefore can never open the circuit and hand the audit a `contract_gate_demoted` waiver. Because the breaker never counts it, the ladder's breaker-count escalation and the salvage re-prompt (which warns of a breaker about to open) do not apply; each correction is the plain directive. A report that is *absent* (missing or empty, `ReviewResult.DeliverableAbsent`) is not malformed: it keeps the abort, because a non-admitted missing deliverable never reaches Ship ([audit-repair isolation](audit-repair-isolation.md), #542). Since the breaker never counts it either, it can no longer be demoted into an approval. |
| A finding with an unknown dimension or no `Fix:` | Recorded as written, and counted. Never dropped. |
| The ledger write fails | `REVIEW_FINDINGS{recorded: false}` at WARN. In enforce, the loop stops with `REVIEW_SKIPPED{reason: ledger}`, and the digest carries the parsed findings from memory, which the audit treats as OPEN. It never resolves the loop. |
| The build omits a disposition | The row stays OPEN, and `REVIEW_DISPOSITION_MISSING{ids}` is emitted. |
| The build fix round fails | The existing build-failure path. |
| The build rewrites the ledger or the report | `REVIEW_ROW_TAMPERED`, and the kernel restores its copy (§5.7). |
| An unknown stage, threshold or index key in config | The default stands, and `REVIEW_FINDINGS` carries the config warning at WARN. |
| The reviewer runs on the builder's model family | `REVIEW_SAME_FAMILY` WARN. Never a block. |

### 8. Specialists (phase B, data-driven; operator Q-D4)

**Not built in phase A.** Specialists are added when the divergence data (§6.3) shows the lead reviewer is too lenient on a dimension. A specialist is an existing lens phase (perf-profile, race-condition-scan, error-handling-scan, security-scan, query-performance-scan, cache-strategy-scan, resilience-gap-scan, telemetry-coverage-check, idempotency-check, type-safety-audit), mapped to a dimension.

**Configuration:**

```jsonc
"workflow": { "review": {
  "specialists": [
    { "phase": "race-condition-scan", "dimension": "concurrency", "when": [{"field": "diff.touches_concurrency", "op": "eq", "value": true}] }
  ],
  "specialist_trigger": { "audit_lower_share": 0.3, "window_cycles": 20 },
  "hot_path_packages": []
}}
```

- **The trigger** evaluates from the first enforce wave onward, over `review-divergence.json` (§6.3); the plan adds a specialist only after at least two enforce waves of that data.
- **Triggers** are deterministic diff signals from a small classifier: concurrency primitives, hot-path packages (`hot_path_packages`), error returns, security-sensitive paths and SQL. The lead reviewer may also *request* a specialist in its Review Plan, with justification. A request is granted only when it is configured and allowed.
- **Placement:** triggered specialists run in the same round as the lead, in parallel through the evaluate batch (`core/evaluate_batch.go`).
- **Reporting:** their personas are normalized to the §4.1 grammar, and their rows carry `source: code-review/<phase>`.
- **One home:** once review is enforced, the advisor no longer inserts those lens phases standalone.

### 9. Stages and configuration

```jsonc
"workflow": {
  "findings_repair": {
    "code-review": { "stage": "shadow", "threshold": "MEDIUM", "max_rounds": 4 }
  },
  "quality_index": { "thresholds": { "*": 4 }, "divergence": 2 },
  "review": { "specialists": [], "specialist_trigger": { "audit_lower_share": 0.3, "window_cycles": 20 }, "hot_path_packages": [] }
}
```

- **Compiled defaults** are as shown. The block is absent from the checked-in policy until the flip.
- **`enforce` is held at shadow until Q3–Q5 land.** `stage: "enforce"` is refused. It resolves to `shadow`, and `REVIEW_FINDINGS` carries the config warning at WARN (`policy.resolveJudgeRepair`). Without the loop (Q3) and the audit's cross-check (Q5), an enforced review would write OPEN rows that no fix round repairs.
- **Each key lands with its reader:** `stage` and `threshold` (landing 1), `quality_index.thresholds` (Q1), `max_rounds` (Q3), `divergence` (Q5), `review.*` (Q7).
- **Validation.** `threshold` is a severity; `max_rounds` is at least 1; a threshold is an integer from 1 to 5, with `"*"` applied first and per-key entries overriding it. A value that fails validation is ignored with a config warning, and the compiled default stands.
- **Shadow:**
  - the review runs, plans and scores every round;
  - rows are DEFERRED (reason: the stage);
  - there is no fix round;
  - the loop's decision is computed as if enforced and signalled (§5.4);
  - the audit's own index scoring and the cross-check run in shadow: the would-fail is computed and signalled, the verdict is not flipped, and no correction rung runs for the audit's index sections (§6.2);
  - divergence is recorded;
  - the review never touches the audit. The ledger's dedupe, cap and stand-in are kept per source, and the lineage owes no shadow row (§4.2). The review's contract blocks never reach the contract gate's breaker (§7).
- **Enforce:**
  - §5's loop runs;
  - the audit's qualification is binding;
  - the auditor persona's slimming lands with the flip, because in shadow the audit still reviews code.
- **`threshold`** sets the strict severity (§5.2): findings at or above it must be FIXED or accepted-DEFERRED, and only they can be rejected or DISPUTED. MEDIUM follows Q-D2. Landing 1 first staged HIGH; Q2 moves the compiled default to MEDIUM.

### 10. Signals

All codes but the last are in module `review`. `AUDIT_QUALIFICATION_GAP` belongs to module `audit`, which emits it. **Every code's field set fits the Signal Center's cap** (`signalcenter.MaxFields`, 12), optional fields included: the Center drops what does not fit and only counts it in `fields.truncated`, so a field over the cap is a silent metric loss. That is why `REVIEW_FINDINGS` carries the severity counts as one field (pinned by `TestEvent_TheWorstCaseFieldSetPassesTheCenterUntruncated`).

| Code | When | Fields | Lands |
|---|---|---|---|
| `REVIEW_FINDINGS` | every completed round | `round`, `findings` (counts by severity: `critical=N,high=N,medium=N,low=N`), `verdict`, `would_repair`, `stage`, `threshold`, `recorded`, `scores` (the vector, `key=N` or `key=N/A`), `gaps` (empty when the scores qualify); `overflow`, `config_warning` or `error` when present | landing 1 (Q2 folds the counts into `findings` and adds `scores` and `gaps`) |
| `REVIEW_PLAN` | every completed round | `round`, `required`, `na`, `deep` (comma lists of dimensions) | Q6 |
| `REVIEW_ROUND` | after the decision | `round`, `decision`, `stage`, `unresolved`, `finding_mass`, `gap_mass`, `progress` | Q3 |
| `REVIEW_REPAIR_GRANTED` | a fix round is granted | `round`, `findings`, `gaps` | Q3 |
| `REVIEW_RESOLVED` | the resolved predicate holds | `rounds` | Q3 |
| `REVIEW_UNRESOLVED` | a stop other than resolved | `reason` (budget, no-progress, oscillation), `rounds`, `unresolved` | Q3 |
| `REVIEW_FINDING_DISPUTED` | a strict deferral is rejected twice | `id`, `severity`, `dimension` | Q4 |
| `REVIEW_DISPOSITION_MISSING` | the build left ids undispositioned | `ids` | Q3 |
| `REVIEW_ROW_TAMPERED` | the ledger or report changed during a build dispatch | `artifact` | Q3 |
| `REVIEW_SKIPPED` | a round is skipped or the loop stops on a fault | `reason` (capacity, malformed, ledger), `round` | landing 1 for `malformed` (an exhausted correction ladder or a verdict still non-canonical after every retry); Q6 adds `capacity` and `ledger` |
| `REVIEW_SAME_FAMILY` | reviewer family = builder family | `builder_family`, `reviewer_family` | Q6 |
| `REVIEW_BUDGET_SPENT` | an audit-repair re-entry finds no review budget | `rounds` | Q3 |
| `REVIEW_AUDIT_DIVERGENCE` | §6.3 | `dimension`, `review`, `audit`, `direction` | Q5 |
| `AUDIT_QUALIFICATION_GAP` | §6.2 finds a cause | `cause` (gap, open_finding, upheld_dispute, unruled_dispute), `dimension`, `score`, `threshold`, `id`, `stage`, `would_fail` | Q5 |

**`would_repair` before and after Q3.** Until Q3 lands, `REVIEW_FINDINGS.would_repair` is the round-1 projection of the decision. In shadow every round is round 1, and round 1 is resolved only with no finding and qualifying Scores. So landing 1 sets `would_repair` to true when any finding was raised or the Scores do not qualify. From Q3 on, it is `decision == granted`, taken from `REVIEW_ROUND`. `verdict` is the report's derived verdict: FAIL when any finding is strict, WARN when every finding is below the threshold, PASS with none.

### 11. Independence fences

| Fence | How it holds |
|---|---|
| A fresh agent | No access to the builder's transcript. `build-report.md` is a claim to verify. |
| Read-only | The phase does not write source, so `withWorktreeFence` makes its worktree read-only (ADR-0097). Probes and `go test -overlay` mutants write only to the workspace and tmp. |
| A role fence | The hash-and-restore check (§5.7) protects `defect-ledger.json` and `code-review-report.md` from any build CLI. Build writes only `defect-dispositions.json`. |
| Cross-family | The profile is a Claude-floor member at the deep tier (claude-tmux, then claude-p). The builder runs codex today and agy (Gemini 3.8 Flash) once L2's routing table lands, so the families differ either way. The same family is a WARN. |
| Not a gate | The review's verdict never unlocks ship. Only the never-re-rolled audit does. |
| A blind audit | The audit scores from the digest and never opens the raw review report (§6.1). |

### 12. Interplay with existing mechanisms

| Mechanism | Relationship |
|---|---|
| C0c self-simplify (build) | The author's own pass stays in build and is scoped to the build's own files. The review is independent and applies no fixes; the builder applies them. |
| `code-review-simplify` | Builders load it for self-review. The reviewer loads it in **review mode**: it reports and never applies. |
| The lens phases | Phase A leaves them advisor-inserted (advisory), and quality-index folds their checklists into its per-dimension procedures. Phase B maps them to dimensions as triggered specialists, and they stop being standalone. |
| The evaluate batch | Phase A: `code-review` runs alone, first after build (ordering fix: the plan's open question OQ1), and the other evaluate phases batch after the loop settles. Phase B: specialists batch with the lead inside a round. |
| Audit-repair (ADR-0093/0096) | One generalized findings-driven repair, keyed by judge; the audit's behaviour is byte-identical. `remediationDenied` is unchanged. |
| Continuations | OPEN code-review rows left at a FAILed cycle's end are inherited like audit rows, with their provenance (§4.2). Rows still DISPUTED at close become OPEN first (§6.4). Shadow rows are DEFERRED and never owed. |
| ADR-0084 lenses | No new production on-disk repo scan; the diff classifier (phase B) reads the cycle's own diff only. |

### 13. Security surface

These are protected (`ProtectedSurfaceManifest`), because a tampered index or reviewer would silently rewrite what "qualified" means for every cycle:
- `skills/quality-index/`, `skills/architecture-review/` and `skills/code-review-simplify/` (the last by C0);
- `agents/evolve-code-reviewer.md`, `.evolve/phases/code-review/` and `.evolve/profiles/code-reviewer.json`;
- the `qualityindex` and `codereview` packages;
- `go/internal/router/code_review_pin_test.go`, which pins the registry's `conditional_mandatory["code-review"]` rule. The manifest protects whole files, and protecting `docs/architecture/phase-registry.json` would freeze every routing edit, so the pin is guarded by its protected test instead: a lane that removes the pin fails a test it cannot edit;
- the core repair decision (Q3).

Every compiled-default overlay skill also joins the manifest, pinned by `TestProtectedSurface_CompiledDefaultOverlaySkills`.
