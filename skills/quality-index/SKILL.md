---
name: quality-index
description: Use when reviewing or auditing a code change against the shared quality index — the ten dimensions (correctness, architecture, maintainability, test-quality, robustness, concurrency, performance, debuggability, security, docs-consistency), each scored 1–5 or N/A against a bar of 4 — and when writing the Review Plan, findings and Scores sections the code-review phase and the audit share.
---

# Quality index — one definition of "qualified"

> The code-review phase and the audit judge every code change on these ten dimensions, in the same grammar, so the review can aim at the gate's bar and the gate can tell a lenient review from a sound one.
> - Design: [review-loop-and-quality-index.md](../../docs/architecture/review-loop-and-quality-index.md).
> - The code home is `go/internal/qualityindex`: the vocabulary, the threshold resolver, the parsers and `Qualifies`.
> - A test pins this file's dimension table and never-N/A line to that package.

## The dimensions

| Key | Judges | Score 4 (the qualifying bar) | Pulls to ≤3 | N/A allowed when |
|---|---|---|---|---|
| `correctness` | Acceptance met; logic and edge cases | Every acceptance criterion is proven by a test that fails without the change; no known logic defect | An unproven criterion; a reachable unhandled edge case | never |
| `architecture` | One home per rule, coupling, boundaries, patterns with their forces | No duplicated belief introduced; dependencies point inward; every new seam or pattern names its force | A second copy of a rule; a leaked layer; a pattern with no force; a touched over-cap function that did not shrink | never |
| `maintainability` | Clean-code limits, naming, dead code, comments | Functions ≤50 lines, ≤3 params where natural, nesting ≤4, files <800; zero non-machine comments; no dead or unused exports | A breached limit; an added comment; a flag argument; dead code | never |
| `test-quality` | Behaviour over surface, red-first, mutation, determinism | Red-first evidence for new behaviour; changed lines have no surviving non-equivalent mutant; deterministic; no mocks of the project's own logic | A surviving mutant; an assertion-free or mock-only test; a sleep- or order-dependent test | never |
| `robustness` | Error handling, loud failures, recovery rungs, validation at boundaries | No swallowed error; process failures get a recovery rung; inputs are validated at trust boundaries | A swallowed error; a silent degrade; a block on a format-only issue | the diff is pure documentation or data with no code path |
| `concurrency` | Races, leaks, lock ordering, context propagation | `-race` clean on the touched packages; every goroutine has an owner and a stop; the context is honoured | A race; a leaked goroutine; a lock-order hazard; an ignored cancellation | the diff touches no goroutine, channel, mutex, atomic, shared mutable state or context-cancellation path |
| `performance` | Algorithmic cost, allocations, I/O, hot paths, prompt and token cost of agent-facing text | No new superlinear work on a hot path; no I/O in a loop that can be batched; prompt bytes justified | A measurable regression; an unbounded read; repeated whole-file passes | the diff changes no executed path and no agent-facing text |
| `debuggability` | Signals, error context, logs, diagnosability | A failure names its cause and inputs; new behaviour emits a registered signal or log where an operator would look | An error with no context; a silent state change; a log with no fields | the diff changes no runtime behaviour |
| `security` | Input handling, injection, secrets, sandbox and fences, protected surfaces | No new injection path; no secret in code or logs; fences and the protected manifest are respected | A path traversal; command injection; an unvalidated external input; a fence bypass | the diff touches no trust boundary, external input, credential, subprocess or fence |
| `docs-consistency` | Docs as the primary asset; config, not code; no flags | The package docs, CHANGELOG and plan status are updated; behaviour lives in config where policy says so; no feature flag | A stale doc; a flag; a policy literal in code | never |

Never N/A: `correctness`, `architecture`, `maintainability`, `test-quality`, `docs-consistency`.

**Score anchors:**
- **5:** exemplary; nothing to improve within scope.
- **4:** the bar above holds. This is the default threshold (`workflow.quality_index.thresholds`, 1–5 per dimension, `"*"` for all).
- **3:** one item from "pulls to ≤3", or the bar is met only partly.
- **2:** a defect a user would hit.
- **1:** harmful or absent, such as a data-loss path or no tests at all.

**Every score below its threshold cites a finding** (`CR<n>`) on that dimension in its rationale. A gap the builder cannot act on is not a review.

## Review procedure per dimension

Each dimension is reviewed at the depth the Review Plan names:
- **light:** read and reason;
- **standard:** also run the touched packages' tests;
- **deep:** also run probes, `go test -overlay` mutants, `-race` or benchmarks.

All evidence runs in a scratch copy, the workspace or `/tmp`, never in the repository.

