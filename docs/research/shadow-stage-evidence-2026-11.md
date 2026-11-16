# Shadow stage evidence: what failed, and what current practice says (research, 2026-11)

> Design set: plan [shadow-stage-evidence-2026-11](../plans/shadow-stage-evidence-2026-11.md) · decision [ADR-0167](../architecture/adr/0167-shadow-stages-prove-themselves-on-chained-evidence.md) · spec [shadow-stage-contract](../architecture/shadow-stage-contract.md) (the spec of record; this file does not repeat its detail).
> Related: [integration-driver-enforce](../architecture/integration-driver-enforce.md) (the EnforceActor card), [ADR-0165](../architecture/adr/0165-every-gate-is-a-typed-unit-with-a-conformance-contract.md) (the gate contract) and [ADR-0157](../architecture/adr/0157-the-loop-works-its-own-code-a-narrow-fence-and-held-proposals.md) (held proposals).

Date: 2026-11-15. Method:

- three fault-localization reports of 2026-11-15 (A: scan skip; B: main-red stop and hold; C: silent shadow features and the retry ladder);
- the evidence table of the operator for cycles 2356 to 2390;
- eight live web searches on 2026-11-15.

Each local claim names its source. "Synthesis" marks our own inference. The reports read code at HEAD `64bcd83c4`; line numbers move with edits.

## Table of contents

