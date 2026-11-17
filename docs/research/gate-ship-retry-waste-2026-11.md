# Gate, ship and retry waste: where lane time goes, and how merge queues avoid it (research, 2026-11)

> The plan that uses this research: [gate-ship-retry-waste-2026-11](../plans/gate-ship-retry-waste-2026-11.md). The design and the decision record do not exist yet. The plan names them.

Date: 2026-11-16. Method: a read of the run records of waves 173-175, a read of the code at `origin/main` (`1088737ba`), and a web survey. Each local claim names its file, its code line or its command. "Synthesis" marks our own inference.

## The request

The operator wrote on 2026-11-16: "Good finding, then working on the improvement tasks and designs to cut down the wasted time on the gate, ship, retries, ultrathink".

The "finding" is the lane-time split of wave 174 (F1.1). This dossier gives the evidence and the current practice outside. It does not decide. The plan decides.

## Terms used throughout

- **Lane:** one cycle that runs in a fleet slot, from scout to ship.
- **Gate:** deterministic host work after an LLM call: `go test`, `make`, lint and ratchets.
- **Composed gates:** the four `make` targets that re-check the lane after a rebase onto a moved `main` (`go/internal/ciparity/composedgates.go:3`).
- **Repo-contract gate:** the ship-side scan of a fixed package pack, the added tests and the importer closure (`go/internal/phases/ship/repocontract.go:441-455`).
- **Importer closure:** the changed packages and every package that imports them.
- **Rebind:** the ship recovery that keeps the audit after a byte-identical rebase and green composed gates.
- **Re-audit:** a new LLM audit round that a ship error starts.
- **Lane budget:** the CPU share of one lane, `hostCPUs/width` (`go/internal/lanebudget/lanebudget.go:20-25`).
- **PQ9:** the pipeline health index for the post-LLM gate share of build and audit.

## Table of contents

