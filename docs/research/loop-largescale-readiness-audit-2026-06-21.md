# Large-Scale Campaign Readiness Audit — evolve-loop (2026-06-21)

> **Provenance:** ultracode adversarial readiness audit (13 failure-mode dimensions ×
> finder + adversarial verifier + synthesis). 68 raw findings → 64 confirmed real → 7
> distinct must-fix blockers → **5 root-cause fixes**. All claims verified against source.
> Goal: make the evolve loop powerful enough to autonomously drive a difficult large-scale
> project. The flag-reduction campaign is the proving run. Design-of-record for the hardening.

## 1. Verdict

**NOT ready today.** The per-cycle integrity core is **sound under concurrency** (ledger
hash-chain, state.json RMW, ship.lock integrator, integrity floor — all flock-correct,
verified by running their tests) and must NOT be touched. Every blocker lives in the
**campaign orchestration layer** (`cmd_campaign.go:runCampaignRun`) — a bare in-memory wave
loop with no checkpoint, no signal handling, no timeout, no failure classification, no
per-cycle log attribution. A 50-cycle multi-hour run is killed by the first quota wall,
poison task, hung child, or Ctrl-C, and re-burns all completed waves on restart.

## 2. Confirmed must-fix issues (REFUTED dropped)

| # | Issue | Severity | Root cause (file:line) |
|---|---|---|---|
| 1 | No campaign checkpoint/resume; crash re-runs from wave 0 | HIGH | `runCampaignRun` cmd_campaign.go:150-167 pure in-memory loop |
| 2 | No SIGINT/SIGTERM handler; no per-cycle/wave timeout | **BLOCKER** | `supervisor.Run(context.Background(), …)` cmd_campaign.go:155,161 |
| 3 | Quota-walled cycle opaque rc=1, retried w/ zero backoff, aborts campaign | **BLOCKER** | cycle run emits only 0/1/2/10 cmd_cycle.go:160-197; immediate retry cmd_campaign.go:161 |
| 4 | Single poison cycle aborts whole campaign; no skip/quarantine | **BLOCKER** | `return 1` cmd_campaign.go:162-164; Todo has no optional/max_retries |
| 5 | Multi-failure-in-wave: only first failure retried before abort | HIGH | sequential short-circuit cmd_campaign.go:156-166 |
| 6 | No CLI-health canary between waves/before retries | HIGH | `runCLIHealthCanary` only in runLoop cmd_loop.go:284-289 |
| 7 | Concurrent child output interleaved on shared stderr, no prefix | HIGH | shared `cmd.Stdout/Stderr` cmd_fleet.go:152-153 |
| 8 | `clihealth.Store.Bench` unlocked RMW loses strikes under concurrent walls | HIGH | Bench/Clear no flock clihealth.go:150,157 |
| 9 | failure-learning `WriteState` bypasses state.json flock; concurrent clobber | HIGH | bare `WriteState` failure_learning.go:432 → unlocked writeJSONAtomic statejson.go:43 |
| 10 | No campaign status command | HIGH | registry.go:62 maps campaign→study/replan/run only |
| 11 | runCampaignReplan manual-only; no closed-loop adaptive replan | HIGH (velocity) | 3 independent CLI entry points; run never replans |
| 12 | Post-rebase recovery routes to AUDIT not BUILD (ships unbuilt peer code) | MEDIUM | `fleet-rebase-reaudit`→"audit" recovery.go:108-111 |
| 13/15/17 | No post-commit diff-vs-declared-scope guard (underdeclaration) | MEDIUM/HIGH | partition clusters on declared Files only; ship `git add -A` |
| 14/22 | OutputContract dropped Todo→CycleSpec; never seen/verified | MEDIUM | PlanWaves builds Scope+Env only waves.go:47-50 |
| 16 | Preserved-on-FAIL worktrees accumulate; GC blind to `.evolve/worktrees/` | HIGH | gc.Discover walks runs/ only (405MB/9 dirs measured) |
| 18 | MaxCycles=50 hard const, no policy knob/chaining | HIGH | campaign.go:26; study prompt never told the cap |
| 19/20/21/23 | width-11 reject; non-monotonic lastCycleNumber; swarm worktree leak; serialized-todo opacity | LOW–MED | — |