- [1. The question](#1-the-question)
- [2. The seven local findings](#2-the-seven-local-findings)
- [3. Outside practice](#3-outside-practice)
- [4. What each practice implies for evolve-loop](#4-what-each-practice-implies-for-evolve-loop)
- [5. Gaps in the evidence](#5-gaps-in-the-evidence)

## Terms used throughout

- **Stage key:** a `stage` field of `.evolve/policy.json` with the values `off`, `shadow` and `enforce`.
- **Shadow:** a feature computes and records its decision. The pipeline acts as if the feature were off.
- **Forecast field:** a `would_*` field of a signal. It states what `enforce` will do.
- **False stop:** a `would_stop` on a cycle that then shipped with a PASS.
- **Escape:** a `would_skip` on a check that then failed.
- **Evidence record:** one shadow decision, bound into a hash chain.
- **Scan skip:** the ship stage that skips the scan of a package when a stored green result has the same key.
- **NODATA:** the state of a canary metric that has no data.

## 1. The question

The operator asked on 2026-11-15: "Carefully review the shadow features and fix the issues if we observed any." Then: "Build a strong set of test cases to prevent this type of issue from happening." Then: "The evidence and contract is the core of this feature, we must keep both intact and be able to verify, otherwise the whole pipeline is untrustworthy."

The question has two parts:

1. Which shadow features cannot prove themselves, and why?
2. What must the store of shadow evidence guarantee, so that anyone can check the fact behind a flip to `enforce`?

## 2. The seven local findings

**F2.1** Every pool record of the integration train is "not clean" only because of a redelivery. Pool 2373 holds cycle 2374. Pool 2378 holds cycles 2379, 2382, 2383 and 2386. Each record says that the cycle "was delivered again under new patches after it was composed" (operator evidence table; [integration-driver-enforce](../architecture/integration-driver-enforce.md), redelivery items R-a to R-d).
*Implication:* the gate "two clean shadow waves" cannot be met by good behavior. The stage cannot reach its promotion condition.

**F2.2** The ship scan skip keys its verdict cache on the whole tree. `scanShadow.keyOf` passes `Log{Unknown: true}`, which forces the whole-tree key (`go/internal/phases/ship/scanshadow.go:214,220`; `cacheable` is at line 208; `go/internal/testkey/key.go`, `Compute`). 

The key hashes the package path, the Go version, 15 build variables, the tags and the tree.

The escape counter fired on `internal/acssuite` (cycle 2362) and on `internal/guards` (2026-11-13), about 2 escapes in 62 scans (3%).

These packages read the host environment, the `.git` state and run subprocesses (report A; `go/internal/acssuite/acssuite.go:233`). The tree hash does not cover these inputs.
*Implication:* the key is sound only if the test is a pure function of the tree. Report A ranks the cause at 0.55 for the environment, 0.25 for the uncovered file set and 0.20 for a flake. Nobody saw the failing log of cycle 2362, so the cause is a hypothesis.

**F2.3** The main-red stop opens only from `go test` keys of the lane floor. `reportAttributed` calls `openMainRedStop` (`go/internal/core/base_red_floor.go:182-207`). `stopOpening.through` returns when `MainRed` is empty (`go/internal/core/main_red_stop_open.go:99-100`). The key type is `baseattr.Key{Package, Test}` (`go/internal/baseattr/baseattr.go:15`). 

The required CI check "golangci-lint: Lint" was red on main from 2026-11-14T16:03Z for 5 commits (operator evidence table). The stop saw nothing: `checkMainRedStop` returns `nil, nil` when no stop holds (`go/internal/phases/ship/main_red_stop.go:27-30`).
*Implication:* a lint red has no `Package.Test` key. The stop cannot open for it, and `proof` (line 54) cannot prove a fix for it.

**F2.4** Two copies of one rule disagree. `heldproposal.Decide` holds any kind with the prefix `pipeline-` (`go/internal/heldproposal/heldproposal.go:52-56`). `inboxbatch.ConsoleRouted` sends an operator-filed `pipeline-*` item with `route: lane` to a lane (`go/internal/inboxbatch/consoleroute.go:36-45`). 

At `enforce`, the stage holds the PASS landings of cycles 2379, 2383, 2384, 2386 and 2388 with the reason `kind:pipeline-repair` (operator evidence table; report B). Nobody opened these cycle records, so it is not proven that each item had `route: lane`.
*Implication:* the routing decision of 2026-11-13 has no effect on a landing at `enforce`. This is a duplicated rule, which the hard rules forbid.

**F2.5** Three shadow features give no signal or no behavior (report C).

| Feature | State at HEAD | Source |
|---|---|---|
| `audit_cascade` | writes `audit-cascade-shadow.json`, emits no signal | `go/internal/phases/audit/cascade_shadow.go:14-37` |
| `regression_tia` | writes `acs-tia-shadow.json`, emits no signal, can skip at most 9 of 44 predicates | `go/internal/phases/audit/audit.go:382-396`; `go/internal/regressiontia/regressiontia.go:114-149` |
| `parallel_evaluate` | `shadow` acts as `off` | `go/internal/core/orchestrator.go:624` |

For `regression_tia`, `resolveDeps` drops each predicate whose dependency closure reaches `os`, `os/exec`, `os/user`, `syscall`, `net` or `net/http`. In `go/acs/regression/`, 35 of 44 predicate directories import one of them directly.
*Implication:* a review that greps `signals.ndjson` for these features finds nothing, so "0 of 101" is not a measure of the feature. No soak data can exist.

**F2.6** The retry ladder has a second rule outside its home. The record of cycles 2362 and 2377 reads "unknown at R0 stop R0" (action `stop`, empty fingerprint, empty tier).

`retryladder.Next` at HEAD raises an `Unknown` verdict at R0 (`go/internal/retryladder/decide.go:84-87`), and a test pins this (`decide_test.go:300`). The second caller, `cycleRun.escalatedBuildTier` (`go/internal/core/retry_tier_escalation.go:33`), does not call `retryladder.Next`. It calls `nextItemStartRung` (`go/internal/core/retry_ladder.go:209-231`), a hand-coded copy of the decision:

- it starts from the literal `Decision{Action: ActionStop, Rung: R0}` (line 210);
- it raises only for the class `Capability`;
- it emits that literal with an empty `retryladder.Verdict{}` (line 228).

This matches the recorded fields exactly. The cause is therefore found, and it is not a stale binary and not an incomplete input. It is a second decision rule outside `retryladder`.
*Implication:* `escalatedBuildTier` uses the start rung only to suppress the ADR-0076 tier raise at `enforce` (`retry_tier_escalation.go:48-50`). The effect at `enforce` is a lost deterministic escalation for an `unknown` failure, not a stopped build. No input builder can fix this, because the function never builds an input.

**F2.7** The cycle spend limits stop shipped cycles. `max_audit_rounds` 2 stops 40 of 143 shipped cycles in 2200 to 2390 (28%). `max_llm_minutes` 90 stops 9 of 35 shipped cycles in 2356 to 2390 (operator counts; not recomputed). The limiter sums `DurationMS` of all records of the `llm-calls` ledger, so overlapping calls add up (`go/internal/cyclespend/spend.go:33-37`; `LLMMinutes` is at line 90).
*Implication:* a limit set by guess is wrong for the data. A scorer must derive it from shipped cycles.

**F2.8** The evidence has no integrity. The signal stream is a plain append file (`go/internal/signalcenter/sinks.go:22-28`). 

The ledger is chained: `entry_seq` and `prev_hash` (`go/internal/core/ports.go:69-70`), checked by `sealChainsFromPrev` (`go/internal/adapters/ledger/anchor.go:30-38`) and by `VerifyCycle` (`go/internal/ledgerverify/verify.go:65`). 

A shadow decision that lives only in a signal can change, vanish or be cut with no detection. The registry and the conformance tests are ordinary files outside the protected surface (`go/internal/guards/integrity_surface.go:37,270`).
*Implication:* a verdict `ready` that reads only signals is a claim that nobody can check. This is the finding that the operator ranks first.

## 3. Outside practice

The sources are practitioner runbooks and project documents, not standards. Only the URLs below are cited.

**P3.1 Shadow and dark-launch promotion.** Practitioner runbooks give six rules:

- define the exit criteria before the shadow run;
- gate on measured deltas;
- keep a rollback path;
- set a match-rate threshold for promotion;
- use enough sample (one quiet hour is not a baseline);
- do not promote during an unrelated incident window.

Sources: ([LLD dark launch](https://www.techinterview.org/post/3233468857/lld-dark-launch/), [Fairvisor shadow-mode rollout](https://docs-fairvisor.pages.dev/docs/cookbook/shadow-mode-rollout/), [shadow to canary workflow](https://my-slops.github.io/Blog/posts/2026/04/shadow-to-canary-llm-release-workflow/), [shadow to autopilot](https://tianpan.co/blog/2026/04/20/shadow-to-autopilot-ai-feature-autonomy)).

**P3.2 Automated canary judgment.** The Kayenta judge has these parts:

- a pluggable judge;
- a nonparametric test for each metric;
- significance before effect size;
- a weighted score from 0 to 100, with pass, fail and human-approval limits.

Source: ([Spinnaker judge](https://spinnaker.io/docs/guides/user/canary/judge)). 

Empty data is NODATA, and the metric is skipped. The option `nanStrategy` sets this, so a misconfiguration changes the gate without a sign. Repeated looks at the data inflate false positives. The remedy is a fixed window or a sequential test, and a correction for many metrics ([canary-analysis skill](https://claudskills.com/skills/canary-analysis/SKILL.md)).

**P3.3 Tamper-evident logs.** Trillian keeps an append-only Merkle tree. It gives inclusion proofs, consistency proofs and signed tree heads. Monitors keep old tree heads to detect equivocation. A leaf hash is SHA-256 of `0x00` and the value, for domain separation.

A client that does not fully trust the log stores what it signs. Sources: ([transparency.dev](https://transparency.dev/verifiable-data-structures/), [Trillian transparent logging](https://google.github.io/trillian/docs/TransparentLogging.html), [verifiable data structures](https://google.github.io/trillian/docs/papers/VerifiableDataStructures.pdf), [auditable data structures](https://arxiv.org/pdf/2306.01886)). 

A plain hash chain is tamper-evident, but verification replays from the start (general knowledge; not from the sources).

**P3.4 Provenance binding.** In SLSA the builder id is untrusted and must be verified downstream. An in-toto statement binds subject digests to a predicate. A policy that checks only the validity of a signature accepts anything that anyone signed ([slsa-verifier](https://github.com/tahirraza/slsa-verifier), [SLSA provenance v1-rc1](https://slsa.dev/provenance/v1-rc1), [Bazel book](https://bazel.virtuslab.com/book/6~7~6/)).

**P3.5 Test caching and hermeticity.** A cached test result is correct only when declared inputs are the whole cause of the result. An undeclared input gives a false hit and poisons a shared cache. A sandbox enforces hermeticity but does not define it. The host environment and the network are separate leaks ([Bazel book 2.3.1](https://bazel.virtuslab.com/book/2~3~1/), [2.4.3](https://bazel.virtuslab.com/book/2~4~3/), [1.2.1](https://bazel.virtuslab.com/book/1~2~1/)). 

The Go test cache records the environment variables and files that a test reads through a back-channel in package `os` (`internal/testlog`). It reruns the test when a recorded input changes. Files opened in `TestMain` are not tracked (issue 24122). Files younger than 2 seconds are not cached ([testlog](https://pkg.go.dev/internal/testlog@go1.27.2), [cache input test](https://tip.golang.org/src/cmd/go/testdata/script/test_cache_inputs.txt), [issue 24122](https://golang.org/issue/24122)).

**P3.6 Merge queues.** A queue batches changes into a temporary combined change, runs CI, merges the batch and fast-forwards main to the batch head. On a failure it bisects, requeues the innocent members and reuses test results during the bisection ([Graphite](https://graphite.com/blog/merge-queue-batching), [Mergify](https://mergify.com/learn/merge-queue/batching), [Trunk](https://docs.trunk.io/merge-queue/concepts/batching)). The Zuul dependent pipeline runs speculative builds that assume the changes ahead pass. On a failure it removes the failed change and restarts the builds behind it. It reports a merge conflict apart from a test failure, and a window limits the active changes ([Zuul gating](https://zuul-ci.org/docs/zuul/9.4.0/gating.html)).

## 4. What each practice implies for evolve-loop

| Id | Practice | Implication (synthesis) | Finding |
|---|---|---|---|
| I4.1 | Exit criteria before the run (P3.1) | The `promotion` condition is data in the registry. A stage without it cannot have a row. | F2.1, F2.7 |
| I4.2 | NODATA is skipped (P3.2) | A stage that records nothing must score `not ready`, never `ready`. An empty chain is a typed error. | F2.5 |
| I4.3 | Fixed window against repeated looks (P3.2) | The verdict reads a fixed range of cycles that `evolve stage verify` prints. A new look is a new verdict. | F2.7 |
| I4.4 | Plain chain replays from the start (P3.3) | The cost of `verify` grows with the range. A per-cycle digest in the ledger bounds the replay to the range of one stage. | F2.8 |
| I4.5 | Signed tree heads defend against a rewrite (P3.3) | A party that rewrites ledger and sidecar together can forge evidence. A checkpoint outside the repository is the remedy. It needs a key. | F2.8 |
| I4.6 | Bind the digest to what was measured (P3.4) | Each record names the decision version: a content digest of the Go package closure of the decider plus the policy subtree that it reads. It is not the build id and not the whole policy. The loop ships its own code each cycle. A build id therefore changes about once per cycle, and evidence never reaches the sample size. Readiness counts the records of the current version. | F2.2 |
| I4.7 | An undeclared input gives a false hit (P3.5) | The cache is default-deny: only a package that is proven pure is cacheable. One purity predicate serves the scan skip and the TIA (`regressiontia.escapeHatches` and its closure walk). A hand list is an override only. | F2.2, F2.5 |
| I4.8 | Go tracks env and files, not `TestMain` files or children (P3.5) | The Go cache (a dynamic, testlog-tracked key) is the comparison for the static key, not the key itself. The repository rule says `-count=1` is required because the cache does not see files that tests read through `..` (`CLAUDE.md`). | F2.2 |
| I4.9 | Report a conflict apart from a failure (P3.6) | A scorer must count a false stop, an escape and a conflict as three classes. | F2.4, F2.6 |
| I4.10 | Reuse a result only for the same content (P3.6) | A scan skip is valid only for an identical key. An escape is a scorer finding that proposes an override. It never adds a package to the cache by itself. | F2.2 |

## 5. Gaps in the evidence

- The raw artifacts of cycles 2356 to 2390 and their signals were not in the worktree. The claims about those cycles come from the three reports and from the operator.
- The failing log of cycle 2362 was not read. The cause of the `acssuite` escape is a ranked hypothesis.
- The cause of the record "unknown at R0 stop R0" comes from a match of the recorded fields with the literal in `nextItemStartRung`. The raw events of cycles 2362 and 2377 were not read.
- The cycle records behind the five held landings were not opened.
- The counts of F2.7 (40 of 143, 9 of 35) are the operator's. The per-cycle ledger values were not available.
- The outside sources are runbooks and project pages. None sets a number for a sample size.
- The claim that the Go testlog does not see child processes is general knowledge. No cited source states it.
- It is not verified that the test binary accepts a stable `testlogfile` option for a user tool. The plan treats this as open.
- The cost of the content digest of a Go package closure is not measured. The plan reuses the closure walk of `regressiontia`.
- No source here measures the cost of Merkle log verification for a log of this size. The plan gives an estimate.
