# Component decomposition implementation report

**Date:** 2026-09-11
**Branch:** `refactor/component-decomposition`
**Worktree:** `dev/component-decomposition`
**Integration base:** `c1ba55a046bd273ff7477aa8d685aa00292bd434`

## Outcome

This change decomposes the largest Go workflow facades into package-local components while preserving their public entry points, storage formats, phase policies, exit codes, and provider protocols. Development stayed in the isolated feature worktree. Promotion uses reviewed functionality commits from the feature branch, so the runtime/main checkout receives only committed changes through merge.

The work used characterization-first TDD module by module. Existing public-seam tests established behavior before each extraction. New behavior found during live validation used assertion-level RED tests before production fixes. Every module's package tests passed before work moved to the next component.

No Python tooling or Python code was used for the implementation. No production `if` condition added by this change contains a constant `false` expression.

## Resulting component map

| Area | Stable facade | Package-local components | Preserved contract |
|---|---|---|---|
| Loop CLI | `runLoopBatch` / `loopBatchCoordinator.run` | runtime, resume, maintenance, prebatch, boundary/fleet window, sequential dispatch, observation, verification, outcome | signal/socket lifetime, refreshed-head ordering, breakers, exit mapping |
| tmux bridge | `runTmuxREPL` | prepare, boot, prompt dispatch, live channel, completion wait | admission/session disposition, cursor cutover, cancellation-detached final poll, capture |
| Phase runner | `BaseRunner.Run` | preparation, routing, dispatch, reconciliation, classification | one worktree fence across fallback; restore before classification; verified bytes authoritative |
| Core review | `reviewAndGuard` | review preparation, correction ladder, post-review guards | recovery and normalization precede initial and corrected review |
| Core resume | `RunCycleFromPhase` | bootstrap, cursor, execution, shared completion | identity restoration and legacy behavior remain explicit; completion is shared with fresh execution |
| Audit | `hooks.Classify` | evidence classification, host gates, continuation/closure disposition | host precedence, fail-closed predicate errors, late evidence seal |
| Ship | `shipFromWorktree` | typed worktree transaction and integrity checks | path-specific lock, staged-tree binding before commit, post-push tree verification |
| Policy | `Policy` and `Load` | same-package files by policy domain | JSON schema, defaults, pointer unset semantics, environment precedence |
| Triage closeout | Core terminal decision, closeout, resume, and sequential verification | host-owned no-work disposition and shortened ledger floor | failed Triage remains FAIL; successful explicit empty-array commitment stops before implementation |

The decomposition components remain package-local. The no-work repair adds one narrow result field, one reason constant, and `core.IsTriageNoWorkResult` because the command verifier is an outside-package consumer; composed command tests pin that API. No package, dependency, general workflow framework, or single-implementation interface was added.

## TDD and review-driven corrections

The existing package suites supplied the primary characterization coverage: tmux session/cancellation matrices, runner fallback and worktree fencing, Core correction and resume lifecycle, Audit precedence, Ship real-Git transactions, and Policy loading/default tables. Structural tests that intentionally pin source ownership now follow the extracted owner and retain their original behavioral assertions.

Architecture review found that live-channel files were opened before the inbox cursor established its EOF cutover. A focused test first failed, then the cursor initialization moved ahead of channel creation. Its preservation case proves old backlog is skipped while an envelope arriving after cutover remains drainable.

Fresh and resumed cycles now use the same `phaseCompletionRecord` for completed-phase state, final-verdict persistence, judgment learning, authoritative-floor failure recording, and timing. They do not claim identical execution. Fresh-only parallel evaluation, remediation, advisory/debugger routing, and resume-only legacy handling remain explicit because collapsing them would change behavior.

The live waves found three further product gaps:

