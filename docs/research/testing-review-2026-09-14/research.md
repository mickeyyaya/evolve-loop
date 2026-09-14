# Research: testing AI agents and their harnesses

Researched 2026-09-14 from primary sources. These are external findings and their application to this repository, not claims that the proposed features already exist. No new testing framework or paid model service is required for the initial refactors.

## Outcome-based agent evaluation

Anthropic distinguishes the agent harness from the evaluation harness, and the final environment outcome from the agent's transcript. It recommends deterministic grading where possible, balanced positive/negative cases, isolated trials, calibration of model judges, and inspection of transcripts to find unfair graders. Capability evaluations and regression evaluations serve different purposes. Repeated trials expose the difference between success at least once and reliable success every time. [Demystifying evals for AI agents, published 2026-01-09](https://www.anthropic.com/engineering/demystifying-evals-for-ai-agents).

**Application:** retain ACS host execution and evidence sealing as the objective floor. Treat AI-generated report text as data. Add known-good, known-bad and gaming controls to each grader. Separate harness error, required-test skip and product failure in results. Keep live-provider quality measurements separate from deterministic regression gates. Phase order remains an explicit invariant even though incidental agent tool-call sequences need not be pinned.

## Long-running harness behavior

Anthropic's long-running agent work uses explicit progress artifacts, small increments and verification to reduce premature completion and poor handoffs between sessions. Its examples emphasize checking implemented behavior and keeping the environment understandable to the next invocation. This is implementation experience, not a formal guarantee that any particular harness is correct. [Effective harnesses for long-running agents, published 2025-11-26](https://www.anthropic.com/engineering/effective-harnesses-for-long-running-agents).

**Application:** replay interruption, restart and repair-round transitions with controlled inputs. Verify the saved state and artifacts rather than trusting a final success sentence. The discovered legacy simulator illustrates why a happy injected test must be complemented by a test of the installed command's actual composition.

## Adversarial test generation

Google describes identifying adversarial inputs, finding or creating datasets, generating and annotating outputs, then reporting and mitigating failures. Its dataset guidance considers both lexical and semantic diversity and implicitly harmful inputs; it also warns against noisy duplicated examples. Uncertain automated judgments need reviewed labels. [Adversarial Testing for Generative AI, updated 2025-08-25](https://developers.google.com/machine-learning/guides/adv-testing).

**Application:** start from real incidents, add boundary and rejection cases, then vary inputs without restating the same invariant. For each acceptance test, identify the cheapest plausible implementation that could falsely pass. Corpus growth should add new fault sensitivity, not merely more cycle-numbered copies. Persist the minimal reproducer and its independent expected result.

## Fail-to-pass and pass-to-pass evidence

SWE-bench's harness exposes separate `FAIL_TO_PASS` and `PASS_TO_PASS` expectations in its grading logic. This provides a useful explicit distinction between repairing the target defect and preserving already-working behavior. [SWE-bench harness API and grading source](https://www.swebench.com/SWE-bench/api/harness/).

**Application:** every behavior fix needs a witnessed regression plus preservation tests. A pure test refactor preserves the existing pass set and detects the same representative faults. Compilation failure, unrelated infrastructure failure, and a nonexistent selected test do not establish the intended fail-to-pass contract. Preserve test identity as well as command exit status.

## Evaluation logs and isolation

Inspect documents evaluation logs that retain per-sample information, scores, errors and execution events. Its sandboxing documentation distinguishes code running in the host evaluator from work explicitly sent through the sandbox interface. [Inspect evaluation logs](https://inspect.aisi.org.uk/eval-logs.html), [Inspect sandboxing](https://inspect.aisi.org.uk/sandboxing.html).

**Application:** retain host/child provenance in Go test and ACS receipts; do not infer containment from the presence of a sandbox option. Require a successful harmless subprocess control before treating a denied operation as evidence of isolation. A timeout or failed child startup cannot prove the sandbox blocked the requested access. Keep existing real OS containment tests.

## Native Go fuzzing and integration coverage

Go runs committed fuzz seeds during ordinary tests; fuzzing campaigns generate additional inputs. Failing inputs are minimized and can become durable regression seeds. Keep fuzz targets fast and deterministic and bound campaign runtime. [Go fuzzing guide](https://go.dev/doc/security/fuzz/).

Go also supports instrumenting binaries for integration coverage, collecting data through `GOCOVERDIR`, and merging/converting results with `go tool covdata`. This is useful for real subprocess paths that are not represented by the parent test binary's coverage. [Coverage profiling for integration tests](https://go.dev/doc/build-cover).

**Application:** extend the two existing fuzz targets by risk, using native fuzzing and the already-present `rapid` dependency where suitable. Store minimized regressions. Do not interpret parent CLI-test coverage as coverage of an uninstrumented child binary. Compare coverage on the same code/toolchain/OS/tag tuple; merging profiles does not establish assertion equivalence.

## GitHub selection and Go toolchain support

GitHub documents the interaction of path/branch filters and required checks, including workflows that stay pending when skipped. Concurrency controls can cancel obsolete runs, but must be scoped to avoid canceling publication work. [Workflow triggers](https://docs.github.com/en/actions/how-tos/write-workflows/choose-when-workflows-run/trigger-a-workflow), [workflow concurrency](https://docs.github.com/en/actions/how-tos/write-workflows/choose-when-workflows-run/control-workflow-concurrency).

Go supports a major version until two newer major versions exist. At research time, the release history lists Go 1.27.1 and 1.26.8 as the latest patches on the supported lines; repository workflows exclusively select 1.23. [Go release policy and history](https://go.dev/doc/devel/release).

**Application:** test suite selection as code, provide a stable required result, and verify the tagged release revision. Retain minimum-version compatibility where promised while adding a supported toolchain; do not silently raise `go.mod` or use newer testing APIs in a 1.23 lane. Preserve OS-specific evidence and publish first-failure logs before any permitted retry.

## Decisions and rejected alternatives

| Decision | Reason |
|---|---|
| Reuse native Go, existing fixtures and ports | Existing runner/evidence/real-process infrastructure is substantial; a second framework adds maintenance and parity risk |
| Test the evaluator before increasing AI-generated test volume | Confirmed exit-status and AST-scanner defects show the grader can be wrong while tests stay green |
| Keep outcome assertions plus necessary structural invariants | Runtime behavior and source architecture are distinct legitimate contracts |
| Require semantic old/new maps and representative fault tests | Equal coverage and similar source cannot establish equivalent tests |
| Keep live trials optional and statistically explicit | Model variability and provider availability differ from deterministic software failures |
| Avoid universal corpus DSL, global fixture registry and blanket table conversion | Small package-local representations keep setup and assertions understandable |
| Do not require a new external mutation tool for the first slice | The initial defects can be reproduced deterministically with existing Go/Make; any later tool must be pinned and measured |

This research supports the [design](../../architecture/test-refactoring-design-2026-09-14.md) and [case catalog](case-catalog.md). It does not establish measured coverage, model reliability, or mutation scores; those require the repository-specific execution evidence described there.
