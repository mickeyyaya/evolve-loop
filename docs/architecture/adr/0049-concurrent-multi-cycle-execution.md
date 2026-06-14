# ADR-0049: Concurrent Multi-Cycle Execution — Bottom-Up Isolation (agent → phase → cycle)

- **Status:** Accepted — PR-1/PR-2/PR-3 implemented on branch `concurrency-hardening` (PR #93); each slice a no-op for the live sequential loop until S6's floor-removal dial (shipped default-OFF) is flipped. See **Implementation status** below for the per-gap dispositions (the adversarial re-derivation found ~half of the PR-3 prescriptions wrong or unnecessary when checked against code).
- **Date:** 2026-06-14
- **Driver:** an operator request to run multiple `evolve` cycles concurrently — "each cycle not interfering with the others, with the advisor partitioning the backlog across independent cycles" — with the steer: *"make each phase and agent independent and executed in concurrency first; this is fundamental for cycle concurrency."*
- **Evidence:** [concurrency-isolation-research-2026-06-14.md](../concurrency-isolation-research-2026-06-14.md) — a 28-agent codebase isolation audit (gaps G1–G14) + a web research sweep of prior art that **settles the three design decisions below**. Read it for full gap detail, citations, principles, and open risks.
- **Relates to:** ADR-0032 (swarm harness, `EVOLVE_SWARM_STAGE=shadow`), CA.*/CB.*/CC.* run-identity/OCC work, the ship repair ladder (v16.8.0 / ADR-0039 §8), ADR-0048 (resilient ship), `swarm/mergetrain.go` (writer fan-in precedent).

## Problem

A 2nd `evolve` cycle launched while a 1st runs does not run concurrently — it **refuses** at the whole-cycle
project flock (`.evolve/.lock`, `LOCK_EX|LOCK_NB`, held for the entire cycle; `orchestrator.go:1673`). That
coarse mutex is the *only* thing serializing shared state, and it is also the only thing currently *preventing*
the latent G1–G14 races. The codebase already contains per-resource isolation hardening (CA.3 `UpdateState`
RMW, CA.4 allocation lease, RunID-stamped ledger, CB.4 `run.json` mirror, CB.5 session-name `RunScopeToken`) —
built *for* concurrency but **dormant** because the coarse lock still serializes everything. So this is not a
build-from-scratch; it is **activating dormant machinery bottom-up, safety-net before floor-removal.**

## Decisions (settled by prior-art research — see evidence doc)

| # | Decision | Resolution | Basis (high confidence) |
|---|----------|-----------|-------------------------|
| D1 | **Workspace identity** | **Stable `cycle-N` path + RunID in the sidecar/ledger**, with a bounded **concurrency-slot index** + `git clean` on entry for the rare same-N overlap. RunID is **not** a directory-name segment. | GitLab Runner slots, Jenkins `@N`, GH Actions/Buildkite stable paths, Temporal/Airflow run_id-as-identity. ID-in-path is only for ephemeral never-resumed sandboxes (Bazel/Nix) — opposite of our resumable workspaces. |
| D2 | **Audit ledger** | **One flock-serialized GLOBAL hash chain + a RunID filter** on lookups. **No** per-run sharding. | CT/RFC 6962, Trillian single-sequencer, Git ref-lock, WAL. Shard only when one writer can't keep up — sacrifices the global order an audit→ship binding needs. |
| D3 | **Bridge** | **Both:** the Go bridge is the SSOT that mints the RunID token + per-invocation sandbox profile; the four bash drivers **consume** it + get a 5-axis per-dispatch namespace (private TMPDIR, per-job `HOME`, refuse-on-busy sessions, per-invocation sandbox, own process group). | Codex per-invocation sandbox, CERT FIO21-C, XDG spec, tmux `has-session` precheck, Go `exec.Cmd.Env`, git-worktree single-`.git` serialization. Single-source-with-projection. |

## Plan — 8 slices, bottom-up (agent → phase → cycle), safety-net before floor-removal

Each slice closes named gaps and is a **no-op for the live sequential loop** until S6 flips an off-by-default
dial. Order: agent/phase isolation FIRST (the operator's directive — the substrate cycle concurrency stands on).

| Slice | Level | Change | Closes | No-op until |
|-------|-------|--------|--------|-------------|
| **S0 — per-dispatch file & resource isolation (KEYSTONE)** | agent-bridge / phase | Keep stable `cycle-N` workspace (already unique per fresh run via the monotonic allocator). Give each *dispatch* a private TMPDIR (`mktemp -d 0700`) + per-dispatch/per-worker file names for `resolved-prompt.txt`, `challenge-token.txt`; build `sandbox-<phase>.sb` **per invocation** with that child's WritePaths. Add a bounded concurrency-slot index + `git clean`-on-entry for same-N overlap. RunID stays in `run.json`/ledger. | G6, G7 (+ aligns G9/G13) | S6 |
| **S1 — per-worker env injection** | agent | Thread `EVOLVE_MAX_BUDGET_USD` + `EVOLVE_FANOUT_CACHE_PREFIX_FILE` into each worker command's env (mirroring `EVOLVE_FANOUT_WORKER_TOKEN`); delete the `os.Setenv` pair from `fanoutdispatch.Run`. | G8 | the multi-dispatch-in-one-process model (S6) |
| **S2 — ship state via `UpdateState`** | cycle | Replace ship's raw `writeStateMap` `state.json` mutations with `storage.UpdateState` (flock + lossless merge + StateRevision). Make `writeStateMap` private to non-`state.json` paths. Regression test: ship pin + concurrent `UpdateState` lease bump don't clobber. | G2 | S6 |
| **S3 — ship reads `run.json`** | cycle | Thread WorkspacePath/RunID into `ship.Options`; read `active_worktree`/`cycle_id`/`cycle_size_estimate`/inbox from `<workspace>/run.json` (CB.4 mirror), fallback to global for standalone `evolve ship`. | G3 | S6 |
| **S4 — run-scoped audit-binding lookup** | cycle | Keep the ONE global flock chain; add a **RunID filter** to `findLatestAudit` + `Verify` so "latest auditor entry" means latest FOR THIS RUN. (D2: no sharding.) | G5, G4-ledger | S6 |
| **S5 — serialized ship-integrator** | cycle | Short-held `.evolve/ship.lock` around `stage→commit→ff-merge→push→tree-verify` + collider-heal + `go/evolve` discard (reuse `flock` + the mergetrain discipline). **Reclassify `GIT_FF_MERGE_DIVERGED` → rebase-and-re-audit** (test-the-merged-tree; SHA-pin attestation to the pushed tree). Add explicit lock **timeout + crashed-holder recovery**. Held alongside the coarse lock first → no-op. | G1, G12 | S6 |
| **S6 — fleet supervisor (FLOOR REMOVAL)** | cycle | New producer `cmd_fleet.go` + `internal/fleet/`: scope the project lock **per-run** so M cycles run concurrently (integration still serialized by S5); arm `EVOLVE_FLEET=1`; write/refresh `runlease`; mint per-run identity + slot. Read env from `req.Env` snapshot not live `os.Getenv` (G14). **Staged behind an off/advisory/enforce dial, default OFF** (the `EVOLVE_SWARM_STAGE` template). | G10, G11, G14 | — (this IS the enabling slice; no-op only while the dial stays off, the shipped default) |
| **S7 — session-namespace hardening (5-axis)** | agent-bridge | Refuse-on-busy named sessions with a RunID **ownership token** (never blind `-A` reattach); add `RunScopeToken` to the bash ephemeral + named templates; per-dispatch TMPDIR + per-job `XDG_CONFIG_HOME`/`HOME` (move shared `~/.codex/config.toml`); per-child process group. (D3.) | G9, G13, G6/G7 residuals | S6 |

**Advisor backlog-partition (E, separate track):** a pre-fleet phase that splits the backlog into K
**independent** cycle assignments, reusing `swarm/partition.go`'s disjoint-file-ownership + acyclic-DAG
validator lifted from intra-phase to cross-cycle. This is the "advisor partitions todos across independent
cycles" capability; it depends on S6 and is the consumer of the isolation S0–S5 provide.

## Alternatives considered

1. **Remove the coarse lock first, patch races as they appear.** Rejected: replaces "refuses" with "corrupts main" (G1 is irreversible data loss). Safety-net (S0–S5) precedes floor-removal (S6).
2. **RunID-in-path workspace (the audit's first instinct).** Rejected by prior art (D1): unbounded GC + breaks resume/warm-cache; ID-in-path is only for ephemeral never-resumed sandboxes.
3. **Per-run sharded ledgers.** Rejected by prior art (D2): sacrifices the global order the audit→ship binding needs; per-run lookup is a filter, not a shard.
4. **Extend `internal/swarm` for cross-run concurrency.** Rejected: swarm is intra-cycle by design; fleet is a NEW layer above it.

## Consequences

- **Positive:** the keystone (S0) and S1–S5 land with **zero live-behavior change** (the coarse lock still serializes everything); each closes a named, ranked, prior-art-validated gap; the program has an executable, evidence-backed slice list.
- **Negative / risk:** the full program (through S6/E) is multi-cycle work; partial completion leaves the loop sequential (the safe default). S6 is the highest-risk slice (removes the floor) — it must not land before S0–S5 and ships behind a default-off dial.
- **Floor invariant preserved throughout:** until S6's dial is flipped, the whole-cycle lock still serializes everything — every prior slice is a no-op for the running loop and cannot regress it. The 7 open risks (evidence doc Part 4) are each tied to the slice that must resolve them.

## Implementation status — PR-1 / PR-2 / PR-3 (branch `concurrency-hardening`, PR #93)

The 8-slice design was executed as three PRs. An **adversarial re-derivation of the gaps from code** (an 11-agent design dive + an 8-agent gap-hunt that corrected it, then a per-slice *verify-the-prescription-against-code* pass) renumbered the gaps and — crucially — found roughly **half of the PR-3 prescriptions wrong or unnecessary** when checked against the actual code. This table records ground truth, not the prescription (the same discipline that caught the cross-bucket partition error in PR-2).

**PR-1 — state-file isolation + lease floor:** G7 `b928e1b0` (cycle-state.json RMW serialized via the shared `flock.WithPathLock` sidecar) · G16 `ed5fef82` (per-run runlease producer + heartbeat into RunCycle) · G11 `dd8d71ad` (Supervisor.Validate fail-loud-early).

**PR-2 — advisor backlog partition (makes concurrency USEFUL):** partition fix `a3fd9b60` (CROSS-bucket file-disjointness — the committed first-fit partitioner spread same-file todos *across* buckets, the opposite of the required invariant) · `bc5d640c`/`f8140358`/`d4e254c2` (`--plan` + `EVOLVE_FLEET_SCOPE` thread + triage select-only directive + prompt-injection sanitize) · G13a `1433cfe1` (fleet rebase CONFLICT → `CodeGitFleetRebaseConflict` → debugger) · N9 `3a745ecb` (read guard/backfill flags from the per-cycle env snapshot, not live `os.Getenv`).

**PR-3 — per-resource hardening (N-series), each verified against code:**

| Item | Verdict | Commit | Why (code-verified) |
|------|---------|--------|---------------------|
| N15 tmux session-name collision | **IMPLEMENTED** | `0197b13b` | per-process `atomic.Uint64` nonce before the timestamp; covers same-second concurrent + same-phase-retry mints; `truncate64` degrades recency, never uniqueness. |
| N14 fatal-signature torn temp | **IMPLEMENTED** | `a634d9f9` | `PromoteSignature` routed through the `atomicwrite` SSOT (unique `CreateTemp`); two cycles classifying the same novel pane shared `path+".tmp"` → torn YAML / loser's rename ENOENT. Lessons half a proven no-op (already SSOT + cycle-N filenames) → dropped. |
| N10 codex pretrust lost-update | **IMPLEMENTED** | `427919d6` | host-global `~/.codex/config.toml` read-merge-write-rename serialized under `flock.WithPathLock`; last-writer-wins dropped a cycle's trust entry → cycle-122 "Press enter to confirm" modal hang. |
| N17(b) checkpoint TOCTOU | **IMPLEMENTED** | `1a3a15a3` | escalation-guard read + phase-complete write folded under one cycle-state flock (calling the lock-free `applyWithHooks`; flock is non-reentrant); a peer escalation in the TOCTOU window was clobbered to phase-complete. |
| N12 per-cycle GOCACHE | **REVERTED** | `88c91e3b`→`cc019871` | verified silent NO-OP on the default tmux path — `req.Env` never reaches the tmux pane's `go` toolchain (the `Realization` has no Env field; only headless drivers apply `Deps.Env` via `driverEnv`) — AND unnecessary (the Go build cache is concurrency-safe by design). Shipping it manufactured false isolation (its test asserted the wrong altitude). |
| N11 codex version.json lock | **DROPPED** | — | no demonstrable bug: our `dismissCodexUpdateNag` writers write identical keys (no lost-update) and the only lock-free reader (`looppreflight/freeze.go` codex branch) `os.Stat`s existence, never parses content. The named failure (lost `dismissed_version`) is an our-vs-codex race a lock on our side can't fix. |
| N16 bash adapter run-token | **DROPPED** | — | references nonexistent artifacts (`EVOLVE_RUN_SCOPE_TOKEN`, `codex-tmux.sh`, `agy-tmux.sh`); production uses the Go bridge's run-scoped `resolveSession`, and the lone bash prototype already disambiguates via `pid$$` (one OS process per cycle). |
| N17(a) recover-orphans read-lock | **DROPPED** | — | every `processing/` mutation is an atomic `os.Rename` (no torn-read window); recover-orphans has no automatic concurrent caller (manual CLI only). The real latent issue there is a separate multi-active-cycle SCOPE bug a content read-lock would not fix. |
| **mergequeue.lock (S5b)** | **DEFERRED** | — | see note below. |

**Deferred — mergequeue.lock (S5b).** `ship.lock` (`phases/ship/gitops.go`) ALREADY is the bors-style integrator lock serializing the whole shared-main critical section (collider scan → ff-merge → push → audit-bound tree-SHA verify) per ship attempt; main integrity is hard-guaranteed by that binding, so the residual is throughput/liveness, never corruption. A `mergequeue.lock` would additionally hold a lock across the recovering cycle's **multi-minute re-audit** so peers can't move main underneath it — which violates evidence-doc principle 3 (*one short-held integrator lock*) and reintroduces open-risk 1 (a serialized writer held across think-heavy work is a liveness SPOF). PR-2's cross-bucket disjoint partition already makes post-divergence rebase the clean-replay case (never CONFLICT) and keeps re-audit valid, so the only residual is exhausting `maxRecoveryDepth=2` — which needs 3+ peers landing inside one cycle's seconds-wide ship windows at `--count 6`, and whose failure mode is a clean bounded ABORT (work preserved in the worktree, ledger-recorded), never corruption. **Trip-wire to revisit:** repeated `CodeGitFleetRebaseNeeded` → depth-exhaustion aborts under `evolve fleet --count >=6 --plan`. **Prerequisite before any implementation:** an explicit lock timeout + crashed-holder recovery (evidence-doc open-risk 1) — else a bounded-abort liveness issue becomes an unbounded-deadlock SPOF; lock-order `mergequeue.lock < ship.lock`.