1. [Local evidence](#1-local-evidence)
2. [Outside practice](#2-outside-practice)
3. [Adopt or reject](#3-adopt-or-reject)
4. [Refinements to adopt](#4-refinements-to-adopt)
5. [Limits](#5-limits)

## 1. Local evidence

Sources of the numbers:

- the per-cycle `signals.ndjson` and `llm-calls.ndjson` files under `runtime/.evolve/runs/cycle-2379` to `cycle-2427`;
- the same split rule as `go/internal/lanetime/phases.go`: gate time is host time after each LLM call;
- `evolve wave latency --wave 175 --json` and `evolve wave status --json`, read on 2026-11-16 at about 04:40Z;
- the loop logs `runtime/.evolve/logs/20261114T140713Z`, `20261115T110559Z` and `20261116T035132Z`.

Wave 173 is cycles 2379-2399. Wave 174 is cycles 2401-2420. Wave 175 starts at cycle 2422 and was live at read time. Each number below carries its wave. The code lines were read at `1088737ba`; no cited Go file changed since the read commit `3c42e3de0`.

### Worked example: cycle 2419 (wave 174)

The audit passed at 23:56:07Z on 2026-11-15. The early fast-forward check failed with `GIT_FLEET_REBASE_NEEDED` 23 s later. The rebase and the composed gates took 54 min, and then the gates declined on a treewalk leak, not on a failed test. A re-audit followed: 12 min of LLM time and 37 min of audit gates.

A peer landed in that 103 min window, so the lane rebased again. The composed gates took 57.7 min, the rebind passed, and the ship gate took 23 min. The cycle landed at 03:00:47Z, 185 min after the first audit PASS. The whole lane ran 334 min, and its LLM time was 76 min (23%).

### F1.1 Lane time split

| Wave | Lane h | Model h | Post-LLM gates h (PQ9) | Ship h (all host) | Gates and ship share |
|---|---|---|---|---|---|
| 173 | 40.3 | 14.9 | 10.4 (0.26) | 14.42 | 0.62 |
| 174 | 48.7 | 18.2 | 16.7 (0.343) | 13.28 | 0.62 |

Gates and ship are 62% of lane time in waves 173 and 174. The LLM is about 37%. In wave 174, ship took 13.28 h in 30 spans. Ship spans have no LLM call, so all ship time is host time. The ship and lane totals match `evolve wave latency`. The span data of wave 174 sum to 48.2 h of span time and 13.28 h of ship time.

*Implication:* an LLM speed-up cannot cut more than about a third of lane time. The deterministic host work is the larger target.

### F1.2 The CPU budget

`lanebudget.Procs` returns `max(1, hostCPUs/width)` (`go/internal/lanebudget/lanebudget.go:20-25`). The pool passes its limit, not its live count, as the width (`go/internal/fleet/pool.go:258`, `poolSpec(td, p.limit)`). The host has 14 CPUs and the pool limit was 8, so each lane gets 14/8 = 1 CPU. Every lane `go test` then runs at `GOMAXPROCS=1` and `-p 1`:

- the build floor cover run (`go/internal/core/build_floor_reviewer.go:281-285`);
- the composed gates (`go/cmd/evolve/cmd_composition_wiring.go:135`);
- the repo-contract gate (`go/internal/phases/ship/repocontract.go:146`, `:223`);
- the audit integration tier (`go/internal/phases/audit/ciparitygate/tier.go:19-29`).

The mean live lane count was 2.30 (wave 173) and 3.10 (wave 174). Thus about 10 CPUs were idle while gates ran in series. Gate time does not correlate with the live lane count:

| Gate window (waves 173-174) | n | Pearson r (live lanes, minutes) |
|---|---|---|
| Build post-LLM | 60 | 0.03 |
| Audit post-LLM | 57 | -0.17 |
| Composed gates after a rebase | 16 | -0.32 |

A lone lane (live count below 2) still spent a median 54.7 min in the composed gates. PR #1448 (`9c5aafd5a`, 2026-11-14) put the composed gates on the lane budget. It cured the host load of 50-75 in wave 171, but it moved the cost from host load to wall time.

*Implication:* the budget divides the host by slots that do not run. A budget from the live lanes, or a host token pool, gives the same safety and returns the idle CPUs.

### F1.3 The verify lock puts the build-floor coverage in series

The build floor cover run takes a hub-wide lock before it starts (`go/internal/core/build_floor_reviewer.go:279`, `:315-321`). The ACS suite of the audit takes the same lock (`go/internal/acssuite/acssuite.go:159`), so the waits are across both phases. The lock wait has a cap of 15 min (`go/internal/verifylock/verifylock.go:26`). After the cap, the run continues without the lock. Lock waits were 3.14 h in wave 173 (51 waits) and 7.65 h in wave 174 (75 waits). In wave 174, 19 waits hit the 15 min cap.

*Implication:* the lock and the 1-CPU budget throttle the same run twice. A floor after a 15 min wait also starts with a spent context. In wave 174, `go list` and the base read failed with `context deadline exceeded` 6-12 times.

### F1.4 Re-check after `main` moved

The composed-gate recovery after a peer landing took 8.25 h in wave 174 [wave 173: 5.33 h]. The composed gates run four `make` targets in series (`go/cmd/evolve/cmd_composition_wiring.go:99-143`):

1. `make build`;
2. `make test` with `-race -count=1 -cover` over the importer closure, and a treewalk guard (`go/Makefile:107-112`);
3. `make test-acs-durable` over all of `./acs/regression/...`, with no scope (`go/Makefile:203-204`);
4. `make apicover-enforce` with `-tags integration -count=1` (`go/Makefile:250-260`).

For lanes that change Go code, the gates took a median 55.0 min and a p90 of 60.3 min (wave 174). The window from the rebase to the next fast-forward had a median of 83 min. Peer landings were a median 66 min apart. So 4 of 12 windows in wave 174 collided with a new landing and restarted the re-check. [Wave 173: 3 of 10, a 53 min window, a 79 min gap.]

*Implication:* when the re-check is longer than the gap between landings, the re-check restarts about one time in three. A shorter re-check cuts the collisions as well as the gate time.

### F1.5 Importer backstop deadline kill

The repo-contract importer backstop runs the importer closure under a hard 20 min cap (`go/internal/phases/ship/repocontract_importers.go:15`). At 1 CPU, a closure of 80-160 packages does not finish. The retained logs show "N target(s) in 20m0s" for every kill: 2387 141 targets, 2392 160, 2401 80, 2403 160. A pack above 25 targets gets no retry in the ship (`repocontract_importers.go:16`). The gate classes the kill `REPO_CONTRACT_INFRA` with class `precondition`, "safe to re-dispatch" (`go/internal/phases/ship/repocontract.go:531-534`). The router sends each `precondition` ship error to an LLM audit (`go/internal/router/recovery.go:74-84`, `precondition-reaudit`).

So the lane gets an LLM re-audit, then the same ship on an unchanged tree, then the same kill. The cost:

| Wave | Futile kill loop | Failed lanes lost |
|---|---|---|
| 174 | 2.87 h (6 kills, cycles 2401 and 2403) | 2401 162.9 min and 2403 155.1 min, about 5.3 h of lane time |
| 173 | 4.24 h (8 kills, cycles 2382, 2387 and 2392) | the notes give about 6.3 h; this dossier cannot reproduce it |

The same path was live in wave 175: cycle 2423 entered the backstop at 04:19:53Z, ran "160 target(s) in 20m0s", and got `SHIP_REPO_CONTRACT_INFRA` at 04:41:16Z. Cycle 2423 is the third claim of its inbox item.

*Implication:* the kill is a capacity fact, known from the closure size before the ship. An LLM re-audit cannot change it, and the identical retry fails the same way.

### F1.6 Re-audits from ship errors

Ship errors started re-audits that cost 4.44 h in wave 174, of which 0.81 h was LLM time [wave 173: 2.45 h]. Each re-audit costs 5-12 min of LLM time and 10-37 min of audit gates. All 4 composed-gate declines of wave 174 (cycles 2404, 2414, 2418 and 2419) were the treewalk LEAK guard of `make test` (`go/Makefile:110-111`). Tests in `internal/core` and `internal/phases/triage` left `.evolve/` files behind; no test failed. PR #1490 (`7a3537655`, 2026-11-16) fixed the leak.

Two gates attribute a red to its base:

- the ship repo-contract gate (`go/internal/phases/ship/base_red.go:98-110`): red on the base and green on `main` gives a rebase; red on `main` gives `MAIN_RED_PARK`;
- the audit integration tier (`go/internal/phases/audit/ciparitygate/attribution.go:33-55`, `attributeToBase`), for retake reds only.

The composed gates have no base attribution. Any failed target returns `fail` (`go/cmd/evolve/cmd_composition_wiring.go:137-142`), and `declineComposedGates` falls back to a full re-audit (`go/internal/core/composition_carryforward.go:40-54`).

*Implication:* a red that is also red on `main` sends the lane to an LLM that cannot fix it. One run of the red targets on `main` before the re-audit removes this class.

### F1.7 Build floor time and correction rounds

The build floor took 9.92 h of post-LLM gate time in wave 174 [wave 173: 6.23 h]. Of that time, 7.91 h was in windows of 20 min or more. The floor runs `go test -count=1 -timeout 20m -coverprofile` per changed package (`go/internal/core/build_floor_reviewer.go:281-285`). Correction rounds cost 10.01 h in wave 174 (12 corrections) [wave 173: 4.03 h, 7 corrections]. A corrected build span had a median of 117 min; a clean span had a median of 24 min (wave 174).

Each correction re-runs the whole floor. Cheap static checks report only after the long tests:

- `ste-lint` on the cycle's own explanation document: 4 rounds of 50-120 min each;
- the `TestParallelDefault` rule, a static rule: 3 rounds, each after a 330-374 s `cmd/evolve -tags integration` run.

*Implication:* the static checks can run first and fail in seconds. A correction can re-run only the checks that failed and the packages it changed.

### F1.8 Audit gates

The audit gates took 6.74 h in wave 174 [wave 173: 4.12 h]; 2.96 h was in windows of 20 min or more. The integration tier has a 15 min first run and a 15 min retake (`go/internal/phases/audit/ciparitygate/ciparitygate.go:68-69`). The retake waits up to 5 min for a lock (`go/internal/ciparity/tierscope.go:11`). It still runs with `tierProcs()`, the lane budget of 1 CPU (`go/internal/phases/audit/ciparitygate/tier.go:19-29`, `:96-103`). Cycle 2419 hit `AUDIT_CIPARITY_TIER_DEADLINE_NO_VERDICT` twice in one 38 min audit window.

*Implication:* the lock puts the retake in series but gives it no more CPU. The retake buys no capacity.

### F1.9 Explanation-document correction on a PASS audit

When the explanation document needs a correction on a PASS or WARN audit, the orchestrator sends the lane back to Build (`go/internal/core/explanation_correction_rung.go:53-75`). A full re-audit follows. This cost 3.34 h in wave 174 over 5 cycles (2401, 2403, 2404, 2412 and 2414) [wave 173: 0.57 h, 1 cycle]. The longer path also widened the window for a peer landing: 2404 and 2414 then hit `GIT_FLEET_REBASE_NEEDED`.

*Implication:* the fix is a small, prescribed document edit. [ADR-0158](../architecture/adr/0158-a-repair-round-runs-only-for-a-finding-the-lane-can-fix-and-the-auditor-applies-a-small-prescribed-fix.md) already lets the auditor apply such a fix in place.

### F1.10 Repo-contract gate before the cheap binding check

The ship runs the repo-contract gate (`go/internal/phases/ship/ship.go:140`) before `runStages`. The audit binding check runs later, inside `verifyAndLand` (`go/internal/phases/ship/native.go:263-266`, then `verify.go:136` calls `verifyAuditBinding`, `audit.go:27-120`; the head-moved check is `audit.go:84-94`). So an `AUDIT_BINDING_HEAD_MOVED` error wastes the whole gate. In wave 174, the repo-contract gate on PASS ships took 2.45 h. Gates lost to a later head move took 0.43 h. [Wave 173: 2.23 h and 1.00 h.]

After a rebind, the next ship runs the repo-contract gate again on the closure that the composed gates just tested. The ship scan shadow (`go/internal/phases/ship/scanshadow.go`) records which packages a result reuse can skip. On an unchanged-tree retry it found 54-81% of packages skippable (for example 2382: 103-156 of 150-193). After a rebase it found 0% in all wave 174 rebind ships. It recorded 1 escape in 53 scans (cycle 2391).

*Implication:* move the binding check first, because it costs seconds. Result reuse helps only retries on an unchanged tree; a rebase needs an affected-package rule (P2.2).

### F1.11 Ship runs per shipped cycle and the time after the first audit PASS

The mean count of ship runs per shipped cycle was 2.18 in wave 174 and 2.33 in wave 173. The time from the first audit PASS to the landing splits as follows (wave 174, 11 landed cycles, 15.97 h):

| Part | Share |
|---|---|
| Re-check (composed gates) | 52% |
| Re-audit (model and gates) | 20% |
| Ship gate (repo-contract) | 18% |
| Rebuild | 9% |
| Gaps between spans | 1% |

The worst cases were 2382 (wave 173, 439 min, 8 ships, 4 audits), 2404 (wave 174, 333 min) and 2419 (wave 174, 185 min). In wave 174, the 4 cycles over 95 min held 81% of the post-PASS time.

*Implication:* the tail is a chain of re-checks and re-audits, not a wait in a queue. Cut the re-check, and the chain gets shorter.

### F1.12 Integration train I14

The train is in shadow, by default: the live policy has no `fleet.integration` key (`go/internal/policy/fleet_integration.go:14-23`). In waves 173 and 174, 23 lanes landed and 0 landed through the train. The blockers of enforce:

- `EnforceActor` exists (`go/internal/integration/enforce_actor.go:32`, cycle 2414), but the driver passes `nil` (`go/cmd/evolve/cmd_loop_driver.go:117`);
- 5 enforce ports have no production implementation (PlaneSync, LandingFeed, Requeuer, BriefStore, Promoter);
- an enforce compose needs a gate (`go/internal/integration/compose.go:165-166`), and the shadow compose stops before it (`compose.go:194`).

At enforce, the lane stops before `main` (`go/internal/phases/ship/deliverable.go:75-80`, `landing_intent.go:35`). That removes `GIT_FLEET_REBASE_NEEDED` from the lane: 22 events in waves 173-174. It does not remove `AUDIT_BINDING_HEAD_MOVED`, because `audit.go:84-94` has no stage check. With `DefaultMaxAgeMinutes = 45` (`go/internal/integration/cap.go:7`), the median wait from delivery to flush in wave 174 was 45.0 min, before any batch CI. Shadow composes text only, so there is no evidence on batch greenness.

*Implication:* enforce moves the rebase cost to a 45 min wait and keeps one error class. It is not a short path to less waste.

### F1.13 Retry caps and failure detection

The orchestrator chooses a recovery path in `go/internal/core/ship_recovery.go:25-78` and `go/internal/router/recovery.go`. A non-contention ship error has a cap of 2 recoveries (`go/internal/core/retry_backoff.go:78`). A contention error (`AUDIT_BINDING_*`, `GIT_FLEET_REBASE_NEEDED`) has a cap of `max(width+1, 2)`, which is 9 at width 8 (`go/internal/core/ship_recovery_budget.go:12-24`). `cyclespend.VerdictOf` counts only LLM minutes after the first failure (`go/internal/cyclespend/verdict.go:16-35`).

*Implication:* the fail-fast index cannot see host waste. A lane can spend 9 re-checks of 55 min each and still look cheap.

### F1.14 Retry loops in total

The retry loops cost 18.6 h in wave 174, 38% of lane time [wave 173: 11.7 h, 29%]. Each minute counts under one loop only:

| Loop | Wave 173 h | Wave 174 h |
|---|---|---|
| Build floor correction rounds | 4.03 | 10.01 |
| Audit FAIL repair rounds (net) | 2.43 | 1.73 |
| Explanation correction on a PASS audit | 0.57 | 3.34 |
| Repo-contract kill loops | 4.24 | 2.87 |
| TDD floor corrections | 0.10 | 0.30 |
| Triage with no work | 0.32 | 0.34 |

These loops overlap the gate times of F1.3 to F1.10. Do not add them to those times.

*Implication:* about two fifths of lane time repeats work that already ran once. Each loop has a separate cause, so each needs its own fix.

## 2. Outside practice

Sources: a live web survey on 2026-11-16. Three sources were not readable in full:

- the Bors forum thread on batch strategy returned 404; its numbers come from a search snippet only;
- the Graphite page on the Shopify queue returned 404; its claim comes from a search snippet only;
- the Google 2017 flaky-test post: the body was not fetched, only its comments.

**P2.1 Uber SubmitQueue: conflict analysis.** Two changes conflict only if they affect a common build target. Each target has a hash over its inputs. Independent changes land in parallel, with no cross-test. A 2025 follow-up cut Go-monorepo builds per change from 3.39 to 1.85 and P95 wait from 33.69 to 18.64 min. Links: [Keeping master green at scale](https://www.uber.com/blog/research/keeping-master-green-at-scale), [summary](https://blog.acolyer.org/2019/04/18/keeping-master-green-at-scale/), [Slashing CI costs](https://www.uber.com/blog/slashing-ci-costs-at-uber/).

**P2.2 Chromium CQ: result reuse and base attribution.** The CQ does not re-test a change against each new head. It reuses a green result when it is "still fresh enough": 800 commits or 24 h. On a failure, it retries the failed shards, then runs the suite without the change. A failure that also occurs without the change is blamed on the tip of tree. Link: [Chromium CQ](https://chromium.googlesource.com/chromium/src/+/main/docs/infra/cq.md).

**P2.3 Speculative windows: Zuul, GitHub, GitLab, Shopify.** Each queued change is tested against the changes ahead of it. Zuul sizes the window like TCP: +1 per merge, halved per failure, with a floor of 3. Shopify runs batches of 8 and tests 3 batches at a time. Links: [Zuul gating](https://zuul-ci.org/docs/zuul/latest/gating.html), [Zuul pipeline](https://zuul-ci.org/docs/zuul/latest/config/pipeline.html), [GitHub merge queue](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/configuring-pull-request-merges/managing-a-merge-queue), [GitLab merge trains](https://docs.gitlab.com/ci/pipelines/merge_trains/), [Shopify](https://shopify.engineering/successfully-merging-work-1000-developers).

**P2.4 Go test cache rules.** The cache key is the test binary, the env vars that the test reads and the files that it opens inside the module. "The idiomatic way to disable test caching explicitly is to use -count=1." The cache does not track files outside the module, reads by a subprocess, the network, the clock or git state. Links: [cmd/go test packages](https://pkg.go.dev/cmd/go#hdr-Test_packages), [test_cache_inputs.txt](https://go.googlesource.com/go.git/+/439ff996f0ee506fc2eb84b7f11ffc360a6299f2/src/cmd/go/testdata/script/test_cache_inputs.txt), [golang/go#53053](https://golang.org/issue/53053), [golang/go#26790](https://golang.org/issue/26790).

**P2.5 `-trimpath` and the absolute-path cache key.** Without `-trimpath`, the compile action ID hashes the absolute package directory (`addPackageOrigin` in [exec.go](https://go.googlesource.com/go/+/refs/heads/master/src/cmd/go/internal/work/exec.go)). So lane worktrees at different paths never share build or test cache entries. With `-trimpath`, the key does not depend on the path. The cost: `runtime.Caller` paths become module-relative, and tests that find testdata through their own source path can break. Fix history: [aa680c0c](https://go.googlesource.com/go/+/aa680c0c49b55722a72ad3772e590cd2f9af541d%5E%21/).

**P2.6 Admission control and race-detector cost.** By default, `go test -p` runs NumCPU binaries, each sized for the whole host. macOS has no cgroup cap, so the only control is an explicit budget or a semaphore. The race detector costs 5-10x memory and 2-20x time. In a 52-project study, 46.5% of flaky tests were resource-affected. Bazel `--local_test_jobs` and Buildkite concurrency groups are admission control.

Links for P2.6: [go testflag](https://manpages.debian.org/unstable/golang-go/go-testflag.7.en.html), [Go 1.25](https://go.dev/doc/go1.25), [race detector](https://go.dev/doc/articles/race_detector), [RAFT](https://arxiv.org/abs/2310.12132), [Bazel](https://bazel.build/reference/test-encyclopedia), [Buildkite](https://buildkite.com/docs/pipelines/controlling-concurrency).

**P2.7 Culprit and base attribution.** Chromium runs a failed suite again without the change and ignores a failure that the tip also has (P2.2). Google TAP uses flake-aware culprit search, with an accuracy of about 98-99%. TAP rolls back a change automatically only when 10 or more broken targets blame it. Links: [Flake-aware culprit finding](https://hackthology.com/flake-aware-culprit-finding.html), [SWE book ch. 23](https://abseil.io/resources/swe-book/html/ch23.html).

**P2.8 Google SRE retry budgets.** A request gets at most 3 tries. The retry ratio of one client stays under 10%, so load grows about 1.1x and not 3x. Only the layer directly above the failure retries. An "overloaded; don't retry" signal stops retry storms. AWS adds: retry only when the dependency is healthy.

Links for P2.8: [Handling overload](https://sre.google/sre-book/handling-overload/), [AWS timeouts and retries](https://builder.aws.com/content/3EumjoZascWd1oZiEgL8ORlv3qE/timeouts-retries-and-backoff-with-jitter).

**P2.9 Shopify: eject after 3.** A change leaves the queue only on its 4th successive failure. Real failures persist, and flakes do not. At an illustrative 25% flake rate, a tolerance of 3 gives a false-eject rate of 0.39%. Link: [Shopify](https://shopify.engineering/successfully-merging-work-1000-developers).

**P2.10 Google TAP postsubmit.** Presubmit runs a fast subset. A change that passes it has a "95%+" chance to pass the rest, so TAP integrates it with no re-test against the new head. Postsubmit runs all affected tests on `main` and finds the culprit of each break. Speculative Cycles cut the median detection time from 107 to 37 min.

Links for P2.10: [SWE book ch. 23](https://abseil.io/resources/swe-book/html/ch23.html), [Speculative testing at Google](https://hackthology.com/speculative-testing-at-google-with-transition-prediction.html).

**P2.11 Batch size and failure rate.** Model: a batch of n changes, each fails with probability p, and a failed batch costs about `2*log2(n)` extra runs per culprit. Batches save CPU only while p is below about 10-15%. At p of 20% or more, keep n at 2 or fewer. Mergify gives the same rule: larger batches below about 2% failure, smaller above about 5%. Links: [Najafi, Rigby and Shang, FSE 2019](https://ece.uwaterloo.ca/~wshang/pubs/Armin_FSE_2019.pdf), [Mergify batches](https://mergify.com/learn/merge-queue/batching), [Bors forum](https://forum.bors.tech/t/batching-strategy-vs-one-at-a-time/220) (404, snippet only).

**P2.12 Agent pull-request overlap.** In 142k agent pull requests, 27.67% conflicted when merged. In a second study of 33,596 agent pull requests, 79.4% overlapped in time with another agent pull request on the same repository. Their textual conflict rate was 19.8% within one agent and 41.7% across agents. Anthropic's 16-agent compiler project used no merge queue, and merge conflicts were "frequent". Links: [AgenticFlict](https://arxiv.org/abs/2604.03551), [Xu and others 2026](https://arxiv.org/abs/2607.04697), [Building a C compiler](https://www.anthropic.com/engineering/building-c-compiler).

*Synthesis:* no surveyed system answers a base move with an LLM review. Each system answers it with a validity rule (P2.1, P2.2, P2.10) or a queue (P2.3). Each system keeps retries for transient faults and attributes a red before it blames the change (P2.7, P2.8).

## 3. Adopt or reject

| Practice | Decision | Reason |
|---|---|---|
| A CPU budget from the live lanes, or a host token pool (P2.6) | Adopt now | F1.2: the budget divides 14 CPUs by 8 slots while 2.3-3.1 lanes run. |
| Base attribution before any re-audit (P2.2, P2.7) | Adopt now | F1.6: all 4 composed declines of wave 174 were a `main`-side leak; the ship gate and audit tier already attribute. |
| A capacity class for deadline kills; no LLM re-audit of an unchanged tree (P2.8) | Adopt now | F1.5: the kill is known from the closure size; the re-audit and the retry repeat it. |
| Affected-package re-check after a rebase, with a shadow comparison (P2.1, P2.4) | Adopt, shadow first | F1.4, F1.10: the re-check is 52% of post-PASS time; P2.4 lists inputs that an import graph does not see. |
| The cheap binding check before the repo-contract gate | Adopt now | F1.10: a head move found late wastes a 23-26 min gate. |
| A FIFO ship ticket (P2.3) | Adopt later | F1.11: the tail is re-check time, not a race; a ticket helps only after the re-check is short. |
| Speculative batches (P2.3, P2.11) | Reject now | P2.11: batches save CPU only below 10-15% failure; our cycle failure rate is far above 10%. |
| A shared build cache with `-trimpath` (P2.5) | Measure first | P2.5: the gain and the `runtime.Caller` risk are not measured here. |
| A TAP-style postsubmit full suite on `main` with culprit search (P2.10) | Adopt later | It makes scoped re-checks safe; it needs the affected-package rule first. |
| Integration train enforce (F1.12) | Defer | F1.12: it adds a 45 min wait and keeps `AUDIT_BINDING_HEAD_MOVED`. |
| The auditor fixes a small document finding in place, not a Build round | Adopt now | F1.9: 3.34 h in wave 174; ADR-0158 already allows a small prescribed fix. |

## 4. Refinements to adopt

- **R1** Size the lane budget from the live lane count, or from a host token pool with one token per CPU (F1.2, F1.3, F1.8).
- **R2** Before a re-audit, run each red target on `main`; a red on both is base-red and parks, not a re-audit (F1.6, F1.11).
- **R3** Class a gate deadline kill as capacity; never send it to an LLM re-audit on an unchanged tree (F1.5, F1.13).
- **R4** After a rebase, re-check only the affected packages; first run it in shadow beside the full re-check (F1.4, F1.10, F1.11).
- **R5** Run the audit binding check before the repo-contract gate, and reuse the composed-gate result after a rebind (F1.10).
- **R6** Run the cheap static checks first in each floor; a correction re-runs only its failed checks (F1.7, F1.14).
- **R7** Let the auditor apply a small explanation-document fix in place, per ADR-0158 (F1.9, F1.14).
- **R8** Count host minutes in the fail-fast verdict, and give each retry loop a budget in host minutes (F1.13, F1.14).

## 5. Limits

- No gate step records its own duration. Step times come from caps, logs and span arithmetic, not from a timer.
- No host load sampler exists. The correlation in F1.2 uses the live lane count from the loop logs, not the CPU load.
- Wave 175 was live at read time, so its numbers are partial.
- The about 6.3 h for wave 173 in F1.5 comes from the notes, and this dossier cannot reproduce its method.
- The retry-loop hours (F1.14) overlap the gate hours (F1.3 to F1.10). A sum of both counts some minutes twice.
- The outside numbers are from other code bases, other hosts and other failure rates. P2.11 is a model, not a measurement here.
- PR #1490 fixed the treewalk leak after wave 174. The F1.6 count describes wave 174, not the current `main`.