1. `resume_bootstrap.go` moved an IPC environment read outside the source scanner's marker coverage. The ACS registry test failed first. Adding the existing protocol marker beside the extracted reader restored the registry contract.
2. The commit-prefix manifest required ignored generated `go/bin/**` as a repository anchor. A clean tracked-tree regression failed first. The manifest now requires the tracked `go/evolve` build entry only. The integration-tagged guard reads `git ls-files --stage`, resolves tracked symlink targets back to tracked descendants, and rejects absent descendants without copying the repository; default unit tests keep the path-resolution logic process-free.
3. A protected task could pass the selection boundary and reach TDD. Router and Core tests failed in sequence for failed Triage, rollout-stage inconsistency, resume bypass, and the initially unreachable positive no-work path. The repaired host decision stops failed Triage as FAIL in Off, Shadow, Advisory, and Enforce. The real Triage classifier returns PASS for an empty report section only when the decision sidecar contains the explicit empty array; a composed full-cycle test proves it reaches planned closeout without TDD, Build, Audit, or Ship. `top_n: null`, missing, malformed, and wrong-shaped decisions stay unknown. Closeout requires the host reason, terminal Triage, and no implementation; the command verifier requires that same result plus artifact revalidation before accepting the Scout/Triage ledger floor.

## Architecture and simplification decisions

Same-package concrete components fit the existing dependency direction and keep mutable state visible at its actual lifetime boundary. Adding interfaces would have increased mocks and indirection without providing a second implementation.

Two plan targets were deliberately narrowed:

- The REPL completion waiter is isolated but remains an effectful state machine. Ordered pane captures, inbox drains, persistence gates, callbacks, prompt injection, time, and cancellation-detached polling need an explicit event model before a pure reducer would be honest.
- Fresh and resumed cycles share durable completion rather than a complete dispatch engine. Their routing, remediation, debugger, and legacy explanation differences remain documented and tested.

The final review set includes a Go/code-simplification pass, a defensive code review, and an architecture review. Findings must be resolved before the branch is presented for merge.

## Model routing and token accounting

The repository already maps Claude `deep` and `top` tiers to `opus`. The Codex family maps those tiers to `gpt-5.6-sol`; the headless Codex manifest inherits the same family table. No model configuration change was necessary.

Provider cost output is not a universal budget. The live Codex driver reported `Deps.TokenResolver is nil`, so its phases are recorded as unmeasured rather than zero-cost. Wave 1's Claude Audit emitted 47,242 output tokens, read 6,532,938 cache tokens, wrote 166,701 cache tokens, and ran for about 733 seconds. The cause of the excessive use was an unbounded deep-tier audit that repeatedly reopened settled evidence without a stop criterion. Later audit/replay work used early stopping and deterministic replay rather than another deep cycle.

One provider Auditor invoked Python inside its own sandbox during Wave 1 despite the Go-only instruction. That was a phase-compliance failure; the implementation and verification commands in this work used Go, Git, shell, and repository-native tooling only.

## Controlled validation waves

Both logical wave goals ran in a disposable clone rooted at `/tmp/evolve-component-wave.Uv9m4N/repo-waves`. Commits mentioned here exist only in that disposable repository.

### Wave 1: useful material change

- **Goal:** repair the clean-checkout commit-prefix anchor defect.
- **Start:** disposable snapshot `bcb94595`, cycle 1624.
- **Models:** Intent on Codex `gpt-5.6-sol`; Scout/Triage/Builder on Codex balanced tier; TDD on Claude Sonnet; Audit on Claude Opus.
- **Result:** SHIPPED disposable commit `11fbf8bd` with the manifest fix, Go regression tests, and ADR update.
- **Phases:** Intent, Scout, Triage, TDD, Build, Audit, Ship.
- **Verification:** package gates, race/integration checks, repository contract, and Audit passed.
- **Elapsed time:** approximately 42 minutes 55 seconds.
- **Useful evidence:** the wave produced a tested repository change and independently surfaced the resume IPC-marker regression.

Operational findings were preserved rather than hidden: Build correction watched only `build-report.md` even when the corrected explanation lived elsewhere; the Audit sandbox could not read tracked `docs/private`, causing repository-walking false RED results; Codex usage was unmeasured; and the Opus Audit lacked a bounded stop rule.

### Wave 2: protected-surface and no-work behavior

