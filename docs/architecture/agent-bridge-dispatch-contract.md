# Agent-Bridge Dispatch Contract

> Status: living doc for the agent-bridge hardening program (plan: `sleepy-wandering-salamander`).
> Covers the invariants that make every subagent/phase-agent dispatch go through the one bridge
> process, safely and at scale. B1–B6 are complete; invariants I1–I10 below.

## Request

The agent-bridge (`evolve subagent run` / `evolve subagent dispatch-parallel`) must be the **only**
path that dispatches a phase agent or subagent — the in-process Agent/Task tool is banned. Three
structural weaknesses surfaced while fixing the sandbox-confinement bug:

1. The bridge-only invariant was not enforced — a `LEGACY_AGENT_DISPATCH` escape hatch fell back to
   the in-process tool.
2. Recursion (fan-out workers re-invoking the binary) was implicit and unbounded, and its
   sandbox-coherence under nesting was unproven.
3. The output contract / verification / concurrency guarantees were under-tested.

## B1 — Bridge-only dispatch invariant

**Problem.** `LEGACY_AGENT_DISPATCH=1` made `evolve subagent run` print a *"fall back to in-process
Agent tool"* instruction plus a `LEGACY_DISPATCH` stdout token. That token was a **contract signal**
(not a Go branch) telling the LLM orchestrator to dispatch in-process — the exact path that must be
banned.

**Alternatives considered.**
- *Silently ignore the flag (always bridge).* Rejected — a silent change (Rule 3); a caller setting
  the flag would get surprising behavior with no signal.
- *Loud, default-off deprecation with a removal date.* Reasonable, but leaves the banned path
  reachable during the deprecation window.
- *Hard error (chosen).* Setting the flag fails loudly at the single `Run()` chokepoint. No dispatch
  can reach the in-process tool, and the failure is self-describing.

**Design.** `ErrInProcessDispatchBanned` sentinel + one `enforceBridgeOnly(legacyRequested)` predicate,
called early in `Run()` (before profile/CLI resolution) — the one chokepoint all dispatch (single,
fan-out, recursive) funnels through. `RunResult.LegacyDispatch` and the `cmd_subagent` fallback
block/token were deleted. A source-guard test pins that no in-process fallback signal (or
"honored-env" advertisement of the retired flag) can regress.

The agent allow-list was collapsed to an `agentRoles[]` SSOT; `agentRolePattern` derives from it and
the all-roles invariant test iterates it, so a new role is added in exactly one place.

## B2 — Recursion through the same bridge

**Problem.** A fan-out worker recurses by running `evolve subagent run <agent>-worker-<subtask>` —
re-entering the same binary's `Run()` (good: never the in-process tool). But (a) the command was
built inline and untested, (b) the recursion was unbounded, and (c) a recursive agent is
*nested-Claude inside nested-Claude*, so the Part A sandbox-confinement SSOT must stay coherent at
every depth.

**Design.**
- **Tested recursion command.** Extracted pure `buildWorkerRecursionCommand()` — composes
  `PROMPT_FILE_OVERRIDE=… CLAUDECODE_TYPE= EVOLVE_DISPATCH_DEPTH=<n+1> <bin> subagent run
  <role>-worker-<subtask> <cycle> <ws>`. A conformance test proves it re-enters the bridge dispatch
  path for every worker.
- **Depth cap.** `EVOLVE_DISPATCH_DEPTH` threads recursion depth across each bridge re-entry
  (`ReadDispatchDepth` parses it; absent ⇒ 0). `enforceDispatchDepth` rejects past `maxDispatchDepth`
  (3) at both the `Run()` and `DispatchParallel()` entry points (one predicate). A fan-out loop can't
  recurse unboundedly.
- **Sandbox coherence by construction.** The worker command clears `CLAUDECODE_TYPE` — a recursive
  child is, by definition, **not** the top-level host. If a stale `CLAUDECODE_TYPE=host` leaked into a
  child, `sandbox.DetectNested` would return false → `ShouldWrap` would attempt the inner OS sandbox →
  on macOS `sandbox_apply()` EPERMs and hangs the REPL (the Part A failure mode). Since `claude`
  self-sets `CLAUDECODE` in every process, clearing the host marker keeps `DetectNested=true` at any
  depth → no inner wrap.

