# Concurrency Isolation — Investigation + Best-Practice Research (2026-06-14)

> Evidence base for **[ADR-0049: Concurrent Multi-Cycle Execution](adr/0049-concurrent-multi-cycle-execution.md)**.
> Produced by two ultracode workflows: a 28-agent codebase isolation audit (4 levels × adversarial verify)
> and a 6-agent web research sweep of concurrency-isolation prior art. This doc is the *full* record
> (request → findings → prior art → resolved decisions); the ADR is the concise decision SSOT.

## Why

An operator asked to run multiple `evolve` cycles concurrently, "each cycle not interfering with the others,
with the advisor partitioning the backlog across independent cycles," and steered: *"make each phase and
agent independent and executed in concurrency first — this is fundamental for cycle concurrency."* The
investigation established that **concurrent cycle execution is structurally impossible today** — the loop is
strictly sequential *by design* (one whole-cycle project flock). So this is not "improve concurrency"; it is
"build it, bottom-up (agent → phase → cycle), activating the dormant per-resource isolation hardening that
already exists in the tree in the right order."

---

## Part 1 — Codebase isolation audit (4 levels)

28 agents mapped shared mutable resources at four nested scopes, adversarially verified each gap (14 confirmed
real, 14 refuted as already-guarded). Gaps ranked by severity × blast-radius.

