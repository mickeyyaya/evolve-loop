# AI-Factory Pipeline Resilience & Self-Testing — Research + Repo Grounding

**Date:** 2026-06-02
**Trigger:** `/evo:loop` goal — "improve test design & coverage so the pipeline proceeds even when it encounters issues; research how other AI factory pipelines work."
**Method:** Parallel research (external pattern survey) + repo grounding (current test state). This doc seeds a `harden`-strategy run.

---

## TL;DR — the reframe

The evolve-loop **runtime tier is already well-armored** against internal failure (real fault-injection tests at every scary seam). So "raise coverage so the pipeline survives issues" is partly solved — chasing a coverage % would be low-value. The two real holes are about **false confidence** and **missing chaos tests**:

1. **111-site silent-skip anti-pattern** in `acs/cycle*/` — predicates `t.Skip` on missing artifacts after a spurious `Errorf`, so on a fresh clone a third of predicates (48 SKIP / 86 PASS measured) never run their real assertion → suite goes **green-by-absence**. A regression gate that cannot fail closed.
2. **No chaos/negative tests** that feed a phase a *malformed / truncated / stream-cut* upstream artifact and assert graceful WARN-completion (the "does the pipeline survive a bad artifact" test the goal is really asking for).

---

## (A) External resilience patterns