**Why not also strip/inject `CLAUDECODE` itself?** Unnecessary — `claude` sets it in every claude
process, so the nested signal is already present at every depth. Only the *host* marker can wrongly
suppress nesting, so only it is cleared.

## B3 — Output-contract verification SSOT + tunable progress-aware deadline

Two single-source-of-truth fixes: one for the *verdict* a dispatched artifact earns, one for the
*deadline* it is judged within.

### B3a — Artifact verification SSOT

**Problem.** The "is this artifact valid?" ladder (stat → freshness < 5 min → readable/non-empty →
token-bearing → exec status ⇒ `PASS` / `FAIL` / `INTEGRITY_FAIL`) existed in **three** Go copies that
could drift: `classifyArtifact` (`run.go`, single dispatch, bare verdict), `(*Runner).classify`
(`subagent.go`, same ladder + `[]core.Diagnostic`), and the bash `verify_artifact()` they were ported
from. A fourth, weaker copy — the fan-out aggregator's per-worker gate — is unified separately in B5
(it must skip freshness, see below).

**Alternatives considered.**
- *Keep the seams-blob signature (`Verify(opts RunOptions, …)`).* Rejected — couples the verdict to
  the whole dispatch-options struct and to the filesystem, so the ladder can only be tested with temp
  files + `os.Chtimes` (no `t.Parallel`).