- **correctness.** Map every acceptance criterion in the Task Contract to the test that pins it. Trace each changed branch to its callers. Probe each edge case the scenario names: empty, nil, maximum and concurrent input. Deep: run the change's new tests on the pre-change tree (the red-first overlay check) and run mutants of the changed conditions.
- **architecture.** Work `architecture-review`'s rubric. Grep every changed belief (a value, rule, threshold or prose contract) for its other homes. Check dependency direction on the changed imports, and that each new seam or pattern names its force.
- **maintainability.** Measure every added or changed function: lines, nesting, parameters, flag arguments. Count added comments against the code-comments convention. Find dead and unexported-unused code. Use `code-review-simplify`'s Simplification Catalog, in review mode, which reports and never applies.
- **test-quality.** Use `golang-test-review`'s checklist for changed `*_test.go` files, and `engineering-craft`'s tdd-craft §Reviewing for TDD. Deep: after one control mutant, run up to ten single-idea mutants of the changed code, covering the classes a deletion misses (reorder, shift, wrong key, wrong tense, hard-coded input).
- **robustness.** For every new error path: is it handled or propagated with context, never swallowed? Does a process failure get a recovery rung instead of a block? Is input validated at the trust boundary it crosses? Does a fail-open carry a WARN?
- **concurrency.** List every goroutine, channel, mutex, atomic, shared map and context the diff adds or touches. For each goroutine, name its owner and its stop. Check lock order and cancellation. Deep: `go test -race` on the touched packages, and a probe that cancels mid-operation.
- **performance.** Find loops that do I/O or allocation, whole-file reads, repeated passes, and work that grows superlinearly on a hot path. For agent-facing text (prompts, personas, skills), count the bytes added to every dispatch. Deep: a benchmark before and after.
- **debuggability.** For every new failure: does its message name the cause and the inputs? Does new behaviour emit a registered signal (`signalcenter.RegisterCode`) or a log line with fields where an operator would look? Is a state change visible?
- **security.** Use `security-review-scored`. Trace every new external input to its validation. Check subprocess arguments for injection, paths for traversal, and logs and errors for secrets. Check that fences, the sandbox and `ProtectedSurfaceManifest` are respected.
- **docs-consistency.** Check that every changed package's `docs/architecture/packages/<dir>.md`, the CHANGELOG and the plan's status rows are updated. Check that new behaviour lives in config where policy says so. A new env flag or a policy literal in code is a finding.

## Grammars

The code-review report, and later the audit report, write these sections. The kernel parses them (`qualityindex.ParsePlan`, `ParseScores`), and a malformed section gets the contract-correction rung.

**Separators.** Between a value and its text, write an em dash, an en dash or a hyphen with a space on each side (` — `, ` – `, ` - `). The first one splits the line.

### Review Plan (every review round opens with it)

```markdown
## Review Plan
- Inputs read: <the deliverables and the diff you read>
- correctness: required (standard) — 3 acceptance criteria; TDD pinned 2, the refusal path is unpinned
- concurrency: required (deep) — the diff adds a goroutine in usageprobe/evidence.go:96 and a shared cache map
- security: N/A — no trust boundary, external input, subprocess or credential in the diff
- …one line per dimension…
```

- `Inputs read` is required, appears once and is not empty.
- Every dimension appears exactly once, as `required (light|standard|deep) — <why>` or `N/A — <why>`.
- A never-N/A dimension is never N/A.

### Scores (every review round and every audit)

```markdown
## Scores
- correctness: 4 — every acceptance criterion pinned by a red-first test (test-report.md; TestX, TestY)
- concurrency: N/A — the diff adds no goroutine, channel, mutex, atomic or shared state
- performance: 3 — CR2: readAll on a 47 MB file per call at ledger.go:120; batchable
```

- Every dimension appears exactly once, scored with an integer from 1 to 5 or `N/A`, with a non-empty rationale.
- A never-N/A dimension is never N/A.
- The N/A dimensions are the same set as the Review Plan's.
- A score below its threshold cites a finding on that dimension.

### Findings

```markdown
### CR1 (HIGH) — <one-line title>
- Dimension: <a dimension key>
- Location: <path>:<line>
- Scenario: <the concrete failure: the input, the steps, the wrong outcome>
- Evidence: <the probe or mutant command and its output, or the traced lines>
- Fix: <a patch sketch; for a test gap, the test name and its assertion>
```

- The severity is CRITICAL, HIGH, MEDIUM or LOW. CRITICAL is structural damage or a broken behaviour; HIGH should be fixed before merge; MEDIUM is maintainability; LOW is optional.
- Each field is one line. A longer patch goes in a fenced block below the fields.

## What the audit does with it

From ADR-0124 Q5, which has not landed yet, the audit scores the same index blind. It will read the kernel's review digest, never the review's own Scores. The kernel will turn into a FAIL any shippable audit verdict whose vector does not qualify, meaning every applicable dimension at or above its threshold. Until Q5 lands, the audit does not score the index, and the review's Scores are a shadow measurement (`REVIEW_FINDINGS` fields `scores` and `gaps`).