- **Compound-failure math is the motivation.** 10 steps × 99% = ~90% end-to-end; × 95% = ~60%. Resilience is the dominant term in whether multi-phase pipelines finish. ([Anatomy of an Agent Harness](https://blog.dailydoseofds.com/p/the-anatomy-of-an-agent-harness))
- **Errors-as-observations (most important).** OpenHands catches tool/phase errors *inside* the loop and returns them as structured Observations the next turn reads — the loop stays alive and self-corrects. ([OpenHands deep dive](https://dev.to/truongpx396/openhands-deep-dive-build-your-own-guide-1al0))
- **Four-tier error router:** transient→backoff, model-recoverable→reflect, user-fixable→escalate/DLQ, unexpected→fail loud. Makes "skip-and-continue vs fail-loud" a typed decision. ([Anatomy](https://blog.dailydoseofds.com/p/the-anatomy-of-an-agent-harness))
- **Typed cross-provider error taxonomy.** OpenHands normalizes every provider error into ~7 stable types under `LLMError` so recovery is CLI-agnostic — fits our "any CLI × any phase" invariant. ([OpenHands error handling](https://docs.openhands.dev/sdk/guides/llm-error-handling))
- **Devin's documented weakness:** persisting on infeasible paths instead of failing fast = missing *semantic* dead-step detection. ([DeployHQ](https://www.deployhq.com/guides/devin))

## (B) Self-healing / retry

- Checkpoint at step boundaries; resume by skipping completed steps. ([MightyBot](https://mightybot.ai/blog/fault-tolerant-ai-agent-pipelines/))
- Idempotency at the **tool/effect layer**, not the generation layer (LLM retries diverge — naive idempotent-retry does NOT transfer). ([Chaos Engineering for AI Agents](https://tianpan.co/blog/2026-04-12-chaos-engineering-ai-agents-injecting-failures-before-production))
- Bounded retries, hard cap ~2 (matches our `EVOLVE_PHASE_MAX_ATTEMPTS=2`).
- **Quality circuit breaker:** N consecutive low-quality outputs trips open → fall back to a different model family (we already do cross-family Auditor; generalize). ([DZone](https://dzone.com/articles/algorithmic-circuit-breakers-agent-safety))
- Dead-letter queue for persistently-failing items instead of crash-or-loop. ([MightyBot](https://mightybot.ai/blog/fault-tolerant-ai-agent-pipelines/))

## (C) Self-testing patterns

- **Eval graduation:** high-pass capability eval "graduates" into a continuous regression suite; **100% pass = stagnation alarm.** Start with 20–50 tasks from real failures. ([Anthropic — Demystifying evals](https://www.anthropic.com/engineering/demystifying-evals-for-ai-agents))
- **Grade outcomes, not the path.** Layer code-based + LLM-as-judge + human. Read the transcripts.
- **Chaos engineering for agents** — inject failures the agent will actually face: LLM 429/500/timeout/**stream-cut mid-response**, tool **malformed responses**, context degradation, **silent failures** (plausible-but-wrong). ([Chaos](https://tianpan.co/blog/2026-04-12-chaos-engineering-ai-agents-injecting-failures-before-production); [agent-chaos](https://github.com/deepankarm/agent-chaos))
- **Semantic validation > operational success** — check the output is *correct*, not just that the call returned 200.
- **Mutation-guided eval strengthening** — mutate code, check whether evals kill the mutants; quantitatively replaces a heuristic tautology detector. ([Mutation-Guided Diagnosis](https://arxiv.org/pdf/2604.01518))
- **SWE-bench golden gate:** ground-truth patch must pass fail-to-pass AND pass-to-pass in isolated containers. ([SWE-bench FAQ](https://www.swebench.com/SWE-bench/faq/))

## Top transferable ideas (source → how we'd apply)

1. **Errors-as-observations** (OpenHands) → generalize the backfill flag: a malformed Build artifact should *inform* Audit, not kill the cycle.
2. **Agent-chaos negative tests as a CI gate** (agent-chaos) → feed each phase a malformed/truncated prior artifact, assert WARN-complete. **This is the headline deliverable.**
3. **Four-tier error router** (Anatomy) → replace per-phase ad-hoc retry with one typed classifier.
4. **Semantic dead-step detection** (Devin weakness) → detect same-diff/same-error-twice, route to DLQ rather than burn retry budget; complements the time-based phase-observer.
5. **Mutation-guided eval kill-rate** (arXiv) → quantitative upgrade to `evolve eval quality-check`.
6. **Eval graduation + saturation alarm** (Anthropic) → formalize capability→regression lifecycle; alarm at 100% pass.

---

## (D) Repo grounding — current test state

- **Build:** `go build ./...` clean. **CI:** green (`.github/workflows/go.yml` excludes `acs/`). Full-tree `go test ./...` = **1 FAIL**: stale version-pin `acs/cycle106/predicates_test.go:147 TestC106_011_BinaryVersionIsV12_1_1` (asserts `12.1.1`; repo is v16.1.0). Not a runtime regression.
- **Scale:** 451 `*_test.go`, ~80 prod packages, 54 `acs/cycle*` predicate dirs (CI-excluded).
- **Well-armored seams (confirmed fault-injection):** subagent crash/non-zero-exit (`subagent_test.go`), malformed/missing artifacts (`TestClassifyArtifact_*IsIntegrityFail`), bridge artifact-timeout (`bridge/engine_artifact_timeout_test.go`), transient-retry exhaustion (`core/orchestrator_transient_test.go`), ship recovery + integrity (`ship/*_test.go`), checkpoint/resume corruption (`checkpoint_test.go`, `core/resume_test.go`), backfill degradation (`backfill_test.go`).

### Prioritized gaps
1. **HIGH — 111-site silent-skip.** `grep -rn "if !acsassert.FileExists" acs/ → 111`. `acsassert.FileExists` (`pkg/acsassert/assertions.go:43`) calls `tb.Errorf` AND returns false → predicates then `t.Skip`, a confused double-signal. Fix: pure `FilePresent(path) bool` helper (no Errorf) for skip-guards; reserve `FileExists(tb,…)` for real assert sites. Contained entirely within `acs/`.
2. **MEDIUM — no direct floor-violation phase-gate test.** Integrity floor (`ship ⇒ build ∧ audit ∧ tdd`) validated only indirectly at clamp layer (`core/floor_activation_scenarios_test.go`). No direct "floor-violating plan rejected at gate" negative test.
3. **MEDIUM — phase-entry artifact-shape regressions.** Each `internal/phases/<phase>` has 1 test file; a structurally-valid-but-semantically-wrong artifact slips through unless a downstream gate happens to assert it.
4. **LOW — coverage gate warning-only** (`go.yml:83-85`, `exit 0`; documented 85%/internal-pkg target in `go/Makefile:87`).

### Test infra (present & good)
`go/Makefile`: `make test` (`-race -cover`, excludes acs), `make cover` (coverprofile + 85% gate). Tiered build tags: `integration`, `e2e`. 218 test files use inject/fake/stub/malformed/timeout idioms. Seams: injectable `gitHEAD`, `stdoutFilterFn`, `fakeT`. Eval tooling: `internal/evalqualitycheck` (tautology), `diversity.go`, `internal/verifyeval`, wired via `cmd_eval.go`.

---

## Cross-cutting caveat
Classic idempotent-retry and call-volume circuit breakers do **not** transfer cleanly to LLMs (retries diverge; the failure is in the *decision after* an error, not the call count). Apply idempotency at the tool/effect layer and breakers on output quality.
