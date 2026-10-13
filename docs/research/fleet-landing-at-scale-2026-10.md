# Fleet landing at scale: conflicts and rebases with 200 concurrent loops (research, 2026-10)

> The plan that uses this research: [fleet-landing-at-scale-2026-10.md](../plans/fleet-landing-at-scale-2026-10.md). The decision record: [ADR-0134](../architecture/adr/0134-fleet-landings-scale-through-content-keyed-verification-and-group-commit.md). The design that this research extends: [fleet-landing-queue.md](../architecture/fleet-landing-queue.md) and [ADR-0128](../architecture/adr/0128-landing-queue-tiered-reverification.md). The earlier dossier for a fleet of width 3: [concurrent-cycle-landing-2026-10.md](concurrent-cycle-landing-2026-10.md).

Date: 2026-10-13. Method: a read of the ship, recovery and scan code at `908e47dc0`, and of the runtime scan logs and loop logs (read only). Also one `go list` of the module, and a survey of merge queues, speculation, test selection and git. Each local claim names its file, its command or its log.

"Synthesis" marks our own inference. "Estimate" marks a number that the plan measures before use. Each outside source has its link, its date of access (2026-10-13) and its publication date when the page gives one.

**Terms used throughout:**

- **Lane:** one fleet cycle, with its own process and worktree. The operator calls it a "loop".
- **N:** the number of lanes that run at the same time.
- **Scan:** the repo-contract gate that ship runs before the landing (`go/internal/phases/ship/repocontract.go`).
- **Importer backstop:** the third layer of the scan. It runs `go test` on each package that imports a changed package.
- **Divergence:** the fast-forward check fails because a peer moved `main` (`GIT_FLEET_REBASE_NEEDED`).
- **Composed tree (`C`):** the lane change merged onto the current tip of `main`, as in ADR-0128.
- **Content key:** a digest of every input of one package test run. Two runs with one key test the same bytes.
- **Group commit:** one push that lands several verified commits at once.

## Table of contents