**Refuted/dropped:** stale post-rebase tree-SHA (binding re-emitted on rebased tree); child-tty BOOT-SMOKE (nil Stdin ⇒ /dev/null, tmux `-d`); shipDirect lock; concurrent `git worktree add` race; goroutine fan-out / Result.Index ordering (all sound).

## 3. The 5 root-cause fixes (must-fix-before-launch)

### Fix A — checkpoint + `--resume` + `status` (issues 1,5,10) · Memento/Repository · M · **1st**
New `internal/campaign/progress.go`: `CampaignProgress{PlanSHA, CompletedWaves []int,
CompletedCycleIDs, FailedCycleIDs []string}` → `.evolve/campaign-progress-<goalHash>.json`
via atomic temp+rename (`writeJSONAtomic`) under a `<path>.lock` flock. On start: load,
verify `PlanSHA`, slice off completed waves. `--resume` reads it; `--ignore-progress` escape.
New `runCampaignStatus` + registry.go dispatch. **Red test:** `TestCampaignResume_SkipsCompletedWaves`
— crash after wave 2, re-run `--resume`, assert only wave-3 scopes launched.

### Fix B — signal-aware cancellable ctx + per-cycle deadline (issue 2) · Context+Timeout Decorator · S · **2nd**
Replace `context.Background()` with `signal.NotifyContext(…, SIGINT, SIGTERM)` (pattern at
cmd_bridge_watch.go:58). `context.WithTimeout(ctx, policy.Campaign.CycleTimeoutS)` threaded
through `Supervisor.Run`→`launchOne`. In `execCycleLaunch` set `cmd.Cancel` (SIGTERM) +
`cmd.WaitDelay` (grace→SIGKILL). On cancel, flush Fix-A progress. **Red test:**
`TestCampaignRun_HungChildIsReapedByDeadline` — LaunchFn blocks on ctx.Done; assert return
within timeout+ε with `DeadlineExceeded`, not hang.