- *One pure verdict-only function, callers re-emit their own diagnostics.* Rejected — re-introduces the
  per-call-site duplication for the diagnostic wording (the exact drift we're removing).
- *Pure core + one I/O adapter (chosen).* A pure `Verify(VerifyInput) VerifyResult` holds the ladder
  and the diagnostics; one `VerifyArtifact(stat, read, now, …)` adapter does the filesystem gather and
  delegates. Both call sites collapse onto the adapter.

**Design (the projection pattern).**
- `Verify(in VerifyInput) VerifyResult` — pure: no field touches the filesystem or the wall clock
  (`MTime`/`Body`/`Now`/`MaxAge` are passed in). The verdict ladder and the diagnostic wording live
  here, once. Table-tested with literals → every case is `t.Parallel`.
- `VerifyArtifact(stat, read, now, path, token, exitCode, execErr)` — the single I/O-bearing entry
  point. Gathers the stat (and, only on stat success, the body — preserving the "missing artifact is
  judged before any read" short-circuit), then calls `Verify` against `ArtifactMaxAge`.
- `run.go` step 13 calls `VerifyArtifact(…).Verdict`; `(*Runner).classify` calls it and returns
  `(.Verdict, .Diagnostics)`. `classifyArtifact` is deleted. Verdict and diagnostic bytes are
  unchanged — the four legacy FS tests retarget onto `VerifyArtifact`, and the new contract table adds
  the branches they never covered (token mismatch, exit≠0 ⇒ `FAIL`, happy `PASS`, the leading
  bridge-error diagnostic that does **not** short-circuit integrity).

The `evolve subagent check-token` CLI probe (`checktoken.go`) is deliberately **not** folded in: it is
a lighter exists+token check for operators, not a dispatch verdict, and adding freshness would change
that contract.

### B3b — Tunable deadline: env-resolution SSOT

**Problem.** The artifact-wait deadline is already progress-aware — the `deterministicReviewer` extends
**unbounded** while the pane shows substantive new content (`Progressed`) and up to `maxExtends` while
it is busy-but-quiet (`Busy`), so a thorough agent is not killed mid-write (the cycle-311/312 and
254/255 fixes). What was broken is the **tunable** part: the `EVOLVE_ARTIFACT_TIMEOUT_S` override
silently did nothing. Root cause is a second SSOT split — the *subprocess* env is built by
`driverEnv = os.Environ() + deps.Env`, but the *in-process* int reads went through
`lookupEnv → deps.LookupEnv` **only**, never consulting `deps.Env`. An override carried on the launch
(in `deps.Env`, the same place all other request env lives) reached the inner CLI but was invisible to
the deadline. `envInt`'s doc-comment even *claimed* it read "the Deps env overlay" — it didn't.

**Alternatives considered.**
- *Fold the `LaunchArgs` `env` arg into `deps.Env`.* Broader blast radius — would export every
  request env var to the subprocess; not needed to fix the knob.
- *Make the env override outrank an explicit `cfg.ArtifactTimeoutS`.* Rejected — livesmoke and
  integration tests set `cfg.ArtifactTimeoutS` deliberately (1–120 s); a stray global `=600` must not
  make them hang.
- *Make `lookupEnv` consult `deps.Env` first (chosen).* The minimal SSOT fix: the in-process env view
  now matches the subprocess env view, the lying doc-comment becomes true, and the existing precedence
  (`cfg.ArtifactTimeoutS` > `EVOLVE_ARTIFACT_TIMEOUT_S` > 300 default) is preserved.

**Design.** `lookupEnv(deps, key)` resolves `deps.Env[key]` → `deps.LookupEnv` → `os.LookupEnv`. Every
`envInt` consumer (the deadline interval and `EVOLVE_ARTIFACT_MAX_EXTENDS`) now honors the launch
overlay. Tests pin: the overlay is consulted and wins over `LookupEnv`, falls through when absent, the
override reaches the recorded `StopEvent.IntervalS` end-to-end, and `cfg.ArtifactTimeoutS` still wins.

## B4 — Subagent conformance suite

**Problem.** The invariants above were each enforced by a bespoke test; nothing guaranteed they held
*uniformly for every role*, so a new agent could silently miss one.

**Design.** `subagent/conformance_test.go` is a single suite driven by a **fake-bridge seam** (the
`RunOptions.ExecAdapter`, standing in for `evolve subagent run` with no LLM cost) and **table-driven
over `agentRoles`** — adding a role to the registry auto-subjects it to every invariant. Each role is
run through: bridge-only rejection (I1), B3 contract verdicts incl. the integrity-guard negatives
(I6), isolated dispatch (unique token bound into its own artifact under its own project root),
sandbox-coherent recursion command (I3+I5), depth-cap rejection (I4), no cross-agent token leakage,
and concurrent dispatch under `-race`. The suite is characterization, not new behavior: a failing row
is a real conformance gap. All 14 roles pass.

## Invariants (enforced by tests)

| # | Invariant | Enforced by |
|---|-----------|-------------|
| I1 | No dispatch reaches the in-process Agent tool | `enforceBridgeOnly` @ `Run()`; source-guard + conformance |
| I2 | Agent allow-list is single-sourced | `agentRoles[]` → `agentRolePattern`; all-roles test |
| I3 | Fan-out workers recurse via `subagent run` | `buildWorkerRecursionCommand` + conformance |
| I4 | Recursion depth is bounded | `EVOLVE_DISPATCH_DEPTH` + `enforceDispatchDepth` (cap 3); conformance |
| I5 | Recursive children stay nested (no inner wrap) | `CLAUDECODE_TYPE` cleared per worker; conformance |
| I6 | Artifact verdict is single-sourced | `Verify`/`VerifyArtifact` (`contract.go`); contract table + conformance |
| I7 | In-process env view == subprocess env view | `lookupEnv` consults `deps.Env`; overlay + e2e tests |
| I8 | Every registry role satisfies I1/I3–I6 uniformly | `conformance_test.go` (fake-bridge, table over `agentRoles`) |
| I9 | Fan-out honors its concurrency cap + isolates workers | `fanoutdispatch` race tests (observed bound, high-N partial-failure) |
| I10 | Allow-list ↔ profiles stay single-sourced; reaping releases sessions | `conformance_registry_test.go` (role↔profile drift-guard); `internal/swarm` reap tests |

## B5 — Fan-out concurrency stability

**Problem.** `fanoutdispatch.Run` fans N worker commands across a semaphore-bounded goroutine pool
(`sema := make(chan struct{}, Concurrency)`), recording results in a mutex-guarded map and per-worker
`.meta`/`.out` files. The existing `TestRun_BoundedConcurrency` only asserted all workers *complete* —
it never observed the cap was honored (it would pass with a broken semaphore), and isolation /
partial-failure were only exercised at N=4.

**Design (tests; no production change — the path was already `-race`-clean).**
- `TestRun_ConcurrencyBoundIsObserved` — each worker drops a marker file for the lifetime of its
  command and snapshots how many markers exist. A marker's lifetime ⊆ its held semaphore slot, so no
  snapshot can exceed `Concurrency`; with N≫`Concurrency` + an overlap sleep, at least one snapshot
  must observe real parallelism (guards against a vacuous serial pass). Asserts `2 ≤ max ≤ Concurrency`.
- `TestRun_HighFanoutIsolationPartialFailure` — 12 workers at concurrency 4, half failing: every
  worker's recorded exit code matches its intent, each `.out` carries only its own line (no
  cross-clobbering of the results map / files), and `Run` returns `ExitWorkerFail`. Stable under
  `-race -count=3`.

## H1 — Fan-out per-worker provenance verification

**Problem.** A capstone `evolve loop` cycle implemented parent-side per-worker artifact verification, but
ran the B3 `Verify` with an **empty token** — `bytes.Contains(body, []byte("")) == true` for any
non-empty body — so a compromised/buggy worker runner could write arbitrary content to the expected
artifact path and pass (presence, not provenance). The pipeline's adversarial audit caught this as a
HIGH finding; this change closes it (the deferred token-threading).

**Design (provenance by a parent-dictated token).** The fan-out parent already derives a per-worker
token `parentToken+"-"+subtask`. The fix threads it end-to-end: `buildWorkerRecursionCommand` exports
`EVOLVE_FANOUT_WORKER_TOKEN`; `cmd_subagent` wires it to `RunRequest.ChallengeTokenOverride`; `run.go`
uses it as the challenge token instead of minting a fresh one (empty ⇒ mint, the normal path), so the
worker **writes the parent-known token** into its artifact; the parent's `defaultVerifyWorkerArtifact`
sets `VerifyInput.Token` to the expected value and runs the SSOT `Verify` (freshness skipped via
`MaxAge=MaxInt64` — the worker's recursive child already verified freshness at write time, and the
parent re-checks only after all workers finish). A wrong-token or tokenless worker artifact now fails
verification, which skips aggregation. This also demonstrates the self-correction loop: the audit found
the defect, and it was completed to PASS rather than shipped with a deferral.

| I11 | Fan-out worker artifacts are provenance-verified | parent-dictated `EVOLVE_FANOUT_WORKER_TOKEN`; `defaultVerifyWorkerArtifact` token check |

## B6 — Allow-list ↔ profile single-sourcing (+ reaping)

**Problem.** B1 made the allow-list one source (`agentRoles` → `agentRolePattern`), but the allow-list
and the on-disk profiles could still drift: a role added to `agentRoles` without a `<role>.json`
profile (or a profile removed out from under a role) would only fail at dispatch time (`run.go:179`
loads `ProfilesDir/<role>.json`).

**Design (test; no production change).** `conformance_registry_test.go`:
- `TestAgentRoles_EveryRoleHasProfile` — every canonical role has a tracked `<role>.json` profile, so
  the allow-list and profiles cannot drift (`runtime.Caller`-rooted lookup → stable regardless of cwd).
- `TestAgentRoles_SSOTIntegrity` — no duplicate roles; `agentRolePattern` matches every canonical role
  and rejects non-members, case variants, and worker-name-shaped strings (those take the separate
  `parseAgentName` path).

The other B6 scale knob — **session reaping** — is already covered by `internal/swarm`
(`TestReap_KillsAllLiveAndMarksReaped`, `TestReapRunSessions_KillsOwnRegistryOnly`,
`TestReapRunSessions_RefusesUnsafeNames`, registry round-trip/idempotency), so it is referenced rather
than re-tested (single-source).

## Status

B1–B6 of the agent-bridge hardening program are complete. The dispatch contract is enforced by
invariants I1–I10, each backed by tests.