| ID | Level | Gap (one line) | Severity | Key location |
|----|-------|----------------|----------|--------------|
| **G1** | cycle | Shared **main working tree + branch**: `git add -A`→`checkout HEAD -- go/evolve`→`merge --ff-only`→`push`→tree-verify is RMW'd with no per-run sync. Two ships interleave. The result is false `DIVERGED`, peer-binary wipe, cross-contaminated commit and false `INTEGRITY_TREE_DRIFT`. **Irreversible loss of a peer cycle's work.** | **HIGH** | `phases/ship/gitops.go:154-253,259-445`; `repair.go:236-269`; `gitops.go:397,649-696` |
| **G2** | cycle | Ship mutates `state.json` with **raw lockless `writeStateMap`**. This bypasses the flock+StateRevision RMW of `storage.UpdateState`. It is a 2nd incompatible write discipline on the file that CA.3 made single-path. It is a lost-update + lease-rollback engine (the 278/279 clobber, from the ship side). | **HIGH** | `ship/verify.go:75-92`, `postship.go:52-75,206-247`, `repair.go:153-168` through `statefile.go:46-79` |
| **G3** | cycle | Ship resolves run-defining inputs (`active_worktree`, `cycle_id`, `cycle_size_estimate`) from **host-global `cycle-state.json`** (last writer wins). It does not use the CB.4 per-run `run.json` mirror. Ship therefore integrates the WRONG run's worktree under the WRONG number. | MEDIUM | `gitops.go:145-152,450-458`, `postship.go:53,83`, `verify.go:312-318` |
| **G4** | phase | Host-global `state.json` cycle-number RMW + **`ledger.jsonl` single hash chain**. Both number-key the run workspace AND worktree branch. Two runs that allocate the same N collide on `runs/cycle-N`, `worktrees/cycle-N` and branch `cycle-N`. | MEDIUM | `core/alloc.go:33-82`, `worktree.go:45-79`, `runworkspace.go:20-22`, `ledger/ledger.go:80-148` |
| **G5** | cycle | Audit→ship binding (`findLatestAudit`) + ledger `Verify` scan the **global ledger for the latest `role:auditor` entry with NO RunID filter**. Ship can therefore bind to the auditor entry of a concurrent run (cross-run `AUDIT_BINDING_HEAD_MOVED`). The failure is fail-closed (it refuses and does not corrupt). | MEDIUM | `ship/audit.go:179-209`, `orchestrator.go:481-611` |
| **G6** | agent-bridge | Sandbox **SBPL profile `sandbox-<phase>.sb` keyed by phase name** in the shared workspace, not per-dispatch. Two same-phase dispatches with different WritePaths race. A is confined to the allowlist of B, so A gets EPERM on its legit writes. Security-relevant. | MEDIUM | `bridge/sandbox_wrap.go:118-119` |
| **G7** | agent-bridge | `resolved-prompt.txt` + `challenge-token.txt` are **fixed filenames in the shared workspace**. The prompt write of B lands between the write of A and the tmux load-buffer read of A. A then pastes the prompt of B (silent wrong artifact). The token RMW is last-writer-wins, and this causes a provenance reject. | MEDIUM | `driver_tmux_repl.go:172`, `driver_common.go:63-77`; bash `claude/codex/agy-tmux.sh:~82` |
| **G8** | agent | Fanout injects `EVOLVE_MAX_BUDGET_USD` / `EVOLVE_FANOUT_CACHE_PREFIX_FILE` with **process-global `os.Setenv`**, not per-worker. Across two concurrent `DispatchParallel` in one process, a worker runs under the wrong budget/cache-prefix. Silent cost/integrity defect. | MEDIUM | `fanoutdispatch/fanoutdispatch.go:118-123` |
| **G9** | agent-bridge | Named tmux sessions (`evolve-bridge-named-<name>`, swarm `c<cycle>-<workerID>`) keyed solely on name, **no RunID**. The REPL driver **reattaches on `has-session`**. A resume that reuses the cycle and a concurrent fresh run at the same N,w0 make two prompts interleave in one live REPL. | LOW | `driver_tmux_repl.go:180-182,936-938`; `swarm/dispatcher.go:30-34` |
| **G10** | cycle | `EVOLVE_FLEET` is **consumer-only with no producer**. The cwd-fallback-refusal guard is armed but unpulled. There is no `evolve fleet` and no `internal/fleet/`. | MEDIUM | consumer `driver_tmux_repl.go:150`; producer: none |
| **G11** | cycle | **No runlease writer**. gc reads `.lease` for liveness, but nothing refreshes it. The CE.3 scheduler is unbuilt, so gc can mis-collect a live run. | LOW | consumer `gc/discover.go:166`; `runlease/runlease.go:71` never called |
| **G12** | cycle | **Inbox item selection** reads the shared inbox root with no cross-run reservation. Two triages claim the same item. The result is double work or a stranded item. | LOW | `ship/postship.go:82-129` |
| **G13** | agent-bridge | Bash ephemeral session names carry pid+epoch-second but **no RunID** (the Go mirror has CB.5 RunScopeToken). Two same-cycle same-agent dispatches whose pid+second coincide collide. The EXIT trap of one driver tears down the pane of the other. Bash is strictly weaker than Go. | LOW | `claude-tmux.sh:107`, `codex-tmux.sh:95`, `agy-tmux.sh:78` |
| **G14** | cycle | A few sites read **live process env** and not the per-cycle `req.Env` snapshot (`EVOLVE_WORKTREE_BASE`, guard kill-switch; git inherits `os.Environ()`). Same-process concurrency lets one cycle repoint the worktree base of another cycle. | LOW | `worktree.go:38-43`, `orchestrator.go:1729-1730`, `gitops.go:177` |

**The keystone insight:** the codebase is full of **latent, dormant isolation hardening** built *for*
concurrency. It is not yet load-bearing, because one coarse cycle-long mutex (`.evolve/.lock`,
`orchestrator.go:1673`) still serializes everything. The hardening items are:

- CA.3 `UpdateState` RMW;
- CA.4 allocation lease;
- RunID-stamped ledger;
- CB.4 `run.json` mirror;
- CB.5 session-name `RunScopeToken`.

Concurrency means that you activate
these in the right order. You do not build from scratch.

---

## Part 2 — Best-practice research (web prior art)

### Decision 1 — Workspace identity → **STABLE path + RunID in sidecar/ledger** (NOT RunID-in-path). High confidence.

Near-unanimous across long-lived stateful CI/orchestration workers:

- **GitLab Runner** — build dir = stable IDs + a bounded **concurrency-slot** integer (`CI_CONCURRENT_ID` 0,1,2…), reused by the next job in that lane; run-ID is metadata. `git clean` resets between jobs. ([docs](https://docs.gitlab.com/runner/configuration/advanced-configuration/), [#38307](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/38307))
- **Jenkins** — one stable workspace; appends `@2`/`@3` **only** under genuine concurrent overlap; the suffix is a collision counter chosen at lock time, **not** the build number. ([JENKINS-30231](https://issues.jenkins.io/browse/JENKINS-30231))
- **GitHub Actions** — stable `_work/<repo>/<repo>`, one job at a time per runner; concurrency = more runner instances each with its own `_work`. ([docs](https://docs.github.com/actions/hosting-your-own-runners))
- **Buildkite** — stable `build-path/<agent>/<org>/<pipeline>`; `BUILDKITE_BUILD_ID` is env, never a path segment. ([docs](https://buildkite.com/docs/agent/v3/configuration))
- **Temporal / Airflow** — `run_id` is **durable identity in the event store/DB**. Temporal explicitly warns *not* to branch logic on the volatile run_id. Resume/replay keys off the ID, not a path. ([Temporal](https://docs.temporal.io/workflow-execution/workflowid-runid))
- **Bazel / Nix** — the *only* camp that puts a unique ID in the live dir. It does this *only because those dirs are ephemeral and never resumed*. A re-run is a brand-new sandbox. This is the opposite of our `--resume`-into-the-same-workspace requirement. ([Bazel sandboxing](https://bazel.build/docs/sandboxing))

**Why over RunID-in-path:** ID-in-path creates an unbounded per-run dir set that needs its own GC. It breaks
the warm-cache/resume model that every long-lived worker depends on. RunID belongs in the sidecar/ledger (which
already threads it). Then a stable path stays cache-warm, and the ID stays the immutable provenance key.

### Decision 2 — Ledger → **one flock-serialized GLOBAL chain + RunID filter** (NOT sharded). High confidence.

Unanimous across tamper-evident-log prior art — a single serialized writer/sequencer over one chain:

- **Certificate Transparency / RFC 6962** — "a single, ever-growing, append-only Merkle Tree." ([RFC 6962](https://www.rfc-editor.org/rfc/rfc6962.html))
- **Google Trillian** — one log signer per tree through leader election; *no concurrent sequencing of a particular Merkle tree*. ([writeup](https://blog.system-transparency.org/posts/trillian-log-sequencing-demystified/))
- **Let's Encrypt Sunlight** — single-node writer + CAS fork-guard. ([intro](https://letsencrypt.org/2024/03/14/introducing-sunlight))
- **Git** — single commit chain behind a per-ref `.lock` compare-and-swap. ([git-update-ref](https://git-scm.com/docs/git-update-ref))
- **Write-ahead logs** — one monotonic LSN = total order. ([Fowler](https://martinfowler.com/articles/patterns-of-distributed-systems/write-ahead-log.html))

The prior art absorbs concurrency by **batching behind the one writer**, never by adding a second. The flock
on `ledger.lock` (CA.1) of evolve-loop *is* that single-writer serialization — keep it. Sharding (Kafka partitions, event
streams) is the documented move *only* when one writer cannot keep up. It sacrifices the cheap cross-shard
global order that an audit→ship binding needs. Per-run lookup (the only thing sharding buys) is a RunID filter on
the one chain. If whole-history `verify` ever gets expensive, layer an RFC-6962 Merkle tree over the *same*
chain for O(log N) inclusion + consistency proofs.

### Decision 3 — Bridge → **Go bridge SSOT mints the token; bash drivers consume it + 5-axis per-dispatch namespace** (both). High confidence.

- **Codex sandbox** generates an SBPL/Landlock profile **per invocation** with that child's writable roots injected. ([writeup](https://codex.danielvaughan.com/2026/04/08/codex-sandbox-platform-implementation/))
- **SEI CERT FIO21-C** — per-process private temp dirs, never shared `/tmp`. ([rule](https://cmu-sei.github.io/secure-coding-standards/sei-cert-c-coding-standard/recommendations/input-output-fio/fio21-c))
- **XDG base-dir spec** + boxxy/Nix/Bazel — per-job `XDG_CONFIG_HOME`/`HOME` to redirect XDG-ignorant tools (exactly the shared `~/.codex/config.toml` case). ([spec](https://specifications.freedesktop.org/basedir-spec/latest/))
- **tmux** — unique session names; `has-session` precheck is the documented fix for the `new-session -A` foot-gun that attaches to a stranger's live session. ([tmux wiki](https://github.com/tmux/tmux/wiki/Getting-Started))
- **Go `exec.Cmd.Env`** + `t.Setenv` parallel-refusal + [golang/go#63567](https://github.com/golang/go/issues/63567) — process env is a single MT-Unsafe global; build each child's env explicitly, never `os.Setenv` on a concurrent launch path.
- **git worktree** — `.git` object store is shared single-process state; naive parallel `worktree add`/commit corrupts it ([claude-code#34645](https://github.com/anthropics/claude-code/issues/34645)). Serialize git mutations behind one broker, and use `git worktree lock` on in-use trees against prune.

**Why both:** a bridge-only fix (no bash hardening) leaves the real holes (shared `~/.codex/config.toml`, shared
`/tmp`, `os.Setenv`, named-session `-A` reattach). Hardening of the bash drivers without the bridge SSOT duplicates
run-scope/sandbox logic across four CLIs (this violates single-source-with-projection). The bridge mints the RunID
token + sandbox profile. Drivers consume the token + their own per-dispatch TMPDIR/HOME.

---

## Part 3 — Cross-cutting isolation principles (apply program-wide)

1. **Run-ID is durable IDENTITY, not a path.** Store it in the ledger/sidecar. Map it to a stable-or-slot-indexed workspace. Never branch logic on it as a location. Never make it the dir name of a resumable workspace. (Temporal/Airflow/GitLab/Jenkins consensus.)
2. **Immutable shared inputs, per-worker mutable scratch.** Workers can share read-only inputs (Ray per-job `working_dir`). Any path that a unit MUTATES must be per-worker. Copy config/`.env` into each worktree, never symlink.
3. **One short-held integrator lock, everything else concurrent.** Serialize ONLY the git `stage→validate→ff-push` critical section (`.evolve/ship.lock`). Never serialize the whole think-heavy cycle (critical-section minimization).
4. **Test-the-merged-tree, then promote (Not-Rocket-Science).** Merge/rebase produces a NEW tree, so the prior green is stale. On a diverged base, **rebase-and-re-audit**. SHA-pin the attestation to the exact pushed tree, so that what-you-audited == what-you-shipped.
5. **Single serialized writer for the audit chain; batch (don't multi-write) for throughput.** Shard only when a single batched writer provably cannot keep up. Then use a root-of-roots anchor to restore whole-history verifiability.
6. **Per-child explicit env, never process-global `os.Setenv` on a concurrent launch path.** Build each `exec.Cmd.Env` as an explicit snapshot. This also makes orchestrator tests `t.Parallel`-safe.
7. **Five-axis per-dispatch namespace for every spawned child.** The five axes are:
   - private TMPDIR (`mktemp -d 0700`);
   - per-job `XDG`/`HOME` config home;
   - unique refuse-on-busy session name;
   - per-invocation sandbox profile;
   - own process group for sibling-safe tree-kill.
8. **Serialize git-store mutations behind one broker; lock live worktrees.** Gate every git mutation through the single integrator. Use `git worktree lock` on in-use trees, so that prune cannot delete uncommitted work. GC by slot-recycle + `git worktree prune`, not by sweeps of per-run dirs.
9. **Single-source-with-projection for isolation logic.** The bridge is the SSOT that mints the RunID token + sandbox profile. The four drivers consume/project it and do not re-implement run-scoping.

## Part 4 — Open risks (decide before/within the relevant slice)

1. **Stale-lock liveness** on the single global ledger/ship.lock. A serialized writer is a liveness SPOF (compare the too-short 1s packed-refs timeout of Git). The design needs an explicit lock timeout + crashed-holder recovery. The CA.1 flock appears blocking with no documented timeout — verify.
   - *This is the hard prerequisite for the deferred `mergequeue.lock` (S5b).*
   - *A lock that is held across the multi-minute re-audit of a recovering cycle, without timeout/crashed-holder recovery, turns a bounded abort into an unbounded-deadlock SPOF.*
   - *Full deferral rationale + trip-wire: [ADR-0049 → Implementation status → Deferred: mergequeue.lock](adr/0049-concurrent-multi-cycle-execution.md).*
2. **Stable-path resume against concurrency collision.** Two runs of the SAME cycle-N can overlap (fleet + a resumed cycle). Then a stable path needs a bounded concurrency-slot index (GitLab/Jenkins `@N`) + git-clean-on-entry. Decide slot-allocation + clean-on-entry now.
3. **Named-session resume policy under crash.** Proposal: replace `new-session -A` with a `has-session` precheck. In that case the precheck needs an **ownership token (RunID)** on the session. A stranger must not reclaim a stale-but-LIVE session from a crashed run. The session must not deadlock resume forever.
4. **XDG/HOME redirect first-run auth seeding.** A per-dispatch `HOME` can lack pre-trusted auth. First-run auth must be seeded into each per-job home, or dispatch fails.
5. **Orphan TMPDIR/worktree disk leak on crash.** Per-dispatch `mktemp -d` + per-run worktrees rely on best-effort exit cleanup, and they leak on SIGKILL. The design needs a sweeper keyed off RunID/pgid (+ `git worktree prune`).
6. **Merge-skew if the integrator skips re-audit on divergence.** Two individually-green cycles can produce a broken main (rename-vs-old-caller skew). The lock MUST enforce the expected-base-SHA check + re-audit-on-rebase.
7. **Audit-binding semantics if the ledger ever shards later.** The binding assumes one global order. A future throughput patch must first redesign around a root-of-roots anchor (hard prerequisite — flag it).

## Part 5 — Reuse, don't rebuild (from the existing substrate)

1. `adapters/flock/flock.go` — blocking cross-process primitive → the S5 ship-integrator lock + existing ledger/state critical sections.
2. `storage/updatestate.go` (CA.3 lossless flock RMW + StateRevision) — verbatim, the single legal `state.json` write path S2 routes ship through.
3. `core/alloc.go` (CA.4 number-allocation lease + max-merge) — already-serialized globally-unique number source; activates under the per-run lock.
4. `core/runid.go` `MintRunID` + the stamping-ledger decorator — identity backbone for sidecar naming, ledger filtering, session namespacing.
5. CB.5 `sessionrecord.RunScopeToken` (`driver_tmux_repl.go:913-922`) — the run-scope token bash drivers consume in S7.
6. `core/runworkspace.go` `RunStateFile` + `storage/statejson.go` `mirrorRunState` (CB.4 `run.json`) — per-run read path S3 extends to ship.
7. `swarm/mergetrain.go` `RunMergeTrain` (strictly-sequential topo-ordered writer fan-in, `merge --abort` on conflict) — the integration discipline the S5 integrator mirrors.
8. `swarm/partition.go` (pure disjoint-file-ownership + acyclic-DAG validator) — the advisor's cross-cycle backlog partitioner (E).
9. `swarm/reap_runsessions.go` `ReapRunSessions` + per-run `sessionrecord` registry — session teardown under M runs.
10. The `EVOLVE_SWARM_STAGE` off/advisory/enforce rollout-dial shape — the template for staging the S6 fleet supervisor default-off.
11. The `EVOLVE_FLEET` consumer guard (`driver_tmux_repl.go:150`) + the complete `runlease` package — armed-but-unproduced; S6 supplies the single missing producer.

> **Do NOT extend `internal/swarm` for cross-run work** — it is intra-cycle (N-workers-in-one-cycle) by
> design and correct as-is. The fleet supervisor is a NEW layer *above* it, not a swarm modification.