### Fix C — failure-classification chain (issues 3,4,6) · Chain-of-Responsibility + Strategy · L · **3rd**
New `internal/campaign/failpolicy.go`: `FailureDisposition.Decide(spec,result,attempt) →
Retry|RetryAfter(d)|Skip|Abort`. **Sub-task: structured quota signal** — cycle run can't emit
one today (0/1/2/10 only); decision below. transient→`executeRetryBackoff` (retry_backoff.go:33);
quota→consult `clihealth.Store.Active()`/`BenchedUntil`, sleep to cooldown (cap 4h) before
retry; poison (exit 2 / N-exhausted)→ if `Optional` or under max-failures budget, log to
`campaign-quarantine.json` and **continue**, else abort. Batch all failed specs → one
`supervisor.Run` (fixes #5, recovers parallelism). Add `runCLIHealthCanary` between waves +
before retries. Add `Optional bool`/`MaxRetries int` to `fleet.Todo`; thread `WorkflowConfig`
(has `MaxConsecutiveFails`). **Red tests:** quota-cooldown-before-retry; optional-poison-skip-
and-continue; all-wave-failures-retried-before-abort.

### Fix D — per-cycle log attribution (issue 7) · Decorator · S · indep
`execCycleLaunch`: `prefixWriter{w, prefix=strings.Join(spec.Scope,"+"), mu}` line-buffers +
serializes through one shared mutex; wrap shared stdout/stderr per CycleSpec. **Red test:**
`TestExecCycleLaunch_PrefixesAndSerializesOutput` — concurrent fakes, every line prefixed, none torn.

### Fix E — lock clihealth + failure-learning shared writes (issues 8,9) · flock + merge-RMW · M · indep
`clihealth.go:Bench/Clear` → wrap in `flock.WithPathLock(path+".lock")` (mirror
cliadmit.WithPathLock). `failure_learning.go:432` → route through `storage.UpdateState` with
id-keyed append-merge of `FailedAt`/`CarryoverTodos` (reuse `carryoverTodoExists`); fix
`persistCycleEndState` `*s=state` to merge-append too. Required because `EVOLVE_FLEET=1` drops
the global lock that previously made these safe. **Red tests:** concurrent-strikes-not-lost;
concurrent-fails-preserve-all-records.

**Dependency graph:** A → (B, C); C needs the quota-signal sub-task; D, E independent.
**Critical path: A → B → C.** Sequence: A, B, C, E, D.

## 4. Fix-on-the-way (loop limps with operator attention)
12 post-rebase→build (recovery.go:111, S); 13/15/17 scope ScopeVerifier + declared-files env
(M); 14/22 OutputContract into CycleSpec.Env + post-wave checker (S); 16 worktree GC stanza +
campaign post-run hook (M); 18 `campaign.max_cycles` policy knob (S); 11 closed-loop replan
strategy (L); 19/20/21/23 (S each). Several are **good campaign cycle-tasks** (file-disjoint):
D (cmd_fleet.go), E-clihealth (clihealth.go), 14 (waves.go), 18 (policy.go+campaign.go), 20
(postship.go), 12 (recovery.go) — eligible to be run by the loop they harden once Phase 1 lands.

## 5. Sound — do not gold-plate
Ledger hash-chain (l.mu + LOCK_EX, tmp+rename); state.json RMW (shared flock, lease max-merge);
ship gate (blocking flock collider-scan→ff-merge→push→verify, tree-SHA binding pre+post);
integrity floor under fleet (`ClampPlanToFloorWith`; `EVOLVE_FLEET=1` gates ONLY the global
lock, never build∧audit∧tdd); Supervisor concurrency (stable Index, semaphore, var-capture).

## 6. Decisions taken (recommended defaults; operator may override)
1. **Quota signal (Fix C) → option (a): campaign launches `evolve loop --max-cycles 1` per
   cycle** so it inherits the loop's existing rc=5 / QUOTA-PAUSE decorator (cmd_loop.go:372-379)
   rather than inventing a parallel failure-artifact protocol — reuses the SSOT, no duplication.
   (Revisit if the per-cycle `evolve cycle run` path must stay direct for perf.)
2. **Poison policy → skip-and-continue for `Optional:true` cycles, else abort after
   `MaxRetries` (default per-cycle 1 transient + quota-cooldown retries).** Quarantine recorded
   in `campaign-quarantine.json` (Fix A repository).
3. **A/B/C/E-failure-learning hand-built on this branch** (trust-boundary substrate); D +
   E-clihealth may later be campaign cycle-tasks.

## 7. Remaining work (status as of 2026-06-21, after Fixes A–E + derived-artifact #182/#185 merged)

Fixes A–E and the two derived-artifact fixes (#182 rebase-regen, #185 post-build projection regen)
are **MERGED on `84de1011`**. The §4 "fix-on-the-way" items were deliberately deferred and several
are still open; one new blocker surfaced during the live run. Tracked here so nothing stays invisible.

| # | Item | Source | Status | Blocks 47→0? |
|---|------|--------|--------|--------------|
| **F** | **Cross-session campaign ownership lease** (reap-loop fix; found in the live run, in no prior doc) | live run; **ADR-0059** | **DONE on `feat/campaign-ownership-lease`** (`flock.TryLock` + `internal/campaign` ownership lease + `runCampaignRun` refuse-or-attach; TDD, 159 pkgs green) | Indirectly — it kept killing the run |
| 16 | Worktree GC for `.evolve/worktrees/` (405MB/9-dir bloat on long runs) | §4 issue 16 | OPEN | No (disk grows per FAIL cycle) |
| 18 | `campaign.max_cycles` policy knob | §4 issue 18 | OPEN — still `const 50` (`campaign.go:26`) | No (plan has 11 cycles) |
| 13/15/17 | Post-commit diff-vs-declared-scope guard (underdeclaration) | §4 issue 13/15/17 | OPEN | No |
| 11 | Closed-loop adaptive replan in `campaign run` | §4 issue 11 | OPEN | No (velocity only) |
| 12 | Post-rebase recovery routes BUILD not AUDIT | §4 issue 12 | LIKELY SUPERSEDED by #182/#185 — verify | No |
| §6.2 | Env-deprecation `evolve doctor` WARN (7 operator dials) + CHANGELOG migration table | flag design §6.2 | OPEN — not in the campaign plan JSON | Terminal/UX of 47→0 |
| DA-a/b | Derived-artifact follow-ups: `maxRecoveryDepth=2` starvation; SSOT adjacent-row deletion → debugger | derived-artifact doc | ACCEPTED (fail-closed) | No |

**Decision (2026-06-21):** implement **F** now (the proven blocker); CAPTURE 16/18/13/11/12/§6.2/DA as
their own later scoped efforts (several are good campaign cycle-tasks once F lands). 16, 18, 13, 12 are
file-disjoint and eligible to be run by the loop they harden.
