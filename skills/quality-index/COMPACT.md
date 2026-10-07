# quality-index — COMPACT projection (the injected form)

> One index shared by the code-review phase and the audit. Score every dimension 1–5 or N/A. **The bar is 4** (or the configured threshold). For a dimension your Review Plan marks standard or deep, read its procedure in `skills/quality-index/SKILL.md` §Review procedure per dimension.

| Key | Score 4 means | N/A allowed when |
|---|---|---|
| `correctness` | every acceptance criterion proven by a test that fails without the change; no known logic defect | never |
| `architecture` | no duplicated belief; dependencies point inward; every new seam or pattern names its force | never |
| `maintainability` | functions ≤50 lines, nesting ≤4, files <800; no added comment; no dead code; no flag argument | never |
| `test-quality` | red-first evidence; no surviving non-equivalent mutant on changed lines; deterministic; no mocks of own logic | never |
| `robustness` | no swallowed error; a recovery rung for process failures; validation at trust boundaries | pure docs or data, no code path |
| `concurrency` | `-race` clean; every goroutine owned and stoppable; context honoured | no goroutine, channel, mutex, atomic, shared state or cancellation in the diff |
| `performance` | no new superlinear hot-path work; no batchable I/O in a loop; prompt bytes justified | no executed path and no agent-facing text changed |
| `debuggability` | failures name cause and inputs; new behaviour emits a registered signal or a log with fields | no runtime behaviour changed |
| `security` | no injection, traversal or secret leak; fences and the protected manifest respected | no trust boundary, input, credential, subprocess or fence touched |
| `docs-consistency` | package docs, CHANGELOG and plan updated; config, not code; no flag | never |

Never N/A: `correctness`, `architecture`, `maintainability`, `test-quality`, `docs-consistency`.

**Anchors:** 5 exemplary · 4 the bar holds · 3 one "pulls to ≤3" item · 2 a defect a user would hit · 1 harmful or absent.

**Grammars.** The separator is ` — ` (or ` – `, or ` - `). Every dimension appears exactly once in each section.

```markdown
## Review Plan
- Inputs read: <deliverables and diff read>
- <key>: required (light|standard|deep) — <why>
- <key>: N/A — <concrete reason tied to the diff>

## Scores
- <key>: <1-5> — <rationale; a score below the threshold cites a finding: CR2: …>
- <key>: N/A — <reason; the same N/A set as the plan>

### CR1 (HIGH) — <title>
- Dimension: <key>
- Location: <path>:<line>
- Scenario: <input → steps → wrong outcome>
- Evidence: <probe or mutant output, or traced lines>
- Fix: <patch sketch; for a test gap, the test name and its assertion>
```
