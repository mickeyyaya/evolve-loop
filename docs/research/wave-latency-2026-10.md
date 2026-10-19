# Research: where the time of a wave goes

- **Status:** complete, 2026-10-19.
- **Plan:** [wave-latency-2026-10](../plans/wave-latency-2026-10.md). **Decision:** [ADR-0143](../architecture/adr/0143-admission-holds-only-shared-edits-and-the-ship-gate-owns-api-drift.md).
- **Related:** [wave-orchestrator-2026-10](wave-orchestrator-2026-10.md), [ci-wall-time-2026-10](ci-wall-time-2026-10.md), [pool-resupply-2026-10](../plans/pool-resupply-2026-10.md).
- **Method:**
  - a read-only analysis of the run artifacts of cycles 1930 to 1983 (`signals.ndjson`, `llm-calls.ndjson`, the reports);
  - the `LOOP_ORCH_*` signals of waves 130 to 133;
  - `go list` over the module graph at `origin/main` e66b2a667;
  - code reads with `path:line`;
  - live web searches on 2026-10-19 (section 6).

## Table of contents

- [1. The request](#1-the-request)
- [2. The width of a wave](#2-the-width-of-a-wave)
- [3. The time inside a lane](#3-the-time-inside-a-lane)
- [4. The gates](#4-the-gates)
- [5. Rework and waits](#5-rework-and-waits)
- [6. Current practice](#6-current-practice)
- [7. Limits](#7-limits)

## Terms used throughout

- **Slot:** one of the `fleet.count` places where a lane can run. Waves 130 to 133 had 4 slots.
- **Slot-hour:** one slot for one hour. A wave of 2 hours at 4 slots has 8 slot-hours.
- **Lane time:** the wall time of a cycle, from its first phase dispatch to its last phase outcome.
- **Model time:** the time that the agent works, from `llm-calls.ndjson`.
- **Host time:** lane time minus model time. The host runs the deterministic gates in it.
- **Reach:** the packages that the edited packages of a todo import (`go/internal/admission/admission.go:100`).

## 1. The request

The operator asked on 2026-10-19: "investigate why it takes so long to complete one wave, ultrathink to deep dive into the blocking component, identify what slows down the pipeline and improve it".

## 2. The width of a wave

**F2.1** Waves 130 to 133 used 24.3 of 48 slot-hours, which is 51%. The average live lanes was 2.10 (wave 130), 2.13 (wave 131), 1.58 (wave 132) and 2.26 (wave 133). The figures come from the `fields.live` of each `LOOP_ORCH_*` signal, weighted by time.
*Implication:* half of the capacity was idle. Width is the largest lever of the wave time.

**F2.2** In wave 130 the pool ran 1 lane for 3.8 hours. The pool took its backlog once at the start, and items that became ready later were never supplied.
*Implication:* PR #991 fixed this cause. In wave 133 the pool resupplied `item-failure-count-is-never-bumped` 12 minutes after it was filed (`LOOP_ORCH_POOL_RESUPPLIED`, 12:24:35Z).

**F2.3** In wave 133 the pool held 13 ready items from 11:21Z to 12:32Z at 2 or 3 live lanes (`LOOP_ORCH_POOL_UNDERFILLED`, `ready=13`). The running lane `go-test-env-projection-pinned-lane` edits `go/internal/core/task_contract.go`. When that lane exited at 12:32Z, the pool admitted 3 lanes in the same second.
*Implication:* the reach rule held the slots, not a lack of work. This was after the leaf rule of cycle 1972, which does not apply because the lane edits `internal/core`.

**F2.4** `internal/core` imports 86 internal packages directly and 102 through any path (`go list`, 2026-10-19). It imports `router`, `clihealth` and `llmroute` directly.
*Implication:* a reach of direct imports only (approach (a) of the earlier admission item) frees almost nothing for a lane that edits core. The reach rule itself is the limit.

**F2.5** The reach rule guards against one risk: a candidate changes a package that a running lane compiles against. The ship already checks this risk. It rebases each lane onto main and runs the composed-tree gate before the push (ADR-0105). Since PR #990, a byte-identical rebase with green gates keeps its audit.
*Implication:* the admission check repeats a check that the ship does later, at a much higher cost in width.

**F2.6** The census of wave 133 found 254 items. Of these, 94 were console-owned for a protected path, 75 for a pipeline-repair kind and 64 by a console route. Another 5 waited on deps and 13 on a cell. Only 3 were admissible at the start (`LOOP_ORCH_SUPPLY`).
*Implication:* the lanes can do a small part of the inbox. The supply is short even with a better admission rule.

## 3. The time inside a lane

**F3.1** Cycles 1930 to 1983 had 24.1 hours of lane time. Build takes 45.7%, audit 16.5%, scout 10.0%, tdd 9.8%, ship 7.9% and triage 5.1%. The gaps between phases take 3.0%.
*Implication:* build is the phase to improve first.

**F3.2** Model time is 77% of build, 40% of audit and 0% of ship. Host time is 2.5 hours in build, 2.4 hours in audit and 1.9 hours in ship: 28% of all lane time.

**F3.3** The total build model time of a cycle has a median of 24.4 minutes on agy (Gemini 3.8 Flash, 15 cycles). On claude it is 10.0 minutes (36 cycles). One build call has a median of 11.5 minutes on agy and 3.0 minutes on claude at balanced.
*Implication:* agy is slower in the build seat. The task mix differs between the two sets, so the ratio is an estimate, not a controlled result.

**F3.4** The artifact of a call is written about 0.1 minutes before the bridge ends the call, on every family.
*Implication:* the bridge detects the end at once. The time of agy is the time of the model, not a detection delay.

**F3.5** The Claude week window was 14% used and the session window 8% used on 2026-10-19 (`evolve doctor live claude-tmux`).
*Implication:* the quota reason that put agy first in the build seat does not hold today. The choice depends on the headroom, and the headroom changes each week.

## 4. The gates

**F4.1** After each build call the host runs the build handoff floor for a median of 3.0 minutes (135 calls, 6.4 hours). After each audit call it runs the deliverable checks and the ACS suite for a median of 3.0 minutes (66 calls, 3.3 hours). The ship scan runs a median of 3.5 minutes.

**F4.2** The build floor and the ship both run the repo-contract scanner pack (CLAUDE.md, "repo_contract_gate"). Only the ship scan reads and writes the verdict store (`go/internal/phases/ship/scanshadow.go`). No other gate imports `internal/testverdict`.
*Implication:* the ship tests again what the floor already tested on the same tree.

**F4.3** In shadow, the ship scan found 127 of 1826 package results that a stored green result covered (7%), with 0 escapes, over 33 scans.
*Implication:* the content key is safe for entries that the ship writes. The hit rate is low because only the ship writes to the store. More writers widen the risk, so a shadow with 0 mismatches must come before enforce.

## 5. Rework and waits

**F5.1** In wave 130, before PR #990, a cycle ran audit 1.83 times, ship 1.25 times and build 1.67 times. In waves 131 to 133 the counts were 0.95, 0.90 and 1.10.
*Implication:* the identity carry now works, and rework is no longer a main cost.

**F5.2** The wait for the ship-window lease sits inside the audit phase, after the deliverable check. Over 66 audits it totals 0.8 hours, with spikes of 15.0, 9.5, 8.8 and 8.6 minutes (cycles 1956, 1930, 1955 and 1969).
*Implication:* the lease is a minor cost. The console item `ship-window-holds-only-the-rebase-and-push` covers it.

**F5.3** The gap between two waves was 23, 16 and 7 minutes for the boundaries after waves 130, 131 and 132. About 4 minutes of each gap is `evolve wave next`. The rest is the merge and the rebase of console PRs.

## 6. Current practice

The searches ran on 2026-10-19. "acc." marks a page with no date of its own.

### Parallel changes to one repository

**O1** Uber SubmitQueue treats two changes as in conflict when they affect a common set of build targets. It does not reject them. It tests them together on a speculation tree ([summary of EuroSys 2019](https://blog.acolyer.org/2019/04/18/keeping-master-green-at-scale/), 2019-04-18). A 2025 follow-up cut CI resources by 53% and P95 wait by 37% ([arXiv 2501.03440](https://arxiv.org/abs/2501.03440), 2025).

**O2** The build tools define "affected" by reverse dependencies: the changed target plus each target that depends on it ([Nx](https://nx.dev/docs/features/ci-features/affected), acc.; [bazel-diff `rdeps`](https://github.com/Tinder/bazel-diff), acc.). No source that the search found holds a change on the forward import closure.
*Implication:* our reach rule looks in a direction that no surveyed system uses.

**O3** Merge queues use overlap to decide which changes they test together, not to block a change from its start. Aviator tests PRs with no overlapping targets in parallel and stacks the others ([Aviator](https://docs.aviator.co/mergequeue/concepts/affected-targets), acc.).

Mergify links PRs with overlapping scopes into batches ([Mergify](https://docs.mergify.com/merge-queue/parallel-scopes), acc.). The GitHub merge queue is fully optimistic. It tests each PR on the base plus the PRs ahead of it, and it drops a PR that fails ([GitHub docs](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/configuring-pull-request-merges/managing-a-merge-queue), acc.).
*Implication:* our ship is a merge queue of size one. Overlap belongs at the ship, not at admission.

**O4** A gate on the composed tree is necessary. Before a merge queue, 7% to 10% of the CI failures of Jira came from the merge step ([Atlassian](https://www.atlassian.com/blog/bitbucket/merge-queues-how-we-ship-faster-with-fewer-incidents), 2026-04-29).
*Implication:* the composed-tree gate of the ship must stay when admission becomes optimistic.

**O5** A reverse-dependency rule is stricter than ours, not looser. When lane B edits a package that lane A imports, the reverse dependencies of both edits include the package of A. In this module `cmd/evolve` depends on 270 of the 302 internal packages (`go list`, 2026-10-19). So two edits inside that set always share a dependent.
*Implication:* the middle ground is overlap of edited packages at admission, plus a serial ship for lanes that overlap. The ship-window lease already makes the ship serial.

### Test results keyed by content

**O6** The Go test cache keys a result on the test binary and on the files and variables that the test reads inside the module. `-count=1` turns it off ([cmd/go](https://pkg.go.dev/cmd/go), acc.). It misses a file that is added to a directory that a test globs ([golang/go#22583](https://golang.org/issue/22583), acc.).
*Implication:* our tests read `.evolve/profiles` through `..`, outside the module, so the Go cache is not safe here. That is why the repository requires `-count=1`. A store with an explicit key is needed.

**O7** A verify mode runs the test again on a cache hit and compares the result (`GODEBUG=gocacheverify=1`, [golang/go#25659](https://golang.org/issue/25659), acc.). Bazel warns that only trusted runners can write the remote cache, and that a poisoned cache must be wiped ([Bazel](https://bazel.build/remote/caching), acc.). Gradle lets only main-branch CI write by default ([Gradle](https://community.gradle.org/github-actions/docs/setup-gradle), acc.).
*Implication:* the shared store needs one writer class, a shadow that runs and compares, and a one-step wipe. A FAIL is never reused.

### Routing on latency and quota

**O8** LiteLLM drops a deployment that is over its usage limit, then routes to the lowest measured latency. A quota error starts a cooldown ([LiteLLM](https://docs.litellm.ai/docs/routing), acc.). OpenRouter sorts by latency or throughput and never blocks a request ([OpenRouter](https://openrouter.ai/docs/features/provider-routing), acc.).
*Implication:* "filter on headroom, then order by measured speed" is the standard shape of D4.

**O9** Across coding agents, wall time and cost vary about tenfold, and neither predicts the pass rate (DeepSWE, [arXiv 2607.07946](https://arxiv.org/abs/2607.07946), 2026; abstract only).
*Implication:* D4 orders families by the measured lane time to a PASS for each family, not by the time of one call.

### Pools and stragglers

**O10** A Kubernetes Job keeps `parallelism` pods running and starts one when one ends ([Kubernetes](https://kubernetes.io/docs/concepts/workloads/controllers/job/), acc.). Spark starts a backup copy of a task that runs 1.5 times slower than the median ([Databricks](https://kb.databricks.com/scala/understanding-speculative-execution), acc.).
*Implication:* the pool of PR #991 matches practice. A backup copy of an agent lane is not useful, because the lane writes to one tree.

**O11** By Little's law, the throughput is the work in flight divided by the time of each item. At 51% slot use, a move to about 90% gives about 1.75 times the ships per hour, if the gate failures do not grow (synthesis).

## 7. Limits

- The run dirs of 21 cycles in the range were pruned by gc, so section 3 uses 33 cycles.
- The search found no published wall times of Claude Code and the Gemini CLI on the same tasks. The full texts of the SubmitQueue and DeepSWE papers were not read; their abstracts and summaries were.
- The agy and claude sets of F3.3 have different tasks. A controlled comparison needs the same task on both families.
- The local analysis used a script outside the repository. Component W0 of the plan adds the same report as an `evolve` verb, so the numbers can be measured again.