- **Goal:** attempt to refactor `go/internal/guards/build_explanation_wiring_test.go`, an intentionally protected integrity surface.
- **Initial result:** cycle 1625 reached TDD, where the role guard correctly blocked the write. Audit correctly rejected the zero-change result, but automatic repair tried the same impossible target again.
- **First repair:** Router stops after a failed Triage verdict.
- **Second repair:** the host routing boundary stops Triage across all routing stages and on resume. Cycle 1628 ran only Intent, Scout, and Triage. Its decision deferred the protected target with `top_n: []`; Triage returned FAIL, that FAIL remained the cycle outcome, and no TDD, Builder, Auditor, or deep model session started.
- **Third repair:** the production Triage classifier recognizes a valid empty report only when the explicit empty-array decision corroborates it. Successful closeout records `SKIPPED_UNKNOWN` plus the host no-work reason, cleans the worktree, and verifies the shortened ledger chain. Failed Triage remains FAIL even when an empty sidecar exists.
- **Final evidence:** the real cycle-1628 artifacts prove protected-task selection and early termination under the original FAIL classifier. A composed cycle through the repaired production Triage classifier, plus resume, JSON-shape, ledger, and command-loop tests, establishes the successful planned no-work path and its negative cases end to end.

A paid replay after the third repair was stopped in Intent. Two Codex corrections omitted required sections, then correction escalation paired Claude with the Codex model name `gpt-5.6-sol`; the phase sandbox also denied the contracted artifact write. Continuing would not exercise the Triage repair and would waste more tokens. That setup attempt was sealed and reaped, and it is not counted as a third logical wave.

### Cleanup

Cycles 1624 through 1629 in the disposable repository were shipped, sealed, or reset according to disposition. Preserved Wave 2 worktrees and branches were removed after evidence capture. The final process/session scan found no live loop process or cycle-1629 tmux session. The feature worktree remains separate from runtime/main.

## Promotion batches

Architecture review fixed the order before promotion. The commit-prefix correction lands first so every later clean-checkout gate uses tracked anchors. Core, Router, Triage, ledger verification, cycle result fields, and the CLI land atomically because successful no-work closeout and its shortened ledger floor are one runtime contract. The source-wiring guard moves its Runner/Audit assertions with that extraction and its Core/Resume assertions with the atomic runtime batch.

| Batch | Commit | Scope | Independent verification |
|---|---|---|---|
| 1 | `66b79256` | tracked commit-prefix anchors | unit, clean-checkout integration, repository suite, commit gate |
| 2 | `666c8bae` | Policy facade split by domain | Policy suite, repository suite, commit gate |
| 3 | `2891f2f5` | phase Runner and Audit components | Runner, Audit, guards, repository suite, commit gate |
| 4 | `2806b011` | Ship worktree transaction | Ship real-Git suite, repository suite, commit gate |
| 5 | `397e6eb9` | tmux bridge lifecycle | Bridge suite, race suite, repository suite, commit gate |
| 6 | `16589cf5` | Core/routing/Triage/ledger and CLI | affected packages, Core integration, race suites, repository suite, commit gate |
| 7 | this documentation commit | architecture, plan, registry, and results | projection checks, repository suite, commit gate |

## Verification status

| Check | Result |
|---|---|
| Per-module package tests | PASS for CLI, Bridge, Core, Runner, Audit, Ship, Policy, Router, Triage, ledger verification, and commit-prefix gate |
| Combined touched-package tests | PASS |
| `go vet ./...` | PASS after final live-derived repairs |
| Bridge race suite | PASS after cursor-order fix |
| Runner race suite | PASS |
| Repository-wide `go test -count=1 ./...` | PASS after final live-derived repairs |
| Affected integration-tag suites | PASS, including the complete Core integration suite and composed resume no-work proof |
| Documentation projection check | PASS after final documentation update; expected user-phase warnings only |
| Native commit gate | PASS for all six code batches; the documentation batch is gated after its final edit and before promotion |
| Go/code-simplification review | PASS after final Triage and tracked-symlink corrections |
| Defensive/code-simplifier review | PASS after final Triage and tracked-symlink corrections |
| Architecture review | PASS; package boundaries, host provenance, resume parity, and documentation approved |
| Controlled live wave 1 | PASS: useful tested change shipped in disposable repository |
| Controlled live wave 2 | PASS as an evidence-backed early stop; successful no-work closeout verified by deterministic replay |

The rebased combined tree and every code batch were reviewed and tested before promotion. The final documentation batch runs the tree-bound commit gate after its last edit and before promotion.