1. [The request](#1-the-request)
2. [Local evidence](#2-local-evidence)
3. [The cost of today's path at N = 2, 20 and 200](#3-the-cost-of-todays-path-at-n--2-20-and-200)
4. [Current practice: merge queues](#4-current-practice-merge-queues)
5. [Current practice: speculation and conflict analysis](#5-current-practice-speculation-and-conflict-analysis)
6. [Batches, bisection and their arithmetic](#6-batches-bisection-and-their-arithmetic)
7. [Test selection by content and by graph](#7-test-selection-by-content-and-by-graph)
8. [Git mechanics](#8-git-mechanics)
9. [Adopt or reject](#9-adopt-or-reject)
10. [Refinements to adopt](#10-refinements-to-adopt)

## 1. The request

The operator wrote this on 2026-10-13:

> "Assume there are over 2 (said 200) concurrent loop running and merge randomly, research online to find the best and most efficient way to handle the confliction and provide me at least 3 approaches to handle this issue. I want a full plan to make the conflict / rebase as fast as possible"

Earlier on the same day the operator asked: "Why does ship take over 5 minutes? Explain in detail".

The two questions have one answer. Ship is slow because one test package takes five minutes, and a divergence makes ship pay that cost again. At 200 lanes, the same design pays it about 20,000 times for one wave (§3). The research looks for the designs that remove the repeat and remove the serial step.

## 2. Local evidence

### 2.1 Why one ship takes five minutes

**F2.1** Ship runs the scan before the fast-forward check. `runNative` calls `runRepoContractGate` first, then `runStages` (`go/internal/phases/ship/ship.go:177-184`). The fast-forward check is inside the landing stage (`go/internal/phases/ship/landing_intent.go:32`).

*Implication:* if a peer lands while the scan runs, the scan result is for a tree that will never land.

**F2.2** The scan has three layers in sequence: a fixed scanner pack with tree-reading tests by name, an added-test backstop and the importer backstop (`go/internal/phases/ship/repocontract.go:410-432`). Each layer runs `go test -json -count=1` (`repocontract.go:148-155`).

*Implication:* `-count=1` turns off the Go test cache, so no result carries from one scan to the next.

**F2.3** The scan of cycle 1865 ran 35 test packages. `go/cmd/evolve` took 298 s, and the slowest other package took 19 s (`go/acs/cycle1865`). The importer backstop line says "1 target(s) in 5m0s", for the target `./cmd/evolve/...`. Source: the `ok` lines of `runtime/.evolve/runs/cycle-1865/ship-repocontract-scan.log`, read on 2026-10-13. A later `evolve gc` removed that log, so the figures stand as that measurement.

*Implication:* one package is the five minutes. Any design that reruns `cmd/evolve` in full keeps the five minutes.

**F2.4** The importer backstop took between 1 min 53 s and 6 min 21 s over 18 runs in the loop logs (2026-10-11 and 2026-10-12). The median is 2 min 58 s and the p90 is 6 min 4 s. The runs had from 1 to 109 targets. Source: `grep "importer backstop: N target(s) in"` over `runtime/.evolve/logs/2026101*/loop.log`.

*Implication:* the backstop is most of each scan. In cycle 1865 the scan took 354 s of wall time, and the backstop 300 s of it (the brief of 2026-10-13). The other layers thus took about 54 s.

**F2.5** `cmd/evolve` holds 611 Go files, 432 test files, 2,286 top-level test functions and 96,884 lines. 327 test files call `t.Parallel()`, 1,497 times in all. 114 test files call `os.Chdir`, `t.Chdir`, `t.Setenv` or `os.Setenv`. Source: `ls`, `grep -c` and `wc -l` over `go/cmd/evolve`, 2026-10-13.

*Implication:* the tests that change the process state cannot run in parallel inside one process. Separate processes can run them at the same time.

**F2.6** `cmd/evolve` depends on 84% of the 293 module packages outside `go/acs/`. Source: `go list -f '{{.ImportPath}}|{{join .Deps ","}}' ./...` (1.4 s), 2026-10-13.

*Implication:* almost every Go change puts `cmd/evolve` in the importer closure. A cache at package granularity rarely skips it.

### 2.2 What a divergence costs today

**F2.7** After a divergence, the recovery rebases the lane and sends it to ship again, and ship runs the whole scan again. The brief of 2026-10-13 lists three cases, each with the first and the second scan time:

- `cli-carryover-list`: 3 min 55 s, then 4 min 9 s;
- `remote-open-pr-uses-gh-dash-c`: 2 min 47 s, then 2 min 24 s;
- `gc-dev-archive-atomic-under-budget`: 1 min 59 s, then 1 min 53 s.

Cycles 1858, 1862 and 1865 also rebased.

*Implication:* the second scan repeats each test of the first scan, also the tests that the peer change cannot reach.

**F2.8** The loop log of cycle 1865 shows the full ladder. The backstop took 5 min 0 s, and then ship diverged. After a backoff of 0.76 s, the recovery unwound the lane to its audited tree. The identity carry then declined: "composed-tree gates not green (apicover)". The lane went to a re-audit without a Build. Source: `runtime/.evolve/logs/current/loop.log`, 2026-10-12.

*Implication:* one divergence at N = 2 cost a second scan and an LLM Audit.

**F2.9** The recovery budget for a contention code is `max(2, fleet width + 1)` tries (`go/internal/core/ship_recovery_budget.go:24-33`). Each try waits for a jittered backoff of up to 30 s (`ship_recovery_budget.go:51-66`).

*Implication:* at width 200, a lane can try 201 times, and each try pays a scan.

**F2.10** The earlier census measured 45 fleet-rebase recoveries in 14 days. Each cost 24.2 min and 1.49 LLM phases on average ([concurrent-cycle-landing-2026-10.md](concurrent-cycle-landing-2026-10.md) F3.2). Of 39 classified rebases, 18 needed no LLM phase, 19 met through a package edge or a shared path, and 2 conflicted (F3.9 there).

*Implication:* about half of the rebases at width 2 to 3 are disjoint. The other half need a semantic check.

### 2.3 The state of the landing queue

**F2.11** ADR-0128 designs one durable landing queue with tiered re-verification. Its plan status (2026-10-13): Q1 (the derived catalog), Q3 (the overlap proof `internal/overlap`) and Q12 (the interim fix) are done. Q2, Q4 to Q11 and Q13 to Q19 are not started ([plan](../plans/concurrent-cycle-landing-2026-10.md) §16).

*Implication:* the proof exists as code. The queue, the composition, the gates and the wiring do not.

**F2.12** The ADR-0128 queue is FIFO with no order change (D3). A T3 candidate reviews only at the head (D10). The ship at the landing still runs the fixed pack and the importer backstop on `C` ([spec](../architecture/fleet-landing-queue.md) §9).

*Implication:* the serial step of that queue is at least one ship scan. At a median of about 4 min, the queue lands at most about 14 candidates in one hour (§3).

**F2.13** The ADR-0127 event kind `ship.landed` exists (`go/internal/signalcenter/event.go:135`).

*Implication:* a waiter can wake on a landing through the kernel wait of ADR-0127. No design here needs a poll.

### 2.4 The shape of the work

**F2.14** A cycle runs 58.9 min at the median, with a p10 of 28.6 min and a p90 of 102.4 min. The span is from the first to the last signal of each of 67 cycles from 1800 to 1869. Source: `runtime/.evolve/runs/cycle-18*/signals.ndjson`, 2026-10-13.

*Implication:* one lane offers about one landing each hour. N lanes offer about N landings each hour.

**F2.15** Two module packages have a structural overlap when one depends on the other. Over the 42,778 pairs of the 293 packages, 19.2% overlap. Source: the `go list` output of F2.6. A pair overlaps when one package lists the other in `Deps`. The plan's component S13 makes this count a command (`evolve landing sim`).

*Implication (synthesis):* for a lane that changes one package, the chance that k peers with one package each are all disjoint is about 0.808^k. That is 81% for k = 1, 34% for k = 5, 1.4% for k = 20 and almost zero for k = 200.

**F2.16** The import graph has hubs: `internal/sysexec` has 147 importers, `internal/log` 125, `internal/atomicwrite` 124 and `internal/signalcenter` 120. The median package overlaps with 62 others, and the p90 package with 121. A greedy search found at most 139 packages with no edge between any two of them. It took the packages in the order of fewest edges, over 200 random tie orders. Source: as F2.15.

*Implication (synthesis):* at most about 139 lanes, each in one package, can change Go code with no structural overlap. At N = 200 some lanes must overlap, or must wait at dispatch, or must work outside `go/`.

**F2.17** The host is shared. Its load average reached 37 to 63 when lanes ran their floors (the brief of 2026-10-13).

*Implication:* more parallel test processes on one host do not give a linear gain. Verification at N = 200 needs more than one host, or much less work for each landing.

**F2.18** The repository runs each test with `-count=1`, because tests read files outside the module root through `..` (CLAUDE.md, Go Project Conventions). The Go test cache only tracks "files within the package's module" (§7, P7.3).

*Implication:* the Go test cache cannot be the landing cache. A repository cache must put the outside paths in its key.

## 3. The cost of today's path at N = 2, 20 and 200

This section is a model (synthesis). Its inputs are the measured numbers of §2. The plan replaces it with a simulation before any enforce stage.

**F3.1** The serial step. A landing today holds the lane until its scan and its git work end. The scan takes about 3.9 min at the median: 0.9 min for the first two layers and 2.97 min for the backstop (F2.4). Git work takes about 0.3 min (estimate). The step is thus about 4.2 min, and one `main` takes at most about 14.4 landings in one hour.

**F3.2** The load. With one landing for each lane in each hour (F2.14), the offered load is about 2, 20 and 204 landings in one hour. The use of the serial step is about 0.14, 1.42 and 14.2.

*Implication:* at N = 20 the queue grows with no limit. At N = 200 it is 14 times over its capacity.

**F3.3** A wave that starts together. Each round lands one lane, and each other lane diverges and scans again. A wave of N lanes thus runs N(N + 1)/2 scans and N(N − 1)/2 recoveries.

| N | Scans | Scan hours | Recoveries | LLM phases at 1.49 each (F2.10) | Lower bound of the landing time |
|---|---|---|---|---|---|
| 2 | 3 | 0.2 | 1 | 1.5 | 8.3 min |
| 20 | 210 | 13.5 | 190 | 283 | 83 min |
| 200 | 20,100 | 1,296 | 19,900 | 29,651 | 13.9 h |

- The lower bound is N times the serial step. It leaves out the time of each LLM recovery.
- The recovery count is an upper bound: a recovery can carry the verdict with no LLM phase (F2.10).
- The table at N = 2 matches the observation: cycle 1865 scanned twice and paid one Audit (F2.8).

**F3.4** The tier mix at scale. The semantic check depends on k, the number of peers that landed since the lane's base. Without a refresh of the base, k is about the number of landings in one cycle (F2.14). With F2.15 and k = N − 1, about 81% of single-package lanes have no overlap at N = 2 (synthesis). The share is about 2% at N = 20 and zero at N = 200.

*Implication:* at N ≥ 20, almost each landing is a T3 of ADR-0128, with an LLM review. A fast test path alone does not make the landing fast. The design must also keep k small or make the lanes structurally disjoint.

**F3.5** A refresh of the base at the start of the audit cuts the window to the audit and the ship. That is about 8.5 min: Audit 5.7 min and ship 2.8 min (earlier dossier F3.6). Then k is about 0.14 (N − 1), which is 0.1 at N = 2, 2.7 at N = 20 and 27.9 at N = 200 (synthesis). The share with no overlap rises to about 97%, 56% and 0%.

*Implication:* the refresh helps at N = 20 and does not help at N = 200. At N = 200 only a structural partition of the work keeps the landings disjoint (F2.16).

## 4. Current practice: merge queues

**P4.1** The GitHub merge queue groups the changes of a pull request "with the latest version of the `base_branch`" into a `merge_group`. It has a "Build concurrency" of "between `1` and `100`" `merge_group` runs, and a minimum and a maximum group size. A pull request with "failed required status checks or conflicts with the base branch" leaves the queue. Source: https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/configuring-pull-request-merges/managing-a-merge-queue (accessed 2026-10-13). The earlier dossier records the same page and GitHub's own scale (F6.2 there).

*Implication:* the queue owns the composition, and the group size and the concurrency are the two dials of throughput.

**P4.2** A GitLab merge train tests each merge request with the target branch and every merge request ahead of it: "The pipelines all run in parallel." The default limit is 20 parallel pipelines for each train. When a pipeline fails, GitLab removes that merge request, starts new pipelines for the requests behind it and cancels the old ones. Source: https://docs.gitlab.com/ci/pipelines/merge_trains/ (accessed 2026-10-13).

*Implication:* speculation on the prefix gives parallel verification with a serial order. A failure costs a restart of each pipeline behind it.

**P4.3** Mergify tests "cumulative merges (PR #1), (PR #1 + PR #2), (PR #1 + PR #2 + PR #3)" in parallel, up to `max_parallel_checks`. A batch holds up to `batch_size` pull requests, from 1 to 128. The size can adapt between a `min` and a `max` "to keep the backlog at zero". A failed batch splits until "a batch contains only one pull request and still fails". A split can "run only the tests that failed in the parent batch". Sources: https://docs.mergify.com/merge-queue/batches/ and https://docs.mergify.com/merge-queue/speculative-checks/ (accessed 2026-10-13).

*Implication:* adopt the adaptive group size and the rerun of only the failed tests. Mergify itself warns that in a batch "hidden failures inside a passing batch can land on main".

**P4.4** Mergify's parallel mode puts pull requests into scopes. "Batches with no shared scopes (and therefore no dependency) are tested by CI at the same time", and a failure "in one scope queue does not block unrelated scope queues". `scopes.capacities` caps the concurrent checks of one scope. Source: https://docs.mergify.com/merge-queue/queue-modes/ (accessed 2026-10-13).

*Implication:* independence by scope removes the cascade of P4.2. A capacity for each scope is the admission control that F2.16 needs.

**P4.5** Aviator's affected targets make "dynamic queues" for monorepos "where CI runs are long compared to the rate of merge requests". There, "basic FIFO queues will never catch up". Two pull requests with no overlapping targets merge "independently in any order". Overlapping ones stack and test together. Source: https://docs.aviator.co/mergequeue/concepts/affected-targets (published 2025-03-24, accessed 2026-10-13).

*Implication:* the same diagnosis as F3.2: a FIFO queue cannot keep up when the verification is long against the arrival rate.

**P4.6** Aviator's optimistic validation waits for later batches when one batch fails: if a later batch that holds the same change passes, it "assumes that the CI run would have been passed". The depth is capped at 3, and "real errors will take longer to be kicked out of the queue". Source: https://docs.aviator.co/mergequeue/concepts/managing-flaky-tests-in-mergequeue (published 2026-08-28, accessed 2026-10-13).

*Implication:* a flake screen that uses later work is cheap, but it delays the ejection of a real red. ADR-0128 already chose the Chromium screen (earlier dossier F6.9).

**P4.7** Graphite runs "CI on the top-most PR (which contains all downstream changes)" of a stack. "If CI passes, all PRs in the stack fast-forward merge in sequence". On a failure it bisects. Source: https://graphite.com/blog/the-first-stack-aware-merge-queue (published 2025-06-02, accessed 2026-10-13).

*Implication:* one run for a stack is cheap, but each commit under the top is not tested alone.

**P4.8** Bors-NG tests approved pull requests "in batches", and splits a failed batch into two ("bisecting"). A batch of one that fails "gets kicked back to the creator". Its README has a deprecation notice and points to "GitHub's built-in merge queue". Source: https://github.com/bors-ng/bors-ng (accessed 2026-10-13).

A forum thread gives the cost of batches as O(E log N) runs for E failures in N pull requests. One at a time costs O(N). Source: https://forum.bors.tech/t/batching-strategy-vs-one-at-a-time/220 (accessed 2026-10-13).

*Implication:* batches pay off only when failures are rare (§6).

**P4.9** Kodiak keeps pull requests "up to date with `master` automatically", then merges them "once CI and Approvals pass". Source: https://github.com/chdsbd/kodiak (accessed 2026-10-13).

*Implication:* this is the update-then-merge loop that the fleet runs today. It is serial by design, and each update pays a full CI run.

**P4.10** Trunk tests "independent changes in parallel from the impacted targets each PR touches", and bisects a red batch (earlier dossier F6.5). Zuul tests each change "applied to the tip of the branch exactly as it is going to be merged", and tests again without a failed change (earlier dossier F6.1).

*Implication:* the products agree on three parts: verify the exact tree that lands, verify in parallel on a predicted prefix, and split work by impact.

## 5. Current practice: speculation and conflict analysis

**P5.1** Uber's SubmitQueue keeps "an always green master branch at scale: all build steps ... successfully execute for every commit point". It "can scale to thousands of daily commits to giant monolithic repositories". A speculation graph with "a probabilistic model, powered by logistic regression" picks the builds "most likely to succeed" and runs them in parallel. A conflict analyzer builds "a conflict graph among pending changes" to "determine independent changes that can commit in parallel". It improved the p95 turnaround of their oracle by "up to 60%". Source: Ananthanarayanan and others, "Keeping Master Green at Scale", EuroSys 2019, https://dl.acm.org/doi/10.1145/3302424.3303970 (published 2019-03-25, accessed 2026-10-13).

*Implication:* "green at every commit point" is our rule too. Independence from the build graph is what lets two changes commit in parallel. The model of success probability is a later refinement, not a first need.

**P5.2** The same paper defines the composed build: to land `C1`, the system tests `H ⊕ C1`. It starts `B1.2` for `H ⊕ C1 ⊕ C2` in parallel with `B1`, and uses it if `B1` passes. Source: as P5.1.

*Implication:* this is the speculative chain. Our lanes already hold an audited tree, so each element of the chain is a merge-tree composition, not a build of new code.

**P5.3** Google's "Flake Aware Culprit Finding" (ICST 2023, industry track) finds the breaking commit with Bayesian inference and a noisy binary search, over more than 13,000 test breakages. Source: https://research.google/pubs/flake-aware-culprit-finding/ and https://hackthology.com/pdfs/icst-2023.pdf (accessed 2026-10-13). TAP runs a minimal presubmit by the dependency graph and splits a failed postsubmit batch (earlier dossier F6.8).

*Implication:* postsubmit culprit finding is for a system that lets a red reach the main line. Our rule forbids that, so culprit finding stays for CI after the landing, where each commit maps to one cycle.

**P5.4** Meta. I found no primary public source on 2026-10-13 that describes how Meta lands diffs at scale. The searches found only general pages about Sapling and Mononoke (for example https://lwn.net/Articles/915187/). This dossier does not use Meta as evidence.

## 6. Batches, bisection and their arithmetic

**P6.1** Let p be the chance that one candidate is red on its composed tree, and b the batch size. A batch is green with the chance (1 − p)^b (synthesis). With one culprit, a binary split needs about 2 log2 b more runs. The expected runs for each candidate are about [1 + (1 − (1 − p)^b) × 2 log2 b] / b.

| p | b = 1 | b = 4 | b = 8 | b = 16 |
|---|---|---|---|---|
| 0.05 (T4 share, earlier dossier F3.9) | 1.00 | 0.44 | 0.38 | 0.34 |
| 0.10 | 1.00 | 0.59 | 0.55 | 0.47 |

- The runs fall fast from b = 1 to b = 4, then slowly. Each red adds about 2 log2 b rounds of latency.
- The model assumes one culprit and no flakes. Flakes raise p, and Mergify adapts b to the backlog for this reason (P4.3).

**P6.2** A batch that tests only its top tree does not test the commits under the top (P4.3, P4.7). Our rule is "never ship an untested merged tree" (the brief), and SubmitQueue's rule is green "for every commit point" (P5.1).

*Implication:* we cannot adopt top-only batches as they are. A batch helps us only when each commit point in it has its own proof. §7 shows how a content key gives that proof at the cost of a top-only batch.

## 7. Test selection by content and by graph

**P7.1** bazel-diff finds "the exact affected set of impacted targets between two Git revisions" with hashes over the build graph, "Merkle tree" style. Its design note says that a cheap approximation can give "a system you can't trust". Source: https://github.com/Tinder/bazel-diff (accessed 2026-10-13).

*Implication:* key each test run by a digest of all its inputs. Then "same key" means "same result" under hermetic tests, and the key is exact, not a guess.

**P7.2** Datadog skips a test only when "the covered and tracked files are identical". Ekstazi selects tests by the files that each test reads at run time (earlier dossier F8.2, F8.3).

*Implication:* per-test selection inside one large package needs a map from each test to its inputs. That map is the only way to scope `cmd/evolve` below the package (F2.3, F2.6).

**P7.3** The Go test cache matches a run when "the run involves the same test binary" and the flags are cacheable. "Tests that open files within the package's module or that consult environment variables only match future runs in which the files and environment variables are unchanged." `-count=1` turns it off. Source: https://pkg.go.dev/cmd/go, section "Test packages" (accessed 2026-10-13).

*Implication:* the Go rule is a content key that misses files outside the module (F2.18). A repository key can use the same idea and add the outside read roots.

## 8. Git mechanics

**P8.1** `git merge-tree --write-tree` merges with no worktree and no index, and a local probe composed in well under one second (earlier dossier F9.1, F9.2).

*Implication:* composition is not the cost. The tests are.

**P8.2** The `ort` merge backend replaced `recursive` as the default in 2021. Its author measured the no-renames case of a 35-patch rebase at 5.235 s before a performance series and about 0.204 s after it. Sources: the commit "Change default merge backend from recursive to ort" (6a5fb96672) and the patch series, mirrored at https://timplab.syktsu.ru/admin/git/commit/6a5fb96672 and https://patchwork.kernel.org/comment/23882643 (accessed 2026-10-13); https://www.theregister.com/2021/08/17/git_233/ (published 2021-08-17).

*Implication:* the rebase itself takes seconds. A fast rebase plan must make the re-verification fast, not the merge.

**P8.3** `git rerere` records hand resolutions and applies them "to their corresponding automerge results" when the same conflict comes back. Source: https://git-scm.com/docs/git-rerere (accessed 2026-10-13).

*Implication:* rerere helps a person who resolves the same conflict many times. A genuine conflict of a lane goes to repair and a new audit, so a recorded resolution has no verdict. Reject for the landing path.

**P8.4** Sparse and partial checkouts make a worktree smaller. They change the provisioning time of a lane, not the landing. This dossier did not measure them.

## 9. Adopt or reject

| Practice | Decision | Reason |
|---|---|---|
| Verify the exact tree that lands (Zuul, GitHub, GitLab) | adopt | Our rule; ADR-0128 already composes `C` |
| Parallel verification on a predicted prefix (GitLab, Mergify, SubmitQueue) | adopt | F3.2: a serial step cannot carry the load at N ≥ 20 |
| Independence by build graph (SubmitQueue, Aviator, Mergify scopes, Trunk) | adopt | F2.15: 81% of single-package pairs are disjoint |
| Capacity for each scope (Mergify) | adopt as admission at dispatch | F2.16: no more than about 139 disjoint Go lanes |
| Adaptive group size (Mergify) | adopt for the push, not for the proof | P6.2: each commit point needs its own proof |
| Top-only batches with bisection (Bors-NG, Graphite, Mergify) | reject | P6.2: commits under the top stay untested |
| Rerun only the failed tests of a parent run (Mergify) | adopt | It is the flake screen with a smaller scope |
| Content keys for test runs (bazel-diff, the Go test cache) | adopt, with outside read roots in the key | P7.1, P7.3, F2.18 |
| Per-test input maps (Ekstazi, Datadog) | adopt in shadow, for `cmd/evolve` only | F2.3, F2.6 |
| Optimistic land and postsubmit culprit finding (TAP) | reject for landing; keep for CI on `main` | It lets a red reach `main` |
| Optimistic validation by later batches (Aviator) | reject | ADR-0128 uses the Chromium screen; this one delays a real red |
| Update-then-merge loops (Kodiak, today's ladder) | retire for fleets | F3.3: N(N + 1)/2 scans for one wave |
| `git rerere` | reject | P8.3 |
| A success model by logistic regression (SubmitQueue) | defer | It needs history; first measure the red rate |

## 10. Refinements to adopt

| Id | Refinement | From |
|---|---|---|
| R1 | Check the fast-forward, or compose, before the scan, so that no scan tests a tree that cannot land. | F2.1 |
| R2 | Key each package test run by its content, with the read roots outside the module, and keep each green result. | F2.2, F2.18, P7.1, P7.3 |
| R3 | After a divergence, run only the package runs whose key changed on `C`. | F2.7, R2 |
| R4 | Split `cmd/evolve` runs over processes now, and move its tests to their packages over time. | F2.3, F2.5, F2.6 |
| R5 | Map each `cmd/evolve` test to its inputs, and select by the map in shadow first. | P7.2 |
| R6 | Verify candidates off the head of the queue, in parallel, on their predicted prefix. | P4.2, P5.2, F2.12 |
| R7 | Let candidates with no edge between them land in any order. | P4.4, P4.5, P5.1 |
| R8 | Land the verified run of candidates with one push, and keep one record for each commit point. | P4.3, P6.2 |
| R9 | Cap the in-flight lanes of each cell of the import graph at dispatch. | F2.16, P4.4 |
| R10 | Refresh the lane's base at the start of the audit when the composition is clean. | F3.5 |
| R11 | Wake each waiter on `ship.landed` and on queue records, never on a timer. | F2.13 |
| R12 | Prove each gain in a seeded simulation at N = 2, 20 and 200 before any enforce stage. | §3 is a model |
